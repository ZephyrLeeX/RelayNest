package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/push"
)

func syncCreateRequest(f *botHTTPFixture, bot bots.Created, admin bool, key string) (int, string, http.Header) {
	if admin {
		return clientMessageRequest(f, bot.Bot.ID, key, clientMessageBody)
	}
	return f.request("POST", "/api/v1/bot/messages", clientMessageBody, "android", bot.APIToken.Token, map[string]string{"Idempotency-Key": key})
}
func syncCreateJobs(t *testing.T, f *botHTTPFixture) []push.Job {
	t.Helper()
	out := []push.Job{}
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT id FROM push_jobs WHERE state<>'pending' ORDER BY id")
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			job, err := push.ReadJobTx(context.Background(), tx, id)
			if err != nil {
				return err
			}
			out = append(out, job)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
func syncCreateRegister(t *testing.T, f *botHTTPFixture, token string, version, expected int64) {
	t.Helper()
	status, raw, h := f.admin("PUT", assembledPushPath, assembledRegistration(token, version, expected))
	assertAssembledPush(t, status, 200, raw, h, "")
}
func syncCreateReceipt(t *testing.T, status int, raw string, h http.Header, admin bool) historyMessage {
	t.Helper()
	assertBotResponse(t, status, 201, raw, h)
	if strings.Contains(raw, "sync:") || strings.Contains(raw, "registration") || strings.Contains(raw, "synthetic-sync-secret") {
		t.Fatal("internal snapshot leaked", raw)
	}
	if !admin {
		return assertHistoryMessage(t, raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["delivery_id"] == nil {
		t.Fatal("administrator contract changed", raw)
	}
	return assertHistoryMessage(t, string(fields["message"]))
}

func TestAssembledSyncCreateSnapshotsAndReplay(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f)
			// 无注册、仅OPPO均成功，不捏造完整job。
			for i := 0; i < 2; i++ {
				if i == 1 {
					status, raw, h := f.admin("PUT", assembledPushPath, `{"provider":"opush","token":"synthetic-opush-only","registration_version":1,"expected_version":0}`)
					assertAssembledPush(t, status, 200, raw, h, "")
				}
				status, raw, h := syncCreateRequest(f, bot, admin, fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1))
				syncCreateReceipt(t, status, raw, h, admin)
				if len(syncCreateJobs(t, f)) != 0 {
					t.Fatal("invented FCM target")
				}
			}
			syncCreateRegister(t, f, "synthetic-sync-secret-v1", 7, 0)
			status, original, h := syncCreateRequest(f, bot, admin, clientMessageKey)
			m := syncCreateReceipt(t, status, original, h, admin)
			jobs := syncCreateJobs(t, f)
			if len(jobs) != 1 {
				t.Fatal("missing HTTP-generated job", jobs)
			}
			j := jobs[0]
			created, _ := time.Parse(time.RFC3339Nano, m.CreatedAt)
			if j.Snapshot.MessageID != m.ID || j.Snapshot.BotID != bot.Bot.ID || fmt.Sprint(j.Snapshot.BotSeq) != m.BotSeq || j.Snapshot.SessionID != f.androidID || j.Snapshot.DeploymentID != f.instanceID || j.Snapshot.SessionGeneration != 1 || j.Snapshot.RegistrationVersion != 7 || j.Snapshot.SettingsVersion != 0 || j.State != "queued" || j.Snapshot.Purpose != "sync" || j.Snapshot.Provider != "fcm" || j.Snapshot.AlertEligible || !j.Snapshot.CreatedAt.Equal(created) || !j.Snapshot.ExpiresAt.Equal(created.Add(24*time.Hour)) {
				t.Fatal("bad original snapshot", j)
			}
			var messageSeq, highest int64
			if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
				if err := tx.QueryRow("SELECT created_change_seq FROM messages WHERE id=?", m.ID).Scan(&messageSeq); err != nil {
					return err
				}
				return tx.QueryRow("SELECT max(change_seq) FROM changes").Scan(&highest)
			}); err != nil {
				t.Fatal(err)
			}
			if j.Snapshot.TargetChangeSeq != highest || (admin && highest <= messageSeq) {
				t.Fatal("early message water mark", j, highest, messageSeq)
			}
			// 轮换和none/DND只影响未来事件快照；旧回执不选新目标或刷新期限。
			syncCreateRegister(t, f, "synthetic-sync-secret-v2", 8, 7)
			settings := strings.Replace(assembledNotifications(0, "none"), `"enabled":false`, `"enabled":true`, 1)
			status, raw, h := f.admin("PUT", assembledNotificationsPath, settings)
			assertAssembledPush(t, status, 200, raw, h, "")
			s, secret, err := f.sessions.Create(context.Background(), 1, auth.SessionInput{ClientType: "android", DeviceID: "00000000-0000-4000-8000-000000000001", DeviceName: "same device second"})
			if err != nil {
				t.Fatal(err)
			}
			status, raw, h = f.request("PUT", assembledPushPath, assembledRegistration("synthetic-second-session", 2, 0), "android", secret.Token, nil)
			assertAssembledPush(t, status, 200, raw, h, "")
			for i := 0; i < 2; i++ {
				if i == 1 {
					f.sql("DELETE FROM messages WHERE id=?", m.ID)
				}
				before := clientMessageState(t, f.store)
				status, raw, h = syncCreateRequest(f, bot, admin, clientMessageKey)
				assertBotResponse(t, status, 201, raw, h)
				if raw != original || before != clientMessageState(t, f.store) || !reflect.DeepEqual(jobs, syncCreateJobs(t, f)) {
					t.Fatal("old receipt renewed or reconstructed targets")
				}
			}
			status, raw, h = syncCreateRequest(f, bot, admin, "00000000-0000-4000-8000-000000000005")
			next := syncCreateReceipt(t, status, raw, h, admin)
			all := syncCreateJobs(t, f)
			n := 0
			for _, job := range all {
				if job.Snapshot.MessageID == next.ID {
					n++
					if job.Snapshot.SettingsVersion != 1 || job.Snapshot.AlertEligible || (job.Snapshot.SessionID == s.ID && job.Snapshot.RegistrationVersion != 2) || (job.Snapshot.SessionID == f.androidID && job.Snapshot.RegistrationVersion != 8) {
						t.Fatal("current session/version or none+DND policy", job)
					}
				}
			}
			if n != 2 {
				t.Fatal("same device sessions collapsed", n)
			}
		})
	}
}

func TestAssembledSyncCreateConcurrentAndLostResponse(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f)
			syncCreateRegister(t, f, "synthetic-sync-secret", 1, 0)
			// 原12并发同键，不放宽超时或串行替代。
			var wg sync.WaitGroup
			results := make(chan string, 12)
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					status, raw, h := syncCreateRequest(f, bot, admin, clientMessageKey)
					if status != 201 || h.Get("Cache-Control") != "no-store" {
						results <- fmt.Sprintf("FAIL %d %s", status, raw)
						return
					}
					results <- raw
				}()
			}
			wg.Wait()
			close(results)
			original := ""
			for result := range results {
				if strings.HasPrefix(result, "FAIL") {
					t.Fatal(result)
				}
				if original == "" {
					original = result
				} else if result != original {
					t.Fatal("concurrent receipts differ")
				}
			}
			if len(syncCreateJobs(t, f)) != 1 {
				t.Fatal("same-key duplicated jobs")
			}
			// 首请求确已提交但完全丢弃响应；同键重试必须复用唯一快照。
			path := "/api/v1/bot/messages"
			token := bot.APIToken.Token
			if admin {
				path = "/api/v1/client/bots/" + bot.Bot.ID + "/messages"
				token = f.android.Token
			}
			transport := f.server.Client().Transport.(*http.Transport)
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", strings.TrimPrefix(f.server.URL, "https://"), transport.TLSClientConfig.Clone())
			if err != nil {
				t.Fatal(err)
			}
			const lostKey = "00000000-0000-4000-8000-000000000099"
			req, _ := http.NewRequest("POST", f.server.URL+path, strings.NewReader(clientMessageBody))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-RelayNest-Instance-ID", f.instanceID)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", lostKey)
			if err = req.Write(conn); err != nil {
				conn.Close()
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for len(syncCreateJobs(t, f)) != 2 {
				if time.Now().After(deadline) {
					conn.Close()
					t.Fatal("lost request did not commit")
				}
				time.Sleep(10 * time.Millisecond)
			}
			conn.Close()
			before := syncCreateJobs(t, f)
			status, raw, h := syncCreateRequest(f, bot, admin, lostKey)
			syncCreateReceipt(t, status, raw, h, admin)
			if !reflect.DeepEqual(before, syncCreateJobs(t, f)) {
				t.Fatal("lost response refreshed jobs")
			}
		})
	}
}

func TestAssembledSyncCreateFailuresAreAtomic(t *testing.T) {
	faults := []struct{ name, sql string }{
		{"select", `ALTER TABLE device_notification_settings RENAME TO hidden_settings`},
		{"second-job", `CREATE TRIGGER sync_fault BEFORE INSERT ON push_jobs WHEN (SELECT count(*) FROM push_jobs WHERE purpose='sync')=1 BEGIN SELECT RAISE(ABORT,'synthetic-sync-secret-fault');END`},
		{"receipt", `CREATE TRIGGER sync_fault BEFORE INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'synthetic-sync-secret-fault');END`},
		{"commit", `CREATE TABLE sync_deferred(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER sync_fault AFTER INSERT ON push_jobs BEGIN INSERT INTO sync_deferred VALUES('absent');END`},
	}
	for _, admin := range []bool{false, true} {
		for _, fault := range faults {
			t.Run(fmt.Sprintf("%t/%s", admin, fault.name), func(t *testing.T) {
				f := botHTTP(t, nil)
				bot := clientMessageBot(t, f)
				syncCreateRegister(t, f, "synthetic-sync-secret", 1, 0)
				_, secret, err := f.sessions.Create(context.Background(), 1, auth.SessionInput{ClientType: "android", DeviceID: "00000000-0000-4000-8000-000000000002", DeviceName: "second"})
				if err != nil {
					t.Fatal(err)
				}
				status, raw, h := f.request("PUT", assembledPushPath, assembledRegistration("synthetic-second", 1, 0), "android", secret.Token, nil)
				assertAssembledPush(t, status, 200, raw, h, "")
				if admin {
					f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), f.androidID)
				}
				f.sql(fault.sql)
				before, authorization := clientMessageState(t, f.store), f.snapshot()
				status, raw, h = syncCreateRequest(f, bot, admin, clientMessageKey)
				assertBotResponse(t, status, 503, raw, h)
				if !strings.Contains(raw, "SERVICE_UNAVAILABLE") || strings.Contains(raw, "synthetic") || strings.Contains(raw, "delivery_id") || before != clientMessageState(t, f.store) || authorization != f.snapshot() {
					t.Fatal("failure leaked or committed partial transaction", status, raw)
				}
			})
		}
	}
}

// 真实HTTP凭据拒绝发生在首次创建之前，已有FCM目标也不能留下job或续期。
func TestAssembledSyncCreateAuthorizationAndTargetExpiry(t *testing.T) {
	for _, change := range []string{"UPDATE sessions SET revoked=1 WHERE client_type='android'", "UPDATE sessions SET idle_expires_at=1 WHERE client_type='android'", "UPDATE sessions SET idle_expires_at=1,absolute_expires_at=1 WHERE client_type='android'", "UPDATE administrator SET auth_generation=auth_generation+1"} {
		t.Run(change, func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f)
			syncCreateRegister(t, f, "synthetic-sync-secret", 1, 0)
			f.sql(change)
			before, authorization := clientMessageState(t, f.store), f.snapshot()
			status, raw, h := syncCreateRequest(f, bot, true, clientMessageKey)
			assertBotResponse(t, status, 401, raw, h)
			if before != clientMessageState(t, f.store) || authorization != f.snapshot() {
				t.Fatal("rejected actor changed state")
			}
			// Bot仍有权创建，但失效Android目标必须跳过，不能反过来阻塞消息。
			status, raw, h = syncCreateRequest(f, bot, false, clientMessageKey)
			syncCreateReceipt(t, status, raw, h, false)
			if len(syncCreateJobs(t, f)) != 0 {
				t.Fatal("invalid session selected")
			}
		})
	}
	f := botHTTP(t, nil)
	bot := clientMessageBot(t, f)
	syncCreateRegister(t, f, "synthetic-sync-secret", 1, 0)
	// Bot API自身凭据拒绝；实例门禁沿Android入口的既定合同验证。
	for _, who := range []struct {
		token, path string
		headers     map[string]string
		status      int
	}{
		{bot.WebhookKey.Key, "/api/v1/bot/messages", map[string]string{"Idempotency-Key": clientMessageKey}, 401},
		{f.android.Token, "/api/v1/client/bots/" + bot.Bot.ID + "/messages", map[string]string{"Idempotency-Key": clientMessageKey, "X-RelayNest-Instance-ID": "22222222-2222-4222-8222-222222222222"}, 409},
	} {
		before, authorization := clientMessageState(t, f.store), f.snapshot()
		status, raw, h := f.request("POST", who.path, clientMessageBody, "android", who.token, who.headers)
		assertBotResponse(t, status, who.status, raw, h)
		if before != clientMessageState(t, f.store) || authorization != f.snapshot() {
			t.Fatal("bad credential/instance changed state")
		}
	}
	// 响应流已被其他用例完整读出；保留io引用以显式验证无多余JSON尾数据。
	status, raw, h := syncCreateRequest(f, bot, false, clientMessageKey)
	syncCreateReceipt(t, status, raw, h, false)
	decoder := json.NewDecoder(strings.NewReader(raw))
	var object any
	if decoder.Decode(&object) != nil || decoder.Decode(&object) != io.EOF {
		t.Fatal("invalid response JSON")
	}
}

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
)

type historyMessage struct {
	ID          string                         `json:"id"`
	BotID       string                         `json:"bot_id"`
	BotSeq      string                         `json:"bot_seq"`
	Revision    int64                          `json:"revision"`
	Sender      string                         `json:"sender"`
	Format      string                         `json:"format"`
	Text        string                         `json:"text"`
	Attachments []messages.AttachmentReference `json:"attachments"`
	Reply       *string                        `json:"reply_to_message_id"`
	Buttons     [][]messages.Button            `json:"buttons"`
	CreatedAt   string                         `json:"created_at"`
	UpdatedAt   string                         `json:"updated_at"`
}
type historyPage struct {
	Items []historyMessage `json:"items"`
	Next  *string          `json:"next_cursor"`
	More  bool             `json:"has_more"`
}

func historyCreate(t *testing.T, f *botHTTPFixture, bot, key, text string, reply *string) historyMessage {
	t.Helper()
	data, err := json.Marshal(map[string]any{"format": "plain", "text": text, "reply_to_message_id": reply})
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	key = digest[:8] + "-" + digest[8:12] + "-4" + digest[13:16] + "-8" + digest[17:20] + "-" + digest[20:32]
	status, raw, _ := clientMessageRequest(f, bot, key, string(data))
	if status != 201 {
		t.Fatalf("create status=%d %s", status, raw)
	}
	var receipt struct {
		Message historyMessage `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt.Message
}
func historyGet(t *testing.T, f *botHTTPFixture, path string) string {
	t.Helper()
	status, raw, headers := f.admin("GET", path, "")
	if status != 200 || headers.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET %s: %d %s", path, status, raw)
	}
	return raw
}
func historyReadPage(t *testing.T, f *botHTTPFixture, path string) historyPage {
	t.Helper()
	raw := historyGet(t, f, path)
	var page historyPage
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || page.More != (page.Next != nil) {
		t.Fatalf("invalid page %s", raw)
	}
	return page
}
func assertHistoryMessage(t *testing.T, raw string) historyMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	keys := []string{"id", "bot_id", "bot_seq", "revision", "sender", "format", "text", "attachments", "reply_to_message_id", "buttons", "created_at", "updated_at"}
	if len(fields) != len(keys) {
		t.Fatalf("message keys: %s", raw)
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			t.Fatalf("missing %s: %s", key, raw)
		}
	}
	var m historyMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	seq, err := strconv.ParseInt(m.BotSeq, 10, 64)
	if err != nil || seq < 1 || strconv.FormatInt(seq, 10) != m.BotSeq || m.Revision < 1 || m.Attachments == nil || m.Buttons == nil {
		t.Fatalf("invalid contract %s", raw)
	}
	for _, timestamp := range []string{m.CreatedAt, m.UpdatedAt} {
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil || !strings.HasSuffix(timestamp, "Z") {
			t.Fatalf("invalid UTC %s", raw)
		}
	}
	return m
}

func TestAssembledHistoryPaginationAndReference(t *testing.T) {
	f := botHTTP(t, nil)
	bot := clientMessageBot(t, f)
	base := "/api/v1/client/bots/" + bot.Bot.ID + "/messages"
	empty := historyReadPage(t, f, base)
	if len(empty.Items) != 0 || empty.More || empty.Next != nil {
		t.Fatal(empty)
	}
	first := historyCreate(t, f, bot.Bot.ID, "history-first", "source-history", nil)
	second := historyCreate(t, f, bot.Bot.ID, "history-second", "reply-history", &first.ID)
	third := historyCreate(t, f, bot.Bot.ID, "history-third", "newest-history", nil)
	page := historyReadPage(t, f, base+"?limit=2")
	if len(page.Items) != 2 || page.Items[0].ID != third.ID || page.Items[1].ID != second.ID || page.Next == nil {
		t.Fatal(page)
	}
	detail := assertHistoryMessage(t, historyGet(t, f, "/api/v1/client/messages/"+second.ID))
	if detail.Reply == nil || *detail.Reply != first.ID || detail.Text != "reply-history" {
		t.Fatal(detail)
	}
	// 编辑保持原序号；并发新增只进入首屏，已有边界不会重复或跳过旧消息。
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		f.sql("UPDATE messages SET text='edited-history',revision=revision+1 WHERE id=?", first.ID)
	}()
	fourth := historyCreate(t, f, bot.Bot.ID, "history-fourth", "added-history", nil)
	wg.Wait()
	tail := historyReadPage(t, f, base+"?limit=2&cursor="+url.QueryEscape(*page.Next))
	if len(tail.Items) != 1 || tail.Items[0].ID != first.ID || tail.Items[0].Revision != 2 || tail.Items[0].BotSeq != first.BotSeq || tail.More || tail.Next != nil {
		t.Fatal(tail)
	}
	head := historyReadPage(t, f, base+"?limit=1")
	if head.Items[0].ID != fourth.ID {
		t.Fatal(head)
	}
	// 边界消息与源消息物理删除后，cursor仍按序号继续，引用仍保留原ID。
	f.sql("DELETE FROM messages WHERE id=?", second.ID)
	afterDeletedBoundary := historyReadPage(t, f, base+"?limit=2&cursor="+url.QueryEscape(*page.Next))
	if len(afterDeletedBoundary.Items) != 1 || afterDeletedBoundary.Items[0].ID != first.ID {
		t.Fatal(afterDeletedBoundary)
	}
	reply := historyCreate(t, f, bot.Bot.ID, "history-reference", "retained-reference", &first.ID)
	f.sql("DELETE FROM messages WHERE id=?", first.ID)
	retained := assertHistoryMessage(t, historyGet(t, f, "/api/v1/client/messages/"+reply.ID))
	if retained.Reply == nil || *retained.Reply != first.ID {
		t.Fatal(retained)
	}
	status, raw, headers := f.admin("GET", "/api/v1/client/messages/"+first.ID, "")
	if status != 404 || !strings.Contains(raw, "RESOURCE_NOT_FOUND") || strings.Contains(raw, "edited-history") || headers.Get("Cache-Control") != "no-store" {
		t.Fatalf("deleted %d %s", status, raw)
	}
	f.sql("UPDATE bots SET enabled=0 WHERE id=?", bot.Bot.ID)
	historyReadPage(t, f, base)
	historyGet(t, f, "/api/v1/client/messages/"+reply.ID)
	// 当前 schema 没有关联附件；已上传独立文件不会混入历史。按钮快照不能丢失。
	f.sql(`UPDATE messages SET buttons_json='[[{"id":"url","label":"链接","url":"https://example.test"},{"id":"callback","label":"回调","callback_data":""}]]',created_at='2026-01-01T08:00:00+08:00' WHERE id=?`, reply.ID)
	projected := assertHistoryMessage(t, historyGet(t, f, "/api/v1/client/messages/"+reply.ID))
	if len(projected.Buttons) != 1 || len(projected.Buttons[0]) != 2 || projected.Buttons[0][1].CallbackData == nil || *projected.Buttons[0][1].CallbackData != "" || projected.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Fatal(projected)
	}
}

func TestAssembledHistoryQueryAndCredentials(t *testing.T) {
	f := botHTTP(t, nil)
	bot := clientMessageBot(t, f)
	other := clientMessageBot(t, f)
	base := "/api/v1/client/bots/" + bot.Bot.ID + "/messages"
	m := historyCreate(t, f, bot.Bot.ID, "history-query-a", "credential-protected-text", nil)
	historyCreate(t, f, bot.Bot.ID, "history-query-b", "credential-protected-text", nil)
	first := historyReadPage(t, f, base+"?limit=1")
	for _, query := range []string{"cursor=", "cursor=garbage", "limit=0", "limit=-1", "limit=x", "limit=1&limit=2", "cursor=x&cursor=y", "offset=1", "cursor=%zz"} {
		status, raw, h := f.admin("GET", base+"?"+query, "")
		if status != 400 || !strings.Contains(raw, "INVALID_REQUEST") || strings.Contains(raw, m.Text) || h.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s => %d %s", query, status, raw)
		}
	}
	status, raw, _ := f.admin("GET", "/api/v1/client/bots/"+other.Bot.ID+"/messages?cursor="+url.QueryEscape(*first.Next), "")
	if status != 400 || !strings.Contains(raw, "INVALID_REQUEST") {
		t.Fatalf("cross bot %d %s", status, raw)
	}
	historyReadPage(t, f, base+"?limit=999999")
	for _, path := range []string{base, "/api/v1/client/messages/" + m.ID} {
		for _, identity := range []struct{ kind, token string }{{"web", f.web.Token}, {"android", bot.APIToken.Token}, {"android", bot.WebhookKey.Key}, {"android", ""}} {
			before := f.snapshot()
			status, raw, h := f.request("GET", path, "", identity.kind, identity.token, nil)
			if status != 401 || strings.Contains(raw, m.Text) || h.Get("Cache-Control") != "no-store" || f.snapshot() != before {
				t.Fatalf("forbidden credential %d %s", status, raw)
			}
		}
		// 默认夹具带实例头；缺失头必须另构造请求，不能与显式空值混淆。
		request, err := http.NewRequest("GET", f.server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+f.android.Token)
		response, err := f.server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		missingRaw, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 400 || !strings.Contains(string(missingRaw), "INSTANCE_REQUIRED") || strings.Contains(string(missingRaw), m.Text) || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("missing instance %d %s", response.StatusCode, missingRaw)
		}
		for _, v := range []struct {
			header string
			status int
			code   string
		}{{"", 400, "INVALID_REQUEST"}, {"invalid", 400, "INVALID_REQUEST"}, {"00000000-0000-4000-8000-000000000099", 409, "INSTANCE_MISMATCH"}} {
			status, raw, _ := f.request("GET", path, "", "android", f.android.Token, map[string]string{"X-RelayNest-Instance-ID": v.header})
			if status != v.status || !strings.Contains(raw, v.code) || strings.Contains(raw, m.Text) {
				t.Fatalf("instance %d %s", status, raw)
			}
		}
	}
}

// 装配真实Android授权，仅在明确时点改变SQLite状态；候选正文不得越过最终读复核。
func historyIntercept(f *botHTTPFixture, before func(), after func(*sql.Tx) error) {
	policy, err := httpapi.NewOriginPolicy(f.server.URL, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	authorize := clientMessageAuthorization(f.sessions, auth.NewRequestSecurity(f.sessions, policy))
	f.handler.MessageHandler = httpapi.NewMessageHandler(nil, messages.New(f.store), func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
		if write {
			f.t.Error("历史请求错误使用写授权")
		}
		access, ok := authorize(w, r, write)
		if !ok {
			return access, false
		}
		read := access.Read
		access.Read = func(ctx context.Context, fn func(*sql.Tx) error) error {
			if before != nil {
				before()
			}
			return read(ctx, func(tx *sql.Tx) error {
				if err := fn(tx); err != nil {
					return err
				}
				if after != nil {
					return after(tx)
				}
				return nil
			})
		}
		return access, true
	})
}
func TestAssembledHistoryAuthorizationInvalidation(t *testing.T) {
	// 共享资源归父测试所有；child 只持有绑定当前 testing.T 的浅拷贝。
	shared := botHTTP(t, nil)
	sharedBot := clientMessageBot(t, shared).Bot.ID
	sharedMessage := historyCreate(t, shared, sharedBot, "history-invalid", "do-not-release-candidate", nil)
	sharedBaseline := readRollbackBaseline(t, shared)
	mutations := []struct {
		name, statement string
		args            func(*botHTTPFixture) []any
	}{
		{"revoked", "UPDATE sessions SET revoked=1 WHERE id=?", func(f *botHTTPFixture) []any { return []any{f.androidID} }},
		{"expired", "UPDATE sessions SET idle_expires_at=0 WHERE id=?", func(f *botHTTPFixture) []any { return []any{f.androidID} }},
		{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", func(*botHTTPFixture) []any { return nil }},
		{"session-generation", "UPDATE sessions SET auth_generation=auth_generation+1 WHERE id=?", func(f *botHTTPFixture) []any { return []any{f.androidID} }},
		{"device", "UPDATE sessions SET device_id=? WHERE id=?", func(f *botHTTPFixture) []any { return []any{"00000000-0000-4000-8000-000000000099", f.androidID} }},
		{"instance", "UPDATE instance_identity SET instance_id=? WHERE id=1", func(*botHTTPFixture) []any { return []any{"00000000-0000-4000-8000-000000000099"} }},
	}
	for _, mutation := range mutations {
		for _, phase := range []string{"before", "final"} {
			for _, detail := range []bool{false, true} {
				if !t.Run(fmt.Sprintf("%s/%s/detail=%v", mutation.name, phase, detail), func(t *testing.T) {
					var f *botHTTPFixture
					var bot string
					var m historyMessage
					if phase == "before" {
						// before 提交身份变化，仍保持每例独立的真实夹具及 instance observer。
						f = botHTTP(t, nil)
						bot = clientMessageBot(t, f).Bot.ID
						m = historyCreate(t, f, bot, "history-invalid", "do-not-release-candidate", nil)
					} else {
						// final 的变化必须由授权读事务回滚；仅这一组复用真实 TLS/SQLite 夹具。
						local := *shared
						local.t = t
						f, bot, m = &local, sharedBot, sharedMessage
						original := *shared
						originalHandler := f.handler.MessageHandler
						t.Cleanup(func() {
							f.handler.MessageHandler = originalHandler
							f.afterAuthorize, f.beforeRead = original.afterAuthorize, original.beforeRead
							f.sessions.SetRevocationHook(nil)
							*shared = original
							if readRollbackBaseline(t, f) != sharedBaseline {
								t.Fatal("共享历史夹具未恢复全表及 schema 基线")
							}
						})
						if readRollbackBaseline(t, f) != sharedBaseline {
							t.Fatal("共享历史夹具起始全表及 schema 基线改变")
						}
					}
					if session, _, err := f.sessions.Inspect(context.Background(), f.android.Token, "android"); err != nil || session.ID != f.androidID {
						t.Fatalf("历史请求起始会话无效: %v", err)
					}
					var reached atomic.Int32
					var baseline [32]byte
					observed := *f
					if phase == "before" {
						historyIntercept(f, func() {
							reached.Add(1)
							f.sql(mutation.statement, mutation.args(f)...)
							if mutation.name == "instance" {
								// 生产Store保留原绑定并失败关闭；新Store仅用于独立观测副作用。
								observer, err := storage.Open(context.Background(), storage.Config{Path: f.databasePath})
								if err != nil {
									t.Fatal(err)
								}
								observed.store = observer
								t.Cleanup(func() { observer.Close() })
							}
							baseline = observed.snapshot()
						}, nil)
					} else {
						baseline = f.snapshot()
						historyIntercept(f, nil, func(tx *sql.Tx) error {
							// 原拦截点在真实候选读取及投影成功之后、最终授权复核之前。
							reached.Add(1)
							_, err := tx.Exec(mutation.statement, mutation.args(f)...)
							return err
						})
					}
					path := "/api/v1/client/bots/" + bot + "/messages"
					if detail {
						path = "/api/v1/client/messages/" + m.ID
					}
					status, raw, h := f.admin("GET", path, "")
					if reached.Load() != 1 {
						t.Fatalf("历史请求未恰好一次到达 %s 授权拦截边界: %d", phase, reached.Load())
					}
					wantStatus, wantCode := 401, "AUTHENTICATION_REQUIRED"
					if mutation.name == "instance" {
						wantStatus, wantCode = 409, "INSTANCE_MISMATCH"
						if phase == "before" {
							wantStatus, wantCode = 503, "INSTANCE_UNAVAILABLE"
						}
					}
					if status != wantStatus || !strings.Contains(raw, wantCode) || strings.Contains(raw, m.Text) || h.Get("Cache-Control") != "no-store" || observed.snapshot() != baseline {
						t.Fatalf("invalidated status=%d %s", status, raw)
					}
				}) {
					// 包含 cleanup 失败；污染时停止父组，禁止后续共享用例提前拒绝造成假通过。
					return
				}
			}
		}
	}
}

func TestAssembledHistoryCorruptionFailsEntirePage(t *testing.T) {
	for _, statement := range []string{
		`UPDATE messages SET buttons_json='[null]' WHERE id=?`,
		`UPDATE messages SET created_at='invalid' WHERE id=?`,
	} {
		t.Run(statement, func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f)
			m := historyCreate(t, f, bot.Bot.ID, "history-corrupt", "corrupted-private-text", nil)
			historyCreate(t, f, bot.Bot.ID, "history-valid", "valid-private-text", nil)
			f.sql(statement, m.ID)
			before := f.snapshot()
			for _, path := range []string{"/api/v1/client/bots/" + bot.Bot.ID + "/messages", "/api/v1/client/messages/" + m.ID} {
				status, raw, h := f.admin("GET", path, "")
				if status != 503 || strings.Contains(raw, "private-text") || strings.Contains(raw, "SELECT") || h.Get("Cache-Control") != "no-store" || f.snapshot() != before {
					t.Fatalf("corruption %d %s", status, raw)
				}
			}
		})
	}
}

func TestAssembledHistoryUTCSerializationBoundary(t *testing.T) {
	for _, field := range []string{"created_at", "updated_at"} {
		for _, timestamp := range []string{"9999-12-31T23:59:59-08:00", "0000-01-01T00:00:00+08:00"} {
			t.Run(field+"/"+timestamp, func(t *testing.T) {
				f := botHTTP(t, nil)
				bot := clientMessageBot(t, f)
				m := historyCreate(t, f, bot.Bot.ID, "history-time-corrupt", "corrupted-private-text", nil)
				// 新消息先投影成功，较旧的坏行仍须阻止整页正文发布。
				historyCreate(t, f, bot.Bot.ID, "history-time-valid", "valid-private-text", nil)
				f.sql("UPDATE messages SET "+field+"=? WHERE id=?", timestamp, m.ID)
				// 固定为短于正常续期的有效过期时间，避免零副作用断言偶然通过。
				f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), f.androidID)
				readIdle := func() int64 {
					var idle int64
					if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
						return tx.QueryRow("SELECT idle_expires_at FROM sessions WHERE id=?", f.androidID).Scan(&idle)
					}); err != nil {
						t.Fatal(err)
					}
					return idle
				}
				idleBefore := readIdle()
				before := f.snapshot()
				for _, path := range []string{"/api/v1/client/bots/" + bot.Bot.ID + "/messages", "/api/v1/client/messages/" + m.ID} {
					status, raw, h := f.admin("GET", path, "")
					idleAfter := readIdle()
					if status != 503 || !strings.Contains(raw, "SERVICE_UNAVAILABLE") || strings.Contains(raw, "private-text") || strings.Contains(raw, "SELECT") || h.Get("Cache-Control") != "no-store" || f.snapshot() != before || idleAfter != idleBefore {
						t.Fatalf("UTC 边界 %s: status=%d body=%s idle=%d→%d", path, status, raw, idleBefore, idleAfter)
					}
				}
			})
		}
	}
}

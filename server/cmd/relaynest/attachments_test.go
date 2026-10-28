package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"relaynest/server/internal/storage"
	"strings"
	"syscall"
	"testing"
	"time"

	"relaynest/server/internal/attachments"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
)

const uploadKey = "42e088ba-9df1-49fa-9878-8b98ed2f0a71"

type uploadFixture struct {
	*botHTTPFixture
	bot       bots.Created
	directory string
}

func uploadHTTP(t *testing.T, limits attachments.Limits) *uploadFixture {
	t.Helper()
	f := botHTTP(t, nil)
	dir := filepath.Join(t.TempDir(), "files")
	files, err := attachments.OpenFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { files.Close() })
	f.handler.AttachmentHandler = httpapi.NewAttachmentHandler(f.store, files, limits, f.handler.attachmentAuthorize)
	f.server.Config.Handler = httpapi.New(health.New(nil), f.handler)
	bot, _, err := bots.New(f.store).Create(context.Background(), bots.CreateRequest{Name: "upload", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	return &uploadFixture{f, bot, dir}
}
func (f *uploadFixture) path(id string) string {
	return "/api/v1/attachments?" + url.Values{"bot_id": {id}, "kind": {"file"}, "filename": {"../../秘密.txt"}, "media_type": {"text/plain"}}.Encode()
}
func (f *uploadFixture) send(token, key, body string) (int, string, http.Header) {
	return f.request("POST", f.path(f.bot.Bot.ID), body, "android", token, map[string]string{"Content-Type": "application/octet-stream", "Idempotency-Key": key})
}
func (f *uploadFixture) count(state string) int {
	f.t.Helper()
	var n int
	err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT count(*) FROM attachments WHERE state=?", state).Scan(&n)
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return n
}

func TestAttachmentHTTPBotAndroidAndReplay(t *testing.T) {
	f := uploadHTTP(t, attachments.Limits{Image: 8, File: 8})
	for index, token := range []string{f.bot.APIToken.Token, f.android.Token} {
		key := fmt.Sprintf("42e088ba-9df1-49fa-9878-%012d", index)
		status, raw, headers := f.send(token, key, "12345678")
		if status != 201 || headers.Get("Cache-Control") != "no-store" {
			t.Fatalf("%d %s", status, raw)
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte("12345678"))
		if len(out) != 9 || out["size"] != "8" || out["sha256"] != hex.EncodeToString(sum[:]) || out["state"] != "ready" {
			t.Fatal(raw)
		}
		status, replay, _ := f.send(token, key, "12345678")
		if status != 201 || replay != raw {
			t.Fatalf("replay %d %s", status, replay)
		}
		status, raw, _ = f.send(token, key, "87654321")
		if status != 409 || !strings.Contains(raw, "IDEMPOTENCY_CONFLICT") {
			t.Fatalf("conflict %d %s", status, raw)
		}
	}
	if f.count("ready") != 2 {
		t.Fatal("unexpected ready count")
	}
	entries, err := os.ReadDir(f.directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "秘密") || strings.Contains(entry.Name(), "..") {
			t.Fatal(entry.Name())
		}
	}
}

type uploadReadSpy struct{ calls int }

func (s *uploadReadSpy) Read([]byte) (int, error) { s.calls++; return 0, io.EOF }
func (*uploadReadSpy) Close() error               { return nil }

// 真实安全入口和 SQLite 授权保持不变，只替换 Body 以精确观察首次 Read。
func TestAttachmentHTTPRejectsBeforeRead(t *testing.T) {
	f := uploadHTTP(t, attachments.DefaultLimits())
	receive, _, err := bots.New(f.store).Create(context.Background(), bots.CreateRequest{Name: "receive", Enabled: true, Capabilities: bots.Capabilities{Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, token, instance, cookie, target, code string
		status                                      int
	}{
		{"bot-no-instance", receive.APIToken.Token, "", "", f.bot.Bot.ID, "FORBIDDEN", 403},
		{"cross-bot", f.bot.APIToken.Token, "bad", "", receive.Bot.ID, "RESOURCE_NOT_FOUND", 404},
		{"web-cookie", "", "", f.web.Token, f.bot.Bot.ID, "AUTHENTICATION_REQUIRED", 401},
		{"mixed", f.bot.APIToken.Token, "bad", f.web.Token, f.bot.Bot.ID, "AUTHENTICATION_REQUIRED", 401},
		{"webhook", f.bot.WebhookKey.Key, f.instanceID, "", f.bot.Bot.ID, "AUTHENTICATION_REQUIRED", 401},
		{"android-missing", f.android.Token, "", "", f.bot.Bot.ID, "INSTANCE_REQUIRED", 400},
		{"android-malformed", f.android.Token, "bad", "", f.bot.Bot.ID, "INVALID_REQUEST", 400},
		{"android-wrong", f.android.Token, "00000000-0000-4000-8000-000000000099", "", f.bot.Bot.ID, "INSTANCE_MISMATCH", 409},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spy := &uploadReadSpy{}
			req := httptest.NewRequest("POST", f.server.URL+f.path(test.target), spy)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Idempotency-Key", uploadKey)
			if test.token != "" {
				req.Header.Set("Authorization", "Bearer "+test.token)
			}
			if test.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "relaynest_session", Value: test.cookie})
			}
			if test.instance != "" {
				req.Header.Set("X-RelayNest-Instance-ID", test.instance)
			}
			rec := httptest.NewRecorder()
			f.server.Config.Handler.ServeHTTP(rec, req)
			if rec.Code != test.status || !strings.Contains(rec.Body.String(), test.code) || spy.calls != 0 || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%d %s reads=%d", rec.Code, rec.Body.String(), spy.calls)
			}
		})
	}
	if f.count("staging") != 0 || f.count("ready") != 0 {
		t.Fatal("unauthorized rows")
	}
	// Bot 分支确定后忽略实例头，包括畸形值。
	status, raw, _ := f.request("POST", f.path(f.bot.Bot.ID), "ok", "android", f.bot.APIToken.Token, map[string]string{"Content-Type": "application/octet-stream", "Idempotency-Key": uploadKey, "X-RelayNest-Instance-ID": "bad"})
	if status != 201 {
		t.Fatalf("%d %s", status, raw)
	}
}

func TestAttachmentHTTPActualLimitAndFailures(t *testing.T) {
	for _, kind := range []string{"limit", "ready-sql", "commit"} {
		t.Run(kind, func(t *testing.T) {
			f := uploadHTTP(t, attachments.Limits{Image: 8, File: 8})
			if kind == "ready-sql" {
				f.sql(`CREATE TRIGGER reject_ready BEFORE UPDATE OF state ON attachments WHEN NEW.state='ready' BEGIN SELECT RAISE(ABORT,'private-path-token'); END`)
			}
			if kind == "commit" {
				f.sql(`CREATE TABLE upload_commit_failure(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
				f.sql(`CREATE TRIGGER fail_ready_commit AFTER UPDATE OF state ON attachments WHEN NEW.state='ready' BEGIN INSERT INTO upload_commit_failure VALUES('absent'); END`)
			}
			body := "12345678"
			want := 503
			if kind == "limit" {
				body += "9"
				want = 413
			}
			status, raw, _ := f.send(f.bot.APIToken.Token, uploadKey, body)
			if status != want || strings.Contains(raw, "private-path-token") || f.count("ready") != 0 || f.count("staging") != 1 {
				t.Fatalf("%d %s", status, raw)
			}
			entries, err := os.ReadDir(f.directory)
			if err != nil || len(entries) == 0 {
				t.Fatalf("missing recovery evidence: %v %v", entries, err)
			}
		})
	}
}

func TestAttachmentHTTPStreamingReauthorization(t *testing.T) {
	for _, change := range []string{"bot-revoke", "bot-rotate", "android-revoke", "instance-change", "slow-valid"} {
		t.Run(change, func(t *testing.T) {
			f := uploadHTTP(t, attachments.DefaultLimits())
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			token := f.bot.APIToken.Token
			if change == "android-revoke" || change == "instance-change" {
				token = f.android.Token
			}
			req, err := http.NewRequest("POST", f.server.URL+f.path(f.bot.Bot.ID), reader)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Idempotency-Key", uploadKey)
			req.Header.Set("X-RelayNest-Instance-ID", f.instanceID)
			type result struct {
				status int
				raw    string
				err    error
			}
			done := make(chan result, 1)
			go func() {
				res, err := f.server.Client().Do(req)
				if err != nil {
					done <- result{err: err}
					return
				}
				defer res.Body.Close()
				raw, err := io.ReadAll(res.Body)
				done <- result{res.StatusCode, string(raw), err}
			}()
			if _, err = writer.Write([]byte("first")); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for f.count("staging") == 0 {
				if time.Now().After(deadline) {
					t.Fatal("no staging")
				}
				time.Sleep(5 * time.Millisecond)
			}
			// 慢流中执行真实写操作，证明上传没有持有 SQLite 事务。
			mutate := func() {
				switch change {
				case "bot-revoke":
					f.sql("UPDATE bots SET enabled=0 WHERE id=?", f.bot.Bot.ID)
				case "bot-rotate":
					f.sql("UPDATE bot_credentials SET generation=generation+1 WHERE bot_id=? AND kind='api_token'", f.bot.Bot.ID)
				case "android-revoke":
					f.sql("UPDATE sessions SET revoked=1 WHERE id=?", f.androidID)
				case "instance-change":
					f.sql("UPDATE instance_identity SET instance_id=? WHERE id=1", "00000000-0000-4000-8000-000000000099")
				default:
					f.sql("UPDATE bots SET name='slow writable' WHERE id=?", f.bot.Bot.ID)
				}
			}
			mutate()
			writer.Write([]byte("last"))
			writer.Close()
			select {
			case out := <-done:
				if out.err != nil {
					t.Fatal(out.err)
				}
				want := 201
				switch change {
				case "bot-revoke":
					want = 403
				case "bot-rotate":
					want = 401
				case "android-revoke":
					want = 401
				case "instance-change":
					want = 503
				}
				if out.status != want {
					t.Fatalf("%d %s", out.status, out.raw)
				}
				if change == "instance-change" {
					// 旧 Store 在实例切换后按既定契约失败关闭；新 Store 只读核查恢复证据。
					reopened, err := storage.Open(context.Background(), storage.Config{Path: f.databasePath})
					if err != nil {
						t.Fatal(err)
					}
					defer reopened.Close()
					var ready, staging int
					err = reopened.Read(context.Background(), func(tx *sql.Tx) error {
						return tx.QueryRow("SELECT sum(state='ready'),sum(state='staging') FROM attachments").Scan(&ready, &staging)
					})
					if err != nil || ready != 0 || staging != 1 || !strings.Contains(out.raw, "INSTANCE_UNAVAILABLE") {
						t.Fatalf("instance publication: ready=%d staging=%d err=%v", ready, staging, err)
					}
					return
				}
				if want != 201 && (f.count("ready") != 0 || f.count("staging") != 1) {
					t.Fatal("published after revoke")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("stream stuck")
			}
		})
	}
}

func TestAttachmentHTTPDiscoveryFailureAndDisabled(t *testing.T) {
	for _, failure := range []string{"disabled", "discovery"} {
		t.Run(failure, func(t *testing.T) {
			f := uploadHTTP(t, attachments.DefaultLimits())
			if failure == "disabled" {
				f.sql("UPDATE bots SET enabled=0 WHERE id=?", f.bot.Bot.ID)
			} else {
				f.sql("ALTER TABLE bot_credentials RENAME TO private_unavailable")
			}
			spy := &uploadReadSpy{}
			req := httptest.NewRequest("POST", f.server.URL+f.path(f.bot.Bot.ID), spy)
			req.Header.Set("Authorization", "Bearer "+f.bot.APIToken.Token)
			rec := httptest.NewRecorder()
			f.server.Config.Handler.ServeHTTP(rec, req)
			want, code := 403, "FORBIDDEN"
			if failure == "discovery" {
				want, code = 503, "INSTANCE_UNAVAILABLE"
			}
			if rec.Code != want || !strings.Contains(rec.Body.String(), code) || spy.calls != 0 {
				t.Fatalf("%d %s reads=%d", rec.Code, rec.Body.String(), spy.calls)
			}
		})
	}
}

func TestAttachmentHTTPFencingAndConcurrentKey(t *testing.T) {
	f := uploadHTTP(t, attachments.DefaultLimits())
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	req, _ := http.NewRequest("POST", f.server.URL+f.path(f.bot.Bot.ID), reader)
	req.Header.Set("Authorization", "Bearer "+f.bot.APIToken.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Idempotency-Key", uploadKey)
	type result struct {
		status int
		err    error
	}
	done := make(chan result, 1)
	go func() {
		res, err := f.server.Client().Do(req)
		if err != nil {
			done <- result{err: err}
			return
		}
		defer res.Body.Close()
		io.Copy(io.Discard, res.Body)
		done <- result{status: res.StatusCode}
	}()
	writer.Write([]byte("late"))
	deadline := time.Now().Add(3 * time.Second)
	for f.count("staging") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no staging")
		}
		time.Sleep(5 * time.Millisecond)
	}
	status, _, _ := f.send(f.bot.APIToken.Token, uploadKey, "new")
	if status != 503 {
		t.Fatalf("active %d", status)
	}
	f.sql("UPDATE attachment_uploads SET lease_until=0")
	status, raw, _ := f.send(f.bot.APIToken.Token, uploadKey, "new")
	if status != 201 {
		t.Fatalf("replacement %d %s", status, raw)
	}
	writer.Close()
	select {
	case out := <-done:
		if out.err != nil || out.status != 503 {
			t.Fatalf("late: %+v", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("late stuck")
	}
	if f.count("ready") != 1 {
		t.Fatal("duplicate ready")
	}
}

func TestAttachmentHTTPIgnoresContentLengthAndTruncation(t *testing.T) {
	f := uploadHTTP(t, attachments.Limits{Image: 8, File: 8})
	req := httptest.NewRequest("POST", f.server.URL+f.path(f.bot.Bot.ID), strings.NewReader("123456789"))
	req.ContentLength = 1
	req.Header.Set("Authorization", "Bearer "+f.bot.APIToken.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Idempotency-Key", uploadKey)
	rec := httptest.NewRecorder()
	f.server.Config.Handler.ServeHTTP(rec, req)
	if rec.Code != 413 {
		t.Fatalf("false length %d %s", rec.Code, rec.Body.String())
	}
	// 实际网络流中断不能把已经读到的前缀发布为附件。
	reader, writer := io.Pipe()
	defer reader.Close()
	req, _ = http.NewRequest("POST", f.server.URL+f.path(f.bot.Bot.ID), reader)
	req.Header.Set("Authorization", "Bearer "+f.bot.APIToken.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Idempotency-Key", "42e088ba-9df1-49fa-9878-8b98ed2f0a72")
	done := make(chan struct{})
	go func() {
		defer close(done)
		res, _ := f.server.Client().Do(req)
		if res != nil {
			res.Body.Close()
		}
	}()
	writer.Write([]byte("abc"))
	writer.CloseWithError(io.ErrUnexpectedEOF)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel stuck")
	}
	if f.count("ready") != 0 {
		t.Fatal("truncated published")
	}
}

func TestMainProcessAttachmentConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.sqlite")
	files := filepath.Join(dir, "volume")
	for _, bad := range []string{"0", "-1", "9223372036854775808", "invalid"} {
		cmd := mainProcess("-db", path, "-external-origin", "https://relay.example", "-file-max-bytes", bad)
		if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "configuration_failed") {
			t.Fatalf("invalid limit %q: %v %s", bad, err, out)
		}
	}
	store, err := storage.Open(context.Background(), storage.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	bot, _, err := bots.New(store).Create(context.Background(), bots.CreateRequest{Name: "production", Enabled: true, Capabilities: bots.Capabilities{Send: true}})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	reserve, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reserve.Addr().String()
	reserve.Close()
	cmd := mainProcess("-db", path, "-external-origin", "https://relay.example", "-listen", addr, "-attachments-dir", files, "-image-max-bytes", "9223372036854775807", "-file-max-bytes", "9")
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			cmd.Process.Kill()
			<-done
		}
	}()
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := client.Get("http://" + addr + "/health/live")
		if err == nil {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("startup timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	target := "http://" + addr + "/api/v1/attachments?" + url.Values{"bot_id": {bot.Bot.ID}, "kind": {"file"}, "filename": {"private-filename"}, "media_type": {"text/plain"}}.Encode()
	for n, want := range map[int]int{9: 201, 10: 413} {
		req, _ := http.NewRequest("POST", target, strings.NewReader(strings.Repeat("x", n)))
		req.Header.Set("Authorization", "Bearer "+bot.APIToken.Token)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Idempotency-Key", fmt.Sprintf("42e088ba-9df1-49fa-9878-%012d", n))
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("%d %s", res.StatusCode, raw)
		}
	}
	cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-done:
		stopped = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("shutdown stuck")
	}
	for _, secret := range []string{bot.APIToken.Token, "private-filename", target, files, path} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("private upload data entered process logs")
		}
	}
	entries, err := os.ReadDir(files)
	if err != nil || len(entries) != 2 {
		t.Fatalf("actual attachment volume %v %v", entries, err)
	}
	assertNoDatabaseFDs(t, dir)
}

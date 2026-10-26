package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
)

const messageKey = "abcdefff-1234-4567-89ab-123456789abc"
const messageBody = `{"format":"markdown","text":" 正文 α\n"}`

type messageFixture struct {
	store         *storage.Store
	bots          *bots.Repository
	handler       *MessageHandler
	server        *httptest.Server
	path          string
	first, second bots.Created
}

func setupMessageHTTP(t *testing.T) *messageFixture {
	t.Helper()
	f := &messageFixture{path: filepath.Join(t.TempDir(), "messages.sqlite")}
	f.open(t)
	var err error
	f.first, _, err = f.bots.Create(context.Background(), bots.CreateRequest{Name: "first", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.second, _, err = f.bots.Create(context.Background(), bots.CreateRequest{Name: "second", Enabled: true, Capabilities: bots.Capabilities{Send: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.server.Close()
		if err := f.store.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}
func (f *messageFixture) open(t *testing.T) {
	t.Helper()
	var err error
	f.store, err = storage.Open(context.Background(), storage.Config{Path: f.path})
	if err != nil {
		t.Fatal(err)
	}
	f.bots = bots.New(f.store)
	f.handler = NewMessageHandler(f.bots, messages.New(f.store))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/bot/messages", f.handler.CreateBotMessage)
	f.server = httptest.NewServer(mux)
}
func (f *messageFixture) request(t *testing.T, token, key, body, instance string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("POST", f.server.URL+"/api/v1/bot/messages", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	if instance != "" {
		req.Header.Set("X-RelayNest-Instance-ID", instance)
	}
	res, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	if res.StatusCode != 201 {
		var obj map[string]any
		if json.Unmarshal(data, &obj) != nil {
			t.Fatal(string(data))
		}
		v := obj["error"].(map[string]any)
		if v["request_id"] == "" || v["retryable"] != (res.StatusCode == 503) {
			t.Fatalf("invalid error: %s", data)
		}
	}
	return res.StatusCode, data
}
func (f *messageFixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if err := f.store.Write(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec(q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func (f *messageFixture) state(t *testing.T) []int64 {
	t.Helper()
	var out []int64
	err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, q := range []string{"SELECT sum(bot_seq) FROM bots", "SELECT count(*) FROM messages", "SELECT count(*) FROM changes", "SELECT count(*) FROM push_jobs", "SELECT count(*) FROM deliveries", "SELECT count(*) FROM idempotency"} {
			var n int64
			if err := tx.QueryRow(q).Scan(&n); err != nil {
				return err
			}
			out = append(out, n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func messageID(t *testing.T, data []byte) string {
	t.Helper()
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		t.Fatal(string(data))
	}
	if len(m) != 12 || m["bot_seq"] != "1" || m["sender"] != "application" || m["reply_to_message_id"] != nil {
		t.Fatalf("bad projection %s", data)
	}
	return m["id"].(string)
}

func TestMessageHTTPRecoveryAndIsolation(t *testing.T) {
	f := setupMessageHTTP(t)
	// 首请求通过真实 TCP 发出，确认数据库提交后直接丢弃连接，完全不读取响应。
	conn, err := net.Dial("tcp", f.server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(conn, "POST /api/v1/bot/messages HTTP/1.1\r\nHost: relay.example\r\nAuthorization: Bearer %s\r\nContent-Type: application/json\r\nIdempotency-Key: %s\r\nContent-Length: %d\r\n\r\n%s", f.first.APIToken.Token, messageKey, len(messageBody), messageBody)
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for f.state(t)[1] != 1 {
		if time.Now().After(deadline) {
			conn.Close()
			t.Fatal("first request did not commit")
		}
		time.Sleep(time.Millisecond)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error { return tx.QueryRow("SELECT id FROM messages").Scan(&id) }); err != nil {
		t.Fatal(err)
	}
	code, body := f.request(t, f.first.APIToken.Token, messageKey, messageBody, "")
	if code != 201 || messageID(t, body) != id {
		t.Fatal(code, string(body))
	}
	before := f.state(t)
	// 模拟调用方丢弃首个成功回执，并完全关闭 HTTP 与 SQLite 后重新启动。
	f.server.Close()
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	f.open(t)
	for _, instance := range []string{"", "malformed", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"} {
		code, replay := f.request(t, f.first.APIToken.Token, strings.ToUpper(messageKey), `{"format":"markdown","text":" 正文 α\n","attachments":[],"buttons":[],"reply_to_message_id":null}`, instance)
		if code != 201 || string(replay) != string(body) {
			t.Fatalf("reopen replay %d %s", code, replay)
		}
	}
	if !reflect.DeepEqual(before, f.state(t)) {
		t.Fatal("replay created side effects")
	}
	rotated, _, err := f.bots.RotateAPIToken(context.Background(), f.first.Bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ = f.request(t, f.first.APIToken.Token, messageKey, messageBody, ""); code != 401 {
		t.Fatal("old token", code)
	}
	if code, replay := f.request(t, rotated.Token, messageKey, messageBody, ""); code != 201 || string(replay) != string(body) {
		t.Fatal("reset changed identity", code, string(replay))
	}
	if code, other := f.request(t, f.second.APIToken.Token, messageKey, messageBody, ""); code != 201 || messageID(t, other) == id || strings.Contains(string(other), f.first.Bot.ID) {
		t.Fatal("cross bot leak", code, string(other))
	}
	caps := bots.Capabilities{Send: false, Receive: true}
	b, err := f.bots.Get(context.Background(), f.first.Bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.bots.Patch(context.Background(), b.ID, bots.PatchRequest{ExpectedVersion: b.Version, Capabilities: &caps})
	if err != nil {
		t.Fatal(err)
	}
	if code, _ = f.request(t, rotated.Token, messageKey, messageBody, ""); code != 403 {
		t.Fatal("capability checked after replay", code)
	}
}

func TestMessageHTTPConcurrentKeys(t *testing.T) {
	f := setupMessageHTTP(t)
	before := f.state(t)
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	bodies := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, body := f.request(t, f.first.APIToken.Token, messageKey, messageBody, "")
			codes <- code
			bodies <- string(body)
		}()
	}
	wg.Wait()
	close(codes)
	close(bodies)
	var original string
	for code := range codes {
		if code != 201 {
			t.Fatal(code)
		}
	}
	for body := range bodies {
		if original == "" {
			original = body
		}
		if original != body {
			t.Fatal("concurrent duplicate result")
		}
	}
	after := f.state(t)
	if after[0] != before[0]+1 || after[1] != before[1]+1 || after[2] != before[2]+1 || after[3] != before[3] || after[4] != before[4] || after[5] != before[5]+1 {
		t.Fatal("duplicate side effects", before, after)
	}
	conflict := make(chan int, 2)
	for _, text := range []string{"a", "b"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			code, _ := f.request(t, f.first.APIToken.Token, "00000000-0000-4000-8000-000000000001", `{"format":"plain","text":"`+text+`"}`, "")
			conflict <- code
		}(text)
	}
	wg.Wait()
	a, b := <-conflict, <-conflict
	if !((a == 201 && b == 409) || (a == 409 && b == 201)) {
		t.Fatal(a, b)
	}
}

func TestMessageHTTPCommitRollback(t *testing.T) {
	f := setupMessageHTTP(t)
	before := f.state(t)
	f.exec(t, `CREATE TABLE fault_parent(id PRIMARY KEY);CREATE TABLE fault_child(id REFERENCES fault_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN INSERT INTO fault_child VALUES(1);END`)
	code, body := f.request(t, f.first.APIToken.Token, messageKey, messageBody, "")
	if code != 503 || strings.Contains(string(body), "正文") || strings.Contains(string(body), "FOREIGN KEY") {
		t.Fatal(code, string(body))
	}
	if !reflect.DeepEqual(before, f.state(t)) {
		t.Fatal("COMMIT failure leaked writes", before, f.state(t))
	}
	f.exec(t, "DROP TRIGGER fail")
	if code, _ = f.request(t, f.first.APIToken.Token, messageKey, messageBody, ""); code != 201 {
		t.Fatal(code)
	}
}

func TestMessageHTTPValidation(t *testing.T) {
	f := setupMessageHTTP(t)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"format":"plain","text":"ok","bot_id":"x"}`, 400},
		{`{"Format":"plain","text":"ok"}`, 400},
		{`{"format":"plain","text":5}`, 400},
		{`{"format":"plain","text":"ok"} {}`, 400},
		{`{"format":null,"text":"ok"}`, 422},
		{`{"text":"ok"}`, 422},
		{`{"format":"plain"}`, 422},
		{`{"format":"plain","text":"ok","attachments":null}`, 422},
		{`{"format":"plain","text":"ok","attachments":[{"id":"x","kind":"file","secret":1}]}`, 400},
		{`{"format":"plain","text":"ok","attachments":[{"id":"x","kind":"file"}]}`, 422},
		{`{"format":"plain","text":"ok","reply_to_message_id":"other"}`, 422},
		{`{"format":"plain","text":"ok","buttons":[[{"id":"a","label":"b","url":"https://example.com"}]]}`, 422},
		{`{"format":"plain","text":"` + strings.Repeat("a", 65537) + `"}`, 413},
		{`{"format":"plain","text":"` + strings.Repeat("a", 262144) + `"}`, 413},
	} {
		t.Run(strconvStatus(tc.status)+"/"+tc.body[:min(len(tc.body), 50)], func(t *testing.T) {
			code, body := f.request(t, f.first.APIToken.Token, messageKey, tc.body, "")
			if code != tc.status {
				t.Fatalf("got %d %s", code, body)
			}
		})
	}
	if code, _ := f.request(t, f.first.WebhookKey.Key, messageKey, messageBody, ""); code != 401 {
		t.Fatal("webhook accepted", code)
	}
	if code, _ := f.request(t, f.first.APIToken.Token, "not-uuid", messageBody, ""); code != 400 {
		t.Fatal("invalid key", code)
	}
	if f.state(t)[1] != 0 {
		t.Fatal("invalid requests created messages")
	}
}
func strconvStatus(n int) string {
	if n == 400 {
		return "structure"
	}
	if n == 413 {
		return "limit"
	}
	return "semantic"
}

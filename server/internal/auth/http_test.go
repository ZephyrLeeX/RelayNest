package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

// 生命周期通过真实 TLS HTTP 和 SQLite 驱动；固定时钟只用于期限边界。
func TestHTTPLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http.sqlite")
	var db *storage.Store
	var service *SessionStore
	var ts *httptest.Server
	var clock atomic.Int64
	now := time.Now().UTC()
	clock.Store(now.UnixNano())
	start := func() {
		var err error
		db, err = storage.Open(context.Background(), storage.Config{Path: path})
		if err != nil {
			t.Fatal(err)
		}
		service = NewSessionStore(db)
		service.now = func() time.Time { return time.Unix(0, clock.Load()).UTC() }
		ts = httptest.NewUnstartedServer(nil)
		ts.StartTLS()
		policy, err := httpapi.NewOriginPolicy(ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		ts.Config.Handler = httpapi.New(health.New(nil), NewHandler(db, service, policy))
	}
	start()
	defer func() { ts.Close(); _ = db.Close() }()
	if err := InitAdministrator(context.Background(), db, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, token, client, csrf, origin string) (*http.Response, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(instanceHeader, testInstanceID(t, db))
		if token != "" {
			if client == "web" {
				req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
			} else {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("no-store")
		}
		var out map[string]any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}
		}
		for _, secret := range []string{"password", "generation", "hash"} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("secret metadata")
			}
		}
		return resp, out
	}
	login := func(client string) (string, string) {
		t.Helper()
		body := `{"username":"Admin","password":"correct","client_type":"` + client + `","device_name":"fixture"`
		origin := ""
		if client == "android" {
			body += `,"device_id":"11111111-1111-4111-8111-111111111111"`
		} else {
			origin = ts.URL
		}
		body += `}`
		resp, out := request("POST", "/api/v1/auth/login", body, "", "", "", origin)
		if resp.StatusCode != 200 {
			t.Fatalf("login %d %v", resp.StatusCode, out)
		}
		count := 7
		if client == "android" {
			count = 8
			if out["session"].(map[string]any)["instance_id"] != testInstanceID(t, db) {
				t.Fatal("Android instance DTO")
			}
		}
		if len(out["session"].(map[string]any)) != count {
			t.Fatal("session DTO")
		}
		if client == "web" {
			cookies := resp.Cookies()
			if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != "/api/v1" || cookies[0].SameSite != http.SameSiteLaxMode || len(out) != 2 {
				t.Fatal("web DTO")
			}
			return cookies[0].Value, out["csrf_token"].(string)
		}
		if len(resp.Cookies()) != 0 || len(out) != 3 || out["token_type"] != "Bearer" {
			t.Fatal("android DTO")
		}
		return out["access_token"].(string), ""
	}
	web, csrf := login("web")
	android, _ := login("android")
	for _, tc := range []struct{ token, client string }{{web, "android"}, {android, "web"}, {"bot-secret", "android"}, {"invalid", "web"}} {
		resp, _ := request("GET", "/api/v1/auth/session", "", tc.token, tc.client, "", "")
		if resp.StatusCode != 401 {
			t.Fatal("channel accepted")
		}
	}
	now = now.Add(time.Hour)
	clock.Store(now.UnixNano())
	for _, tc := range []struct{ token, client string }{{web, "web"}, {android, "android"}} {
		resp, out := request("GET", "/api/v1/auth/session", "", tc.token, tc.client, "", "")
		if resp.StatusCode != 200 || out["access_token"] != nil || len(resp.Cookies()) > 0 {
			t.Fatal("get DTO")
		}
	}
	renewed, _, err := service.Inspect(context.Background(), web, "web")
	if err != nil || !renewed.IdleExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatal("web renewal", renewed, err)
	}
	before, _, _ := service.Inspect(context.Background(), web, "web")
	for _, tc := range []struct{ origin, csrf string }{{ts.URL, "bad"}, {"https://cross.invalid", csrf}, {"", csrf}, {ts.URL + ":443", csrf}} {
		now = now.Add(time.Minute)
		clock.Store(now.UnixNano())
		resp, _ := request("POST", "/api/v1/auth/logout", "", web, "web", tc.csrf, tc.origin)
		if resp.StatusCode != 403 || len(resp.Cookies()) > 0 {
			t.Fatal("unsafe logout")
		}
	}
	after, _, _ := service.Inspect(context.Background(), web, "web")
	if !before.IdleExpiresAt.Equal(after.IdleExpiresAt) {
		t.Fatal("failure renewed")
	}
	ts.Close()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	start()
	for _, tc := range []struct{ token, client string }{{web, "web"}, {android, "android"}} {
		resp, _ := request("GET", "/api/v1/auth/session", "", tc.token, tc.client, "", "")
		if resp.StatusCode != 200 {
			t.Fatal("restart")
		}
	}
	resp, out := request("POST", "/api/v1/auth/logout", "", web, "web", csrf, ts.URL)
	if resp.StatusCode != 204 || out != nil || !strings.Contains(resp.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatal("logout wire")
	}
	resp, _ = request("GET", "/api/v1/auth/session", "", web, "web", "", "")
	if resp.StatusCode != 401 {
		t.Fatal("revoked")
	}
	resp, _ = request("GET", "/api/v1/auth/session", "", android, "android", "", "")
	if resp.StatusCode != 200 {
		t.Fatal("device isolation")
	}
	now = now.Add(30 * 24 * time.Hour)
	clock.Store(now.UnixNano())
	resp, _ = request("GET", "/api/v1/auth/session", "", android, "android", "", "")
	if resp.StatusCode != 401 {
		t.Fatal("idle expiry")
	}
	android, _ = login("android")
	for i := 0; i < 4; i++ {
		now = now.Add(25 * 24 * time.Hour)
		clock.Store(now.UnixNano())
		resp, _ = request("GET", "/api/v1/auth/session", "", android, "android", "", "")
		if (i < 3 && resp.StatusCode != 200) || (i == 3 && resp.StatusCode != 401) {
			t.Fatal("absolute expiry")
		}
	}
	web, csrf = login("web")
	android, _ = login("android")
	if err := ResetPassword(context.Background(), db, []byte("changed")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ token, client string }{{web, "web"}, {android, "android"}} {
		resp, _ := request("GET", "/api/v1/auth/session", "", tc.token, tc.client, "", "")
		if resp.StatusCode != 401 {
			t.Fatal("reset accepted")
		}
	}
}

func TestHTTPSourceAndInput(t *testing.T) {
	db := testStore(t, filepath.Join(t.TempDir(), "source.sqlite"))
	if err := InitAdministrator(context.Background(), db, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.NewOriginPolicy("https://nas.example", []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(health.New(nil), NewHandler(db, NewSessionStore(db), policy))
	body := `{"username":"Admin","password":"correct","client_type":"web","device_name":"browser"}`
	for _, tc := range []struct {
		name, body, origin, proto, host, forwarded string
		tls                                        bool
		status                                     int
	}{
		{name: "proxy", body: body, origin: "https://nas.example", proto: "https", host: "nas.example", status: 200},
		{name: "direct", body: body, origin: "https://nas.example", tls: true, status: 200},
		{name: "missing", body: body, tls: true, status: 403},
		{name: "cross", body: body, origin: "https://evil.example", tls: true, status: 403},
		{name: "port", body: body, origin: "https://nas.example:8443", tls: true, status: 403},
		{name: "forwarded", body: body, origin: "https://nas.example", forwarded: "proto=https", tls: true, status: 403},
		{name: "partial", body: body, origin: "https://nas.example", proto: "https", status: 403},
		{name: "conflict", body: body, origin: "https://nas.example", proto: "http", host: "nas.example", tls: true, status: 403},
		{name: "unknown", body: strings.TrimSuffix(body, "}") + `,"unknown":1}`, tls: true, status: 400},
		{name: "webDeviceNull", body: strings.TrimSuffix(body, "}") + `,"device_id":null}`, tls: true, status: 422},
		{name: "oversize", body: `{"password":"` + strings.Repeat("a", 256<<10) + `"}`, tls: true, status: 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "https://nas.example/api/v1/auth/login", strings.NewReader(tc.body))
			req.Header.Set(instanceHeader, testInstanceID(t, db))
			req.RemoteAddr = "127.0.0.1:1234"
			req.TLS = nil
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			req.Header.Set("Content-Type", "application/json")
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.host != "" {
				req.Header.Set("X-Forwarded-Host", tc.host)
				req.Header.Set("X-Forwarded-For", "192.0.2.1")
			}
			if tc.forwarded != "" {
				req.Header.Set("Forwarded", tc.forwarded)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("%d %s", rr.Code, rr.Body.String())
			}
			if tc.status == 200 && !strings.Contains(rr.Header().Get("Set-Cookie"), "Secure") {
				t.Fatal("insecure cookie")
			}
			if tc.status != 200 && rr.Header().Get("Set-Cookie") != "" {
				t.Fatal("failure cookie")
			}
		})
	}
	var count int
	if err := db.Read(context.Background(), func(tx *sql.Tx) error { return tx.QueryRow("SELECT count(*) FROM sessions").Scan(&count) }); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("failure created session %d", count)
	}
}

// HTTP 夹具只注入正式 handler 与真实 SQLite，不依赖进程 main 的未提交接线。
type httpFixture struct {
	instanceID string
	t          *testing.T
	db         *storage.Store
	sessions   *SessionStore
	server     *httptest.Server
	clock      atomic.Int64
}

func newHTTPFixture(t *testing.T, initialized bool) *httpFixture {
	f := &httpFixture{t: t, db: testStore(t, filepath.Join(t.TempDir(), "auth.sqlite"))}
	f.instanceID = testInstanceID(t, f.db)
	f.clock.Store(time.Now().UTC().UnixNano())
	f.sessions = NewSessionStore(f.db)
	f.sessions.now = func() time.Time { return time.Unix(0, f.clock.Load()).UTC() }
	var handler http.Handler
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(f.server.Close)
	policy, err := httpapi.NewOriginPolicy(f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler = httpapi.New(health.New(nil), NewHandler(f.db, f.sessions, policy))
	if initialized {
		if err := InitAdministrator(context.Background(), f.db, "Admin", []byte("correct")); err != nil {
			t.Fatal(err)
		}
	}
	return f
}
func (f *httpFixture) request(method, path, body string, headers http.Header) (*http.Response, map[string]any) {
	f.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(instanceHeader, f.instanceID)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		f.t.Fatal(err)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		f.t.Fatal("missing no-store")
	}
	var out map[string]any
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &out); err != nil {
			f.t.Fatal(err)
		}
	}
	if resp.StatusCode == 204 && len(raw) != 0 {
		f.t.Fatal("204 body")
	}
	return resp, out
}

const webLogin = `{"username":"Admin","password":"correct","client_type":"web","device_name":"browser"}`

func (f *httpFixture) login(client string) (string, string) {
	body := webLogin
	h := http.Header{"Origin": {f.server.URL}}
	if client == "android" {
		body = strings.Replace(body, `"web"`, `"android"`, 1)
		body = strings.TrimSuffix(body, "}") + `,"device_id":"11111111-1111-4111-8111-111111111111"}`
	}
	resp, out := f.request("POST", "/api/v1/auth/login", body, h)
	if resp.StatusCode != 200 {
		f.t.Fatalf("login %d %v", resp.StatusCode, out)
	}
	if client == "web" {
		return resp.Cookies()[0].Value, out["csrf_token"].(string)
	}
	return out["access_token"].(string), ""
}
func (f *httpFixture) snapshot() string {
	f.t.Helper()
	var result string
	err := f.db.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT id,idle_expires_at,absolute_expires_at,revoked FROM sessions ORDER BY id")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var idle, absolute int64
			var revoked sql.NullInt64
			if err := rows.Scan(&id, &idle, &absolute, &revoked); err != nil {
				return err
			}
			result += fmt.Sprint(id, idle, absolute, revoked)
		}
		return rows.Err()
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return result
}
func (f *httpFixture) reject(method, path, body string, h http.Header, status int, code string) {
	f.t.Helper()
	before := f.snapshot()
	f.clock.Add(int64(time.Minute))
	resp, out := f.request(method, path, body, h)
	if resp.StatusCode != status || out["error"].(map[string]any)["code"] != code || len(resp.Header.Values("Set-Cookie")) != 0 || f.snapshot() != before {
		f.t.Fatalf("rejection side effect/status: %d %v", resp.StatusCode, out)
	}
}

func TestHTTPRejectionsHaveNoEffects(t *testing.T) {
	f := newHTTPFixture(t, false)
	origin := http.Header{"Origin": {f.server.URL}}
	f.reject("POST", "/api/v1/auth/login", webLogin, origin, 401, "AUTHENTICATION_FAILED")
	if err := InitAdministrator(context.Background(), f.db, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{strings.Replace(webLogin, "Admin", "wrong", 1), strings.Replace(webLogin, "correct", "wrong", 1)} {
		f.reject("POST", "/api/v1/auth/login", body, origin, 401, "AUTHENTICATION_FAILED")
	}
	web, csrf := f.login("web")
	android, _ := f.login("android")
	for _, h := range []http.Header{
		{"Cookie": {sessionCookie + "=" + web}, "Authorization": {"Bearer " + android}},
		{"Cookie": {sessionCookie + "=" + web + "; " + sessionCookie + "=" + web}},
		{"Authorization": {"Bearer " + android, "Bearer " + android}},
		{"Authorization": {"Bearer " + web}}, {"Cookie": {sessionCookie + "=" + android}},
		{"Authorization": {"Bearer bot-secret"}}, {"Authorization": {"Basic x"}}, {},
	} {
		h.Set("Origin", f.server.URL)
		for _, path := range []string{"session", "logout"} {
			method := "GET"
			if path == "logout" {
				method = "POST"
			}
			f.reject(method, "/api/v1/auth/"+path, "", h, 401, "AUTHENTICATION_FAILED")
		}
	}
	for _, h := range []http.Header{
		{"Origin": {f.server.URL}, "X-CSRF-Token": {"bad"}}, {"Origin": {f.server.URL}, "X-CSRF-Token": {csrf, csrf}},
		{"Origin": {"https://evil.invalid"}, "X-CSRF-Token": {csrf}}, {"X-CSRF-Token": {csrf}},
		{"Referer": {"https://evil.invalid/path"}, "X-CSRF-Token": {csrf}}, {"Referer": {f.server.URL + "/path", f.server.URL + "/other"}, "X-CSRF-Token": {csrf}},
		{"Origin": {f.server.URL}, "Referer": {"https://evil.invalid/path"}, "X-CSRF-Token": {csrf}},
	} {
		h.Set("Cookie", sessionCookie+"="+web)
		f.reject("POST", "/api/v1/auth/logout", "", h, 403, "FORBIDDEN")
	}
	f.reject("POST", "/api/v1/auth/login", webLogin, http.Header{"Referer": {f.server.URL + "/login"}}, 403, "FORBIDDEN")
	resp, _ := f.request("POST", "/api/v1/auth/logout", "", http.Header{"Cookie": {sessionCookie + "=" + web}, "X-CSRF-Token": {csrf}, "Referer": {f.server.URL + "/settings?q=x"}})
	if resp.StatusCode != 204 {
		t.Fatal("Referer fallback", resp.StatusCode)
	}
	f.reject("GET", "/api/v1/auth/session", "", http.Header{"Cookie": {sessionCookie + "=" + web}}, 401, "AUTHENTICATION_FAILED")
	if err := ResetPassword(context.Background(), f.db, []byte("changed")); err != nil {
		t.Fatal(err)
	}
	f.reject("GET", "/api/v1/auth/session", "", http.Header{"Authorization": {"Bearer " + android}}, 401, "AUTHENTICATION_FAILED")
}

func TestHTTPClientDeadlinesAndLogout(t *testing.T) {
	for _, client := range []string{"web", "android"} {
		t.Run(client, func(t *testing.T) {
			f := newHTTPFixture(t, true)
			token, csrf := f.login(client)
			other, _ := f.login(client)
			headers := func(token string) http.Header {
				if client == "web" {
					return http.Header{"Cookie": {sessionCookie + "=" + token}, "Origin": {f.server.URL}, "X-CSRF-Token": {csrf}}
				}
				return http.Header{"Authorization": {"Bearer " + token}}
			}
			idle, absolute := sessionTTL(client)
			created := time.Unix(0, f.clock.Load()).UTC()
			f.clock.Add(int64(time.Hour))
			resp, out := f.request("GET", "/api/v1/auth/session", "", headers(token))
			if resp.StatusCode != 200 {
				t.Fatal(resp.StatusCode)
			}
			session := out["session"].(map[string]any)
			if session["idle_expires_at"] != created.Add(time.Hour+idle).Format(time.RFC3339Nano) || session["absolute_expires_at"] != created.Add(absolute).Format(time.RFC3339Nano) {
				t.Fatal("renewal/absolute", session)
			}
			resp, _ = f.request("POST", "/api/v1/auth/logout", "", headers(token))
			if resp.StatusCode != 204 {
				t.Fatal(resp.StatusCode)
			}
			if client == "android" && len(resp.Cookies()) != 0 {
				t.Fatal("android cookie")
			}
			f.reject("GET", "/api/v1/auth/session", "", headers(token), 401, "AUTHENTICATION_FAILED")
			resp, _ = f.request("GET", "/api/v1/auth/session", "", headers(other))
			if resp.StatusCode != 200 {
				t.Fatal("other device revoked")
			}
			f.clock.Add(int64(idle))
			f.reject("GET", "/api/v1/auth/session", "", headers(other), 401, "AUTHENTICATION_FAILED")
			token, _ = f.login(client)
			deadline := time.Unix(0, f.clock.Load()).Add(absolute)
			step := idle / 2
			for time.Unix(0, f.clock.Load()).Add(step).Before(deadline) {
				f.clock.Add(int64(step))
				resp, _ = f.request("GET", "/api/v1/auth/session", "", headers(token))
				if resp.StatusCode != 200 {
					t.Fatal("premature absolute", resp.StatusCode)
				}
			}
			f.clock.Store(deadline.UnixNano())
			f.reject("GET", "/api/v1/auth/session", "", headers(token), 401, "AUTHENTICATION_FAILED")
		})
	}
}

// 在密码校验已结束、Create 取随机数时屏障，不新增生产安全接口。
type loginGate struct {
	once             sync.Once
	entered, release chan struct{}
}

func (g *loginGate) Read(p []byte) (int, error) {
	g.once.Do(func() { close(g.entered); <-g.release })
	return rand.Read(p)
}
func TestHTTPResetLoginOrdering(t *testing.T) {
	for _, resetFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(resetFirst), func(t *testing.T) {
			f := newHTTPFixture(t, true)
			gate := &loginGate{entered: make(chan struct{}), release: make(chan struct{})}
			f.sessions.random = gate
			created, finish := make(chan struct{}), make(chan struct{})
			if !resetFirst {
				var once sync.Once
				original := f.server.Config.Handler
				f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					original.ServeHTTP(w, r)
					if r.URL.Path == "/api/v1/auth/login" {
						once.Do(func() { close(created); <-finish })
					}
				})
			}
			type result struct {
				resp *http.Response
				out  map[string]any
			}
			done := make(chan result, 1)
			go func() {
				resp, out := f.request("POST", "/api/v1/auth/login", webLogin, http.Header{"Origin": {f.server.URL}})
				done <- result{resp, out}
			}()
			<-gate.entered
			before := f.snapshot()
			if resetFirst {
				if err := ResetPassword(context.Background(), f.db, []byte("changed")); err != nil {
					t.Fatal(err)
				}
			}
			close(gate.release)
			if !resetFirst {
				<-created
				if err := ResetPassword(context.Background(), f.db, []byte("changed")); err != nil {
					t.Fatal(err)
				}
				close(finish)
			}
			got := <-done
			if !resetFirst {
				if got.resp.StatusCode != 200 {
					t.Fatal(got.resp.StatusCode)
				}
				f.reject("GET", "/api/v1/auth/session", "", http.Header{"Cookie": {sessionCookie + "=" + got.resp.Cookies()[0].Value}}, 401, "AUTHENTICATION_FAILED")
			} else if got.resp.StatusCode != 401 || len(got.resp.Cookies()) != 0 || f.snapshot() != before {
				t.Fatal("old generation created session", got.resp.StatusCode, got.out)
			}
			f.reject("POST", "/api/v1/auth/login", webLogin, http.Header{"Origin": {f.server.URL}}, 401, "AUTHENTICATION_FAILED")
		})
	}
}

func TestHTTPRealSourceComposition(t *testing.T) {
	for _, tc := range []struct {
		name           string
		trusted, plain bool
		headers        http.Header
		status         int
	}{
		{"direct", false, false, nil, 200},
		{"untrusted spoof cannot establish HTTPS", false, true, http.Header{"X-Forwarded-Proto": {"https"}, "X-Forwarded-Host": {"configured"}, "X-Forwarded-For": {"192.0.2.1"}}, 403},
		{"trusted HTTPS termination", true, true, http.Header{"X-Forwarded-Proto": {"https"}, "X-Forwarded-Host": {"configured"}, "X-Forwarded-For": {"192.0.2.1"}}, 200},
		{"duplicate proto", true, false, http.Header{"X-Forwarded-Proto": {"https", "https"}, "X-Forwarded-Host": {"configured"}, "X-Forwarded-For": {"192.0.2.1"}}, 403},
		{"malformed for", true, false, http.Header{"X-Forwarded-Proto": {"https"}, "X-Forwarded-Host": {"configured"}, "X-Forwarded-For": {"bad"}}, 403},
		{"conflicting forwarded", true, false, http.Header{"Forwarded": {"proto=https"}, "X-Forwarded-Proto": {"https"}, "X-Forwarded-Host": {"configured"}, "X-Forwarded-For": {"192.0.2.1"}}, 403},
		{"missing origin", false, false, http.Header{"Origin": {}}, 403}, {"duplicate origin", false, false, http.Header{"Origin": {"configured", "configured"}}, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHTTPFixture(t, true)
			if tc.plain {
				f.server.Close()
				f.server = httptest.NewServer(nil)
				t.Cleanup(f.server.Close)
			}
			origin := "https://" + strings.TrimPrefix(strings.TrimPrefix(f.server.URL, "https://"), "http://")
			proxies := []string(nil)
			if tc.trusted {
				proxies = []string{"127.0.0.1", "::1"}
			}
			policy, err := httpapi.NewOriginPolicy(origin, proxies)
			if err != nil {
				t.Fatal(err)
			}
			f.server.Config.Handler = httpapi.New(health.New(nil), NewHandler(f.db, f.sessions, policy))
			headers := tc.headers.Clone()
			if headers == nil {
				headers = make(http.Header)
			}
			if _, ok := headers["Origin"]; !ok {
				headers.Set("Origin", origin)
			}
			for key, values := range headers {
				for i, v := range values {
					if v == "configured" {
						if key == "Origin" {
							values[i] = origin
						} else {
							values[i] = strings.TrimPrefix(origin, "https://")
						}
					}
				}
			}
			if tc.status != 200 {
				f.reject("POST", "/api/v1/auth/login", webLogin, headers, tc.status, "FORBIDDEN")
				return
			}
			resp, _ := f.request("POST", "/api/v1/auth/login", webLogin, headers)
			if resp.StatusCode != 200 || len(resp.Cookies()) != 1 || !resp.Cookies()[0].Secure {
				t.Fatal("secure source", resp.StatusCode)
			}
		})
	}
}

func TestHTTPInputRejectionsHaveNoEffects(t *testing.T) {
	f := newHTTPFixture(t, true)
	f.login("web")
	headers := http.Header{"Origin": {f.server.URL}}
	for _, tc := range []struct {
		body   string
		status int
		code   string
	}{
		{`{"username":12}`, 400, "INVALID_REQUEST"},
		{strings.TrimSuffix(webLogin, "}") + `,"unknown":1}`, 400, "INVALID_REQUEST"},
		{webLogin + ` {}`, 400, "INVALID_REQUEST"}, {`{`, 400, "INVALID_REQUEST"},
		{strings.Replace(webLogin, `"browser"`, `""`, 1), 422, "VALIDATION_FAILED"},
		{strings.Replace(webLogin, `"web"`, `"android"`, 1), 422, "VALIDATION_FAILED"},
		{strings.TrimSuffix(strings.Replace(webLogin, `"web"`, `"android"`, 1), "}") + `,"device_id":123}`, 400, "INVALID_REQUEST"},
		{strings.TrimSuffix(webLogin, "}") + `,"device_id":null}`, 422, "VALIDATION_FAILED"},
		{`{"password":"` + strings.Repeat("x", 256<<10) + `"}`, 413, "PAYLOAD_TOO_LARGE"},
	} {
		f.reject("POST", "/api/v1/auth/login", tc.body, headers, tc.status, tc.code)
	}
}

func TestHTTPLoginExactFieldNames(t *testing.T) {
	f := newHTTPFixture(t, true)
	f.login("web")
	f.login("android")
	headers := http.Header{"Origin": {f.server.URL}}
	android := strings.TrimSuffix(strings.Replace(webLogin, `"web"`, `"android"`, 1), "}") + `,"device_id":"11111111-1111-4111-8111-111111111111"}`
	for _, body := range []string{webLogin, android} {
		for _, field := range []string{"username", "password", "client_type", "device_name", "device_id"} {
			if !strings.Contains(body, `"`+field+`"`) {
				continue
			}
			for _, alias := range []string{strings.ToUpper(field[:1]) + field[1:], strings.ToUpper(field)} {
				f.reject("POST", "/api/v1/auth/login", strings.Replace(body, `"`+field+`"`, `"`+alias+`"`, 1), headers, 400, "INVALID_REQUEST")
				f.reject("POST", "/api/v1/auth/login", strings.TrimSuffix(body, "}")+`,"`+alias+`":"extra"}`, headers, 400, "INVALID_REQUEST")
			}
		}
	}
}

func TestHTTPLoginDTOContract(t *testing.T) {
	f := newHTTPFixture(t, true)
	f.login("web")
	f.login("android")
	headers := http.Header{"Origin": {f.server.URL}}
	for _, body := range []string{`[]`, `"text"`, `42`, `true`, `{`, webLogin + ` []`, webLogin + ` null`, webLogin + ` true`, webLogin + ` garbage`, strings.TrimSuffix(webLogin, "}") + `,"unknown":null}`, strings.TrimSuffix(webLogin, "}") + `,"username":"Admin","Username":"Admin"}`, `{"username":12,"username":"Admin"}`} {
		f.reject("POST", "/api/v1/auth/login", body, headers, 400, "INVALID_REQUEST")
	}
	f.reject("POST", "/api/v1/auth/login", `null`, headers, 422, "VALIDATION_FAILED")
	for _, client := range []string{"web", "android"} {
		body := strings.Replace(webLogin, `"web"`, `"`+client+`"`, 1)
		for _, value := range []string{`12`, `true`, `[]`, `{}`} {
			f.reject("POST", "/api/v1/auth/login", strings.TrimSuffix(body, "}")+`,"device_id":`+value+`}`, headers, 400, "INVALID_REQUEST")
		}
		for _, value := range []string{`null`, `""`, `"invalid"`} {
			f.reject("POST", "/api/v1/auth/login", strings.TrimSuffix(body, "}")+`,"device_id":`+value+`}`, headers, 422, "VALIDATION_FAILED")
		}
	}
	f.reject("POST", "/api/v1/auth/login", strings.TrimSuffix(webLogin, "}")+`,"device_id":"11111111-1111-4111-8111-111111111111"}`, headers, 422, "VALIDATION_FAILED")
	for _, field := range []string{"username", "password", "client_type", "device_name"} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(webLogin), &fields); err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{`12`, `true`, `[]`, `{}`} {
			fields[field] = json.RawMessage(value)
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			f.reject("POST", "/api/v1/auth/login", string(body), headers, 400, "INVALID_REQUEST")
		}
		for _, value := range []string{`""`, `null`, ""} {
			if value == "" {
				delete(fields, field)
			} else {
				fields[field] = json.RawMessage(value)
			}
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			f.reject("POST", "/api/v1/auth/login", string(body), headers, 422, "VALIDATION_FAILED")
		}
	}
	// 冻结契约未禁止重复精确键，保留 encoding/json 的后值覆盖语义。
	resp, out := f.request("POST", "/api/v1/auth/login", strings.TrimSuffix(webLogin, "}")+`,"username":"Admin"}`, headers)
	if resp.StatusCode != 200 || len(resp.Header.Values("Set-Cookie")) != 1 || out["session"] == nil {
		t.Fatalf("duplicate exact key login: %d %v", resp.StatusCode, out)
	}
}

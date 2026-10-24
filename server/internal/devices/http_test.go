package devices_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/devices"
	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

type deviceHTTP = devices.Handler
type management struct {
	*auth.Handler
	*deviceHTTP
}
type fixture struct {
	store            *storage.Store
	sessions         *auth.SessionStore
	mux              http.Handler
	web, android     auth.SessionSecrets
	webID, androidID string
	hooks            []string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Config{Path: filepath.Join(t.TempDir(), "devices.sqlite")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	var sqliteVersion string
	if err := store.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion) }); err != nil {
		t.Fatal(err)
	}
	t.Logf("SQLite %s（临时文件数据库）", sqliteVersion)
	if err := auth.InitAdministrator(ctx, store, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	admin, err := auth.ReadAdministrator(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{store: store, sessions: auth.NewSessionStore(store)}
	for _, client := range []string{"web", "android"} {
		in := auth.SessionInput{ClientType: client, DeviceName: "测试设备"}
		if client == "android" {
			in.DeviceID = "00000000-0000-4000-8000-000000000001"
		}
		s, secrets, err := f.sessions.Create(ctx, admin.AuthGeneration, in)
		if err != nil {
			t.Fatal(err)
		}
		if client == "web" {
			f.web = secrets
			f.webID = s.ID
		} else {
			f.android = secrets
			f.androidID = s.ID
		}
	}
	policy, err := httpapi.NewOriginPolicy("https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	security := auth.NewRequestSecurity(f.sessions, policy)
	f.mux = httpapi.New(health.New(nil), management{auth.NewHandler(store, f.sessions, policy), devices.NewHandler(f.sessions, security)})
	// 回调再次读取真实库，只有已提交的精确目标才能通知消费者。
	f.sessions.SetRevocationHook(func(id string) {
		var revoked int
		err := store.Read(ctx, func(tx *sql.Tx) error {
			return tx.QueryRow("SELECT revoked FROM sessions WHERE id=?", id).Scan(&revoked)
		})
		if err != nil || revoked != 1 {
			t.Error("hook在提交前运行或目标不正确")
		}
		f.hooks = append(f.hooks, id)
	})
	return f
}
func (f *fixture) request(method, path, body, client, token string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://example.com"+path, strings.NewReader(body))
	if token != "" {
		if client == "web" {
			r.AddCookie(&http.Cookie{Name: "relaynest_session", Value: token})
		} else {
			r.Header.Set("Authorization", "Bearer "+token)
		}
	}
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	return w
}
func (f *fixture) snapshot(t *testing.T) string {
	t.Helper()
	var rowsState []string
	err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT id,revoked,idle_expires_at FROM sessions ORDER BY id")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var revoked, expiry int64
			if err := rows.Scan(&id, &revoked, &expiry); err != nil {
				return err
			}
			rowsState = append(rowsState, fmt.Sprint(id, revoked, expiry))
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(rowsState, ";")
}
func assertResponse(t *testing.T, w *httptest.ResponseRecorder, want int, code string) {
	t.Helper()
	if w.Code != want || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status/cache=%d/%s want=%d", w.Code, w.Header().Get("Cache-Control"), want)
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatal("非自撤分支设置Cookie")
	}
	if code != "" {
		var p struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(w.Body.Bytes(), &p) != nil || p.Error.Code != code {
			t.Fatalf("错误码不匹配: %s", w.Body.String())
		}
	}
}

func TestDeviceHTTPRejectedRequestsDoNotMutate(t *testing.T) {
	f := setup(t)
	target := "/api/v1/devices/" + f.androidID + "/session"
	valid := map[string]string{"Origin": "https://example.com", "X-CSRF-Token": f.web.CSRFToken}
	for _, tc := range []struct {
		name, method, path, body, client, token string
		headers                                 map[string]string
		want                                    int
		code                                    string
	}{
		{"missing", "GET", "/api/v1/devices", "", "", "", nil, 401, "AUTHENTICATION_FAILED"},
		{"bearer-random", "DELETE", target, "", "android", "not-a-session", nil, 401, "AUTHENTICATION_REQUIRED"},
		{"origin", "DELETE", target, "", "web", f.web.Token, map[string]string{"Origin": "https://evil.example", "X-CSRF-Token": f.web.CSRFToken}, 403, "FORBIDDEN"},
		{"csrf", "DELETE", target, "", "web", f.web.Token, map[string]string{"Origin": "https://example.com"}, 403, "FORBIDDEN"},
		{"body", "DELETE", target, "{}", "web", f.web.Token, valid, 400, "INVALID_REQUEST"},
		{"whitespace-body", "DELETE", target, " ", "web", f.web.Token, valid, 400, "INVALID_REQUEST"},
		{"bad-id", "DELETE", "/api/v1/devices/bad/session", "", "web", f.web.Token, valid, 400, "INVALID_REQUEST"},
		{"unknown", "DELETE", "/api/v1/devices/00000000-0000-4000-8000-000000000099/session", "", "web", f.web.Token, valid, 404, "RESOURCE_NOT_FOUND"},
		{"install-is-not-id", "DELETE", "/api/v1/devices/00000000-0000-4000-8000-000000000001/session", "", "web", f.web.Token, valid, 404, "RESOURCE_NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.snapshot(t)
			w := f.request(tc.method, tc.path, tc.body, tc.client, tc.token, tc.headers)
			assertResponse(t, w, tc.want, tc.code)
			if f.snapshot(t) != before || len(f.hooks) != 0 {
				t.Fatal("拒绝分支续期或修改目标")
			}
		})
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=x", "limit=1&limit=2", "limit=9223372036854775808", "cursor=", "cursor=invalid", "cursor=a&cursor=b", "limit=%zz"} {
		t.Run(query, func(t *testing.T) {
			before := f.snapshot(t)
			w := f.request("GET", "/api/v1/devices?"+query, "", "android", f.android.Token, nil)
			assertResponse(t, w, 400, "INVALID_REQUEST")
			if f.snapshot(t) != before {
				t.Fatal("分页拒绝续期")
			}
		})
	}
}

func TestDeviceHTTPCommittedHookAndUnavailable(t *testing.T) {
	f := setup(t)
	headers := map[string]string{"Referer": "https://example.com/settings", "X-CSRF-Token": f.web.CSRFToken}
	path := "/api/v1/devices/" + f.androidID + "/session"
	w := f.request("DELETE", path, "", "web", f.web.Token, headers)
	assertResponse(t, w, 204, "")
	if w.Body.Len() != 0 || len(f.hooks) != 1 || f.hooks[0] != f.androidID {
		t.Fatal("204或精确hook错误")
	}
	w = f.request("DELETE", path, "", "web", f.web.Token, headers)
	assertResponse(t, w, 404, "RESOURCE_NOT_FOUND")
	if len(f.hooks) != 1 {
		t.Fatal("重试重复通知")
	}
	// logout 和设备撤销复用同一个提交后通知机制。
	w = f.request("POST", "/api/v1/auth/logout", "", "web", f.web.Token, headers)
	if w.Code != 204 || len(f.hooks) != 2 || f.hooks[1] != f.webID {
		t.Fatal("logout未组合到相同通知")
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	w = f.request("GET", "/api/v1/devices", "", "android", f.android.Token, nil)
	assertResponse(t, w, 503, "SERVICE_UNAVAILABLE")
	w = f.request("DELETE", path, "", "android", f.android.Token, nil)
	assertResponse(t, w, 503, "SERVICE_UNAVAILABLE")
}

func TestDeviceHTTPBoundedPagesAndExpiredTarget(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin, err := auth.ReadAdministrator(ctx, f.store)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 101; i++ {
		_, _, err := f.sessions.Create(ctx, admin.AuthGeneration, auth.SessionInput{ClientType: "android", DeviceName: "分页设备", DeviceID: "00000000-0000-4000-8000-000000000001"})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		query string
		want  int
	}{{"", 50}, {"?limit=100", 100}, {"?limit=1000", 100}} {
		w := f.request("GET", "/api/v1/devices"+tc.query, "", "android", f.android.Token, nil)
		assertResponse(t, w, 200, "")
		var p struct {
			Items []json.RawMessage `json:"items"`
			More  bool              `json:"has_more"`
			Next  *string           `json:"next_cursor"`
		}
		if json.Unmarshal(w.Body.Bytes(), &p) != nil || len(p.Items) != tc.want || !p.More || p.Next == nil {
			t.Fatal("分页边界错误")
		}
	}
	if err := f.store.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE sessions SET idle_expires_at=1 WHERE id=?", f.androidID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t)
	headers := map[string]string{"Origin": "https://example.com", "X-CSRF-Token": f.web.CSRFToken}
	w := f.request("DELETE", "/api/v1/devices/"+f.androidID+"/session", "", "web", f.web.Token, headers)
	assertResponse(t, w, 404, "RESOURCE_NOT_FOUND")
	if f.snapshot(t) != before || len(f.hooks) != 0 {
		t.Fatal("过期目标失败仍改变会话")
	}
}

func TestDeviceHTTPSelfRevokeThroughTrustedProxy(t *testing.T) {
	f := setup(t)
	policy, err := httpapi.NewOriginPolicy("https://example.com", []string{"192.0.2.1/32"})
	if err != nil {
		t.Fatal(err)
	}
	f.mux = httpapi.New(health.New(nil), management{auth.NewHandler(f.store, f.sessions, policy), devices.NewHandler(f.sessions, auth.NewRequestSecurity(f.sessions, policy))})
	request := func(proto string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("DELETE", "http://internal/api/v1/devices/"+f.webID+"/session", nil)
		r.RemoteAddr = "192.0.2.1:1234"
		r.AddCookie(&http.Cookie{Name: "relaynest_session", Value: f.web.Token})
		for k, v := range map[string]string{"X-Forwarded-Host": "example.com", "X-Forwarded-Proto": proto, "X-Forwarded-For": "198.51.100.1", "Referer": "https://example.com/settings", "X-CSRF-Token": f.web.CSRFToken} {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		f.mux.ServeHTTP(w, r)
		return w
	}
	before := f.snapshot(t)
	assertResponse(t, request("http"), 403, "FORBIDDEN")
	if f.snapshot(t) != before || len(f.hooks) != 0 {
		t.Fatal("代理来源拒绝改变会话")
	}
	w := request("https")
	cookies := w.Result().Cookies()
	if w.Code != 204 || w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-store" || len(cookies) != 1 || !cookies[0].Secure || cookies[0].MaxAge != -1 || len(f.hooks) != 1 || f.hooks[0] != f.webID {
		t.Fatal("可信代理自撤未使用解析后的HTTPS来源")
	}
}

func TestDeviceHTTPRemoteWebDoesNotClearCookie(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin, err := auth.ReadAdministrator(ctx, f.store)
	if err != nil {
		t.Fatal(err)
	}
	remote, _, err := f.sessions.Create(ctx, admin.AuthGeneration, auth.SessionInput{ClientType: "web", DeviceName: "另一个浏览器"})
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Origin": "https://example.com", "X-CSRF-Token": f.web.CSRFToken}
	w := f.request("DELETE", "/api/v1/devices/"+remote.ID+"/session", "", "web", f.web.Token, headers)
	assertResponse(t, w, 204, "")
	if len(f.hooks) != 1 || f.hooks[0] != remote.ID {
		t.Fatal("远程Web目标通知错误")
	}
	w = f.request("GET", "/api/v1/devices", "", "web", f.web.Token, nil)
	assertResponse(t, w, 200, "")
}

// 请求体只在授权之后读取，以真实数据库变更覆盖事务入口复核。
type invalidatingBody struct{ invalidate func() }

func (b invalidatingBody) Read([]byte) (int, error) { b.invalidate(); return 0, io.EOF }
func (b invalidatingBody) Close() error             { return nil }

func TestDeviceHTTPRequiredCredentials(t *testing.T) {
	f := setup(t)
	created, _, err := bots.New(f.store).Create(context.Background(), bots.CreateRequest{Name: "transport", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cookie := "relaynest_session=" + f.web.Token
	for _, tc := range []struct {
		name    string
		headers http.Header
	}{
		{"missing", http.Header{}},
		{"empty", http.Header{"Authorization": {""}}},
		{"malformed", http.Header{"Authorization": {"Basic bad"}}},
		{"random", http.Header{"Authorization": {"Bearer random"}}},
		{"cookie", http.Header{"Cookie": {"relaynest_session=invalid"}, "Origin": {"https://example.com"}}},
		{"duplicate-bearer", http.Header{"Authorization": {"Bearer " + f.android.Token, "Bearer " + f.android.Token}}},
		{"duplicate-cookie", http.Header{"Cookie": {cookie + "; " + cookie}}},
		{"mixed", http.Header{"Cookie": {cookie}, "Authorization": {"Bearer " + f.android.Token}}},
		{"bot-token", http.Header{"Authorization": {"Bearer " + created.APIToken.Token}}},
		{"webhook-key", http.Header{"Authorization": {"Bearer " + created.WebhookKey.Key}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.snapshot(t)
			for _, method := range []string{"DELETE", "GET"} {
				path, code := "/api/v1/devices", "AUTHENTICATION_FAILED"
				if method == "DELETE" {
					path += "/" + f.androidID + "/session"
					code = "AUTHENTICATION_REQUIRED"
				}
				r := httptest.NewRequest(method, "https://example.com"+path, nil)
				r.Header = tc.headers.Clone()
				w := httptest.NewRecorder()
				f.mux.ServeHTTP(w, r)
				assertResponse(t, w, 401, code)
			}
			if before != f.snapshot(t) || len(f.hooks) != 0 {
				t.Fatal("拒绝请求改变会话或通知")
			}
		})
	}
}

func TestDeviceHTTPRequiredLifecycleAndTransactionRecheck(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		for _, state := range []string{"revoked", "idle", "absolute", "generation", "database"} {
			t.Run(fmt.Sprintf("delayed=%v/%s", delayed, state), func(t *testing.T) {
				f := setup(t)
				var baseline string
				invalidate := func() {
					if state == "database" {
						if err := f.store.Close(); err != nil {
							t.Fatal(err)
						}
						return
					}
					err := f.store.Write(context.Background(), func(tx *sql.Tx) error {
						var err error
						switch state {
						case "revoked":
							_, err = tx.Exec("UPDATE sessions SET revoked=1 WHERE id=?", f.webID)
						case "idle":
							_, err = tx.Exec("UPDATE sessions SET idle_expires_at=0 WHERE id=?", f.webID)
						case "absolute":
							_, err = tx.Exec("UPDATE sessions SET idle_expires_at=0,absolute_expires_at=0 WHERE id=?", f.webID)
						case "generation":
							_, err = tx.Exec("UPDATE administrator SET auth_generation=auth_generation+1")
						}
						return err
					})
					if err != nil {
						t.Fatal(err)
					}
					baseline = f.snapshot(t)
				}
				r := httptest.NewRequest("DELETE", "https://example.com/api/v1/devices/"+f.androidID+"/session", nil)
				r.AddCookie(&http.Cookie{Name: "relaynest_session", Value: f.web.Token})
				r.Header.Set("Origin", "https://example.com")
				r.Header.Set("X-CSRF-Token", f.web.CSRFToken)
				if delayed {
					r.Body = invalidatingBody{invalidate}
				} else {
					invalidate()
				}
				w := httptest.NewRecorder()
				f.mux.ServeHTTP(w, r)
				status, code := 401, "AUTHENTICATION_REQUIRED"
				if state == "database" {
					status, code = 503, "SERVICE_UNAVAILABLE"
				}
				assertResponse(t, w, status, code)
				if len(f.hooks) != 0 {
					t.Fatal("拒绝后发送撤销通知")
				}
				if state != "database" && f.snapshot(t) != baseline {
					t.Fatal("请求产生额外写入或续期")
				}
			})
		}
	}
}

package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"relaynest/server/internal/health"
)

func TestHealthRoutes(t *testing.T) {
	h := New(health.New(nil))
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/health/live", 200},
		{"HEAD", "/health/live", 200},
		{"GET", "/health/ready", 503},
		{"POST", "/health/live", 405},
		{"GET", "/health/live/extra", 404},
		{"GET", "/admin", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}

func TestStaticRoutes(t *testing.T) {
	h := New(health.New(nil))
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/", 200},
		{"HEAD", "/", 200},
		{"POST", "/", 405},
		{"GET", "/api/v1/missing", 404},
		{"GET", "/chat", 404},
		{"GET", "/history", 404},
		{"GET", "/src/main.ts", 404},
		{"GET", "/assets/", 404},
		{"GET", "/assets/missing.js", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
		if tc.path == "/" && tc.method == "GET" && !strings.Contains(w.Body.String(), "RelayNest 管理") {
			t.Error("首页未提供管理入口")
		}
	}
}

type routeProbe struct{}

func (routeProbe) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "login")
	w.WriteHeader(204)
}
func (routeProbe) GetSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "session")
	w.WriteHeader(204)
}
func (routeProbe) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "logout")
	w.WriteHeader(204)
}

func TestExactAuthenticationRoutes(t *testing.T) {
	h := New(health.New(nil), routeProbe{})
	for _, tc := range []struct {
		method, path, route string
		status              int
	}{
		{"POST", "/api/v1/auth/login", "login", 204},
		{"GET", "/api/v1/auth/session", "session", 204},
		{"HEAD", "/api/v1/auth/session", "session", 204},
		{"POST", "/api/v1/auth/logout", "logout", 204},
		{"GET", "/api/v1/auth/login", "", 405},
		{"POST", "/api/v1/auth/session", "", 405},
		{"GET", "/api/v1/auth/logout", "", 405},
		{"POST", "/api/v1/auth/login/", "", 404},
		{"GET", "/api/v1/auth/session/extra", "", 404},
		{"POST", "/api/v1/auth/logout/", "", 404},
		{"GET", "/api/v1/missing", "", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || w.Header().Get("X-Route") != tc.route {
			t.Errorf("%s %s: %d %q", tc.method, tc.path, w.Code, w.Header().Get("X-Route"))
		}
		if strings.Contains(w.Header().Get("Content-Type"), "text/html") || strings.Contains(w.Body.String(), "<!doctype") {
			t.Errorf("API 返回 HTML: %s", tc.path)
		}
	}
}

type deviceRouteProbe struct{ routeProbe }

func (deviceRouteProbe) ListDeviceSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "devices")
	w.WriteHeader(204)
}
func (deviceRouteProbe) RevokeDeviceSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Session-ID", r.PathValue("id"))
	w.WriteHeader(204)
}

func TestExactDeviceRoutes(t *testing.T) {
	h := New(health.New(nil), deviceRouteProbe{})
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/v1/devices", 204}, {"HEAD", "/api/v1/devices", 204},
		{"DELETE", "/api/v1/devices/target/session", 204},
		{"POST", "/api/v1/devices", 405}, {"GET", "/api/v1/devices/target/session", 405},
		{"GET", "/api/v1/devices/", 404}, {"DELETE", "/api/v1/devices/target/session/", 404},
		{"DELETE", "/api/v1/devices/target/session/extra", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("%s %s=%d want=%d", tc.method, tc.path, w.Code, tc.want)
		}
		if tc.method == "DELETE" && tc.want == 204 && w.Header().Get("X-Session-ID") != "target" {
			t.Fatal("Session.id未传递")
		}
	}
}

type devicePushRouteProbe struct{ deviceRouteProbe }

func (devicePushRouteProbe) GetCurrentPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "push-get")
	w.WriteHeader(204)
}
func (devicePushRouteProbe) PutCurrentPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "push-put")
	w.WriteHeader(204)
}
func (devicePushRouteProbe) DeleteCurrentPush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "push-delete")
	w.Header().Set("X-Provider", r.PathValue("provider"))
	w.WriteHeader(204)
}
func (devicePushRouteProbe) GetCurrentNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "notifications-get")
	w.WriteHeader(204)
}
func (devicePushRouteProbe) PutCurrentNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Route", "notifications-put")
	w.WriteHeader(204)
}

func TestExactCurrentDevicePushRoutes(t *testing.T) {
	h := New(health.New(nil), devicePushRouteProbe{})
	for _, tc := range []struct {
		method, suffix, route string
		status                int
	}{
		{"GET", "push", "push-get", 204}, {"HEAD", "push", "push-get", 204},
		{"PUT", "push", "push-put", 204}, {"DELETE", "push/fcm", "push-delete", 204},
		{"GET", "notifications", "notifications-get", 204}, {"HEAD", "notifications", "notifications-get", 204},
		{"PUT", "notifications", "notifications-put", 204},
		{"POST", "push", "", 405}, {"DELETE", "push", "", 405}, {"GET", "push/fcm", "", 405},
		{"POST", "notifications", "", 405}, {"DELETE", "notifications", "", 405},
		{"GET", "push/extra/segment", "", 404}, {"DELETE", "push/fcm/", "", 404},
		{"GET", "notifications/extra", "", 404}, {"PUT", "notifications/", "", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/v1/devices/current/"+tc.suffix, nil))
		if w.Code != tc.status || w.Header().Get("X-Route") != tc.route || strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Errorf("%s %s=%d/%q want=%d/%q", tc.method, tc.suffix, w.Code, w.Header().Get("X-Route"), tc.status, tc.route)
		}
		if tc.route == "push-delete" && w.Header().Get("X-Provider") != "fcm" {
			t.Fatal("provider未从正式路由传递")
		}
	}
}

type instanceRouteProbe struct {
	routeProbe
	*InstanceHandler
}

func TestInstanceProbeAliasesOverHTTP(t *testing.T) {
	for _, protocol := range []string{"http1", "tls-http1", "http2"} {
		t.Run(protocol, func(t *testing.T) {
			resetProbeBucket(t, 40)
			store := probeStore(t)
			server := httptest.NewUnstartedServer(New(health.New(nil), instanceRouteProbe{InstanceHandler: NewInstanceHandler(store)}))
			server.EnableHTTP2 = protocol == "http2"
			if protocol == "http1" {
				server.Start()
			} else {
				server.StartTLS()
			}
			defer server.Close()
			server.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			probeHTTP(t, server, "GET", "", "", nil, 200, "")
			// 非法路径必须先于限流和存储，不得消耗额度或访问数据库。
			resetProbeBucket(t, 0)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			for _, alias := range []string{
				"/api/v1//instance", "/api/v1/x/../instance", "/api/./v1/instance",
				"//api/v1/instance", "/api/v1/instance/.", "/api/v1/instance/x/..",
				"/%61pi/v1//%69nstance", "/api/v1//instance?", "/api/v1//instance?%invalid",
				"/api/v1//instance?account=audit-sensitive-marker",
			} {
				for _, method := range []string{"GET", "POST", "HEAD"} {
					t.Run(method+alias, func(t *testing.T) {
						req, err := http.NewRequest(method, server.URL+alias, nil)
						if err != nil {
							t.Fatal(err)
						}
						req.Header["Authorization"] = []string{"audit-sensitive-marker"}
						res, err := server.Client().Do(req)
						if err != nil {
							t.Fatal(err)
						}
						defer res.Body.Close()
						raw, err := io.ReadAll(res.Body)
						if err != nil {
							t.Fatal(err)
						}
						wantProtocol := 1
						if protocol == "http2" {
							wantProtocol = 2
						}
						if res.ProtoMajor != wantProtocol || res.StatusCode != 400 || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Content-Type") != "application/json; charset=utf-8" {
							t.Fatalf("unexpected response: %s %d %v %s", res.Proto, res.StatusCode, res.Header, raw)
						}
						for _, name := range []string{"Location", "ETag", "Last-Modified", "Retry-After"} {
							if _, exists := res.Header[name]; exists {
								t.Fatal("unexpected header", name)
							}
						}
						if strings.Contains(string(raw), "audit-sensitive-marker") || strings.Contains(string(raw), "instance") || strings.Contains(string(raw), "<") {
							t.Fatal("response reflected request data", string(raw))
						}
						if method != "HEAD" {
							var envelope map[string]map[string]any
							if json.Unmarshal(raw, &envelope) != nil || len(envelope) != 1 || len(envelope["error"]) != 4 {
								t.Fatalf("invalid error envelope: %s", raw)
							}
							e := envelope["error"]
							if e["code"] != "INVALID_REQUEST" || e["message"] != "请求无效" || e["retryable"] != false || e["request_id"] == "" || e["request_id"] == nil {
								t.Fatalf("invalid error fields: %s", raw)
							}
						}
					})
				}
			}
			probeHTTP(t, server, "GET", "", "", nil, 429, "RATE_LIMITED")
			resetProbeBucket(t, 1)
			probeHTTP(t, server, "GET", "", "", nil, 503, "INSTANCE_UNAVAILABLE")
		})
	}
}

func TestInstanceProbeAliasGuardPreservesOtherRoutes(t *testing.T) {
	h := New(health.New(nil), routeProbe{})
	for _, tc := range []struct {
		path, location string
		status         int
	}{
		{"/health//live", "/health/live", 307},
		{"/api/v1/auth//session", "/api/v1/auth/session", 307},
		{"/api/v1//missing", "/api/v1/missing", 307},
		{"/assets//missing.js", "/assets/missing.js", 307},
		{"/api%2fv1//instance", "/api%2fv1/instance", 307},
		{"/api/v1//instance/", "/api/v1/instance/", 307},
		{"/api/v1/x/%2e%2e/instance", "", 404},
		{"/api/v1/instance/", "", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || w.Header().Get("Location") != tc.location {
			t.Errorf("%s: %d %q", tc.path, w.Code, w.Header().Get("Location"))
		}
	}
}

package httpapi

import (
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

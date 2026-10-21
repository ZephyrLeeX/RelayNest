package httpapi

import (
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

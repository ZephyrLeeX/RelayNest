package httpapi

import (
	"net/http/httptest"
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

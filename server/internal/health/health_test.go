package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthResponses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probe Probe
		stop  bool
		want  int
	}{
		{"not_configured", nil, false, 503},
		{"ready", func(context.Context) error { return nil }, false, 200},
		{"failed", func(context.Context) error { return errors.New("secret=/private/db token=credential") }, false, 503},
		{"stopping", func(context.Context) error { return nil }, true, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := New(tc.probe)
			if tc.stop {
				h.Stop()
			}
			for _, ready := range []bool{false, true} {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/health", nil)
				want, body := 200, "ok\n"
				if ready {
					h.Ready(w, r)
					want = tc.want
					if want == 503 {
						body = "unavailable\n"
					}
				} else {
					h.Live(w, r)
				}
				if w.Code != want || w.Body.String() != body {
					t.Fatalf("code=%d body=%q, want %d %q", w.Code, w.Body.String(), want, body)
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("健康结果不得缓存")
				}
			}
		})
	}
}

func TestProbeReceivesCancellationAndDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := New(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("缺少探针超时")
		}
		if ctx.Err() != context.Canceled {
			t.Fatal("请求取消未传递")
		}
		return nil
	})
	w := httptest.NewRecorder()
	h.Ready(w, httptest.NewRequest("GET", "/health/ready", nil).WithContext(ctx))
	if w.Code != 503 {
		t.Fatalf("取消后返回 %d", w.Code)
	}
}

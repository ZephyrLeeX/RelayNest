package httpapi

import (
	"net/http"

	"relaynest/server/internal/health"
	"relaynest/server/internal/webui"
)

func New(h *health.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", h.Live)
	mux.HandleFunc("GET /health/ready", h.Ready)
	// 精确挂载管理首页和构建资源，未知 API 不回落为 HTML。
	static := webui.New()
	mux.Handle("GET /{$}", static)
	mux.Handle("GET /assets/", static)
	return mux
}

package httpapi

import (
	"net/http"

	"relaynest/server/internal/health"
)

func New(h *health.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", h.Live)
	mux.HandleFunc("GET /health/ready", h.Ready)
	return mux
}

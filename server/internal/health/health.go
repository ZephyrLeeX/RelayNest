package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// Probe 由存储层组合数据库和挂载检查；实现必须响应上下文取消。
type Probe func(context.Context) error

type Handler struct {
	probe    Probe
	stopping atomic.Bool
}

func New(probe Probe) *Handler { return &Handler{probe: probe} }

// Stop 在停止接收请求前撤销就绪状态，避免关闭期间继续宣称可用。
func (h *Handler) Stop() { h.stopping.Store(true) }

func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, "ok\n")
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	// 未接入真实依赖时不能把空工程当作业务服务就绪。
	if h.stopping.Load() || h.probe == nil {
		respond(w, http.StatusServiceUnavailable, "unavailable\n")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	err := h.probe(ctx)
	if err != nil || ctx.Err() != nil || h.stopping.Load() {
		// 不将探针原始错误、资源路径或凭据带入公开响应。
		respond(w, http.StatusServiceUnavailable, "unavailable\n")
		return
	}
	respond(w, http.StatusOK, "ok\n")
}

func respond(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	// 客户端断开后的响应写错误无需重复暴露或记录请求内容。
	_, _ = w.Write([]byte(body))
}

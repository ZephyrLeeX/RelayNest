package httpapi

import (
	"context"
	"log/slog"
	"regexp"
)

var logID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var logCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// NewSafeLogHandler 构成公共日志边界。丢弃原始消息及非白名单字段，
// 不调用未知 LogValuer/Stringer，不展开分组或错误，避免嵌套凭据与正文泄漏。
// 白名单标识符必须由服务端生成，不应把任何客户端输入放入这些字段。
func NewSafeLogHandler(next slog.Handler) slog.Handler { return &safeLogHandler{next: next} }

type safeLogHandler struct {
	next    slog.Handler
	grouped bool
}

func (h *safeLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h *safeLogHandler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, "event", 0)
	r.Attrs(func(a slog.Attr) bool {
		if !h.grouped && safeLogAttr(a) {
			clean.AddAttrs(a)
		}
		return true
	})
	return h.next.Handle(ctx, clean)
}
func (h *safeLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if !h.grouped && safeLogAttr(a) {
			clean = append(clean, a)
		}
	}
	return &safeLogHandler{next: h.next.WithAttrs(clean), grouped: h.grouped}
}
func (h *safeLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &safeLogHandler{next: h.next, grouped: true}
}
func safeLogAttr(a slog.Attr) bool {
	switch a.Key {
	case "request_id", "entity_id":
		return a.Value.Kind() == slog.KindString && logID.MatchString(a.Value.String())
	case "error_code":
		return a.Value.Kind() == slog.KindString && logCode.MatchString(a.Value.String())
	case "status":
		return a.Value.Kind() == slog.KindInt64 && a.Value.Int64() >= 100 && a.Value.Int64() <= 599
	case "duration":
		return a.Value.Kind() == slog.KindDuration && a.Value.Duration() >= 0
	}
	return false
}

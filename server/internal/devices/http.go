package devices

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"relaynest/server/internal/auth"
)

// Handler 仅投影设备会话 HTTP 契约，身份复核、续期和撤销事务由既有存储负责。
type Handler struct {
	sessions *auth.SessionStore
	security *auth.RequestSecurity
}

// NewHandler 消费启动阶段共享的会话存储及来源安全入口。
func NewHandler(sessions *auth.SessionStore, security *auth.RequestSecurity) *Handler {
	return &Handler{sessions: sessions, security: security}
}

// 仅显式投影冻结的公开元数据，避免认证模型扩展时泄露凭据。
type sessionMetadata struct {
	ID            string `json:"id"`
	Administrator struct {
		Username string `json:"username"`
	} `json:"administrator"`
	ClientType        string    `json:"client_type"`
	DeviceID          string    `json:"device_id"`
	DeviceName        string    `json:"device_name"`
	IdleExpiresAt     time.Time `json:"idle_expires_at"`
	AbsoluteExpiresAt time.Time `json:"absolute_expires_at"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": http.StatusText(status), "request_id": rand.Text(), "retryable": status == 503}})
}

func (h *Handler) ListDeviceSessions(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.security.Authorize(w, r, auth.RequestRead)
	if !ok {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST")
		return
	}
	limit, cursor := 50, ""
	if values, exists := q["limit"]; exists {
		if len(values) != 1 {
			writeError(w, 400, "INVALID_REQUEST")
			return
		}
		n, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil || n < 1 {
			writeError(w, 400, "INVALID_REQUEST")
			return
		}
		if n > 100 {
			n = 100
		}
		limit = int(n)
	}
	if values, exists := q["cursor"]; exists {
		if len(values) != 1 || values[0] == "" {
			writeError(w, 400, "INVALID_REQUEST")
			return
		}
		cursor = values[0]
	}
	// 仓储在同一事务检查 actor、列出有效 Session 并续期，失败前不写成功响应。
	page, err := h.sessions.ListDeviceSessions(r.Context(), actor, cursor, limit)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidSessionPage) {
			writeError(w, 400, "INVALID_REQUEST")
		} else {
			auth.WriteSessionError(w, err)
		}
		return
	}
	items := make([]sessionMetadata, 0, len(page.Items))
	for _, s := range page.Items {
		item := sessionMetadata{ID: s.ID, ClientType: s.ClientType, DeviceID: s.DeviceID, DeviceName: s.DeviceName, IdleExpiresAt: s.IdleExpiresAt, AbsoluteExpiresAt: s.AbsoluteExpiresAt}
		item.Administrator.Username = s.Username
		items = append(items, item)
	}
	writeJSON(w, 200, struct {
		Items []sessionMetadata `json:"items"`
		Next  *string           `json:"next_cursor"`
		More  bool              `json:"has_more"`
	}{items, page.NextCursor, page.HasMore})
}

var sessionID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func (h *Handler) RevokeDeviceSession(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.security.AuthorizeRequired(w, r, auth.RequestWrite)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !sessionID.MatchString(id) {
		writeError(w, 400, "INVALID_REQUEST")
		return
	}
	// 契约无请求体；只读一个字节即可拒绝任意正文，包括分块传输。
	if r.Body != nil {
		var b [1]byte
		n, err := r.Body.Read(b[:])
		if n != 0 || err != io.EOF {
			writeError(w, 400, "INVALID_REQUEST")
			return
		}
	}
	if err := h.sessions.RevokeDeviceSession(r.Context(), actor, id); err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			writeError(w, 404, "RESOURCE_NOT_FOUND")
		} else {
			auth.WriteSessionErrorRequired(w, err)
		}
		return
	}
	if id == actor.Session.ID {
		actor.ClearCookie(w)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

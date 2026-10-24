package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"relaynest/server/internal/bots"
)

// BotAccess 的闭包绑定本次真实管理员身份；依赖方向保持 auth -> httpapi。
// 写闭包必须调用 SessionStore.WithAuthorizedWrite，读闭包必须调用 RenewRead。
type BotAccess struct {
	RenewRead    func(context.Context) error
	Write        bots.WriteFunc
	SessionError func(http.ResponseWriter, error)
}
type BotAuthorize func(http.ResponseWriter, *http.Request, bool) (BotAccess, bool)
type BotHandler struct {
	repo      *bots.Repository
	authorize BotAuthorize
}

func NewBotHandler(repo *bots.Repository, authorize BotAuthorize) *BotHandler {
	return &BotHandler{repo, authorize}
}

func botJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func botError(w http.ResponseWriter, status int, code string) {
	botJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": http.StatusText(status), "request_id": rand.Text(), "retryable": status == 503}})
}
func botBusinessError(w http.ResponseWriter, access BotAccess, err error) {
	switch {
	case errors.Is(err, bots.ErrInvalid):
		botError(w, 422, "VALIDATION_FAILED")
	case errors.Is(err, bots.ErrConflict):
		botError(w, 409, "VERSION_CONFLICT")
	case errors.Is(err, bots.ErrNotFound):
		botError(w, 404, "RESOURCE_NOT_FOUND")
	default:
		access.SessionError(w, err)
	}
}

// 解码大小受限、拒绝额外字段与尾随文档；必填及 null 由具体模型验证。
func botDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		botError(w, 400, "INVALID_REQUEST")
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			botError(w, 413, "PAYLOAD_TOO_LARGE")
		} else {
			botError(w, 400, "INVALID_REQUEST")
		}
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		botError(w, 400, "INVALID_REQUEST")
		return false
	}
	for _, value := range fields {
		if strings.TrimSpace(string(value)) == "null" {
			botError(w, 422, "VALIDATION_FAILED")
			return false
		}
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		botError(w, 400, "INVALID_REQUEST")
		return false
	}
	return true
}

type botCapabilitiesInput struct {
	Send    *bool `json:"send"`
	Receive *bool `json:"receive"`
}

func (c *botCapabilitiesInput) valid() bool { return c != nil && c.Send != nil && c.Receive != nil }
func (c *botCapabilitiesInput) value() bots.Capabilities {
	return bots.Capabilities{Send: *c.Send, Receive: *c.Receive}
}

type botInput struct {
	Name         *string               `json:"name"`
	Enabled      *bool                 `json:"enabled"`
	Capabilities *botCapabilitiesInput `json:"capabilities"`
}

func (h *BotHandler) CreateBot(w http.ResponseWriter, r *http.Request) {
	access, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	var in botInput
	if !botDecode(w, r, &in) {
		return
	}
	if in.Name == nil || *in.Name == "" || in.Enabled == nil || !in.Capabilities.valid() {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	result, _, err := h.repo.CreateWithWrite(r.Context(), bots.CreateRequest{Name: *in.Name, Enabled: *in.Enabled, Capabilities: in.Capabilities.value()}, access.Write)
	if err != nil {
		botBusinessError(w, access, err)
		return
	}
	botJSON(w, 201, result)
}
func (h *BotHandler) PatchBot(w http.ResponseWriter, r *http.Request) {
	access, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	if !botID(r.PathValue("id")) {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	var in struct {
		botInput
		ExpectedVersion *int64 `json:"expected_version"`
	}
	if !botDecode(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 1 || (in.Name != nil && *in.Name == "") || (in.Capabilities != nil && !in.Capabilities.valid()) {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	req := bots.PatchRequest{ExpectedVersion: *in.ExpectedVersion, Name: in.Name, Enabled: in.Enabled}
	if in.Capabilities != nil {
		c := in.Capabilities.value()
		req.Capabilities = &c
	}
	result, _, err := h.repo.PatchWithWrite(r.Context(), r.PathValue("id"), req, access.Write)
	if err != nil {
		botBusinessError(w, access, err)
		return
	}
	botJSON(w, 200, result)
}
func (h *BotHandler) ListBots(w http.ResponseWriter, r *http.Request) {
	access, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		botError(w, 400, "INVALID_REQUEST")
		return
	}
	limit := 50
	if values, exists := q["limit"]; exists {
		if len(values) != 1 {
			botError(w, 400, "INVALID_REQUEST")
			return
		}
		n, e := strconv.ParseInt(values[0], 10, 64)
		if e != nil || n < 1 {
			botError(w, 400, "INVALID_REQUEST")
			return
		}
		if n > 100 {
			n = 100
		}
		limit = int(n)
	}
	after := ""
	if values, exists := q["cursor"]; exists {
		if len(values) != 1 {
			botError(w, 400, "INVALID_REQUEST")
			return
		}
		b, e := base64.RawURLEncoding.DecodeString(values[0])
		if e != nil || !strings.HasPrefix(string(b), "bots:") || !botID(strings.TrimPrefix(string(b), "bots:")) {
			botError(w, 400, "INVALID_REQUEST")
			return
		}
		after = string(b[5:])
	}
	items, err := h.repo.ListPage(r.Context(), after, limit+1)
	if err != nil {
		botBusinessError(w, access, err)
		return
	}
	var next *string
	more := len(items) > limit
	if more {
		items = items[:limit]
		s := base64.RawURLEncoding.EncodeToString([]byte("bots:" + items[len(items)-1].ID))
		next = &s
	}
	// 成功读取后重新核对原身份；在确认前不输出任何成功响应字节。
	if err = access.RenewRead(r.Context()); err != nil {
		access.SessionError(w, err)
		return
	}
	botJSON(w, 200, struct {
		Items []bots.Bot `json:"items"`
		Next  *string    `json:"next_cursor"`
		More  bool       `json:"has_more"`
	}{items, next, more})
}
func botID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

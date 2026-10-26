package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/idempotency"
	"relaynest/server/internal/messages"
)

// MessageHandler 分别消费 Bot 身份与装配层绑定的 Android 受控事务能力。
type MessageHandler struct {
	bots      *bots.Repository
	messages  *messages.Repository
	authorize BotAuthorize
}

func NewMessageHandler(b *bots.Repository, m *messages.Repository, authorize ...BotAuthorize) *MessageHandler {
	h := &MessageHandler{bots: b, messages: m}
	if len(authorize) > 0 {
		h.authorize = authorize[0]
	}
	return h
}

// CreateClientMessage 在同一会话事务中查重及持久化；候选回执只能在提交成功后发布。
func (h *MessageHandler) CreateClientMessage(w http.ResponseWriter, r *http.Request) {
	if h.authorize == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return
	}
	access, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	if !botID(r.PathValue("id")) {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !idempotency.ValidKey(keys[0]) {
		botError(w, 400, "INVALID_REQUEST")
		return
	}
	in, ok := decodeMessage(w, r)
	if !ok {
		return
	}
	var out messages.Created
	err := access.Write(r.Context(), func(tx *sql.Tx) error {
		var err error
		out, err = h.messages.CreateIdempotentAdministratorTx(r.Context(), tx, r.PathValue("id"), keys[0], in, nil)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, bots.ErrForbidden), errors.Is(err, bots.ErrNotFound), errors.Is(err, idempotency.ErrConflict), errors.Is(err, idempotency.ErrExpired), errors.Is(err, messages.ErrTooLarge), errors.Is(err, messages.ErrInvalid), errors.Is(err, messages.ErrUnsupported):
			messageError(w, err)
		default:
			access.SessionError(w, err)
		}
		return
	}
	if out.Delivery == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return
	}
	m := out.Message
	botJSON(w, 201, struct {
		Message    messageResponse `json:"message"`
		DeliveryID string          `json:"delivery_id"`
	}{messageResponse{m.ID, m.BotID, strconv.FormatInt(m.BotSeq, 10), m.Revision, m.Sender, m.Format, m.Text, []messages.AttachmentReference{}, m.OriginalReplyToMessageID, [][]messages.Button{}, m.CreatedAt, m.UpdatedAt}, out.Delivery.DeliveryID})
}

func (h *MessageHandler) CreateBotMessage(w http.ResponseWriter, r *http.Request) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		botError(w, 401, "UNAUTHORIZED")
		return
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		botError(w, 401, "UNAUTHORIZED")
		return
	}
	identity, match, err := h.bots.DiscoverAPIIdentity(r.Context(), parts[1])
	if err != nil {
		messageError(w, err)
		return
	}
	if match != bots.APIMatched {
		botError(w, 401, "UNAUTHORIZED")
		return
	}
	if !identity.Capabilities().Send {
		botError(w, 403, "FORBIDDEN")
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !idempotency.ValidKey(keys[0]) {
		botError(w, 400, "INVALID_REQUEST")
		return
	}
	in, ok := decodeMessage(w, r)
	if !ok {
		return
	}
	// 仓储在查重前及写入事务中复核启用、send 和凭据代次；成功仅在 COMMIT 后发布。
	out, err := h.messages.CreateIdempotentApplication(r.Context(), identity, keys[0], in, nil)
	if err != nil {
		messageError(w, err)
		return
	}
	m := out.Message
	botJSON(w, 201, messageResponse{m.ID, m.BotID, strconv.FormatInt(m.BotSeq, 10), m.Revision, m.Sender, m.Format, m.Text, []messages.AttachmentReference{}, m.OriginalReplyToMessageID, [][]messages.Button{}, m.CreatedAt, m.UpdatedAt})
}

type messageResponse struct {
	ID          string                         `json:"id"`
	BotID       string                         `json:"bot_id"`
	BotSeq      string                         `json:"bot_seq"`
	Revision    int64                          `json:"revision"`
	Sender      messages.Sender                `json:"sender"`
	Format      string                         `json:"format"`
	Text        string                         `json:"text"`
	Attachments []messages.AttachmentReference `json:"attachments"`
	Reply       *string                        `json:"reply_to_message_id"`
	Buttons     [][]messages.Button            `json:"buttons"`
	CreatedAt   time.Time                      `json:"created_at"`
	UpdatedAt   time.Time                      `json:"updated_at"`
}

func messageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bots.ErrUnauthorized):
		botError(w, 401, "UNAUTHORIZED")
	case errors.Is(err, bots.ErrForbidden):
		botError(w, 403, "FORBIDDEN")
	case errors.Is(err, bots.ErrNotFound):
		botError(w, 404, "RESOURCE_NOT_FOUND")
	case errors.Is(err, idempotency.ErrConflict):
		botError(w, 409, "IDEMPOTENCY_CONFLICT")
	case errors.Is(err, idempotency.ErrExpired):
		botError(w, 410, "RESOURCE_EXPIRED")
	case errors.Is(err, messages.ErrTooLarge):
		botError(w, 413, "PAYLOAD_TOO_LARGE")
	case errors.Is(err, messages.ErrInvalid), errors.Is(err, messages.ErrUnsupported):
		botError(w, 422, "VALIDATION_FAILED")
	default:
		botError(w, 503, "SERVICE_UNAVAILABLE")
	}
}

func decodeMessage(w http.ResponseWriter, r *http.Request) (messages.CreateRequest, bool) {
	var in messages.CreateRequest
	fail := func(status int, code string) (messages.CreateRequest, bool) {
		botError(w, status, code)
		return in, false
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return fail(400, "INVALID_REQUEST")
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			return fail(413, "PAYLOAD_TOO_LARGE")
		}
		return fail(400, "INVALID_REQUEST")
	}
	// encoding/json 会替换非法 UTF-8；必须在解析前拒绝，避免摘要悄悄改变正文。
	if !utf8.Valid(raw) || !messageKeys(raw, "format", "text", "attachments", "reply_to_message_id", "buttons") {
		return fail(400, "INVALID_REQUEST")
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		return fail(400, "INVALID_REQUEST")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for key, value := range fields {
		if key != "reply_to_message_id" && strings.TrimSpace(string(value)) == "null" {
			return fail(422, "VALIDATION_FAILED")
		}
	}
	if in.Format != "plain" && in.Format != "markdown" {
		return fail(422, "VALIDATION_FAILED")
	}
	if len(in.Text) > 64*1024 {
		return fail(413, "PAYLOAD_TOO_LARGE")
	}
	if in.Text == "" && len(in.Attachments) == 0 && len(in.Buttons) == 0 {
		return fail(422, "VALIDATION_FAILED")
	}
	return in, true
}

// 逐个属性检查精确名称及嵌套 schema，拒绝大小写别名与被重复键覆盖的未知输入。
func messageKeys(raw json.RawMessage, allowed ...string) bool {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return false
		}
		name, ok := key.(string)
		if !ok {
			return false
		}
		found := false
		for _, a := range allowed {
			if a == name {
				found = true
			}
		}
		if !found {
			return false
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return false
		}
		switch name {
		case "attachments":
			var items []json.RawMessage
			if json.Unmarshal(value, &items) != nil {
				return false
			}
			for _, item := range items {
				if !messageKeys(item, "id", "kind") {
					return false
				}
			}
		case "buttons":
			var rows [][]json.RawMessage
			if json.Unmarshal(value, &rows) != nil {
				return false
			}
			for _, row := range rows {
				for _, item := range row {
					if !messageKeys(item, "id", "label", "url", "callback_data") {
						return false
					}
				}
			}
		}
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

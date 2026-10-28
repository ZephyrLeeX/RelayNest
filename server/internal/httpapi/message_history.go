package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"relaynest/server/internal/messages"
)

type historyResponse struct {
	Items []messageResponse `json:"items"`
	Next  *string           `json:"next_cursor"`
	More  bool              `json:"has_more"`
}

// ListClientMessages 的正文仅在整个授权读事务（含最终会话复核）成功后发布。
func (h *MessageHandler) ListClientMessages(w http.ResponseWriter, r *http.Request) {
	access, ok := h.historyAccess(w, r)
	if !ok {
		return
	}
	bot := r.PathValue("id")
	if !botID(bot) {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	query, err := historyQuery(r.URL.RawQuery, bot)
	if err != nil {
		botError(w, 400, "INVALID_REQUEST")
		return
	}
	out := historyResponse{Items: []messageResponse{}}
	err = access.Read(r.Context(), func(tx *sql.Tx) error {
		page, err := h.messages.ListHistoryTx(r.Context(), tx, bot, query)
		if err != nil {
			return err
		}
		for _, m := range page.Items {
			item, err := projectHistoryMessage(m)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, item)
		}
		out.More = page.HasMore
		if page.HasMore {
			cursor := encodeHistoryCursor(bot, page.Items[len(page.Items)-1].BotSeq)
			out.Next = &cursor
		}
		return nil
	})
	if err != nil {
		historyError(w, access, err)
		return
	}
	botJSON(w, 200, out)
}

func (h *MessageHandler) GetClientMessage(w http.ResponseWriter, r *http.Request) {
	access, ok := h.historyAccess(w, r)
	if !ok {
		return
	}
	if !botID(r.PathValue("id")) {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	var out messageResponse
	err := access.Read(r.Context(), func(tx *sql.Tx) error {
		m, err := h.messages.GetHistoryMessageByIDTx(r.Context(), tx, r.PathValue("id"))
		if err != nil {
			return err
		}
		out, err = projectHistoryMessage(m)
		return err
	})
	if err != nil {
		historyError(w, access, err)
		return
	}
	botJSON(w, 200, out)
}

func (h *MessageHandler) historyAccess(w http.ResponseWriter, r *http.Request) (BotAccess, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if h.authorize == nil || h.messages == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return BotAccess{}, false
	}
	access, ok := h.authorize(w, r, false)
	if !ok {
		return BotAccess{}, false
	}
	if access.Read == nil || access.SessionError == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return BotAccess{}, false
	}
	return access, true
}

func historyError(w http.ResponseWriter, access BotAccess, err error) {
	if errors.Is(err, messages.ErrHistoryNotFound) {
		botError(w, 404, "RESOURCE_NOT_FOUND")
		return
	}
	access.SessionError(w, err)
}

// 游标仅是带 Bot 域和版本的排他边界，不承担身份认证，不含同步轮次或时效。
func encodeHistoryCursor(bot string, seq int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("history:1:" + bot + ":" + strconv.FormatInt(seq, 10)))
}

func historyQuery(raw, bot string) (messages.HistoryQuery, error) {
	invalid := errors.New("invalid history query")
	query := messages.HistoryQuery{Limit: 50}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return query, invalid
	}
	for key := range values {
		if key != "limit" && key != "cursor" {
			return query, invalid
		}
	}
	if v, exists := values["limit"]; exists {
		if len(v) != 1 {
			return query, invalid
		}
		n, err := strconv.ParseInt(v[0], 10, 64)
		if err != nil || n < 1 {
			return query, invalid
		}
		if n > messages.MaxHistoryLimit {
			n = messages.MaxHistoryLimit
		}
		query.Limit = int(n)
	}
	if v, exists := values["cursor"]; exists {
		if len(v) != 1 || len(v[0]) > 256 {
			return query, invalid
		}
		data, err := base64.RawURLEncoding.Strict().DecodeString(v[0])
		if err != nil {
			return query, invalid
		}
		parts := strings.Split(string(data), ":")
		if len(parts) != 4 || parts[0] != "history" || parts[1] != "1" || parts[2] != bot {
			return query, invalid
		}
		seq, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || seq < 1 || strconv.FormatInt(seq, 10) != parts[3] || encodeHistoryCursor(bot, seq) != v[0] {
			return query, invalid
		}
		query.BeforeBotSeq = &seq
	}
	return query, nil
}

func projectHistoryMessage(m messages.Message) (messageResponse, error) {
	// 当前创建明确拒绝附件，仓储没有消息附件关联；不得把 Bot 的独立上传当作消息附件。
	// 按钮来自已有持久快照，解析失败必须令整页失败，不能默默抹为空数组。
	if !botID(m.ID) || !botID(m.BotID) || m.BotSeq < 1 || m.Revision < 1 || (m.Sender != messages.Application && m.Sender != messages.Administrator) || (m.Format != "plain" && m.Format != "markdown") || !utf8.ValidString(m.Text) || len(m.Text) > 65536 || !utf8.Valid(m.ButtonsJSON) || (m.OriginalReplyToMessageID != nil && !botID(*m.OriginalReplyToMessageID)) {
		return messageResponse{}, errors.New("invalid history message")
	}
	var buttons [][]messages.Button
	decoder := json.NewDecoder(bytes.NewReader(m.ButtonsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&buttons); err != nil {
		return messageResponse{}, err
	}
	if buttons == nil || decoder.Decode(new(any)) != io.EOF {
		return messageResponse{}, errors.New("invalid history buttons")
	}
	if err := validateHistoryButtons(buttons); err != nil {
		return messageResponse{}, err
	}
	if m.Text == "" && len(buttons) == 0 {
		return messageResponse{}, errors.New("invalid history content")
	}
	return messageResponse{
		ID: m.ID, BotID: m.BotID, BotSeq: strconv.FormatInt(m.BotSeq, 10), Revision: m.Revision,
		Sender: m.Sender, Format: m.Format, Text: m.Text, Attachments: []messages.AttachmentReference{},
		Reply: m.OriginalReplyToMessageID, Buttons: buttons, CreatedAt: m.CreatedAt.UTC(), UpdatedAt: m.UpdatedAt.UTC(),
	}, nil
}

// 历史快照也必须满足冻结按钮合同；损坏数据不能成为成功响应。
func validateHistoryButtons(buttons [][]messages.Button) error {
	invalid := errors.New("invalid history buttons")
	if len(buttons) > 10 {
		return invalid
	}
	ids := make(map[string]bool)
	for _, row := range buttons {
		if len(row) < 1 || len(row) > 5 {
			return invalid
		}
		for _, button := range row {
			if button.ID == "" || ids[button.ID] || button.Label == "" || utf8.RuneCountInString(button.Label) > 128 || (button.URL == nil) == (button.CallbackData == nil) {
				return invalid
			}
			ids[button.ID] = true
			if button.CallbackData != nil && len(*button.CallbackData) > 4096 {
				return invalid
			}
			if button.URL != nil {
				parsed, err := url.Parse(*button.URL)
				if err != nil || parsed.Scheme == "" {
					return invalid
				}
				for _, c := range *button.URL {
					if c <= 0x20 || c == 0x7f {
						return invalid
					}
				}
			}
		}
	}
	return nil
}

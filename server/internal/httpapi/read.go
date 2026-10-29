package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	readsync "relaynest/server/internal/sync"
)

// ReadHandler 仅消费装配层绑定的 Android 写事务，不接受客户端身份或时钟。
type ReadHandler struct{ authorize BotAuthorize }

func NewReadHandler(authorize BotAuthorize) *ReadHandler { return &ReadHandler{authorize: authorize} }

type readResponse struct {
	Read struct {
		BotID     string    `json:"bot_id"`
		ReadSeq   string    `json:"read_seq"`
		Version   int64     `json:"version"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"read"`
	ServerTime time.Time `json:"server_time"`
}

func (h *ReadHandler) UpdateClientRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.authorize == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return
	}
	access, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	if access.Write == nil || access.SessionError == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return
	}
	bot := r.PathValue("id")
	if !botID(bot) {
		botError(w, 422, "VALIDATION_FAILED")
		return
	}
	seq, ok := decodeRead(w, r)
	if !ok {
		return
	}
	var out readResponse
	err := access.Write(r.Context(), func(tx *sql.Tx) error {
		state, _, err := readsync.AdvanceReadTx(r.Context(), tx, bot, seq)
		if err != nil {
			return err
		}
		out.Read.BotID = state.BotID
		out.Read.ReadSeq = strconv.FormatInt(state.ReadSeq, 10)
		out.Read.Version = state.StateVersion
		out.Read.UpdatedAt = state.UpdatedAt.UTC()
		out.ServerTime = time.Now().UTC()
		// 只公开冻结字段；候选响应序列化失败必须连同状态、事件和续期一起回滚。
		_, err = json.Marshal(out)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			botError(w, 404, "RESOURCE_NOT_FOUND")
		case errors.Is(err, readsync.ErrInvalidReadSequence):
			botError(w, 422, "READ_SEQ_OUT_OF_RANGE")
		default:
			access.SessionError(w, err)
		}
		return
	}
	botJSON(w, 200, out)
}

// 不使用 map 覆盖解码：即使重复键内容相同也必须拒绝。
func decodeRead(w http.ResponseWriter, r *http.Request) (int64, bool) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		botError(w, 400, "INVALID_REQUEST")
		return 0, false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			botError(w, 413, "PAYLOAD_TOO_LARGE")
		} else {
			botError(w, 400, "INVALID_REQUEST")
		}
		return 0, false
	}
	invalid := func() (int64, bool) { botError(w, 400, "INVALID_REQUEST"); return 0, false }
	d := json.NewDecoder(bytes.NewReader(raw))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return invalid()
	}
	var value json.RawMessage
	found := false
	for d.More() {
		key, err := d.Token()
		if err != nil || key != "through_seq" || found {
			return invalid()
		}
		if d.Decode(&value) != nil {
			return invalid()
		}
		found = true
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || d.Decode(new(any)) != io.EOF {
		return invalid()
	}
	var s string
	if !found || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		botError(w, 422, "VALIDATION_FAILED")
		return 0, false
	}
	if json.Unmarshal(value, &s) != nil {
		return invalid()
	}
	seq, err := strconv.ParseInt(s, 10, 64)
	if err != nil || seq < 0 || strconv.FormatInt(seq, 10) != s {
		botError(w, 422, "VALIDATION_FAILED")
		return 0, false
	}
	return seq, true
}

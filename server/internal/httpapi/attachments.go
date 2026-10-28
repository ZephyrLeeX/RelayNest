package httpapi

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"relaynest/server/internal/attachments"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/idempotency"
	"relaynest/server/internal/storage"
)

// AttachmentHandler 只装配上传；文件状态转换与实际字节限制由存储状态机负责。
type AttachmentHandler struct {
	store     *storage.Store
	bots      *bots.Repository
	repo      *attachments.Repository
	files     *attachments.Files
	limits    attachments.Limits
	authorize BotAuthorize
}

func NewAttachmentHandler(store *storage.Store, files *attachments.Files, limits attachments.Limits, authorize BotAuthorize) *AttachmentHandler {
	return &AttachmentHandler{store, bots.New(store), attachments.New(store), files, limits, authorize}
}

func (h *AttachmentHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// 附件 Bot-first 例外只允许单一 Bearer 通道；Cookie 不能参与身份发现或降级。
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 || len(r.CookiesNamed("relaynest_session")) != 0 || len(headers[0]) <= 7 || !strings.EqualFold(headers[0][:7], "Bearer ") || strings.Count(headers[0], " ") != 1 || strings.ContainsAny(headers[0][7:], "\t\r\n") {
		botError(w, 401, "AUTHENTICATION_REQUIRED")
		return
	}
	if h == nil || h.store == nil || h.files == nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return
	}
	identity, match, err := h.bots.DiscoverAPIIdentity(r.Context(), headers[0][7:])
	if match == bots.APIUnavailable || errors.Is(err, bots.ErrAPIIdentityUnavailable) {
		probeError(w, 503, "INSTANCE_UNAVAILABLE", "实例暂不可用", true)
		return
	}
	var access BotAccess
	var write attachments.AuthorizedWrite
	scope := "administrator"
	switch match {
	case bots.APIMatched:
		if err != nil {
			messageError(w, err)
			return
		}
		if !identity.Capabilities().Send {
			botError(w, 403, "FORBIDDEN")
			return
		}
		scope = "bot:" + identity.BotID()
	case bots.APINoMatch:
		if err != nil {
			probeError(w, 503, "INSTANCE_UNAVAILABLE", "实例暂不可用", true)
			return
		}
		if h.authorize == nil {
			botError(w, 503, "SERVICE_UNAVAILABLE")
			return
		}
		var ok bool
		access, ok = h.authorize(w, r, true)
		if !ok {
			return
		}
		if access.Write == nil {
			botError(w, 503, "SERVICE_UNAVAILABLE")
			return
		}
	default:
		probeError(w, 503, "INSTANCE_UNAVAILABLE", "实例暂不可用", true)
		return
	}
	in, ok := attachmentInput(w, r)
	if !ok {
		return
	}
	in.Identity = scope
	if match == bots.APIMatched {
		// 不暴露目标 Bot 是否存在，且首次 staging 和最终 ready 均复核代次与 send。
		if in.BotID != identity.BotID() {
			botError(w, 404, "RESOURCE_NOT_FOUND")
			return
		}
		write = func(ctx context.Context, fn func(*sql.Tx) error) error {
			return h.store.Write(ctx, func(tx *sql.Tx) error {
				if err := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Upload); err != nil {
					return err
				}
				return fn(tx)
			})
		}
	} else {
		write = func(ctx context.Context, fn func(*sql.Tx) error) error {
			return access.Write(ctx, func(tx *sql.Tx) error {
				var id string
				err := tx.QueryRowContext(ctx, "SELECT id FROM bots WHERE id=?", in.BotID).Scan(&id)
				if errors.Is(err, sql.ErrNoRows) {
					return bots.ErrNotFound
				}
				if err != nil {
					return err
				}
				return fn(tx)
			})
		}
	}
	uploader, err := attachments.NewUploader(h.repo, h.files, h.limits, write)
	if err != nil {
		botError(w, 503, "SERVICE_UNAVAILABLE")
		return
	}
	out, err := uploader.Upload(r.Context(), in, r.Body)
	if err != nil {
		switch {
		case errors.Is(err, attachments.ErrConflict):
			botError(w, 409, "IDEMPOTENCY_CONFLICT")
		case errors.Is(err, attachments.ErrTooLarge):
			botError(w, 413, "PAYLOAD_TOO_LARGE")
		case errors.Is(err, attachments.ErrInvalid):
			botError(w, 422, "VALIDATION_FAILED")
		case errors.Is(err, bots.ErrUnauthorized), errors.Is(err, bots.ErrForbidden), errors.Is(err, bots.ErrNotFound):
			messageError(w, err)
		case errors.Is(err, attachments.ErrActive), errors.Is(err, attachments.ErrFenced), errors.Is(err, attachments.ErrNotReady):
			botError(w, 503, "SERVICE_UNAVAILABLE")
		default:
			if access.SessionError != nil {
				access.SessionError(w, err)
			} else {
				botError(w, 503, "SERVICE_UNAVAILABLE")
			}
		}
		return
	}
	botJSON(w, 201, attachmentResponse{out.ID, out.BotID, out.Kind, out.Filename, out.MediaType, strconv.FormatInt(out.Size, 10), hex.EncodeToString(out.SHA256), out.State, out.CreatedAt})
}

type attachmentResponse struct {
	ID        string `json:"id"`
	BotID     string `json:"bot_id"`
	Kind      string `json:"kind"`
	Filename  string `json:"filename"`
	MediaType string `json:"media_type"`
	Size      string `json:"size"`
	SHA256    string `json:"sha256"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

func attachmentInput(w http.ResponseWriter, r *http.Request) (attachments.Input, bool) {
	fail := func(status int, code string) (attachments.Input, bool) {
		botError(w, status, code)
		return attachments.Input{}, false
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return fail(400, "INVALID_REQUEST")
	}
	for name, values := range q {
		if len(values) != 1 || (name != "bot_id" && name != "kind" && name != "filename" && name != "media_type") {
			return fail(400, "INVALID_REQUEST")
		}
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !idempotency.ValidKey(keys[0]) {
		return fail(400, "INVALID_REQUEST")
	}
	types := r.Header.Values("Content-Type")
	if len(types) != 1 {
		return fail(400, "INVALID_REQUEST")
	}
	media, _, err := mime.ParseMediaType(types[0])
	if err != nil || media != "application/octet-stream" {
		return fail(400, "INVALID_REQUEST")
	}
	in := attachments.Input{Key: keys[0], BotID: q.Get("bot_id"), Kind: q.Get("kind"), Filename: q.Get("filename"), MediaType: q.Get("media_type")}
	_, _, err = mime.ParseMediaType(in.MediaType)
	if !botID(in.BotID) || (in.Kind != "image" && in.Kind != "file") || in.Filename == "" || len(in.Filename) > 4096 || !utf8.ValidString(in.Filename) || strings.ContainsRune(in.Filename, 0) || in.MediaType == "" || len(in.MediaType) > 256 || err != nil {
		return fail(422, "VALIDATION_FAILED")
	}
	return in, true
}

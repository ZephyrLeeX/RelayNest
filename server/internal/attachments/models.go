// Package attachments 管理附件卷及 staging 到 ready 的短事务状态转换。
package attachments

import (
	"context"
	"database/sql"
	"errors"
	"mime"
	"strings"
	"time"
	"unicode/utf8"

	"relaynest/server/internal/idempotency"
)

var (
	ErrInvalid  = errors.New("attachments: invalid input")
	ErrConflict = errors.New("attachments: conflicting upload")
	ErrActive   = errors.New("attachments: active upload")
	ErrFenced   = errors.New("attachments: expired or superseded attempt")
	ErrTooLarge = errors.New("attachments: size limit exceeded")
	ErrNotReady = errors.New("attachments: not ready")
)

// Input.Identity 必须来自可信认证层的稳定身份，不能使用凭据版本或会话 ID。
// 本包不提供 HTTP 入口；Identity 与 BotID 的授权由 AuthorizedWrite 负责。
type Input struct{ Identity, Key, BotID, Kind, Filename, MediaType string }
type Attachment struct {
	ID, BotID, Kind, Filename, MediaType, StorageKey, State string
	Size                                                    int64
	SHA256                                                  []byte
	CreatedAt                                               string
}
type Attempt struct {
	Attachment Attachment
	Token      string
	Generation int64
	Replay     bool
}

// AuthorizedWrite 每次调用都须在同一短写事务复核真实授权并提交；失败不得提交。
// 可封装 auth.WithAuthorizedWrite 或 storage.Write + bots.AuthorizeTx；不得缓存授权结论。
type AuthorizedWrite func(context.Context, func(*sql.Tx) error) error

type Limits struct{ Image, File int64 }

func DefaultLimits() Limits { return Limits{10 * 1024 * 1024, 50 * 1024 * 1024} }
func (l Limits) limit(kind string) (int64, error) {
	if l.Image <= 0 || l.File <= 0 {
		return 0, ErrInvalid
	}
	if kind == "image" {
		return l.Image, nil
	}
	if kind == "file" {
		return l.File, nil
	}
	return 0, ErrInvalid
}
func (in Input) valid() bool {
	if len(in.Identity) == 0 || len(in.Identity) > 256 || len(in.BotID) == 0 || len(in.Filename) == 0 || len(in.Filename) > 4096 || len(in.MediaType) == 0 || len(in.MediaType) > 256 || !utf8.ValidString(in.Filename) || strings.ContainsRune(in.Filename, 0) {
		return false
	}
	_, _, mimeErr := mime.ParseMediaType(in.MediaType)
	return idempotency.ValidKey(in.Key) && mimeErr == nil && (in.Kind == "image" || in.Kind == "file")
}

const defaultLease = time.Minute

package devices

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/storage"
)

// PushProvider 是冻结的通道集合，注册与提醒偏好相互独立。
type PushProvider string

const (
	PushFCM   PushProvider = "fcm"
	PushOPush PushProvider = "opush"
)

func (p PushProvider) valid() bool { return p == PushFCM || p == PushOPush }

var (
	ErrPushInput               = errors.New("devices: invalid push input")
	ErrAndroidRequired         = errors.New("devices: Android session required")
	ErrPushConflict            = errors.New("devices: registration conflict")
	ErrPushExhausted           = errors.New("devices: registration version exhausted")
	ErrSettingsInput           = errors.New("devices: invalid notification settings")
	ErrSettingsConflict        = errors.New("devices: notification settings conflict")
	ErrSettingsExhausted       = errors.New("devices: notification settings version exhausted")
	ErrNotificationBotNotFound = errors.New("devices: notification bot not found")
	ErrProviderNotReady        = errors.New("devices: provider not ready")
)

// VersionError 仅携带恢复元数据，HTTP 后包按冻结错误契约映射。
type VersionError struct {
	Cause         error
	LatestVersion int64
	RecoveryPath  string
}

func (e *VersionError) Error() string { return e.Cause.Error() }
func (e *VersionError) Unwrap() error { return e.Cause }

// PushRegistrationState 只含公开元数据，不含发送秘密。
type PushRegistrationState struct {
	SessionID     string             `json:"session_id"`
	Registrations []PushRegistration `json:"registrations"`
}
type PushRegistration struct {
	Provider            PushProvider `json:"provider"`
	RegistrationVersion int64        `json:"registration_version"`
	Registered          bool         `json:"registered"`
	UpdatedAt           *time.Time   `json:"updated_at"`
}
type PushRegistrationUpdate struct {
	Provider            PushProvider `json:"provider"`
	Token               string       `json:"token"`
	RegistrationVersion int64        `json:"registration_version"`
	ExpectedVersion     int64        `json:"expected_version"`
}

// MarshalJSON 保留输入类型的解码能力，但任何响应/调试编码都不携带 token。
func (in PushRegistrationUpdate) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Provider            PushProvider `json:"provider"`
		RegistrationVersion int64        `json:"registration_version"`
		ExpectedVersion     int64        `json:"expected_version"`
	}{in.Provider, in.RegistrationVersion, in.ExpectedVersion})
}

// Format 阻止常见调试格式意外记录写入凭据。
func (PushRegistrationUpdate) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "PushRegistrationUpdate{redacted}")
}

// VendorReadiness 由启动期可信服务端装配提供；必须为本地、短事务内的资格检查。
// nil 表示尚未核实平台资格，客户端请求不得自行提供该能力。
type VendorReadiness interface {
	Ready(context.Context, *sql.Tx) (bool, error)
}

// PushRepository 复用既有会话能力的鉴权事务，不接受设备或会话选择器。
type PushRepository struct {
	store    *storage.Store
	sessions *auth.SessionStore
	vendor   VendorReadiness
	now      func() time.Time
}

func NewPushRepository(store *storage.Store, sessions *auth.SessionStore, vendor VendorReadiness) *PushRepository {
	return &PushRepository{store: store, sessions: sessions, vendor: vendor, now: time.Now}
}
func (r *PushRepository) authorized(ctx context.Context, actor auth.AdminRequest, read bool, fn func(*sql.Tx, auth.Session) error) error {
	if r == nil || r.sessions == nil {
		return auth.ErrInvalidSession
	}
	work := func(tx *sql.Tx) error {
		// 此处仅在 SessionStore 已核验完整私有签发绑定后执行，公开 Session 不可选取其他身份。
		if actor.Session.ClientType != "android" {
			return ErrAndroidRequired
		}
		return fn(tx, actor.Session)
	}
	if read {
		_, _, err := r.sessions.WithAuthorizedRead(ctx, actor, work)
		return err
	}
	return r.sessions.WithAuthorizedWrite(ctx, actor, work)
}
func pushVersionError(cause error, latest int64) error {
	return &VersionError{cause, latest, "/devices/current/push"}
}
func settingsVersionError(cause error, latest int64) error {
	return &VersionError{cause, latest, "/devices/current/notifications"}
}

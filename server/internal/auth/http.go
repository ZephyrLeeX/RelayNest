package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

const sessionCookie = "relaynest_session"

// Handler 只适配三条认证入口；生命周期及来源策略由已审模块提供。
type Handler struct {
	store      *storage.Store
	sessions   *SessionStore
	origin     *httpapi.OriginPolicy
	protection *loginProtection
	logger     *slog.Logger
}

func NewHandler(store *storage.Store, sessions *SessionStore, origin *httpapi.OriginPolicy) *Handler {
	return &Handler{store: store, sessions: sessions, origin: origin, protection: newLoginProtection(), logger: slog.New(httpapi.NewSafeLogHandler(slog.Default().Handler()))}
}

type loginRequest struct {
	Username   string          `json:"username"`
	Password   string          `json:"password"`
	ClientType string          `json:"client_type"`
	DeviceName string          `json:"device_name"`
	DeviceID   json.RawMessage `json:"device_id,omitempty"`
}

func (in *loginRequest) UnmarshalJSON(data []byte) error {
	// 结构体解码接受大小写别名；先按原始键核对冻结 DTO，再沿用字段类型和重复精确键的解码行为。
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range fields {
		switch name {
		case "username", "password", "client_type", "device_name", "device_id":
		default:
			return errors.New("unknown login field")
		}
	}
	type plainLoginRequest loginRequest
	return json.Unmarshal(data, (*plainLoginRequest)(in))
}

type sessionDTO struct {
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

func respondSession(w http.ResponseWriter, s Session, secrets SessionSecrets, login bool) {
	dto := sessionDTO{ID: s.ID, ClientType: s.ClientType, DeviceID: s.DeviceID, DeviceName: s.DeviceName, IdleExpiresAt: s.IdleExpiresAt, AbsoluteExpiresAt: s.AbsoluteExpiresAt}
	dto.Administrator.Username = s.Username
	result := map[string]any{"session": dto}
	if s.ClientType == "web" {
		result["csrf_token"] = secrets.CSRFToken
	} else if login {
		result["access_token"] = secrets.Token
		result["token_type"] = "Bearer"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func authError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": http.StatusText(status), "request_id": rand.Text(), "retryable": status == 503}})
}

func (h *Handler) sessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidSession) || errors.Is(err, ErrNotInitialized) {
		authError(w, 401, "AUTHENTICATION_FAILED")
		return
	}
	authError(w, 503, "SERVICE_UNAVAILABLE")
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	defer func() { h.logger.InfoContext(r.Context(), "login", "duration", time.Since(started)) }()
	w.Header().Set("Cache-Control", "no-store")
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		authError(w, 400, "INVALID_REQUEST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var in loginRequest
	err = decoder.Decode(&in)
	if err == nil {
		var extra any
		if trailing := decoder.Decode(&extra); trailing != io.EOF {
			err = trailing
			if err == nil {
				err = errors.New("invalid trailing JSON")
			}
		}
	}
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			authError(w, 413, "PAYLOAD_TOO_LARGE")
		} else {
			authError(w, 400, "INVALID_REQUEST")
		}
		return
	}
	var deviceID string
	if len(in.DeviceID) > 0 {
		if err := json.Unmarshal(in.DeviceID, &deviceID); err != nil {
			authError(w, 400, "INVALID_REQUEST")
			return
		}
	}
	if in.Username == "" || in.Password == "" || in.DeviceName == "" || (in.ClientType != "web" && in.ClientType != "android") || (in.ClientType == "web" && len(in.DeviceID) > 0) || (in.ClientType == "android" && (len(in.DeviceID) == 0 || !uuidPattern.MatchString(deviceID))) {
		authError(w, 422, "VALIDATION_FAILED")
		return
	}
	var source httpapi.RequestSource
	if in.ClientType == "web" {
		source, err = h.origin.ValidateLogin(r)
	} else {
		source, err = h.origin.Resolve(r)
	}
	if err != nil {
		authError(w, 403, "FORBIDDEN")
		return
	}
	finish, retry := h.protection.begin(source.ClientIP, time.Now())
	if finish == nil {
		loginLimited(w, retry)
		return
	}
	failed := false
	defer func() { finish(failed) }()
	admin, err := ReadAdministrator(r.Context(), h.store)
	if err != nil {
		h.sessionError(w, err)
		return
	}
	// 保留密码校验读取的代次，重置抢先提交时 Create 必须失败。
	valid, available := tryVerifyPassword([]byte(in.Password), admin.PasswordHash)
	if !available {
		loginLimited(w, time.Second)
		return
	}
	if !valid || in.Username != admin.Username {
		failed = true
		authError(w, 401, "AUTHENTICATION_FAILED")
		return
	}
	input := SessionInput{ClientType: in.ClientType, DeviceName: in.DeviceName}
	if len(in.DeviceID) > 0 {
		input.DeviceID = deviceID
	}
	s, secrets, err := h.sessions.Create(r.Context(), admin.AuthGeneration, input)
	if err != nil {
		h.sessionError(w, err)
		return
	}
	if in.ClientType == "web" {
		writeCookie(w, secrets.Token, source.Secure, false)
	}
	respondSession(w, s, secrets, true)
}

// 认证通道决定客户端类型；拒绝重复凭据和混合通道，不能先猜测数据库内类型。
func credential(r *http.Request) (string, string, bool) {
	headers := r.Header.Values("Authorization")
	cookies := r.CookiesNamed(sessionCookie)
	if len(headers) > 1 || len(cookies) > 1 || (len(headers) > 0 && len(cookies) > 0) {
		return "", "", false
	}
	if len(headers) == 1 {
		parts := strings.Split(headers[0], " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return "", "", false
		}
		return parts[1], "android", true
	}
	if len(cookies) == 1 && cookies[0].Value != "" {
		return cookies[0].Value, "web", true
	}
	return "", "", false
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request, logout bool) {
	w.Header().Set("Cache-Control", "no-store")
	token, client, ok := credential(r)
	if !ok {
		authError(w, 401, "AUTHENTICATION_FAILED")
		return
	}
	var source httpapi.RequestSource
	var err error
	if logout && client == "web" {
		source, err = h.origin.ValidateWrite(r)
	} else {
		source, err = h.origin.Resolve(r)
	}
	if err != nil {
		authError(w, 403, "FORBIDDEN")
		return
	}
	// Inspect 不续期；CSRF 失败不能延长会话。Authenticate 再原子检查重置/撤销/到期。
	_, secrets, err := h.sessions.Inspect(r.Context(), token, client)
	if err != nil {
		h.sessionError(w, err)
		return
	}
	if logout && client == "web" {
		values := r.Header.Values("X-CSRF-Token")
		if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(secrets.CSRFToken)) != 1 {
			authError(w, 403, "FORBIDDEN")
			return
		}
	}
	if logout {
		if err = h.sessions.Revoke(r.Context(), token, client); err != nil {
			h.sessionError(w, err)
			return
		}
		if client == "web" {
			writeCookie(w, "", source.Secure, true)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s, secrets, err := h.sessions.Authenticate(r.Context(), token, client)
	if err != nil {
		h.sessionError(w, err)
		return
	}
	respondSession(w, s, secrets, false)
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) { h.current(w, r, false) }
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request)     { h.current(w, r, true) }

func writeCookie(w http.ResponseWriter, token string, secure, remove bool) {
	c := &http.Cookie{Name: sessionCookie, Value: token, Path: "/api/v1", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode}
	// Go 的 -1 对应线缆 Max-Age=0，清除当前浏览器 Cookie。
	if remove {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

func loginLimited(w http.ResponseWriter, retry time.Duration) {
	seconds := max(1, int((retry+time.Second-1)/time.Second))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	authError(w, http.StatusTooManyRequests, "RATE_LIMITED")
}

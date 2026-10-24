package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"relaynest/server/internal/httpapi"
)

// RequestMode 由路由按操作语义明确指定，不能依据客户端声明推断。
type RequestMode uint8

const (
	RequestRead RequestMode = iota + 1
	RequestWrite
)

// AdminRequest 是本次请求检查时的管理员身份快照；不得编码或记录整个对象。
// 成功不代表业务提交时仍有效。业务必须在自己的事务内按 Session.ID 重新
// 核对 actor 的撤销、期限、管理员代次及目标归属/有效性，再执行变更和续期。
type AdminRequest struct {
	Session Session
	Source  httpapi.RequestSource
	secrets SessionSecrets
	issuer  *RequestSecurity
	store   *SessionStore
	issued  Session
}

// ClearCookie 供成功自撤会话后清除 Web Cookie，沿用已解析的外部 Secure 信息。
func (a AdminRequest) ClearCookie(w http.ResponseWriter) {
	if a.Session.ClientType == "web" {
		writeCookie(w, "", a.Source.Secure, true)
	}
}

// RequestSecurity 复用认证端点的会话存储及来源政策，不提供业务事务授权保证。
type RequestSecurity struct {
	sessions *SessionStore
	origin   *httpapi.OriginPolicy
}

func NewRequestSecurity(sessions *SessionStore, origin *httpapi.OriginPolicy) *RequestSecurity {
	return &RequestSecurity{sessions: sessions, origin: origin}
}

// RenewRead 在普通管理读取成功后重新核对会话并续期；调用方先处理错误再输出响应。
// 它单独提交，不可代替业务写事务内的 actor/target 复核和续期。
func (s *RequestSecurity) RenewRead(ctx context.Context, actor AdminRequest) (Session, error) {
	// 先验证私有签发域及完整快照，不能让公开可变元数据选择身份或续期目标。
	if s == nil || s.sessions == nil || actor.issuer != s || actor.store != s.sessions || actor.issued.ID == "" || actor.Session != actor.issued {
		return Session{}, ErrInvalidSession
	}
	current, _, err := s.sessions.Inspect(ctx, actor.secrets.Token, actor.issued.ClientType)
	if err != nil {
		return Session{}, err
	}
	// Inspect 不续期；只有原身份匹配后才能调用会写库的 Authenticate。
	// idle 期限可由其他成功读取推进，其余身份及绝对期限必须保持一致。
	current.IdleExpiresAt = actor.issued.IdleExpiresAt
	if current != actor.issued {
		return Session{}, ErrInvalidSession
	}
	session, _, err := s.sessions.Authenticate(ctx, actor.secrets.Token, actor.issued.ClientType)
	return session, err
}

// Authorize 验证管理员请求；失败已写统一错误，调用方必须立即返回。
// 本入口仅 Inspect，不续期、不撤销、不发 Cookie；业务成功的续期由提交事务负责。
func (s *RequestSecurity) Authorize(w http.ResponseWriter, r *http.Request, mode RequestMode) (AdminRequest, bool) {
	return s.authorize(w, r, mode, "AUTHENTICATION_FAILED")
}

// AuthorizeRequired 沿用全部请求安全检查，仅按操作契约选择 REQUIRED 认证错误码。
func (s *RequestSecurity) AuthorizeRequired(w http.ResponseWriter, r *http.Request, mode RequestMode) (AdminRequest, bool) {
	return s.authorize(w, r, mode, "AUTHENTICATION_REQUIRED")
}

func (s *RequestSecurity) authorize(w http.ResponseWriter, r *http.Request, mode RequestMode, unauthorizedCode string) (AdminRequest, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if mode != RequestRead && mode != RequestWrite {
		authError(w, 503, "SERVICE_UNAVAILABLE")
		return AdminRequest{}, false
	}
	token, client, ok := credential(r)
	if !ok {
		authError(w, 401, unauthorizedCode)
		return AdminRequest{}, false
	}
	var source httpapi.RequestSource
	var err error
	if mode == RequestWrite && client == "web" {
		source, err = s.origin.ValidateWrite(r)
	} else {
		source, err = s.origin.Resolve(r)
	}
	if err != nil {
		authError(w, 403, "FORBIDDEN")
		return AdminRequest{}, false
	}
	actor, secrets, err := s.sessions.Inspect(r.Context(), token, client)
	if err != nil {
		writeSessionError(w, err, unauthorizedCode)
		return AdminRequest{}, false
	}
	if mode == RequestWrite && client == "web" {
		values := r.Header.Values("X-CSRF-Token")
		if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(secrets.CSRFToken)) != 1 {
			authError(w, 403, "FORBIDDEN")
			return AdminRequest{}, false
		}
	}
	return AdminRequest{Session: actor, Source: source, secrets: secrets, issuer: s, store: s.sessions, issued: actor}, true
}

// WriteSessionError 统一映射请求检查及业务提交前会话复核的错误；不清 Cookie。
func WriteSessionError(w http.ResponseWriter, err error) {
	writeSessionError(w, err, "AUTHENTICATION_FAILED")
}

// WriteSessionErrorRequired 与请求入口使用相同 REQUIRED 语义，不改变错误分类和 Cookie。
func WriteSessionErrorRequired(w http.ResponseWriter, err error) {
	writeSessionError(w, err, "AUTHENTICATION_REQUIRED")
}

func writeSessionError(w http.ResponseWriter, err error, unauthorizedCode string) {
	if errors.Is(err, ErrInvalidSession) || errors.Is(err, ErrNotInitialized) {
		authError(w, 401, unauthorizedCode)
		return
	}
	authError(w, 503, "SERVICE_UNAVAILABLE")
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

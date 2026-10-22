package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"relaynest/server/internal/storage"
)

var (
	ErrInvalidSession      = errors.New("auth: invalid session")
	ErrInvalidSessionInput = errors.New("auth: invalid session input")
	ErrSessionRandom       = errors.New("auth: session randomness failed")
	uuidPattern            = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

// Session 仅包含可公开的元数据，绝不包含密码、摘要或凭据。
type Session struct {
	ID                string
	Username          string
	ClientType        string
	DeviceID          string
	DeviceName        string
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// SessionSecrets 仅交给认证适配层；不得作为普通元数据编码或记录。
type SessionSecrets struct {
	Token     string `json:"-"`
	CSRFToken string `json:"-"`
}

type SessionInput struct {
	ClientType string
	DeviceID   string
	DeviceName string
}

// SessionStore 使用现有 SQLite 写事务串行化创建、鉴权续期和撤销。
// 注入时钟/随机源仅供测试；生产构造请使用 NewSessionStore。
type SessionStore struct {
	store  *storage.Store
	now    func() time.Time
	random io.Reader
}

func NewSessionStore(store *storage.Store) *SessionStore {
	return &SessionStore{store: store, now: time.Now, random: rand.Reader}
}

func sessionTTL(client string) (time.Duration, time.Duration) {
	if client == "web" {
		return 24 * time.Hour, 7 * 24 * time.Hour
	}
	return 30 * 24 * time.Hour, 90 * 24 * time.Hour
}

func (s *SessionStore) uuid() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(s.random, b[:]); err != nil {
		return "", ErrSessionRandom
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// Create 必须传入此次凭据校验所观察的 AuthGeneration；不能在校验后重新读取代次替代它。
func (s *SessionStore) Create(ctx context.Context, observedGeneration int64, in SessionInput) (Session, SessionSecrets, error) {
	if (in.ClientType != "web" && in.ClientType != "android") || in.DeviceName == "" || (in.ClientType == "android" && !uuidPattern.MatchString(in.DeviceID)) || (in.ClientType == "web" && in.DeviceID != "") {
		return Session{}, SessionSecrets{}, ErrInvalidSessionInput
	}
	id, err := s.uuid()
	if err != nil {
		return Session{}, SessionSecrets{}, err
	}
	if in.ClientType == "web" {
		in.DeviceID, err = s.uuid()
		if err != nil {
			return Session{}, SessionSecrets{}, err
		}
	}
	var raw [32]byte
	if _, err = io.ReadFull(s.random, raw[:]); err != nil {
		return Session{}, SessionSecrets{}, ErrSessionRandom
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	var result Session
	err = s.store.Write(ctx, func(tx *sql.Tx) error {
		var generation int64
		var username string
		if err := tx.QueryRowContext(ctx, "SELECT username,auth_generation FROM administrator WHERE id=1").Scan(&username, &generation); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidSession
			}
			return err
		}
		if generation != observedGeneration {
			return ErrInvalidSession
		}
		now := s.now().UTC()
		idle, absolute := sessionTTL(in.ClientType)
		result = Session{id, username, in.ClientType, in.DeviceID, in.DeviceName, now.Add(idle), now.Add(absolute)}
		_, err := tx.ExecContext(ctx, "INSERT INTO sessions(id,token_hash,auth_generation,client_type,device_id,device_name,idle_expires_at,absolute_expires_at) VALUES(?,?,?,?,?,?,?,?)", id, hash[:], generation, in.ClientType, in.DeviceID, in.DeviceName, result.IdleExpiresAt.UnixNano(), result.AbsoluteExpiresAt.UnixNano())
		return err
	})
	if err != nil {
		return Session{}, SessionSecrets{}, sessionError(err)
	}
	return result, secrets(token, in.ClientType), nil
}

func secrets(token, client string) SessionSecrets {
	result := SessionSecrets{Token: token}
	// 域分离派生可在重启后 bootstrap；数据库不增加可用于会话认证的秘密。
	if client == "web" {
		h := hmac.New(sha256.New, []byte(token))
		h.Write([]byte("relaynest.csrf.v1"))
		result.CSRFToken = base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	}
	return result
}

// Authenticate 在已完成其它请求校验后调用，成功才滑动续期。
// expectedClient 由 HTTP 凭据通道决定，防止跨 Cookie/Bearer 通道使用。
func (s *SessionStore) Authenticate(ctx context.Context, token, expectedClient string) (Session, SessionSecrets, error) {
	return s.access(ctx, token, expectedClient, false, true)
}

// Inspect 仅检查当前有效性和提供 bootstrap，不续期；其它校验通过后仍须
// Authenticate 再次核对数据库并续期，不能把 Inspect 结果作为后续有效性保证。
func (s *SessionStore) Inspect(ctx context.Context, token, expectedClient string) (Session, SessionSecrets, error) {
	return s.access(ctx, token, expectedClient, false, false)
}

// Revoke 仅撤销该凭据指向的当前会话，不续期且不影响其它设备/Bot。
func (s *SessionStore) Revoke(ctx context.Context, token, expectedClient string) error {
	_, _, err := s.access(ctx, token, expectedClient, true, false)
	return err
}

func (s *SessionStore) access(ctx context.Context, token, client string, revoke, renew bool) (Session, SessionSecrets, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token || (client != "web" && client != "android") {
		return Session{}, SessionSecrets{}, ErrInvalidSession
	}
	hash := sha256.Sum256([]byte(token))
	var result Session
	err = s.store.Write(ctx, func(tx *sql.Tx) error {
		var idle, absolute int64
		err := tx.QueryRowContext(ctx, `SELECT s.id,a.username,s.client_type,s.device_id,s.device_name,s.idle_expires_at,s.absolute_expires_at FROM sessions s JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation WHERE s.token_hash=? AND s.client_type=? AND s.revoked=0`, hash[:], client).Scan(&result.ID, &result.Username, &result.ClientType, &result.DeviceID, &result.DeviceName, &idle, &absolute)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidSession
		}
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if now.UnixNano() >= idle || now.UnixNano() >= absolute {
			return ErrInvalidSession
		}
		result.AbsoluteExpiresAt = time.Unix(0, absolute).UTC()
		if revoke {
			_, err = tx.ExecContext(ctx, "UPDATE sessions SET revoked=1 WHERE id=?", result.ID)
		} else if renew {
			ttl, _ := sessionTTL(client)
			next := now.Add(ttl).UnixNano()
			if next > absolute {
				next = absolute
			}
			if next < idle {
				next = idle
			}
			idle = next
			_, err = tx.ExecContext(ctx, "UPDATE sessions SET idle_expires_at=? WHERE id=?", idle, result.ID)
		}
		result.IdleExpiresAt = time.Unix(0, idle).UTC()
		return err
	})
	if err != nil {
		return Session{}, SessionSecrets{}, sessionError(err)
	}
	return result, secrets(token, client), nil
}

func sessionError(err error) error {
	if errors.Is(err, ErrInvalidSession) {
		return ErrInvalidSession
	}
	return ErrStorage
}

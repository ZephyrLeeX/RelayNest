package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrSessionNotFound = errors.New("auth: session not found")
var ErrInvalidSessionPage = errors.New("auth: invalid session page")

// DeviceSessionPage 仅返回公开元数据；分页不承诺跨页快照。
type DeviceSessionPage struct {
	Items      []Session
	NextCursor *string
	HasMore    bool
}

// WithAuthorizedWrite 固定在自身 Store 的唯一写事务内鉴权、执行业务并复核续期。
// fn 是受信业务代码，须同步使用 tx、关闭 Rows，不得自行提交、回滚或保留事务。
// 业务和 changes 必须使用所给 tx；仅返回 nil 后才能发布结果或明文凭据。
// 此入口不隔离恶意业务 SQL；设备自撤必须使用 RevokeDeviceSession 的专用路径。
func (s *SessionStore) WithAuthorizedWrite(ctx context.Context, actor AdminRequest, fn func(*sql.Tx) error) error {
	if s == nil || s.store == nil || fn == nil {
		return ErrInvalidSession
	}
	var businessErr error
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
			return err
		}
		if businessErr = fn(tx); businessErr != nil {
			return businessErr
		}
		_, err := s.renewTx(ctx, tx, actor)
		return err
	})
	// 保留业务/CAS 错误身份；鉴权、事务创建和提交错误沿既有会话错误语义返回。
	if businessErr != nil {
		return businessErr
	}
	if err != nil {
		return sessionError(err)
	}
	return nil
}

// authorizeTx 仅供 auth 内部使用；事务必须由当前 SessionStore 自身创建。
func (s *SessionStore) authorizeTx(ctx context.Context, tx *sql.Tx, actor AdminRequest) (Session, error) {
	return s.authorizeStateTx(ctx, tx, actor, 0)
}

func (s *SessionStore) authorizeStateTx(ctx context.Context, tx *sql.Tx, actor AdminRequest, revokedState int) (Session, error) {
	if s == nil || tx == nil || actor.store != s || actor.issuer == nil || actor.issuer.sessions != s || actor.issued.ID == "" || actor.Session != actor.issued {
		return Session{}, ErrInvalidSession
	}
	if err := s.checkBindingTx(ctx, tx, actor); err != nil {
		return Session{}, err
	}
	current, err := s.accessStateTx(ctx, tx, actor.secrets.Token, actor.issued.ClientType, false, false, revokedState)
	if err != nil {
		return Session{}, sessionError(err)
	}
	compare := current
	compare.IdleExpiresAt = actor.issued.IdleExpiresAt
	if compare != actor.issued {
		return Session{}, ErrInvalidSession
	}
	return current, nil
}

// renewTx 仅供内部受控事务在业务和 changes 写入后复核并续期。
func (s *SessionStore) renewTx(ctx context.Context, tx *sql.Tx, actor AdminRequest) (Session, error) {
	if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
		return Session{}, err
	}
	current, err := s.accessTx(ctx, tx, actor.secrets.Token, actor.issued.ClientType, false, true)
	if err != nil {
		return Session{}, sessionError(err)
	}
	// 续期 UPDATE 也可能触发业务约束或代次变动，提交前最后复核。
	if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
		return Session{}, err
	}
	return current, nil
}

// SetRevocationHook 设置提交后的精确 ID 通知；回调应并发安全。实际 WS/provider
// 持久化失效仍须后续接入；本通知不能替代持久化安全。
func (s *SessionStore) SetRevocationHook(hook func(string)) {
	s.hookMu.Lock()
	defer s.hookMu.Unlock()
	s.revokedHook = hook
}
func (s *SessionStore) notifyRevoked(id string) {
	s.hookMu.RLock()
	h := s.revokedHook
	s.hookMu.RUnlock()
	if h != nil {
		h(id)
	}
}

// ListDeviceSessions 按不可复用 Session.id 升序；默认50，最大100，大 limit 限至100。
func (s *SessionStore) ListDeviceSessions(ctx context.Context, actor AdminRequest, cursor string, limit int) (DeviceSessionPage, error) {
	page := DeviceSessionPage{Items: []Session{}}
	if limit < 0 {
		return page, ErrInvalidSessionPage
	}
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	after := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || base64.RawURLEncoding.EncodeToString(raw) != cursor || !strings.HasPrefix(string(raw), "v1:") || !uuidPattern.MatchString(string(raw)[3:]) {
			return page, ErrInvalidSessionPage
		}
		after = string(raw)[3:]
	}
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
			return err
		}
		now := s.now().UTC().UnixNano()
		rows, err := tx.QueryContext(ctx, `SELECT s.id,a.username,s.client_type,s.device_id,s.device_name,s.idle_expires_at,s.absolute_expires_at FROM sessions s JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation WHERE s.revoked=0 AND s.idle_expires_at>? AND s.absolute_expires_at>? AND s.id>? ORDER BY s.id LIMIT ?`, now, now, after, limit+1)
		if err != nil {
			return err
		}
		for rows.Next() {
			var v Session
			var idle, absolute int64
			if err := rows.Scan(&v.ID, &v.Username, &v.ClientType, &v.DeviceID, &v.DeviceName, &idle, &absolute); err != nil {
				rows.Close()
				return err
			}
			v.IdleExpiresAt = time.Unix(0, idle).UTC()
			v.AbsoluteExpiresAt = time.Unix(0, absolute).UTC()
			page.Items = append(page.Items, v)
		}
		err = rows.Err()
		ce := rows.Close()
		if err != nil {
			return err
		}
		if ce != nil {
			return ce
		}
		if len(page.Items) > limit {
			page.HasMore = true
			page.Items = page.Items[:limit]
			next := base64.RawURLEncoding.EncodeToString([]byte("v1:" + page.Items[limit-1].ID))
			page.NextCursor = &next
		}
		_, err = s.renewTx(ctx, tx, actor)
		return err
	})
	if err != nil {
		return DeviceSessionPage{}, deviceSessionError(err)
	}
	return page, nil
}

// RevokeDeviceSession 精确撤销 Session.id，绝不使用安装 ID 回退选择。失败不通知。
func (s *SessionStore) RevokeDeviceSession(ctx context.Context, actor AdminRequest, id string) error {
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
			return err
		}
		now := s.now().UTC().UnixNano()
		result, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked=1 WHERE id=? AND revoked=0 AND idle_expires_at>? AND absolute_expires_at>? AND auth_generation=(SELECT auth_generation FROM administrator WHERE id=1)`, id, now, now)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrSessionNotFound
		}
		// 自撤也在末尾复核期限、代次和身份，但只接受本次写入的 revoked=1。
		// 其他目标复核并续期；任一失败将目标写入一起回滚。
		if id != actor.issued.ID {
			_, err = s.renewTx(ctx, tx, actor)
		} else {
			_, err = s.authorizeStateTx(ctx, tx, actor, 1)
		}
		if err != nil {
			return err
		}
		// 用同一末尾时刻复核双方，防止续期期间目标刚到期；自撤也不跳过 actor 到期。
		now = s.now().UTC().UnixNano()
		actorRevoked := 0
		if id == actor.issued.ID {
			actorRevoked = 1
		}
		for _, check := range []struct {
			id      string
			revoked int
			invalid error
		}{{actor.issued.ID, actorRevoked, ErrInvalidSession}, {id, 1, ErrSessionNotFound}} {
			var valid bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id=? AND revoked=? AND idle_expires_at>? AND absolute_expires_at>? AND auth_generation=(SELECT auth_generation FROM administrator WHERE id=1))`, check.id, check.revoked, now, now).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return check.invalid
			}
		}
		return nil
	})
	if err != nil {
		return deviceSessionError(err)
	}
	s.notifyRevoked(id)
	return nil
}

func deviceSessionError(err error) error {
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrInvalidSessionPage) {
		return err
	}
	return sessionError(err)
}

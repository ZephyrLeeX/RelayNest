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

// AuthorizeTx 在已有写事务复核身份，不续期或提交。tx 必须由同一 storage.Store.Write
// 提供；裸 sql.Tx 无来源查询，可信 Go 调用方负责同库接线并传播错误以回滚。
func (s *SessionStore) AuthorizeTx(ctx context.Context, tx *sql.Tx, actor AdminRequest) (Session, error) {
	return s.authorizeStateTx(ctx, tx, actor, 0)
}

func (s *SessionStore) authorizeStateTx(ctx context.Context, tx *sql.Tx, actor AdminRequest, revokedState int) (Session, error) {
	if s == nil || tx == nil || actor.store != s || actor.issuer == nil || actor.issuer.sessions != s || actor.issued.ID == "" || actor.Session != actor.issued {
		return Session{}, ErrInvalidSession
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

// RenewTx 在业务和 changes 写入后、同一事务提交前复核并续期；错误必须传播回滚。
func (s *SessionStore) RenewTx(ctx context.Context, tx *sql.Tx, actor AdminRequest) (Session, error) {
	if _, err := s.AuthorizeTx(ctx, tx, actor); err != nil {
		return Session{}, err
	}
	current, err := s.accessTx(ctx, tx, actor.secrets.Token, actor.issued.ClientType, false, true)
	if err != nil {
		return Session{}, sessionError(err)
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
		if _, err := s.AuthorizeTx(ctx, tx, actor); err != nil {
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
		_, err = s.RenewTx(ctx, tx, actor)
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
		if _, err := s.AuthorizeTx(ctx, tx, actor); err != nil {
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
			_, err = s.RenewTx(ctx, tx, actor)
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

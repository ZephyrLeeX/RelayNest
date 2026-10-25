package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"relaynest/server/internal/storage"
)

const instanceHeader = "X-RelayNest-Instance-ID"

var errInstanceRequired = errors.New("auth: instance required")
var errInstanceInvalid = errors.New("auth: invalid instance header")

// instanceBinding 仅由认证入口签发；公开 actor 字段不能改变期望实例或认证代次。
type instanceBinding struct {
	expectedID string
	client     string
	sessionID  string
	generation int64
	store      *SessionStore
}

func (s *SessionStore) expectedInstance(ctx context.Context, r *http.Request) (string, error) {
	var values []string
	present := false
	for name, v := range r.Header {
		if strings.EqualFold(name, instanceHeader) {
			present = true
			values = append(values, v...)
		}
	}
	if !present {
		return "", errInstanceRequired
	}
	if len(values) != 1 || !uuidPattern.MatchString(values[0]) || values[0] != strings.ToLower(values[0]) {
		return "", errInstanceInvalid
	}
	expected := values[0]
	err := s.store.Read(ctx, func(tx *sql.Tx) error { return checkInstanceTx(ctx, tx, expected) })
	if err != nil {
		return "", instanceError(err)
	}
	return expected, nil
}

func checkInstanceTx(ctx context.Context, tx *sql.Tx, expected string) error {
	id, err := storage.ReadInstanceIDTx(ctx, tx)
	if err != nil {
		return err
	}
	if id != expected {
		return storage.ErrInstanceMismatch
	}
	return nil
}

func instanceError(err error) error {
	if errors.Is(err, storage.ErrInstanceMismatch) {
		return storage.ErrInstanceMismatch
	}
	return storage.ErrInstanceUnavailable
}

// WithAuthorizedRead 在当前 Store 控制的同一事务内完成身份、会话和业务投影。
// fn 必须同步关闭 Rows，不得提交/保留 tx；闭包候选结果仅在本方法成功后发布。
// 读取成功会续期，因此使用写事务，并在业务结束后重新核对绑定。
func (s *SessionStore) WithAuthorizedRead(ctx context.Context, actor AdminRequest, fn func(*sql.Tx) error) (Session, string, error) {
	if s == nil || s.store == nil || fn == nil {
		return Session{}, "", ErrInvalidSession
	}
	var result Session
	var id string
	var businessErr error
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
			return err
		}
		var err error
		id, err = storage.ReadInstanceIDTx(ctx, tx)
		if err != nil {
			return err
		}
		if businessErr = fn(tx); businessErr != nil {
			return businessErr
		}
		result, err = s.renewTx(ctx, tx, actor)
		return err
	})
	if businessErr != nil {
		return Session{}, "", businessErr
	}
	if err != nil {
		return Session{}, "", sessionError(err)
	}
	return result, id, nil
}

func (s *SessionStore) inspectBound(ctx context.Context, token, client, expected string, issuer *RequestSecurity) (AdminRequest, error) {
	var actor AdminRequest
	err := s.store.Read(ctx, func(tx *sql.Tx) error {
		if client == "android" {
			if err := checkInstanceTx(ctx, tx, expected); err != nil {
				return err
			}
		} else {
			// Web 忽略请求实例头，但事务仍绑定自身读取的真实部署身份。
			var err error
			expected, err = storage.ReadInstanceIDTx(ctx, tx)
			if err != nil {
				return err
			}
		}
		current, err := s.accessTx(ctx, tx, token, client, false, false)
		if err != nil {
			return err
		}
		var generation int64
		if err := tx.QueryRowContext(ctx, "SELECT auth_generation FROM administrator WHERE id=1").Scan(&generation); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidSession
			}
			return err
		}
		actor = AdminRequest{Session: current, secrets: secrets(token, client), issuer: issuer, store: s, issued: current,
			binding: &instanceBinding{expectedID: expected, client: client, sessionID: current.ID, generation: generation, store: s}}
		return nil
	})
	if err != nil {
		return AdminRequest{}, sessionError(err)
	}
	return actor, nil
}

func (s *SessionStore) checkBindingTx(ctx context.Context, tx *sql.Tx, actor AdminRequest) error {
	b := actor.binding
	if b == nil || b.store != s || b.client != actor.issued.ClientType || b.sessionID != actor.issued.ID {
		return ErrInvalidSession
	}
	if err := checkInstanceTx(ctx, tx, b.expectedID); err != nil {
		return err
	}
	var generation int64
	if err := tx.QueryRowContext(ctx, "SELECT auth_generation FROM administrator WHERE id=1").Scan(&generation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidSession
		}
		return err
	}
	if generation != b.generation {
		return ErrInvalidSession
	}
	return nil
}

func (s *SessionStore) revokeBound(ctx context.Context, actor AdminRequest) error {
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		if _, err := s.authorizeTx(ctx, tx, actor); err != nil {
			return err
		}
		if _, err := s.accessTx(ctx, tx, actor.secrets.Token, actor.issued.ClientType, true, false); err != nil {
			return err
		}
		_, err := s.authorizeStateTx(ctx, tx, actor, 1)
		return err
	})
	if err != nil {
		return sessionError(err)
	}
	s.notifyRevoked(actor.issued.ID)
	return nil
}

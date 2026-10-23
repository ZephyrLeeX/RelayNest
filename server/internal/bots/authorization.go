package bots

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
)

type Kind string

const (
	APIToken   Kind = "api_token"
	WebhookKey Kind = "webhook_key"
)

type Operation string

const (
	Send     Operation = "send"
	Edit     Operation = "edit"
	Upload   Operation = "upload"
	Claim    Operation = "claim"
	ACK      Operation = "ack"
	Renew    Operation = "renew"
	Download Operation = "download"
	Manage   Operation = "manage"
)

// Identity 的绑定字段不可由调用方构造或修改。它不是管理员会话，也不是提交授权缓存。
type Identity struct {
	botID        string
	kind         Kind
	credentialID string
	generation   int64
	capabilities Capabilities
}

func (i Identity) Capabilities() Capabilities { return i.capabilities }

func (i Identity) BotID() string     { return i.botID }
func (i Identity) Kind() Kind        { return i.kind }
func (i Identity) Generation() int64 { return i.generation }

func (r *Repository) Authenticate(ctx context.Context, botID string, kind Kind, secret string) (Identity, error) {
	if botID == "" || (kind != APIToken && kind != WebhookKey) {
		return Identity{}, ErrUnauthorized
	}
	id, tail, ok := strings.Cut(secret, ".")
	if !ok || len(id) != 64 || len(tail) != 64 {
		return Identity{}, ErrUnauthorized
	}
	var identity Identity
	err := r.store.Read(ctx, func(tx *sql.Tx) error {
		var digest []byte
		var enabled bool
		var caps Capabilities
		var generation int64
		err := tx.QueryRowContext(ctx, `SELECT c.digest,c.generation,b.enabled,b.can_send,b.can_receive FROM bot_credentials c JOIN bots b ON b.id=c.bot_id WHERE c.bot_id=? AND c.kind=? AND c.credential_id=?`, botID, kind, id).Scan(&digest, &generation, &enabled, &caps.Send, &caps.Receive)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnauthorized
		}
		if err != nil {
			return err
		}
		supplied := sha256.Sum256([]byte(secret))
		if subtle.ConstantTimeCompare(digest, supplied[:]) != 1 {
			return ErrUnauthorized
		}
		if !enabled {
			return ErrForbidden
		}
		identity = Identity{botID, kind, id, generation, caps}
		return nil
	})
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// AuthorizeTx 必须在业务写入使用的同一个 Store.Write 事务内调用，并传播错误。
// SQLite 写事务串行化使停用/能力/凭据撤销与提交有明确先后；不得在外部校验后跨事务复用结果。
// 长轮询在领取/ACK/renew 的最终短事务中再次调用，不能跨网络等待持有事务。
func AuthorizeTx(ctx context.Context, tx *sql.Tx, i Identity, botID string, op Operation) error {
	if tx == nil {
		return ErrInvalid
	}
	if i.botID == "" || i.botID != botID || (i.kind != APIToken && i.kind != WebhookKey) {
		return ErrForbidden
	}
	var enabled, send, receive bool
	err := tx.QueryRowContext(ctx, `SELECT b.enabled,b.can_send,b.can_receive FROM bots b JOIN bot_credentials c ON c.bot_id=b.id WHERE b.id=? AND c.kind=? AND c.credential_id=? AND c.generation=?`, botID, i.kind, i.credentialID, i.generation).Scan(&enabled, &send, &receive)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}
	if !enabled {
		return ErrForbidden
	}
	send = send && i.capabilities.Send
	receive = receive && i.capabilities.Receive
	if i.kind == WebhookKey {
		if op == Send && send {
			return nil
		}
		return ErrForbidden
	}
	switch op {
	case Send, Edit, Upload:
		if send {
			return nil
		}
	case Claim, ACK, Renew, Download:
		if receive {
			return nil
		}
	}
	return ErrForbidden
}

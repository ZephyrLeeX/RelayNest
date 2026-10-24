package bots

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
)

type Repository struct {
	store *storage.Store
	// 默认沿用 issue；实例局部注入只供确定性随机故障测试，不修改全局随机源。
	issueCredential func() (CredentialMetadata, string, []byte, error)
}

func New(store *storage.Store) *Repository { return &Repository{store: store, issueCredential: issue} }

func randomID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func issue() (CredentialMetadata, string, []byte, error) {
	id, err := randomID()
	if err != nil {
		return CredentialMetadata{}, "", nil, err
	}
	secret, err := randomID()
	if err != nil {
		return CredentialMetadata{}, "", nil, err
	}
	token := id + "." + secret
	digest := sha256.Sum256([]byte(token))
	return CredentialMetadata{id, 1}, token, digest[:], nil
}

// WriteFunc 同步运行受信业务回调，只在整个事务成功提交后返回 nil。
type WriteFunc func(context.Context, func(*sql.Tx) error) error

// Create 只有外层事务提交成功后才释放一次性凭据及全局序号。
func (r *Repository) Create(ctx context.Context, req CreateRequest) (Created, changes.Sequence, error) {
	return r.CreateWithWrite(ctx, req, r.store.Write)
}

// CreateWithWrite 将完整业务交给管理员会话拥有的事务；write 必须同步执行且仅提交一次。
// 明文仅在整个授权、业务及续期事务提交成功后释放。
func (r *Repository) CreateWithWrite(ctx context.Context, req CreateRequest, write WriteFunc) (Created, changes.Sequence, error) {
	if write == nil || req.Name == "" {
		return Created{}, 0, ErrInvalid
	}
	id, err := newBotID()
	if err != nil {
		return Created{}, 0, err
	}
	api, token, ad, err := issue()
	if err != nil {
		return Created{}, 0, err
	}
	wh, key, wd, err := issue()
	if err != nil {
		return Created{}, 0, err
	}
	b := Bot{ID: id, Name: req.Name, Enabled: req.Enabled, Capabilities: req.Capabilities, Version: 1, Credentials: Credentials{api, wh}, Connection: Connection{State: "unknown"}}
	var seq changes.Sequence
	err = write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO bots(id,name,enabled,can_send,can_receive,version) VALUES(?,?,?,?,?,1)", id, b.Name, b.Enabled, b.Capabilities.Send, b.Capabilities.Receive); err != nil {
			return err
		}
		for _, c := range []struct {
			kind   Kind
			meta   CredentialMetadata
			digest []byte
		}{{APIToken, api, ad}, {WebhookKey, wh, wd}} {
			if _, err := tx.ExecContext(ctx, "INSERT INTO bot_credentials(bot_id,kind,credential_id,generation,digest) VALUES(?,?,?,?,?)", id, c.kind, c.meta.CredentialID, c.meta.Generation, c.digest); err != nil {
				return err
			}
		}
		seq, err = changes.Append(ctx, tx, changes.Metadata{Type: changes.BotUpsert, EntityID: id, Revision: 1, BotID: &id, CreatedAt: time.Now()})
		return err
	})
	if err != nil {
		return Created{}, 0, err
	}
	return Created{b, IssuedAPIToken{api, token, b.Capabilities}, IssuedWebhookKey{wh, key}}, seq, nil
}

const botSelect = `SELECT b.id,b.name,b.enabled,b.can_send,b.can_receive,b.version,a.credential_id,a.generation,w.credential_id,w.generation FROM bots b JOIN bot_credentials a ON a.bot_id=b.id AND a.kind='api_token' JOIN bot_credentials w ON w.bot_id=b.id AND w.kind='webhook_key'`

func get(ctx context.Context, tx *sql.Tx, id string) (Bot, error) {
	var b Bot
	err := tx.QueryRowContext(ctx, botSelect+" WHERE b.id=?", id).Scan(&b.ID, &b.Name, &b.Enabled, &b.Capabilities.Send, &b.Capabilities.Receive, &b.Version, &b.Credentials.APIToken.CredentialID, &b.Credentials.APIToken.Generation, &b.Credentials.WebhookKey.CredentialID, &b.Credentials.WebhookKey.Generation)
	if errors.Is(err, sql.ErrNoRows) {
		return Bot{}, ErrNotFound
	}
	b.Connection.State = "unknown"
	return b, err
}
func (r *Repository) Get(ctx context.Context, id string) (Bot, error) {
	var b Bot
	err := r.store.Read(ctx, func(tx *sql.Tx) error { var e error; b, e = get(ctx, tx, id); return e })
	if err != nil {
		return Bot{}, err
	}
	return b, nil
}

// List 在一个短读事务返回公开投影；分页由后续 HTTP 层按最终路由契约接入。
func (r *Repository) List(ctx context.Context) ([]Bot, error) {
	out := []Bot{}
	err := r.store.Read(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT id FROM bots ORDER BY id")
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		if e := rows.Close(); err == nil {
			err = e
		}
		if err != nil {
			return err
		}
		for _, id := range ids {
			b, e := get(ctx, tx, id)
			if e != nil {
				return e
			}
			out = append(out, b)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Patch 是管理员仓储能力，不能由 Bot Identity 授予；调用方须先通过独立管理员会话。
func (r *Repository) Patch(ctx context.Context, id string, req PatchRequest) (Bot, changes.Sequence, error) {
	return r.PatchWithWrite(ctx, id, req, r.store.Write)
}

// PatchWithWrite 保持目标检查、CAS、changes 与管理员续期处于同一受控事务。
func (r *Repository) PatchWithWrite(ctx context.Context, id string, req PatchRequest, write WriteFunc) (Bot, changes.Sequence, error) {
	return r.PatchWithLifecycle(ctx, id, req, write, nil)
}

// PatchWithLifecycle 为后续队列适配提供同事务扩展，现有调用保持兼容。
func (r *Repository) PatchWithLifecycle(ctx context.Context, id string, req PatchRequest, write WriteFunc, hook LifecycleFunc) (Bot, changes.Sequence, error) {
	if write == nil || req.ExpectedVersion < 1 || (req.Name != nil && *req.Name == "") {
		return Bot{}, 0, ErrInvalid
	}
	var b Bot
	var seq changes.Sequence
	err := write(ctx, func(tx *sql.Tx) error {
		var err error
		b, err = get(ctx, tx, id)
		if err != nil {
			return err
		}
		if b.Version != req.ExpectedVersion {
			return ErrConflict
		}
		if b.Version == math.MaxInt64 {
			return ErrConflict
		}
		before := b
		if req.Name != nil {
			b.Name = *req.Name
		}
		if req.Enabled != nil {
			b.Enabled = *req.Enabled
		}
		if req.Capabilities != nil {
			b.Capabilities = *req.Capabilities
		}
		result, err := tx.ExecContext(ctx, "UPDATE bots SET name=?,enabled=?,can_send=?,can_receive=?,version=version+1 WHERE id=? AND version=?", b.Name, b.Enabled, b.Capabilities.Send, b.Capabilities.Receive, id, req.ExpectedVersion)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrConflict
		}
		b.Version++
		if hook != nil {
			if err := hook(ctx, tx, LifecycleChange{Action: LifecyclePatch, Before: before, After: b}); err != nil {
				return err
			}
		}
		seq, err = changes.Append(ctx, tx, changes.Metadata{Type: changes.BotUpsert, EntityID: id, Revision: b.Version, BotID: &id, CreatedAt: time.Now()})
		return err
	})
	if err != nil {
		return Bot{}, 0, err
	}
	return b, seq, nil
}

// Bot 实体遵循公共 UUID v4 标识契约；凭据标识使用独立随机命名空间。
func newBotID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ListPage 在短读事务内按稳定 ID 顺序读取有界公开投影；不携带消息历史或秘密。
func (r *Repository) ListPage(ctx context.Context, after string, limit int) ([]Bot, error) {
	if limit < 1 || limit > 101 {
		return nil, ErrInvalid
	}
	out := []Bot{}
	err := r.store.Read(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT id FROM bots WHERE id>? ORDER BY id LIMIT ?", after, limit)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		if e := rows.Close(); err == nil {
			err = e
		}
		if err != nil {
			return err
		}
		for _, id := range ids {
			b, e := get(ctx, tx, id)
			if e != nil {
				return e
			}
			out = append(out, b)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

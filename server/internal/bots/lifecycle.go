package bots

import (
	"context"
	"database/sql"
	"math"
	"time"

	"relaynest/server/internal/changes"
)

type LifecycleAction string

const (
	LifecyclePatch          LifecycleAction = "patch"
	LifecycleRotateAPIToken LifecycleAction = "rotate_api_token"
)

// LifecycleChange 只有公开元数据，前后快照明确目标、状态及凭据代次。
type LifecycleChange struct {
	Action LifecycleAction
	Before Bot
	After  Bot
}

// LifecycleFunc 仅供受信适配层同步使用给定 Tx；错误必须传播以回滚整笔操作。
// 不得保留 Tx、嵌套写事务、执行网络等待或自行提交/回滚。
// 后续队列实现负责解释动作；此处不定义租约 schema 或完整 HTTP 级联。
type LifecycleFunc func(context.Context, *sql.Tx, LifecycleChange) error

func (r *Repository) RotateAPIToken(ctx context.Context, id string) (IssuedAPIToken, changes.Sequence, error) {
	return r.RotateAPITokenWithWrite(ctx, id, r.store.Write, nil)
}

// RotateAPITokenWithWrite 的 hook 与凭据、版本、changes、调用方续期共用一次提交。
func (r *Repository) RotateAPITokenWithWrite(ctx context.Context, id string, write WriteFunc, hook LifecycleFunc) (IssuedAPIToken, changes.Sequence, error) {
	b, meta, secret, seq, err := r.rotate(ctx, id, APIToken, write, hook)
	if err != nil {
		return IssuedAPIToken{}, 0, err
	}
	return IssuedAPIToken{Credential: meta, Token: secret, Capabilities: b.Capabilities}, seq, nil
}

func (r *Repository) RotateWebhookKey(ctx context.Context, id string) (IssuedWebhookKey, changes.Sequence, error) {
	return r.RotateWebhookKeyWithWrite(ctx, id, r.store.Write)
}

// Webhook 独立轮换，不接收也不调用 API 队列扩展。
func (r *Repository) RotateWebhookKeyWithWrite(ctx context.Context, id string, write WriteFunc) (IssuedWebhookKey, changes.Sequence, error) {
	_, meta, secret, seq, err := r.rotate(ctx, id, WebhookKey, write, nil)
	if err != nil {
		return IssuedWebhookKey{}, 0, err
	}
	return IssuedWebhookKey{Credential: meta, Key: secret}, seq, nil
}

func (r *Repository) rotate(ctx context.Context, id string, kind Kind, write WriteFunc, hook LifecycleFunc) (Bot, CredentialMetadata, string, changes.Sequence, error) {
	if write == nil || id == "" {
		return Bot{}, CredentialMetadata{}, "", 0, ErrInvalid
	}
	var b Bot
	var meta CredentialMetadata
	var secret string
	var seq changes.Sequence
	err := write(ctx, func(tx *sql.Tx) error {
		var err error
		b, err = get(ctx, tx, id)
		if err != nil {
			return err
		}
		before := b
		old := b.Credentials.APIToken
		if kind == WebhookKey {
			old = b.Credentials.WebhookKey
		}
		// 先拒绝耗尽，绝不让 SQL 自增转换为浮点数或代次归零。
		if b.Version == math.MaxInt64 || old.Generation == math.MaxInt64 {
			return ErrConflict
		}
		var digest []byte
		meta, secret, digest, err = r.issueCredential()
		if err != nil {
			return err
		}
		meta.Generation = old.Generation + 1
		result, err := tx.ExecContext(ctx, `UPDATE bot_credentials SET credential_id=?,generation=?,digest=? WHERE bot_id=? AND kind=? AND credential_id=? AND generation=?`, meta.CredentialID, meta.Generation, digest, id, kind, old.CredentialID, old.Generation)
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
		result, err = tx.ExecContext(ctx, "UPDATE bots SET version=version+1 WHERE id=? AND version=?", id, b.Version)
		if err != nil {
			return err
		}
		n, err = result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrConflict
		}
		b.Version++
		if kind == APIToken {
			b.Credentials.APIToken = meta
		} else {
			b.Credentials.WebhookKey = meta
		}
		if hook != nil {
			if err := hook(ctx, tx, LifecycleChange{Action: LifecycleRotateAPIToken, Before: before, After: b}); err != nil {
				return err
			}
		}
		seq, err = changes.Append(ctx, tx, changes.Metadata{Type: changes.BotUpsert, EntityID: id, Revision: b.Version, BotID: &id, CreatedAt: time.Now()})
		return err
	})
	// 即使尾部续期或 COMMIT 失败，也不能返回已生成的可用秘密或序号。
	if err != nil {
		return Bot{}, CredentialMetadata{}, "", 0, err
	}
	return b, meta, secret, seq, nil
}

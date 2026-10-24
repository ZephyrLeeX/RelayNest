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
	LifecycleDeleteBefore   LifecycleAction = "delete_before"
	LifecycleDeleteAfter    LifecycleAction = "delete_after"
)

// LifecycleChange 只有公开元数据，前后快照明确目标、状态及凭据代次。
// 删除的两个阶段均以 Before 标识当前实体，After 为零值，不能当作存活 Bot。
type LifecycleChange struct {
	Action LifecycleAction
	Before Bot
	After  Bot
}

// LifecycleFunc 仅供受信适配层同步使用给定 Tx；错误必须传播以回滚整笔操作。
// 不得保留 Tx、嵌套写事务、执行网络等待或自行提交/回滚。
// 后续队列实现负责解释动作；此处不定义租约 schema 或完整 HTTP 级联。
type LifecycleFunc func(context.Context, *sql.Tx, LifecycleChange) error

// Delete 只实现基础删除；正式永久删除装配必须接入 P06-07 完整级联。
func (r *Repository) Delete(ctx context.Context, id string, req DeleteRequest) (changes.Sequence, error) {
	return r.DeleteWithWrite(ctx, id, req, r.store.Write, nil)
}

// DeleteWithWrite 将确认、基础删除、两阶段扩展、删除标记及调用方续期合并提交。
// 扩展必须遵守 LifecycleFunc 约束；空扩展不代表已实现消息/队列/附件永久级联。
func (r *Repository) DeleteWithWrite(ctx context.Context, id string, req DeleteRequest, write WriteFunc, hook LifecycleFunc) (changes.Sequence, error) {
	if write == nil || id == "" || req.ConfirmName == "" || req.ExpectedVersion < 1 {
		return 0, ErrInvalid
	}
	var seq changes.Sequence
	err := write(ctx, func(tx *sql.Tx) error {
		b, err := get(ctx, tx, id)
		if err != nil {
			return err
		}
		if b.Version != req.ExpectedVersion {
			return ErrConflict
		}
		// 按持久名称逐字确认，不在仓储暗中 trim 或变更大小写规则。
		if b.Name != req.ConfirmName {
			return ErrInvalid
		}
		if b.Version == math.MaxInt64 {
			return ErrConflict
		}
		if hook != nil {
			if err := hook(ctx, tx, LifecycleChange{Action: LifecycleDeleteBefore, Before: b}); err != nil {
				return err
			}
		}
		// 凭据沿既有 FK 同事务级联；CAS 仍核对受信扩展后的目标，禁止删错版本。
		result, err := tx.ExecContext(ctx, "DELETE FROM bots WHERE id=? AND version=? AND name=?", id, b.Version, b.Name)
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
		if hook != nil {
			if err := hook(ctx, tx, LifecycleChange{Action: LifecycleDeleteAfter, Before: b}); err != nil {
				return err
			}
		}
		// 终态版本加一；changes 无 Bot 外键，实体删除后仍保留 ID/版本/删除时间。
		seq, err = changes.Append(ctx, tx, changes.Metadata{Type: changes.BotDeleted, EntityID: id, Revision: b.Version + 1, BotID: &id, CreatedAt: time.Now()})
		return err
	})
	if err != nil {
		return 0, err
	}
	return seq, nil
}

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

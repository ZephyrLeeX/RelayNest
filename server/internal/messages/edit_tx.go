package messages

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/changes"
	"relaynest/server/internal/idempotency"
)

// EditApplicationTx消费调用方已持有的短写事务，不开/提交事务，不触发创建回调、
// delivery或push。返回成功仍未提交；事务拥有者必须传播错误并在COMMIT后发布。
func (r *Repository) EditApplicationTx(ctx context.Context, tx *sql.Tx, identity bots.Identity, in EditTextButtonsInput) (Edited, error) {
	if r == nil || tx == nil || r.now == nil {
		return Edited{}, ErrInvalid
	}
	// 身份复核必须早于正文读取；公开Actor字段不能代替真实凭据代次/能力。
	if err := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Edit); err != nil {
		return Edited{}, err
	}
	buttons, err := validateEditTextButtons(in)
	if err != nil {
		return Edited{}, err
	}
	// TECH §2.3要求callback仅属于具备接收能力的Bot；复用既有能力检查，
	// 不执行claim、不生成delivery。URL-only按钮不要求receive。
	callback := editHasCallback(in.Buttons)
	if callback {
		if err := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Claim); err != nil {
			return Edited{}, err
		}
	}
	id := strings.ToLower(in.MessageID)
	m, err := r.GetHistoryMessageTx(ctx, tx, in.BotID, id)
	if err != nil {
		return Edited{}, err
	}
	if m.Sender != Application {
		return Edited{}, bots.ErrForbidden
	}
	// 原引用按既有创建规则持久化为小写UUIDv4，坏值不能在编辑时修复或清空。
	if original := m.OriginalReplyToMessageID; original != nil && (!idempotency.ValidKey(*original) || *original != strings.ToLower(*original)) {
		return Edited{}, ErrInvalid
	}
	if m.BotSeq < 1 || m.Revision < 1 || m.CreatedChangeSeq == nil || *m.CreatedChangeSeq < 1 ||
		m.CreatedAt.IsZero() || m.CreatedAt.UTC().Year() < 1 || m.CreatedAt.UTC().Year() > 9999 ||
		m.UpdatedAt.IsZero() || m.UpdatedAt.UTC().Year() < 1 || m.UpdatedAt.UTC().Year() > 9999 {
		return Edited{}, ErrInvalid
	}
	// 本有限原语保留已有附件，因此attachment-only消息可以清空正文/按钮；
	// 空回复本身不算内容，也不能据此创建无内容的当前快照。
	if in.Text == "" && len(in.Buttons) == 0 && len(m.AttachmentRefs) == 0 {
		return Edited{}, ErrInvalid
	}
	if m.Revision != in.ExpectedRevision {
		return Edited{}, ErrRevisionConflict
	}
	if m.Revision == math.MaxInt64 {
		return Edited{}, ErrRevisionExhausted
	}
	now := r.now().UTC()
	if now.IsZero() || now.Year() < 1 || now.Year() > 9999 {
		return Edited{}, ErrInvalid
	}
	// 显式revision条件与行数检查防止陈旧更新/触发器忽略被当成成功。
	if err := requireOne(tx.ExecContext(ctx, `UPDATE messages SET format=?,text=?,buttons_json=?,revision=?,updated_at=?
        WHERE id=? AND bot_id=? AND sender='application' AND revision=?`,
		in.Format, in.Text, string(buttons), m.Revision+1, now.Format(time.RFC3339Nano), id, in.BotID, m.Revision)); err != nil {
		return Edited{}, err
	}
	seq, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.MessageUpsert, EntityID: id,
		Revision: m.Revision + 1, BotID: &m.BotID, CreatedAt: now})
	if err != nil {
		return Edited{}, err
	}
	// IGNORE可令LastInsertId沿用旧值；本入口须在下一次写入前确认本次插入，
	// 并验证真实行仍与本次编辑匹配，不能让正文成功而同步变更缺失。
	var affected int64
	var matching bool
	if err := tx.QueryRowContext(ctx, `SELECT changes(),EXISTS(
        SELECT 1 FROM changes WHERE change_seq=? AND type=? AND entity_id=? AND revision=? AND bot_id=? AND created_at=?)`,
		seq, string(changes.MessageUpsert), id, m.Revision+1, m.BotID, now.Format(time.RFC3339Nano)).Scan(&affected, &matching); err != nil {
		return Edited{}, err
	}
	if affected != 1 || !matching {
		return Edited{}, ErrAffectedRows
	}
	// SQL触发器等同事务内副作用也不能使已撤销身份带着候选成功返回。
	if err := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Edit); err != nil {
		return Edited{}, err
	}
	if callback {
		if err := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Claim); err != nil {
			return Edited{}, err
		}
	}
	m.Format, m.Text, m.ButtonsJSON, m.Revision, m.UpdatedAt = in.Format, in.Text, buttons, m.Revision+1, now
	return Edited{Message: m, LastChangeSeq: seq}, nil
}

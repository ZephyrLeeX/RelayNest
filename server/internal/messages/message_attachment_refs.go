package messages

import (
	"context"
	"database/sql"
	"errors"
)

// 当前关系必须在原授权读事务中装载，不读取物理卷，也不以ready状态过滤历史引用。
// LEFT JOIN让损坏外键可见；任何跨Bot、kind不一致或损坏行均使整个候选失败关闭。
func historyAttachmentRefsTx(ctx context.Context, tx *sql.Tx, m *Message) error {
	rows, err := tx.QueryContext(ctx, `SELECT r.attachment_id,r.kind,r.position,a.id,a.bot_id,a.kind
        FROM message_attachments r LEFT JOIN attachments a ON a.id=r.attachment_id
        WHERE r.message_id=? ORDER BY r.position,r.attachment_id`, m.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var refs []AttachmentReference
	for rows.Next() {
		var ref AttachmentReference
		var position int64
		var id, bot, kind sql.NullString
		if err := rows.Scan(&ref.ID, &ref.Kind, &position, &id, &bot, &kind); err != nil {
			return err
		}
		if !id.Valid || !bot.Valid || !kind.Valid || ref.ID == "" || id.String != ref.ID || bot.String != m.BotID || kind.String != ref.Kind || (ref.Kind != "image" && ref.Kind != "file") || position < 0 || position >= 10 || len(refs) >= 10 {
			return errors.New("messages: invalid history attachment relation")
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	// 无关联保留内部nil，避免改变既有创建候选／receipt／delivery的空附件语义。
	m.AttachmentRefs = refs
	return nil
}

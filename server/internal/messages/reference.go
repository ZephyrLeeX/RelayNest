package messages

import (
	"context"
	"database/sql"
	"errors"
)

var ErrReplyUnavailable = errors.New("messages: reply unavailable")

// ValidateReplyTx 仅检查同 Bot 已存在引用，供受信创建事务后续接线；
// 不放开当前创建入口，不读取正文，也不取代 INSERT 的同 Bot 触发器约束。
func (r *Repository) ValidateReplyTx(ctx context.Context, tx *sql.Tx, botID, messageID string) error {
	if tx == nil || botID == "" || messageID == "" {
		return ErrInvalid
	}
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM messages WHERE bot_id=? AND id=?`, botID, messageID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrReplyUnavailable
	}
	return err
}

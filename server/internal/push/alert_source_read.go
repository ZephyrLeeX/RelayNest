package push

import (
	"context"
	"database/sql"

	sharedsync "relaynest/server/internal/sync"
)

// alertSourceReadDecision 仅描述可信事务内源消息与共享已读的只读投影。
// 零值只随错误返回；有效原因也不是发送或永久终止授权。
type alertSourceReadDecision string

const (
	alertSourceUnread   alertSourceReadDecision = "unread"
	alertSourceMissing  alertSourceReadDecision = "missing"
	alertSourceMismatch alertSourceReadDecision = "mismatch"
	alertSourceRead     alertSourceReadDecision = "read"
)

// evaluateAlertSourceReadTx 不检查身份、lease、配置、DND 或 attempt。
// 调用方不能跨事务缓存原因，后续终止操作须在自己的可信写事务中重新求值。
// 查询和损坏校验的先后与原 policy 相同：源不存在或错配时不读取共享已读。
func evaluateAlertSourceReadTx(ctx context.Context, tx *sql.Tx, j JobSnapshot) (alertSourceReadDecision, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return "", err
	}
	if !j.valid() {
		return "", ErrJobInput
	}
	var matches, types bool
	err := tx.QueryRowContext(ctx, `SELECT bot_id=? AND bot_seq=?,typeof(id)='text' AND typeof(bot_id)='text' AND typeof(bot_seq)='integer' FROM messages WHERE id=?`, j.BotID, j.BotSeq, j.MessageID).Scan(&matches, &types)
	if err == sql.ErrNoRows {
		return alertSourceMissing, nil
	}
	if err != nil {
		return "", err
	}
	if !types {
		return "", ErrJobData
	}
	if !matches {
		return alertSourceMismatch, nil
	}
	state, err := sharedsync.GetReadStateTx(ctx, tx, j.BotID)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if state.ReadSeq >= j.BotSeq {
		return alertSourceRead, nil
	}
	return alertSourceUnread, nil
}

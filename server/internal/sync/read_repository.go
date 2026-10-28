package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"relaynest/server/internal/changes"
)

// GetReadStateTx 只消费已获授权的事务，不认证公开 Actor 字段，不自开事务。
// Bot 停用不影响管理员读取；缺失状态或持久值损坏必须失败，不隐式修复。
func GetReadStateTx(ctx context.Context, tx *sql.Tx, botID string) (ReadState, error) {
	state, _, err := readStateTx(ctx, tx, botID)
	return state, err
}

func readStateTx(ctx context.Context, tx *sql.Tx, botID string) (ReadState, int64, error) {
	if tx == nil {
		return ReadState{}, 0, changes.ErrInvalidTransaction
	}
	if botID == "" {
		return ReadState{}, 0, fmt.Errorf("sync: empty bot ID")
	}
	// 先区分不存在的 Bot 与已有 Bot 丢失状态；后者不能作为 404 或自动初始化。
	var upper int64
	var integerUpper bool
	if err := tx.QueryRowContext(ctx, `SELECT bot_seq,typeof(bot_seq)='integer' FROM bots WHERE id=?`, botID).Scan(&upper, &integerUpper); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReadState{}, 0, err
		}
		return ReadState{}, 0, fmt.Errorf("%w: %w", ErrReadStateUnavailable, err)
	}
	var state ReadState
	var updated string
	var integerValues bool
	err := tx.QueryRowContext(ctx, `SELECT read_seq,state_version,updated_at,(typeof(read_seq)='integer' AND typeof(state_version)='integer') FROM bot_read_states WHERE bot_id=?`, botID).Scan(&state.ReadSeq, &state.StateVersion, &updated, &integerValues)
	if errors.Is(err, sql.ErrNoRows) {
		return ReadState{}, 0, ErrReadStateUnavailable
	}
	if err != nil {
		return ReadState{}, 0, fmt.Errorf("%w: %w", ErrReadStateUnavailable, err)
	}
	state.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if !integerUpper || !integerValues || err != nil || state.UpdatedAt.IsZero() || !strings.HasSuffix(updated, "Z") || upper < 0 || state.ReadSeq < 0 || state.ReadSeq > upper || state.StateVersion < 1 {
		return ReadState{}, 0, ErrReadStateUnavailable
	}
	state.BotID = botID
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE bot_id=? AND sender='application' AND bot_seq>?`, botID, state.ReadSeq).Scan(&state.UnreadCount); err != nil {
		return ReadState{}, 0, err
	}
	return state, upper, nil
}

// AdvanceReadTx 以 bots.bot_seq 为曾分配水位上界，即使最高消息已删除仍有效。
// 调用方必须传播所有错误并回滚整个事务；状态和事件仅在最终提交后可发布。
// 旧设备相同或更低水位严格 no-op，包括版本已耗尽时。
func AdvanceReadTx(ctx context.Context, tx *sql.Tx, botID string, readSeq int64) (ReadState, changes.Sequence, error) {
	state, upper, err := readStateTx(ctx, tx, botID)
	if err != nil {
		return ReadState{}, 0, err
	}
	if readSeq < 0 || readSeq > upper {
		return ReadState{}, 0, ErrInvalidReadSequence
	}
	if readSeq <= state.ReadSeq {
		return state, 0, nil
	}
	if state.StateVersion == math.MaxInt64 {
		return ReadState{}, 0, ErrReadStateUnavailable
	}
	expectedVersion := state.StateVersion + 1
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE bot_read_states SET read_seq=?,state_version=state_version+1,updated_at=? WHERE bot_id=? AND read_seq=? AND state_version=?`, readSeq, now.Format(time.RFC3339Nano), botID, state.ReadSeq, state.StateVersion)
	if err != nil {
		return ReadState{}, 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return ReadState{}, 0, err
	}
	if n != 1 {
		return ReadState{}, 0, ErrReadStateUnavailable
	}
	seq, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.ReadUpdated, EntityID: botID, Revision: expectedVersion, BotID: &botID, CreatedAt: now})
	if err != nil {
		return ReadState{}, 0, err
	}
	state, _, err = readStateTx(ctx, tx, botID)
	if err != nil {
		return ReadState{}, 0, err
	}
	if state.ReadSeq != readSeq || state.StateVersion != expectedVersion || !state.UpdatedAt.Equal(now) {
		return ReadState{}, 0, ErrReadStateUnavailable
	}
	return state, seq, nil
}

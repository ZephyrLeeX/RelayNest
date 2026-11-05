package push

import (
	"context"
	"database/sql"
	"time"

	"relaynest/server/internal/devices"
	sharedsync "relaynest/server/internal/sync"
)

// CheckAlertPolicyTx 只在可信事务中投影原 alert 快照的当前策略资格。
// 返回值仅在本事务快照内有效，不能缓存为发送许可；身份、租约、attempt 及
// DND 永久丢资格由后续组合负责。本函数不改变原快照或任何持久状态。
func CheckAlertPolicyTx(ctx context.Context, tx *sql.Tx, j JobSnapshot, now time.Time) (bool, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return false, err
	}
	if !j.valid() || !jobTimeValid(now) {
		return false, ErrJobInput
	}
	if j.Purpose != "alert" || !j.AlertEligible {
		return false, nil
	}
	settings, err := devices.ReadNotificationSettingsTx(ctx, tx, j.DeviceID)
	if err != nil {
		return false, err
	}
	if settings.SettingsVersion != j.SettingsVersion || !settings.Enabled || settings.NotifyProvider == nil || string(*settings.NotifyProvider) != j.Provider {
		return false, nil
	}
	for _, bot := range settings.BotOverrides {
		if bot.BotID == j.BotID && !bot.Enabled {
			return false, nil
		}
	}
	if settings.DND.Enabled {
		location, err := time.LoadLocation(settings.DND.Timezone)
		if err != nil {
			return false, ErrJobData
		}
		// 直接比较实际本地钟面，重复的 DST 时刻均匹配，缺失时刻不会凭空产生。
		clock := now.In(location).Format("15:04")
		start, end := settings.DND.Start, settings.DND.End
		if (start < end && clock >= start && clock < end) || (start > end && (clock >= start || clock < end)) {
			return false, nil
		}
	}
	var matches, types bool
	err = tx.QueryRowContext(ctx, `SELECT bot_id=? AND bot_seq=?,typeof(id)='text' AND typeof(bot_id)='text' AND typeof(bot_seq)='integer' FROM messages WHERE id=?`, j.BotID, j.BotSeq, j.MessageID).Scan(&matches, &types)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !types {
		return false, ErrJobData
	}
	if !matches {
		return false, nil
	}
	state, err := sharedsync.GetReadStateTx(ctx, tx, j.BotID)
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return state.ReadSeq < j.BotSeq, nil
}

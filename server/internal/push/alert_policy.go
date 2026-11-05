package push

import (
	"context"
	"database/sql"
	"time"

	"relaynest/server/internal/devices"
	sharedsync "relaynest/server/internal/sync"
)

// alertPolicyDecision 只描述当前可信事务中的策略投影，不含发送所需身份或敏感数据。
// 零值仅用于错误返回，不能解释为任一策略拒绝。
type alertPolicyDecision string

const (
	alertPolicyEligible    alertPolicyDecision = "eligible"
	alertPolicyDND         alertPolicyDecision = "dnd"
	alertPolicyOtherDenied alertPolicyDecision = "other_denied"
)

// CheckAlertPolicyTx 只在可信事务中投影原 alert 快照的当前策略资格。
// 返回值仅在本事务快照内有效，不能缓存为发送许可；身份、租约、attempt 及
// DND 永久丢资格由后续组合负责。本函数不改变原快照或任何持久状态。
func CheckAlertPolicyTx(ctx context.Context, tx *sql.Tx, j JobSnapshot, now time.Time) (bool, error) {
	decision, err := evaluateAlertPolicyTx(ctx, tx, j, now)
	return decision == alertPolicyEligible, err
}

// evaluateAlertPolicyTx 与兼容入口共用唯一策略判断，结果仅对本事务快照有效。
// dnd 仅证明前置原资格、配置版本、provider、全局及 Bot 条件通过并命中当前
// 时段；后置 source/read 尚未求值。它不是缓存票据或跨事务永久丢弃授权，
// 后续终止操作须在自己的写事务中重新求值；此处不检查身份、租约或 attempt。
func evaluateAlertPolicyTx(ctx context.Context, tx *sql.Tx, j JobSnapshot, now time.Time) (alertPolicyDecision, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return "", err
	}
	if !j.valid() || !jobTimeValid(now) {
		return "", ErrJobInput
	}
	if j.Purpose != "alert" || !j.AlertEligible {
		return alertPolicyOtherDenied, nil
	}
	settings, err := devices.ReadNotificationSettingsTx(ctx, tx, j.DeviceID)
	if err != nil {
		return "", err
	}
	if settings.SettingsVersion != j.SettingsVersion || !settings.Enabled || settings.NotifyProvider == nil || string(*settings.NotifyProvider) != j.Provider {
		return alertPolicyOtherDenied, nil
	}
	for _, bot := range settings.BotOverrides {
		if bot.BotID == j.BotID && !bot.Enabled {
			return alertPolicyOtherDenied, nil
		}
	}
	if settings.DND.Enabled {
		location, err := time.LoadLocation(settings.DND.Timezone)
		if err != nil {
			return "", ErrJobData
		}
		// 直接比较实际本地钟面，重复的 DST 时刻均匹配，缺失时刻不会凭空产生。
		clock := now.In(location).Format("15:04")
		start, end := settings.DND.Start, settings.DND.End
		if (start < end && clock >= start && clock < end) || (start > end && (clock >= start || clock < end)) {
			return alertPolicyDND, nil
		}
	}
	var matches, types bool
	err = tx.QueryRowContext(ctx, `SELECT bot_id=? AND bot_seq=?,typeof(id)='text' AND typeof(bot_id)='text' AND typeof(bot_seq)='integer' FROM messages WHERE id=?`, j.BotID, j.BotSeq, j.MessageID).Scan(&matches, &types)
	if err == sql.ErrNoRows {
		return alertPolicyOtherDenied, nil
	}
	if err != nil {
		return "", err
	}
	if !types {
		return "", ErrJobData
	}
	if !matches {
		return alertPolicyOtherDenied, nil
	}
	state, err := sharedsync.GetReadStateTx(ctx, tx, j.BotID)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if state.ReadSeq >= j.BotSeq {
		return alertPolicyOtherDenied, nil
	}
	return alertPolicyEligible, nil
}

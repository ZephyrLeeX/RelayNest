package push

import (
	"context"
	"database/sql"
	"time"

	"relaynest/server/internal/storage"
)

// SyncEvent 是业务首次创建的最小可信元数据，不包含消息正文或设备凭据。
// 调用方须在 delivery 等变化完成之后传入最终水位，不得用旧事件补当前注册快照。
type SyncEvent struct {
	MessageID       string
	BotID           string
	BotSeq          int64
	TargetChangeSeq int64
	CreatedAt       time.Time
}

type syncTarget struct {
	session, device                    string
	generation, registration, settings int64
}

// EnqueueSyncTx 仅在已有业务写事务内生成完整 sync 快照；不提交、不发送。
// 任意错误必须让外层事务回滚，包括已生成的前几页待办及消息/回执。
func EnqueueSyncTx(ctx context.Context, tx *sql.Tx, event SyncEvent) error {
	if err := jobTxInput(ctx, tx); err != nil {
		return err
	}
	expires := event.CreatedAt.Add(24 * time.Hour)
	if event.MessageID == "" || event.BotID == "" || event.BotSeq < 1 || event.TargetChangeSeq < 1 || !jobTimeValid(event.CreatedAt) || !jobTimeValid(expires) || !expires.After(event.CreatedAt) {
		return ErrJobInput
	}
	deployment, err := storage.ReadInstanceIDTx(ctx, tx)
	if err != nil {
		return err
	}
	// 仅约束单页内存，不截断目标总数。页间同一写事务的会话/注册快照不变。
	const pageSize = 128
	cursor := ""
	for {
		rows, err := tx.QueryContext(ctx, `SELECT s.id,s.device_id,s.auth_generation,r.registration_version,
 COALESCE(n.settings_version,0) FROM sessions s
 JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation
 JOIN device_push_registrations r ON r.session_id=s.id AND r.provider='fcm' AND r.token IS NOT NULL
 LEFT JOIN device_notification_settings n ON n.device_id=s.device_id
 WHERE s.id>? AND s.client_type='android' AND s.revoked=0 AND s.idle_expires_at>? AND s.absolute_expires_at>?
 ORDER BY s.id LIMIT ?`, cursor, event.CreatedAt.UnixNano(), event.CreatedAt.UnixNano(), pageSize)
		if err != nil {
			return err
		}
		targets := make([]syncTarget, 0, pageSize)
		for rows.Next() {
			var target syncTarget
			if err = rows.Scan(&target.session, &target.device, &target.generation, &target.registration, &target.settings); err != nil {
				break
			}
			targets = append(targets, target)
		}
		scanErr := rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return err
		}
		if scanErr != nil {
			return scanErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, target := range targets {
			// 原消息与不可复用session组成稳定键；同设备不同session拥有独立资格。
			job := JobSnapshot{ID: "sync:" + event.MessageID + ":" + target.session, SessionID: target.session, DeviceID: target.device,
				Purpose: "sync", Provider: "fcm", TargetChangeSeq: event.TargetChangeSeq, RegistrationVersion: target.registration,
				SettingsVersion: target.settings, DeploymentID: deployment, MessageID: event.MessageID, BotID: event.BotID,
				BotSeq: event.BotSeq, SessionGeneration: target.generation, CreatedAt: event.CreatedAt, ExpiresAt: expires}
			if err = CreateJobTx(ctx, tx, job, event.CreatedAt); err != nil {
				return err
			}
		}
		if len(targets) < pageSize {
			return nil
		}
		cursor = targets[len(targets)-1].session
	}
}

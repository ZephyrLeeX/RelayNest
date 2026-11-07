package push

import (
	"context"
	"database/sql"
	"encoding/base64"
	"reflect"
	"strings"
	"time"

	"relaynest/server/internal/devices"
	"relaynest/server/internal/storage"
)

// EnqueueAlertTx 仅供原消息首次创建的可信写事务调用，不提交、不发送。
// 来源读取不能证明本事务首次创建；与 EnqueueSyncTx 一样由内部调用方保证，
// 不得用历史扫描、重放或配置切换补旧提醒。任意错误必须回滚整个消息事务。
// SyncEvent 只携带最小事件元数据，不接受公开目标选择器。
func EnqueueAlertTx(ctx context.Context, tx *sql.Tx, event SyncEvent) error {
	if err := jobTxInput(ctx, tx); err != nil {
		return err
	}
	expires := event.CreatedAt.Add(24 * time.Hour)
	if event.MessageID == "" || event.BotID == "" || event.BotSeq < 1 || event.TargetChangeSeq < 1 || !jobTimeValid(event.CreatedAt) || !jobTimeValid(expires) || !expires.After(event.CreatedAt) {
		return ErrJobInput
	}
	app, err := alertEventTx(ctx, tx, event)
	if err != nil || !app {
		return err
	}
	deployment, err := storage.ReadInstanceIDTx(ctx, tx)
	if err != nil {
		return err
	}
	// 每页先关闭游标再写入；最终保留原快照回核，防止后页写入破坏前页。
	const pageSize = 128
	cursor := ""
	var created []JobSnapshot
	for {
		rows, err := tx.QueryContext(ctx, `SELECT s.id,s.device_id,s.auth_generation FROM sessions s
 JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation
 WHERE s.id>? AND s.client_type='android' AND s.revoked=0 AND s.idle_expires_at>? AND s.absolute_expires_at>?
 ORDER BY s.id LIMIT ?`, cursor, event.CreatedAt.UnixNano(), event.CreatedAt.UnixNano(), pageSize)
		if err != nil {
			return err
		}
		var targets []syncTarget
		for rows.Next() {
			var t syncTarget
			if err = rows.Scan(&t.session, &t.device, &t.generation); err != nil {
				break
			}
			targets = append(targets, t)
		}
		scanErr, closeErr := rows.Err(), rows.Close()
		if err != nil {
			return err
		}
		if scanErr != nil {
			return scanErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, t := range targets {
			settings, err := devices.ReadNotificationSettingsTx(ctx, tx, t.device)
			if err != nil {
				return err
			}
			if settings.NotifyChannel != "vendor" || settings.NotifyProvider == nil || *settings.NotifyProvider != devices.PushOPush {
				continue
			}
			var version int64
			err = tx.QueryRowContext(ctx, `SELECT registration_version FROM device_push_registrations WHERE session_id=? AND provider='opush' AND token IS NOT NULL`, t.session).Scan(&version)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return err
			}
			// true 只作为原始策略评估的临时输入，不是跨事务发送许可。
			j := JobSnapshot{ID: alertOriginID(deployment, event.MessageID, t.session, "opush"), SessionID: t.session, DeviceID: t.device,
				Purpose: "alert", Provider: "opush", TargetChangeSeq: event.TargetChangeSeq, RegistrationVersion: version,
				SettingsVersion: settings.SettingsVersion, DeploymentID: deployment, MessageID: event.MessageID, BotID: event.BotID,
				BotSeq: event.BotSeq, SessionGeneration: t.generation, CreatedAt: event.CreatedAt, ExpiresAt: expires, AlertEligible: true}
			if err = alertOriginEligibleTx(ctx, tx, j); err != nil {
				return err
			}
			decision, err := evaluateAlertPolicyTx(ctx, tx, j, event.CreatedAt)
			if err != nil {
				return err
			}
			if decision != alertPolicyEligible {
				continue
			}
			// 任何同 ID 旧行都不能刷新，包含轮换、leased 和 suppressed。
			var exists bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM push_jobs WHERE id=?)`, j.ID).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return ErrJobData
			}
			// 不使用通用 CreateJobTx 的无 RowsAffected 返回值，IGNORE 也必须失败。
			result, err := tx.ExecContext(ctx, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,next_attempt_at,
 purpose,provider,registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,
 session_generation,created_at_ns,expires_at_ns,alert_eligible) VALUES(?,?,?,?,'queued',?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				j.ID, j.SessionID, j.DeviceID, j.TargetChangeSeq, j.CreatedAt.UTC().Format(jobTimestamp), j.Purpose, j.Provider,
				j.RegistrationVersion, j.SettingsVersion, j.DeploymentID, j.MessageID, j.BotID, j.BotSeq, j.SessionGeneration, j.CreatedAt.UnixNano(), j.ExpiresAt.UnixNano(), j.AlertEligible)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return ErrJobData
			}
			created = append(created, j)
		}
		if len(targets) < pageSize {
			break
		}
		cursor = targets[len(targets)-1].session
	}
	app, err = alertEventTx(ctx, tx, event)
	if err != nil {
		return err
	}
	if !app {
		return ErrJobData
	}
	for _, j := range created {
		if err = alertOriginEligibleTx(ctx, tx, j); err != nil {
			return err
		}
		decision, err := evaluateAlertPolicyTx(ctx, tx, j, event.CreatedAt)
		if err != nil {
			return err
		}
		if decision != alertPolicyEligible {
			return ErrJobData
		}
		if err = checkAlertOriginRowTx(ctx, tx, j); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// Base64 每段独立编码并以非字母表字符分隔，无歧义地绑定用途及原事件/会话/通道。
// 版本不参与 ID，轮换不能给同一目标的旧事件造新 ID；外部提醒键仍是部署/消息。
func alertOriginID(deployment, message, session, provider string) string {
	parts := []string{deployment, message, session, provider}
	for i := range parts {
		parts[i] = base64.RawURLEncoding.EncodeToString([]byte(parts[i]))
	}
	return "alert:" + strings.Join(parts, ":")
}

func alertEventTx(ctx context.Context, tx *sql.Tx, event SyncEvent) (bool, error) {
	if err := validateSyncEventTx(ctx, tx, event); err != nil {
		return false, err
	}
	var sender string
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT m.sender,typeof(m.sender)='text' AND typeof(m.id)='text' AND typeof(m.bot_id)='text'
 AND typeof(m.bot_seq)='integer' AND typeof(m.created_at)='text' AND typeof(m.created_change_seq)='integer'
 AND typeof(c.change_seq)='integer' AND typeof(c.revision)='integer' AND typeof(c.type)='text'
 AND typeof(c.entity_id)='text' AND typeof(c.bot_id)='text' AND typeof(c.created_at)='text'
 FROM messages m JOIN changes c ON c.change_seq=m.created_change_seq WHERE m.id=?`, event.MessageID).Scan(&sender, &valid)
	if err != nil {
		return false, err
	}
	if !valid || (sender != "application" && sender != "administrator") {
		return false, ErrJobData
	}
	return sender == "application", nil
}

// 该共享 helper 仅读身份，不要求 job 已落库，也不领取租约。
func alertOriginEligibleTx(ctx context.Context, tx *sql.Tx, j JobSnapshot) error {
	if !j.valid() {
		return ErrJobData
	}
	current, err := leaseCurrentSnapshot(ctx, tx, j, j.CreatedAt, alertLeaseScope)
	if err != nil {
		return err
	}
	if !current {
		return ErrJobData
	}
	return validateJobSourceTx(ctx, tx, j)
}

// 原始 SQLite 值比较覆盖全部 26 列及其存储类型，拒绝 typed Scan 的 BLOB 转换。
func checkAlertOriginRowTx(ctx context.Context, tx *sql.Tx, j JobSnapshot) error {
	values := make([]any, 26)
	dest := make([]any, 26)
	for i := range values {
		dest[i] = &values[i]
	}
	err := tx.QueryRowContext(ctx, `SELECT `+jobColumns+`,lease_hash,lease_until_ns,attempt_lease_hash,attempt_lease_until_ns,attempt_started_at_ns,result_kind,result_at_ns FROM push_jobs WHERE id=?`, j.ID).Scan(dest...)
	if err != nil {
		return err
	}
	want := []any{j.ID, j.SessionID, j.DeviceID, j.TargetChangeSeq, "queued", int64(0), j.CreatedAt.UTC().Format(jobTimestamp),
		j.Purpose, j.Provider, j.RegistrationVersion, j.SettingsVersion, j.DeploymentID, j.MessageID, j.BotID, j.BotSeq,
		j.SessionGeneration, j.CreatedAt.UnixNano(), j.ExpiresAt.UnixNano(), int64(1), nil, nil, nil, nil, nil, nil, nil}
	if !reflect.DeepEqual(values, want) {
		return ErrJobData
	}
	return nil
}

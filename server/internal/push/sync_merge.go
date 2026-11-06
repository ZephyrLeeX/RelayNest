package push

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// validateSyncEventTx 校验本次新事件的完整来源；旧 anchor 清理不影响静默同步归并。
// 管理员的最终水位可包含随后追加的 delivery，不能要求等于创建水位。
func validateSyncEventTx(ctx context.Context, tx *sql.Tx, event SyncEvent) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages m
 JOIN changes c ON c.change_seq=m.created_change_seq
 WHERE m.id=? AND m.bot_id=? AND m.bot_seq=? AND m.created_at=?
 AND c.type='message.upsert' AND c.entity_id=m.id AND c.bot_id=m.bot_id
 AND c.revision=1 AND c.created_at=m.created_at AND c.change_seq<=?)`,
		event.MessageID, event.BotID, event.BotSeq, event.CreatedAt.UTC().Format(time.RFC3339Nano), event.TargetChangeSeq).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrJobData
	}
	return nil
}

// mergeOrCreateSyncTx 仅选择同一身份版本的一个确定性 queued 候选，全部操作沿用消息写事务。
func mergeOrCreateSyncTx(ctx context.Context, tx *sql.Tx, incoming JobSnapshot) error {
	old, err := jobScan(tx.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM push_jobs
 WHERE deployment_id=? AND device_id=? AND session_id=? AND session_generation=?
 AND purpose='sync' AND provider='fcm' AND registration_version=? AND settings_version=?
 AND state='queued' AND expires_at_ns>? AND created_at_ns<=?
 ORDER BY created_at_ns,id LIMIT 1`, incoming.DeploymentID, incoming.DeviceID, incoming.SessionID, incoming.SessionGeneration,
		incoming.RegistrationVersion, incoming.SettingsVersion, incoming.CreatedAt.UnixNano(), incoming.CreatedAt.UnixNano()))
	if errors.Is(err, sql.ErrNoRows) {
		return CreateJobTx(ctx, tx, incoming, incoming.CreatedAt)
	}
	if err != nil {
		return err
	}
	if err := validateJobSourceTx(ctx, tx, incoming); err != nil {
		return err
	}
	// 已覆盖水位不写数据库，避免触发器副作用，也不刷新 TTL、退避或尝试预算。
	if old.Snapshot.TargetChangeSeq >= incoming.TargetChangeSeq {
		return nil
	}
	metadata, err := syncMergeMetadataTx(ctx, tx, old.Snapshot.ID)
	if err != nil {
		return err
	}
	changed, err := MergeJobWatermarkTx(ctx, tx, old.Snapshot.ID, incoming.TargetChangeSeq)
	if err != nil {
		return err
	}
	if !changed {
		return ErrJobData
	}
	// RowsAffected 不能证明触发器保留了结果；回读确认投影并失败关闭，不能改走 INSERT。
	after, err := ReadJobTx(ctx, tx, old.Snapshot.ID)
	if err != nil {
		return err
	}
	afterMetadata, err := syncMergeMetadataTx(ctx, tx, old.Snapshot.ID)
	if err != nil {
		return err
	}
	old.Snapshot.TargetChangeSeq = incoming.TargetChangeSeq
	if after != old || metadata != afterMetadata {
		return ErrJobData
	}
	return nil
}

// 此投影覆盖 Job 之外的七列，验证归并没有改变租约或平台结果元数据。
func syncMergeMetadataTx(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var metadata string
	err := tx.QueryRowContext(ctx, `SELECT json_array(hex(lease_hash),lease_until_ns,hex(attempt_lease_hash),attempt_lease_until_ns,attempt_started_at_ns,
 result_kind,result_at_ns) FROM push_jobs WHERE id=?`, id).Scan(&metadata)
	return metadata, err
}

package push

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"
)

// CheckSyncLeaseTx 在调用方的可信事务中仅读复核完整 sync 租约及当前快照。
// 旧、过期、已释放、非 sync 或版本失配返回 nil,nil；输入、SQL 或坏数据
// 返回错误且没有候选。不自开/提交事务，不领取、续期或推进任何状态。
func CheckSyncLeaseTx(ctx context.Context, tx *sql.Tx, lease JobLease, now time.Time) (*SyncLeaseSnapshot, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return nil, err
	}
	if lease.jobID == "" || lease.secret == [32]byte{} || !jobTimeValid(lease.until) || !jobTimeValid(now) {
		return nil, ErrJobInput
	}
	j, err := ReadJobTx(ctx, tx, lease.jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if j.State != "leased" || j.Snapshot.Purpose != "sync" || j.Snapshot.Provider != "fcm" || j.Snapshot.AlertEligible {
		return nil, nil
	}
	var storedHash []byte
	var storedUntil int64
	var hashType, untilType string
	var jobIntegerStorage bool
	// 共享仓储已检查数值范围；本叶在发布候选前另核原存储类，不能让 typed
	// Scan 将数字 BLOB 转换为看似合法的序号、版本、期限或布尔资格。
	err = tx.QueryRowContext(ctx, `SELECT lease_hash,lease_until_ns,typeof(lease_hash),typeof(lease_until_ns),
 typeof(target_change_seq)='integer' AND typeof(attempt)='integer'
 AND typeof(registration_version)='integer' AND typeof(settings_version)='integer'
 AND typeof(bot_seq)='integer' AND typeof(session_generation)='integer'
 AND typeof(created_at_ns)='integer' AND typeof(expires_at_ns)='integer'
 AND typeof(alert_eligible)='integer'
 FROM push_jobs WHERE id=?`, lease.jobID).
		Scan(&storedHash, &storedUntil, &hashType, &untilType, &jobIntegerStorage)
	if err != nil {
		return nil, err
	}
	if !jobIntegerStorage || hashType != "blob" || len(storedHash) != sha256.Size || untilType != "integer" || storedUntil <= 0 || j.NextAttemptAt.Before(j.Snapshot.CreatedAt) {
		return nil, ErrJobData
	}
	hash := sha256.Sum256(lease.secret[:])
	// 同时绑定秘密摘要和原期限，接管后的相同 job 不能复用旧句柄。
	if subtle.ConstantTimeCompare(storedHash, hash[:]) != 1 || storedUntil != lease.until.UnixNano() || storedUntil <= now.UnixNano() ||
		!j.Snapshot.ExpiresAt.After(now) || j.Snapshot.CreatedAt.After(now) || j.NextAttemptAt.After(now) {
		return nil, nil
	}
	valid, err := syncLeaseCurrentSnapshot(ctx, tx, j.Snapshot, now)
	if err != nil || !valid {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &SyncLeaseSnapshot{Job: j}, nil
}

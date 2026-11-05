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
// 租约或当前快照不具资格（包括当前会话不满足原始 TEXT Android 类型）时返回 nil,nil；
// 输入、SQL 错误或受保护快照的显式数据校验失败返回错误且没有候选。
// 不自开/提交事务，不领取、续期或推进任何状态。
func CheckSyncLeaseTx(ctx context.Context, tx *sql.Tx, lease JobLease, now time.Time) (*SyncLeaseSnapshot, error) {
	j, err := checkLeaseTx(ctx, tx, lease, now, syncLeaseScope)
	if err != nil || j == nil {
		return nil, err
	}
	return &SyncLeaseSnapshot{Job: *j}, nil
}

// leaseScope 只允许本包的两个窄入口指定用途；提供方始终取自已验证的原 job。
type leaseScope uint8

const (
	syncLeaseScope leaseScope = iota
	alertLeaseScope
)

func (scope leaseScope) accepts(j JobSnapshot) bool {
	switch scope {
	case syncLeaseScope:
		return j.Purpose == "sync" && j.Provider == "fcm" && !j.AlertEligible
	case alertLeaseScope:
		return j.Purpose == "alert" && (j.Provider == "fcm" || j.Provider == "opush") && j.AlertEligible
	default:
		return false
	}
}

func checkLeaseTx(ctx context.Context, tx *sql.Tx, lease JobLease, now time.Time, scope leaseScope) (*Job, error) {
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
	if j.State != "leased" || !scope.accepts(j.Snapshot) {
		return nil, nil
	}
	var storedHash []byte
	var storedUntil int64
	var hashType, untilType string
	var jobIntegerStorage, jobTextStorage bool
	// 共享仓储已检查值域；本叶另核完整快照的原存储类，不能让 typed Scan
	// 把 BLOB 转成看似合法的数字或文本，使候选资格与 SQL 调度谓词不一致。
	err = tx.QueryRowContext(ctx, `SELECT lease_hash,lease_until_ns,typeof(lease_hash),typeof(lease_until_ns),
 typeof(target_change_seq)='integer' AND typeof(attempt)='integer'
 AND typeof(registration_version)='integer' AND typeof(settings_version)='integer'
 AND typeof(bot_seq)='integer' AND typeof(session_generation)='integer'
 AND typeof(created_at_ns)='integer' AND typeof(expires_at_ns)='integer'
 AND typeof(alert_eligible)='integer',
 typeof(id)='text' AND typeof(session_id)='text' AND typeof(device_id)='text'
 AND typeof(state)='text' AND typeof(next_attempt_at)='text'
 AND typeof(purpose)='text' AND typeof(provider)='text' AND typeof(deployment_id)='text'
 AND typeof(message_id)='text' AND typeof(bot_id)='text'
 FROM push_jobs WHERE id=?`, lease.jobID).
		Scan(&storedHash, &storedUntil, &hashType, &untilType, &jobIntegerStorage, &jobTextStorage)
	if err != nil {
		return nil, err
	}
	if !jobIntegerStorage || !jobTextStorage || hashType != "blob" || len(storedHash) != sha256.Size || untilType != "integer" || storedUntil <= 0 || j.NextAttemptAt.Before(j.Snapshot.CreatedAt) {
		return nil, ErrJobData
	}
	hash := sha256.Sum256(lease.secret[:])
	// 同时绑定秘密摘要和原期限，接管后的相同 job 不能复用旧句柄。
	if subtle.ConstantTimeCompare(storedHash, hash[:]) != 1 || storedUntil != lease.until.UnixNano() || storedUntil <= now.UnixNano() ||
		!j.Snapshot.ExpiresAt.After(now) || j.Snapshot.CreatedAt.After(now) || j.NextAttemptAt.After(now) {
		return nil, nil
	}
	valid, err := leaseCurrentSnapshot(ctx, tx, j.Snapshot, now, scope)
	if err != nil || !valid {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &j, nil
}

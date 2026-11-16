package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"reflect"
	"time"
)

// DropAlertSourceRead 只终止仍持原租约的原 eligible opush alert。
// 源已清理或共享已读须在自己的可信写事务内重新确认；错配和其他策略拒绝不是终止证据。
// 不接发送调用链、不扫描 queued，也不改变既有 Begin/Check 的拒绝语义。
func (r *JobRepository) DropAlertSourceRead(ctx context.Context, lease JobLease, expected JobSnapshot, now time.Time) (bool, error) {
	if ctx == nil || r == nil || r.store == nil || !expected.valid() {
		return false, ErrJobInput
	}
	var changed bool
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		checked, err := CheckAlertLeaseTx(ctx, tx, lease, now)
		if err != nil || checked == nil {
			return err
		}
		original := checked.Job.Snapshot
		if original != expected || !opushAlertSnapshot(original) || !alertAttemptExpiry(original).After(now) {
			return nil
		}
		before, err := readAlertDropRow(ctx, tx, original.ID)
		if err != nil {
			return err
		}
		decision, err := evaluateAlertSourceReadTx(ctx, tx, original)
		if err != nil || (decision != alertSourceMissing && decision != alertSourceRead) {
			return err
		}
		hash := sha256.Sum256(lease.secret[:])
		result, err := tx.ExecContext(ctx, `UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL
 WHERE id=? AND purpose='alert' AND state='leased' AND lease_hash=? AND lease_until_ns=?`, original.ID, hash[:], lease.until.UnixNano())
		if err != nil {
			return err
		}
		if err = syncOneRow(result); err != nil {
			return err
		}
		// 清 lease 后用原快照复核身份；写后触发器不能撤销终止的正证据或改变其他列。
		current, err := leaseCurrentSnapshot(ctx, tx, original, now, alertLeaseScope)
		if err != nil {
			return err
		}
		if !current {
			return ErrJobData
		}
		decision, err = evaluateAlertSourceReadTx(ctx, tx, original)
		if err != nil {
			return err
		}
		if decision != alertSourceMissing && decision != alertSourceRead {
			return ErrJobData
		}
		after, err := readAlertDropRow(ctx, tx, original.ID)
		if err != nil {
			return err
		}
		before[4], before[19], before[20] = jobSuppressed, nil, nil
		if !reflect.DeepEqual(before, after) {
			return ErrJobData
		}
		changed = true
		return ctx.Err()
	})
	// callback 中的暂存结果不能越过真实 COMMIT 故障发布。
	if err != nil {
		return false, err
	}
	return changed, nil
}

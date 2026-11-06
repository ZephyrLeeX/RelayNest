package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"reflect"
	"time"
)

// DropAlertDND 仅终止仍持有原租约且当前明确命中 DND 的原 alert。
// 自有短写事务在真实 COMMIT 成功后才发布 changed；无当前资格返回 false,nil。
// 不扫描 queued，不创建任务，也不把其他策略拒绝或错误解释为永久丢弃。
func (r *JobRepository) DropAlertDND(ctx context.Context, lease JobLease, expected JobSnapshot, now time.Time) (bool, error) {
	if ctx == nil || r == nil || r.store == nil || !expected.valid() {
		return false, ErrJobInput
	}
	var changed bool
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		checked, err := CheckAlertLeaseTx(ctx, tx, lease, now)
		if err != nil || checked == nil {
			return err
		}
		original := checked.Job
		if original.Snapshot != expected {
			return nil
		}
		before, err := readAlertDropRow(ctx, tx, expected.ID)
		if err != nil {
			return err
		}
		decision, err := evaluateAlertPolicyTx(ctx, tx, original.Snapshot, now)
		if err != nil || decision != alertPolicyDND {
			return err
		}
		hash := sha256.Sum256(lease.secret[:])
		result, err := tx.ExecContext(ctx, `UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL
 WHERE id=? AND purpose='alert' AND state='leased' AND lease_hash=? AND lease_until_ns=?`, expected.ID, hash[:], lease.until.UnixNano())
		if err != nil {
			return err
		}
		if err := syncOneRow(result); err != nil {
			return err
		}
		// 租约已清除，不能再调用要求 leased 的全 guard；按原快照复核身份和策略。
		current, err := leaseCurrentSnapshot(ctx, tx, original.Snapshot, now, alertLeaseScope)
		if err != nil {
			return err
		}
		if !current {
			return ErrJobData
		}
		decision, err = evaluateAlertPolicyTx(ctx, tx, original.Snapshot, now)
		if err != nil {
			return err
		}
		if decision != alertPolicyDND {
			return ErrJobData
		}
		after, err := readAlertDropRow(ctx, tx, expected.ID)
		if err != nil {
			return err
		}
		before[4], before[19], before[20] = jobSuppressed, nil, nil
		// interface 保留 SQLite 原存储类：不让 Scan 的文本/数字转换掩盖 trigger 篡改。
		if !reflect.DeepEqual(before, after) {
			return ErrJobData
		}
		changed = true
		return ctx.Err()
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func readAlertDropRow(ctx context.Context, tx *sql.Tx, id string) ([26]any, error) {
	var values [26]any
	var destinations [26]any
	for i := range values {
		destinations[i] = &values[i]
	}
	err := tx.QueryRowContext(ctx, `SELECT id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,
 purpose,provider,registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,
 session_generation,created_at_ns,expires_at_ns,alert_eligible,lease_hash,lease_until_ns,
 attempt_lease_hash,attempt_lease_until_ns,attempt_started_at_ns,result_kind,result_at_ns
 FROM push_jobs WHERE id=?`, id).Scan(destinations[:]...)
	return values, err
}

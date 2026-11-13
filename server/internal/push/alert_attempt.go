package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"time"
)

const MaxAlertAttempts int64 = 8

// AlertAttempt 是 COMMIT 后发布的内部脱敏票据。字段私有且不可序列化；
// 不含注册 token、租约秘密或消息正文，也不代表最终发送策略已通过。
// 每次实际派发仅能消费一次 BeginAlert 返回的非 nil 票据，不能缓存复用。
type AlertAttempt struct {
	snapshot   JobSnapshot
	leaseHash  [32]byte
	leaseUntil time.Time
	attempt    int64
	startedAt  time.Time
}

func (AlertAttempt) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[push alert attempt]") }
func (AlertAttempt) MarshalJSON() ([]byte, error) { return []byte(`null`), nil }

// BeginAlert 持有自己的短写事务；同一 lease 最多返回一次票据、扣一次尝试。
// 即使 Store 的真实 COMMIT 失败也不发布暂存票据，不暴露可被外层误提交的 Tx API。
func (r *JobRepository) BeginAlert(ctx context.Context, lease JobLease, now time.Time) (*AlertAttempt, error) {
	if ctx == nil || r == nil || r.store == nil {
		return nil, ErrJobInput
	}
	now = now.UTC()
	lease.until = lease.until.UTC()
	var ticket *AlertAttempt
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		checked, err := CheckAlertLeaseTx(ctx, tx, lease, now)
		if err != nil || checked == nil {
			return err
		}
		j := checked.Job
		if !opushAlertSnapshot(j.Snapshot) || !alertAttemptExpiry(j.Snapshot).After(now) {
			return nil
		}
		decision, err := evaluateAlertPolicyTx(ctx, tx, j.Snapshot, now)
		if err != nil {
			return err
		}
		if decision == alertPolicyDND {
			_, err = dropAlertDNDTx(ctx, tx, lease, j.Snapshot, now)
			return err
		}
		if decision != alertPolicyEligible {
			return nil
		}
		before, err := readAlertDropRow(ctx, tx, j.Snapshot.ID)
		if err != nil {
			return err
		}
		meta, err := readAlertAttempt(ctx, tx, j)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(lease.secret[:])
		if meta.present && meta.leaseHash == hash && meta.leaseUntil.Equal(lease.until) {
			return nil
		}
		if j.Attempt > MaxAlertAttempts {
			return ErrJobData
		}
		if j.Attempt == MaxAlertAttempts {
			// 最后一次派发崩溃后重领仍不取得第九次资格；耗尽只在 push 内降级。
			if err = updateAlertJob(ctx, tx, j, "degraded", j.NextAttemptAt, nil, nil, nil); err != nil {
				return err
			}
			// 降级已清除 lease；仍按原快照核对写后身份，失败须回滚整个耗尽更新。
			valid, err := leaseCurrentSnapshot(ctx, tx, j.Snapshot, now, alertLeaseScope)
			if err != nil {
				return err
			}
			if !valid {
				return ErrJobData
			}
			return checkAlertAttemptPolicy(ctx, tx, j.Snapshot, now)
		}
		ticket = &AlertAttempt{snapshot: j.Snapshot, leaseHash: hash, leaseUntil: lease.until, attempt: j.Attempt + 1, startedAt: now}
		expected := j
		expected.Attempt++
		result, err := tx.ExecContext(ctx, `UPDATE push_jobs SET attempt=?,attempt_lease_hash=?,attempt_lease_until_ns=?,
 attempt_started_at_ns=?,result_kind=NULL,result_at_ns=NULL
 WHERE id=? AND state='leased' AND attempt=? AND lease_hash=? AND lease_until_ns=?`,
			ticket.attempt, hash[:], lease.until.UnixNano(), now.UnixNano(), j.Snapshot.ID, j.Attempt, hash[:], lease.until.UnixNano())
		if err != nil {
			return err
		}
		if err = syncOneRow(result); err != nil {
			return err
		}
		post, err := CheckAlertLeaseTx(ctx, tx, lease, now)
		if err != nil {
			return err
		}
		if post == nil {
			return ErrJobData
		}
		got := post.Job
		stored, err := readAlertAttempt(ctx, tx, got)
		if err != nil {
			return err
		}
		// 防止 RAISE(IGNORE) 或异常 trigger 让暂存票据与实际落库状态不符。
		if got != expected || stored != (alertAttemptRecord{present: true, leaseHash: hash, leaseUntil: lease.until, startedAt: now}) {
			return ErrJobData
		}
		before[5], before[21], before[22], before[23], before[24], before[25] = ticket.attempt, hash[:], lease.until.UnixNano(), now.UnixNano(), nil, nil
		if err = checkAlertAttemptRow(ctx, tx, j.Snapshot.ID, before); err != nil {
			return err
		}
		return checkAlertAttemptPolicy(ctx, tx, j.Snapshot, now)
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

type alertAttemptRecord struct {
	present               bool
	leaseHash             [32]byte
	leaseUntil, startedAt time.Time
	outcome               AlertOutcome
	resultAt              time.Time
}

// 单独读取原 SQLite 类型；Scan 到 int64/string 的成功不等于原始存储可信。
func readAlertAttempt(ctx context.Context, tx *sql.Tx, j Job) (alertAttemptRecord, error) {
	var hash []byte
	var until, started, result sql.NullInt64
	var outcome sql.NullString
	var ht, ut, st, ot, rt string
	err := tx.QueryRowContext(ctx, `SELECT attempt_lease_hash,attempt_lease_until_ns,attempt_started_at_ns,result_kind,result_at_ns,
 typeof(attempt_lease_hash),typeof(attempt_lease_until_ns),typeof(attempt_started_at_ns),typeof(result_kind),typeof(result_at_ns)
 FROM push_jobs WHERE id=?`, j.Snapshot.ID).Scan(&hash, &until, &started, &outcome, &result, &ht, &ut, &st, &ot, &rt)
	if err != nil {
		return alertAttemptRecord{}, err
	}
	if ht == "null" && ut == "null" && st == "null" && ot == "null" && rt == "null" {
		return alertAttemptRecord{}, nil
	}
	if ht != "blob" || len(hash) != sha256.Size || ut != "integer" || st != "integer" || !until.Valid || !started.Valid ||
		!opushAlertSnapshot(j.Snapshot) || j.Attempt < 1 || j.Attempt > MaxAlertAttempts ||
		until.Int64 <= started.Int64 || started.Int64 < j.Snapshot.CreatedAt.UnixNano() || started.Int64 >= j.Snapshot.ExpiresAt.UnixNano() {
		return alertAttemptRecord{}, ErrJobData
	}
	m := alertAttemptRecord{present: true, leaseUntil: time.Unix(0, until.Int64).UTC(), startedAt: time.Unix(0, started.Int64).UTC()}
	copy(m.leaseHash[:], hash)
	if ot == "null" && rt == "null" {
		return m, nil
	}
	if ot != "text" || rt != "integer" || !outcome.Valid || !result.Valid || !AlertOutcome(outcome.String).valid() ||
		result.Int64 < started.Int64 || result.Int64 >= until.Int64 || result.Int64 >= j.Snapshot.ExpiresAt.UnixNano() {
		return alertAttemptRecord{}, ErrJobData
	}
	m.outcome = AlertOutcome(outcome.String)
	m.resultAt = time.Unix(0, result.Int64).UTC()
	return m, nil
}

// 只扩原资格完整 opush alert，不给 FCM alert 或 legacy 制造尝试。
func opushAlertSnapshot(j JobSnapshot) bool {
	return j.Purpose == "alert" && j.Provider == "opush" && j.AlertEligible
}

func checkAlertAttemptPolicy(ctx context.Context, tx *sql.Tx, j JobSnapshot, now time.Time) error {
	decision, err := evaluateAlertPolicyTx(ctx, tx, j, now)
	if err != nil {
		return err
	}
	if decision != alertPolicyEligible {
		return ErrJobData
	}
	return ctx.Err()
}

// 精确保留全部 SQLite 原值与 storage class，异常 trigger 必须回滚。
func checkAlertAttemptRow(ctx context.Context, tx *sql.Tx, id string, expected [26]any) error {
	got, err := readAlertDropRow(ctx, tx, id)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(got, expected) {
		return ErrJobData
	}
	return nil
}

// 使用 Time 比较而非纳秒加法：CreatedAt+24h 超出 UnixNano 域时，
// 原有效 ExpiresAt 必定更早，仍安全保留原期限；不改写持久快照。
func alertAttemptExpiry(j JobSnapshot) time.Time {
	expiry := j.ExpiresAt.UTC()
	limit := j.CreatedAt.UTC().Add(24 * time.Hour)
	if limit.Before(expiry) {
		return limit
	}
	return expiry
}

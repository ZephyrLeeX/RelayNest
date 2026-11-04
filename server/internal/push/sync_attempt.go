package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"time"
)

const MaxSyncAttempts int64 = 8

// SyncAttempt 是 COMMIT 后发布的内部脱敏票据。字段私有且不可序列化；
// 不含注册 token、租约秘密或消息正文，也不代表最终发送策略已通过。
// 每次实际派发仅能消费一次 BeginSync 返回的非 nil 票据，不能缓存复用。
type SyncAttempt struct {
	snapshot   JobSnapshot
	leaseHash  [32]byte
	leaseUntil time.Time
	attempt    int64
	startedAt  time.Time
}

func (SyncAttempt) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[push sync attempt]") }
func (SyncAttempt) MarshalJSON() ([]byte, error) { return []byte(`null`), nil }

// BeginSync 持有自己的短写事务；同一 lease 最多返回一次票据、扣一次尝试。
// 即使 Store 的真实 COMMIT 失败也不发布暂存票据，不暴露可被外层误提交的 Tx API。
func (r *JobRepository) BeginSync(ctx context.Context, lease JobLease, now time.Time) (*SyncAttempt, error) {
	if ctx == nil || r == nil || r.store == nil {
		return nil, ErrJobInput
	}
	now = now.UTC()
	lease.until = lease.until.UTC()
	var ticket *SyncAttempt
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		checked, err := CheckSyncLeaseTx(ctx, tx, lease, now)
		if err != nil || checked == nil {
			return err
		}
		j := checked.Job
		meta, err := readSyncAttempt(ctx, tx, j)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(lease.secret[:])
		if meta.present && meta.leaseHash == hash && meta.leaseUntil.Equal(lease.until) {
			return nil
		}
		if j.Attempt > MaxSyncAttempts {
			return ErrJobData
		}
		if j.Attempt == MaxSyncAttempts {
			// 最后一次派发崩溃后重领仍不取得第九次资格；耗尽只在 push 内降级。
			return updateSyncJob(ctx, tx, j, "degraded", j.NextAttemptAt, nil, nil, nil)
		}
		ticket = &SyncAttempt{snapshot: j.Snapshot, leaseHash: hash, leaseUntil: lease.until, attempt: j.Attempt + 1, startedAt: now}
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
		post, err := CheckSyncLeaseTx(ctx, tx, lease, now)
		if err != nil {
			return err
		}
		if post == nil {
			return ErrJobData
		}
		got := post.Job
		stored, err := readSyncAttempt(ctx, tx, got)
		if err != nil {
			return err
		}
		// 防止 RAISE(IGNORE) 或异常 trigger 让暂存票据与实际落库状态不符。
		if got != expected || stored != (syncAttemptRecord{present: true, leaseHash: hash, leaseUntil: lease.until, startedAt: now}) {
			return ErrJobData
		}
		return ctx.Err()
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

type syncAttemptRecord struct {
	present               bool
	leaseHash             [32]byte
	leaseUntil, startedAt time.Time
	outcome               SyncOutcome
	resultAt              time.Time
}

// 单独读取原 SQLite 类型；Scan 到 int64/string 的成功不等于原始存储可信。
func readSyncAttempt(ctx context.Context, tx *sql.Tx, j Job) (syncAttemptRecord, error) {
	var hash []byte
	var until, started, result sql.NullInt64
	var outcome sql.NullString
	var ht, ut, st, ot, rt string
	err := tx.QueryRowContext(ctx, `SELECT attempt_lease_hash,attempt_lease_until_ns,attempt_started_at_ns,result_kind,result_at_ns,
 typeof(attempt_lease_hash),typeof(attempt_lease_until_ns),typeof(attempt_started_at_ns),typeof(result_kind),typeof(result_at_ns)
 FROM push_jobs WHERE id=?`, j.Snapshot.ID).Scan(&hash, &until, &started, &outcome, &result, &ht, &ut, &st, &ot, &rt)
	if err != nil {
		return syncAttemptRecord{}, err
	}
	if ht == "null" && ut == "null" && st == "null" && ot == "null" && rt == "null" {
		return syncAttemptRecord{}, nil
	}
	if ht != "blob" || len(hash) != sha256.Size || ut != "integer" || st != "integer" || !until.Valid || !started.Valid ||
		j.Snapshot.Purpose != "sync" || j.Attempt < 1 || j.Attempt > MaxSyncAttempts ||
		until.Int64 <= started.Int64 || started.Int64 < j.Snapshot.CreatedAt.UnixNano() || started.Int64 >= j.Snapshot.ExpiresAt.UnixNano() {
		return syncAttemptRecord{}, ErrJobData
	}
	m := syncAttemptRecord{present: true, leaseUntil: time.Unix(0, until.Int64).UTC(), startedAt: time.Unix(0, started.Int64).UTC()}
	copy(m.leaseHash[:], hash)
	if ot == "null" && rt == "null" {
		return m, nil
	}
	if ot != "text" || rt != "integer" || !outcome.Valid || !result.Valid || !SyncOutcome(outcome.String).valid() ||
		result.Int64 < started.Int64 || result.Int64 >= until.Int64 || result.Int64 >= j.Snapshot.ExpiresAt.UnixNano() {
		return syncAttemptRecord{}, ErrJobData
	}
	m.outcome = SyncOutcome(outcome.String)
	m.resultAt = time.Unix(0, result.Int64).UTC()
	return m, nil
}

func syncOneRow(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrJobData
	}
	return nil
}

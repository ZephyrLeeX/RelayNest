package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"time"
)

// CheckSyncAttempt 在自有短只读事务中复核已提交票据及原租约。
// true 只保证 now 对应的一致快照仍有效；不消费票据，不能缓存为永久许可，
// 也不保证事务结束后的配置变化或网络外发原子性。
func (r *JobRepository) CheckSyncAttempt(ctx context.Context, lease JobLease, ticket SyncAttempt, now time.Time) (bool, error) {
	if ctx == nil || r == nil || r.store == nil || !jobTimeValid(now) ||
		lease.jobID == "" || lease.secret == [32]byte{} || !jobTimeValid(lease.until) ||
		!ticket.snapshot.valid() || ticket.snapshot.Purpose != "sync" ||
		ticket.attempt < 1 || ticket.attempt > MaxSyncAttempts || ticket.leaseHash == [32]byte{} ||
		!jobTimeValid(ticket.leaseUntil) || !jobTimeValid(ticket.startedAt) {
		return false, ErrJobInput
	}
	// 私有字段仍须完整绑定，不能用另一有效租约替代原 Begin 的身份。
	if lease.jobID != ticket.snapshot.ID || sha256.Sum256(lease.secret[:]) != ticket.leaseHash ||
		!lease.until.Equal(ticket.leaseUntil) || now.Before(ticket.startedAt) {
		return false, nil
	}
	var valid bool
	err := r.store.Read(ctx, func(tx *sql.Tx) error {
		checked, err := CheckSyncLeaseTx(ctx, tx, lease, now.UTC())
		if err != nil || checked == nil {
			return err
		}
		j := checked.Job
		if j.Snapshot != ticket.snapshot || j.Attempt != ticket.attempt {
			return nil
		}
		meta, err := readSyncAttempt(ctx, tx, j)
		if err != nil {
			return err
		}
		valid = meta.present && meta.leaseHash == ticket.leaseHash &&
			meta.leaseUntil.Equal(ticket.leaseUntil) && meta.startedAt.Equal(ticket.startedAt) &&
			meta.outcome == "" && meta.resultAt.IsZero()
		return ctx.Err()
	})
	// 事务内的暂存 true 不能越过 Read 的文件身份复核或 COMMIT 失败。
	if err != nil {
		return false, err
	}
	return valid, nil
}

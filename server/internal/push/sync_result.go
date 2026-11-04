package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"time"
)

// SyncOutcome 只记录平台调用结果；accepted 不是展示、同步或共享已读 ACK。
type SyncOutcome string

const (
	SyncAccepted           SyncOutcome = "accepted"
	SyncTargetInvalid      SyncOutcome = "target_invalid"
	SyncTransient          SyncOutcome = "transient"
	SyncRateLimited        SyncOutcome = "rate_limited"
	SyncConfigurationError SyncOutcome = "configuration_error"
	SyncUnknownDelivery    SyncOutcome = "unknown_delivery"
)

func (o SyncOutcome) valid() bool {
	switch o {
	case SyncAccepted, SyncTargetInvalid, SyncTransient, SyncRateLimited, SyncConfigurationError, SyncUnknownDelivery:
		return true
	}
	return false
}

// ApplySyncResult 在短事务中同时 fencing 当前身份版本、完整原快照、租约及 attempt。
// 释放、接管、过期、重复或版本变更后的结果均 no-op；不修改任何注册或业务表。
func (r *JobRepository) ApplySyncResult(ctx context.Context, ticket SyncAttempt, outcome SyncOutcome, now time.Time) (bool, error) {
	if ctx == nil || r == nil || r.store == nil || !outcome.valid() || !jobTimeValid(now) ||
		!ticket.snapshot.valid() || ticket.snapshot.Purpose != "sync" || ticket.attempt < 1 || ticket.attempt > MaxSyncAttempts ||
		ticket.leaseHash == [32]byte{} || !jobTimeValid(ticket.leaseUntil) || !jobTimeValid(ticket.startedAt) || now.Before(ticket.startedAt) {
		return false, ErrJobInput
	}
	now = now.UTC()
	var changed bool
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		// Check 要求持有 lease 秘密；结果票据刻意不持有秘密，因此这里直接比原摘要，
		// 并复用同一个当前身份快照验证器，不把旧检查缓存为永久许可。
		j, err := ReadJobTx(ctx, tx, ticket.snapshot.ID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if j.State != "leased" || j.Snapshot != ticket.snapshot || j.Attempt != ticket.attempt {
			return nil
		}
		if !ticket.leaseUntil.After(now) || !j.Snapshot.ExpiresAt.After(now) {
			return nil
		}
		var hash []byte
		var until int64
		var ht, ut string
		var integerStorage, textStorage bool
		err = tx.QueryRowContext(ctx, `SELECT lease_hash,lease_until_ns,typeof(lease_hash),typeof(lease_until_ns),
 typeof(attempt)='integer' AND typeof(target_change_seq)='integer' AND typeof(registration_version)='integer'
 AND typeof(settings_version)='integer' AND typeof(bot_seq)='integer' AND typeof(session_generation)='integer'
 AND typeof(created_at_ns)='integer' AND typeof(expires_at_ns)='integer' AND typeof(alert_eligible)='integer',
 typeof(id)='text' AND typeof(session_id)='text' AND typeof(device_id)='text' AND typeof(state)='text'
 AND typeof(next_attempt_at)='text' AND typeof(purpose)='text' AND typeof(provider)='text'
 AND typeof(deployment_id)='text' AND typeof(message_id)='text' AND typeof(bot_id)='text'
 FROM push_jobs WHERE id=?`, j.Snapshot.ID).Scan(&hash, &until, &ht, &ut, &integerStorage, &textStorage)
		if err != nil {
			return err
		}
		if ht != "blob" || len(hash) != sha256.Size || ut != "integer" || until <= 0 || !integerStorage || !textStorage {
			return ErrJobData
		}
		var storedHash [32]byte
		copy(storedHash[:], hash)
		if storedHash != ticket.leaseHash || until != ticket.leaseUntil.UnixNano() {
			return nil
		}
		meta, err := readSyncAttempt(ctx, tx, j)
		if err != nil {
			return err
		}
		if !meta.present || meta.leaseHash != ticket.leaseHash || !meta.leaseUntil.Equal(ticket.leaseUntil) || !meta.startedAt.Equal(ticket.startedAt) || meta.outcome != "" {
			return nil
		}
		valid, err := syncLeaseCurrentSnapshot(ctx, tx, j.Snapshot, now)
		if err != nil || !valid {
			return err
		}
		state, next := "degraded", j.NextAttemptAt
		switch outcome {
		case SyncAccepted:
			state = "accepted"
		case SyncTransient, SyncRateLimited, SyncUnknownDelivery:
			if j.Attempt < MaxSyncAttempts {
				// push 的内部参数：10 秒倍增，至多 10 分钟；不沿用业务 delivery 的参数。
				delay := 10 * time.Second * time.Duration(int64(1)<<(j.Attempt-1))
				if delay > 10*time.Minute {
					delay = 10 * time.Minute
				}
				retry := now.Add(delay)
				// 溢出或已抵原 expiry 时失败关闭，不排一个超过原有效期的重试。
				if jobTimeValid(retry) && retry.After(now) && retry.Before(j.Snapshot.ExpiresAt) {
					state, next = "queued", retry
				}
			}
		}
		if err = updateSyncJob(ctx, tx, j, state, next, &outcome, &now, &meta); err != nil {
			return err
		}
		changed = true
		return ctx.Err()
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// 所有状态写入都核 RowsAffected 和落库投影；忽略写入视为错误并整事务回滚。
func updateSyncJob(ctx context.Context, tx *sql.Tx, j Job, state string, next time.Time, outcome *SyncOutcome, at *time.Time, meta *syncAttemptRecord) error {
	var result sql.Result
	var err error
	if outcome == nil {
		result, err = tx.ExecContext(ctx, `UPDATE push_jobs SET state=?,lease_hash=NULL,lease_until_ns=NULL WHERE id=? AND state='leased' AND attempt=?`, state, j.Snapshot.ID, j.Attempt)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE push_jobs SET state=?,next_attempt_at=?,lease_hash=NULL,lease_until_ns=NULL,result_kind=?,result_at_ns=?
 WHERE id=? AND state='leased' AND attempt=? AND result_kind IS NULL`, state, next.UTC().Format(jobTimestamp), string(*outcome), at.UnixNano(), j.Snapshot.ID, j.Attempt)
	}
	if err != nil {
		return err
	}
	if err = syncOneRow(result); err != nil {
		return err
	}
	got, err := ReadJobTx(ctx, tx, j.Snapshot.ID)
	if err != nil {
		return err
	}
	expected := j
	expected.State = state
	expected.NextAttemptAt = next
	if got != expected {
		return ErrJobData
	}
	stored, err := readSyncAttempt(ctx, tx, got)
	if err != nil {
		return err
	}
	if meta != nil {
		want := *meta
		want.outcome = *outcome
		want.resultAt = *at
		if stored != want {
			return ErrJobData
		}
	}
	var cleared bool
	if err = tx.QueryRowContext(ctx, `SELECT lease_hash IS NULL AND lease_until_ns IS NULL FROM push_jobs WHERE id=?`, j.Snapshot.ID).Scan(&cleared); err != nil {
		return err
	}
	if !cleared {
		return ErrJobData
	}
	return nil
}

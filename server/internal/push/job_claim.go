package push

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"time"
)

// ReserveJobsTx 只原子预留候选，不是发送前注册、配置、DND 或共享已读复核。
// 必须使用可信短写事务；返回句柄在外层 COMMIT 成功前均是暂存值，不得发布。
func ReserveJobsTx(ctx context.Context, tx *sql.Tx, now time.Time, ttl time.Duration, limit int) ([]JobReservation, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return nil, err
	}
	until := now.Add(ttl)
	if !jobTimeValid(now) || ttl <= 0 || !jobTimeValid(until) || limit < 1 || limit > MaxJobReservations {
		return nil, ErrJobInput
	}
	var result []JobReservation
	err := jobSavepoint(ctx, tx, func() error {
		// 先为每设备选确定的最早候选，再应用全局边界；不让同设备挤满批次。
		rows, err := tx.QueryContext(ctx, `WITH candidates AS (
 SELECT p.id,p.next_attempt_at,p.device_id,
 row_number() OVER(PARTITION BY p.device_id ORDER BY p.next_attempt_at,p.id) AS rank
 FROM push_jobs p JOIN sessions s ON s.id=p.session_id AND s.device_id=p.device_id
 JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation
 WHERE (p.state='queued' OR (p.state='leased' AND p.lease_until_ns<=?))
 AND p.next_attempt_at<=? AND p.expires_at_ns>?
 AND s.client_type='android' AND s.revoked=0 AND s.auth_generation=p.session_generation
 AND s.idle_expires_at>? AND s.absolute_expires_at>?
 AND NOT EXISTS(SELECT 1 FROM push_jobs holder WHERE holder.device_id=p.device_id AND holder.state='leased' AND holder.lease_until_ns>?)
 ) SELECT id FROM candidates WHERE rank=1 ORDER BY next_attempt_at,id LIMIT ?`,
			now.UnixNano(), now.UTC().Format(jobTimestamp), now.UnixNano(), now.UnixNano(), now.UnixNano(), now.UnixNano(), limit)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		result = make([]JobReservation, 0, len(ids))
		for _, id := range ids {
			// 每个入选设备最多一个旧持有者；只回收本批设备，避免全表清理写入。
			if _, err = tx.ExecContext(ctx, `UPDATE push_jobs SET state='queued',lease_hash=NULL,lease_until_ns=NULL
 WHERE state='leased' AND lease_until_ns<=? AND device_id=(SELECT device_id FROM push_jobs WHERE id=?)`, now.UnixNano(), id); err != nil {
				return err
			}
			lease := JobLease{jobID: id, until: until.UTC()}
			if _, err = rand.Read(lease.secret[:]); err != nil {
				return err
			}
			hash := sha256.Sum256(lease.secret[:])
			updated, e := tx.ExecContext(ctx, `UPDATE push_jobs SET state='leased',lease_hash=?,lease_until_ns=? WHERE id=? AND state='queued'`, hash[:], until.UnixNano(), id)
			if e != nil {
				return e
			}
			n, e := updated.RowsAffected()
			if e != nil {
				return e
			}
			if n != 1 {
				return ErrJobData
			}
			j, e := ReadJobTx(ctx, tx, id)
			if e != nil {
				return e
			}
			result = append(result, JobReservation{Job: j, Lease: lease})
		}
		return ctx.Err()
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReleaseJobTx 在秘密摘要、job 及原租约期限全部吻合且未过期时释放。
// 迟到释放或接管前的旧句柄只会 no-op；不会推进 attempt 或恢复旧提醒资格。
func ReleaseJobTx(ctx context.Context, tx *sql.Tx, lease JobLease, now time.Time) (bool, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return false, err
	}
	if lease.jobID == "" || !jobTimeValid(now) || !jobTimeValid(lease.until) || lease.secret == [32]byte{} {
		return false, ErrJobInput
	}
	hash := sha256.Sum256(lease.secret[:])
	result, err := tx.ExecContext(ctx, `UPDATE push_jobs SET state='queued',lease_hash=NULL,lease_until_ns=NULL
 WHERE id=? AND state='leased' AND lease_hash=? AND lease_until_ns=? AND lease_until_ns>?`,
		lease.jobID, hash[:], lease.until.UnixNano(), now.UnixNano())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1 && err == nil, err
}

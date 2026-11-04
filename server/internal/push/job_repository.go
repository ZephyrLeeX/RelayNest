package push

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"relaynest/server/internal/storage"
)

// JobRepository 仅供内部调度组合短事务；没有公开 Actor 或 HTTP 权限包装。
type JobRepository struct{ store *storage.Store }

func NewJobRepository(store *storage.Store) *JobRepository { return &JobRepository{store: store} }

// CreateJobTx 保存调用方持有的原事件快照，不读取当前配置来提升 legacy。
// 不提交事务；调用方须在任意错误时回滚，提交前不得发布结果。
func CreateJobTx(ctx context.Context, tx *sql.Tx, j JobSnapshot, next time.Time) error {
	if err := jobTxInput(ctx, tx); err != nil {
		return err
	}
	if !j.valid() || !jobTimeValid(next) || next.Before(j.CreatedAt) {
		return ErrJobInput
	}
	deployment, err := storage.ReadInstanceIDTx(ctx, tx)
	if err != nil {
		return err
	}
	if deployment != j.DeploymentID {
		return ErrJobData
	}
	// 目标水位可能包含管理员 delivery 的后续事件；不可当成 message.created 水位。
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
 SELECT 1 FROM messages m JOIN sessions s ON s.id=? AND s.device_id=?
 JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation
 WHERE m.id=? AND m.bot_id=? AND m.bot_seq=? AND m.created_change_seq<=?
 AND s.auth_generation=? AND s.client_type='android' AND s.revoked=0
 AND s.idle_expires_at>? AND s.absolute_expires_at>?
 AND EXISTS(SELECT 1 FROM device_push_registrations r WHERE r.session_id=s.id
            AND r.provider=? AND r.registration_version=? AND r.token IS NOT NULL)
 AND COALESCE((SELECT settings_version FROM device_notification_settings WHERE device_id=s.device_id),0)=?)`,
		j.SessionID, j.DeviceID, j.MessageID, j.BotID, j.BotSeq, j.TargetChangeSeq, j.SessionGeneration,
		j.CreatedAt.UnixNano(), j.CreatedAt.UnixNano(), j.Provider, j.RegistrationVersion, j.SettingsVersion).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrJobData
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,next_attempt_at,
 purpose,provider,registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,
 session_generation,created_at_ns,expires_at_ns,alert_eligible) VALUES(?,?,?,?,'queued',?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.SessionID, j.DeviceID, j.TargetChangeSeq, next.UTC().Format(jobTimestamp), j.Purpose, j.Provider,
		j.RegistrationVersion, j.SettingsVersion, j.DeploymentID, j.MessageID, j.BotID, j.BotSeq,
		j.SessionGeneration, j.CreatedAt.UnixNano(), j.ExpiresAt.UnixNano(), j.AlertEligible)
	return err
}

// MergeJobWatermarkTx 只合并 sync 的目标水位，不改原资格、版本、时间或租约。
func MergeJobWatermarkTx(ctx context.Context, tx *sql.Tx, id string, target int64) (bool, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return false, err
	}
	if id == "" || target < 1 {
		return false, ErrJobInput
	}
	result, err := tx.ExecContext(ctx, `UPDATE push_jobs SET target_change_seq=? WHERE id=? AND purpose='sync' AND state='queued' AND target_change_seq<?`, target, id, target)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1 && err == nil, err
}

const jobColumns = `id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,purpose,provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,
 created_at_ns,expires_at_ns,alert_eligible`

type jobScanner interface{ Scan(...any) error }

func jobScan(row jobScanner) (Job, error) {
	var j Job
	var stamp string
	var created, expires int64
	err := row.Scan(&j.Snapshot.ID, &j.Snapshot.SessionID, &j.Snapshot.DeviceID, &j.Snapshot.TargetChangeSeq,
		&j.State, &j.Attempt, &stamp, &j.Snapshot.Purpose, &j.Snapshot.Provider, &j.Snapshot.RegistrationVersion,
		&j.Snapshot.SettingsVersion, &j.Snapshot.DeploymentID, &j.Snapshot.MessageID, &j.Snapshot.BotID,
		&j.Snapshot.BotSeq, &j.Snapshot.SessionGeneration, &created, &expires, &j.Snapshot.AlertEligible)
	if err != nil {
		return Job{}, err
	}
	j.Snapshot.CreatedAt = time.Unix(0, created).UTC()
	j.Snapshot.ExpiresAt = time.Unix(0, expires).UTC()
	j.NextAttemptAt, err = time.Parse(jobTimestamp, stamp)
	if err != nil || !j.Snapshot.valid() || (j.State != "queued" && j.State != "leased" && j.State != "accepted" && j.State != "degraded") || j.Attempt < 0 {
		return Job{}, ErrJobData
	}
	return j, nil
}

// ReadJobTx 只读取完整 job；legacy 必须继续通过独立的旧 hint 投影消费。
func ReadJobTx(ctx context.Context, tx *sql.Tx, id string) (Job, error) {
	if err := jobTxInput(ctx, tx); err != nil {
		return Job{}, err
	}
	if id == "" {
		return Job{}, ErrJobInput
	}
	return jobScan(tx.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM push_jobs WHERE id=? AND state<>'pending'`, id))
}

// 多行预留在保存点内原子执行，即使调用方错误地继续提交，也不留下半批预留。
// 取消后清理使用独立 context；底层已自动回滚时保留原错误。
func jobSavepoint(ctx context.Context, tx *sql.Tx, fn func() error) error {
	if _, err := tx.ExecContext(ctx, `SAVEPOINT push_job_operation`); err != nil {
		return err
	}
	err := fn()
	if err == nil {
		_, err = tx.ExecContext(ctx, `RELEASE push_job_operation`)
	}
	if err != nil {
		_, rollbackErr := tx.ExecContext(context.Background(), `ROLLBACK TO push_job_operation`)
		_, releaseErr := tx.ExecContext(context.Background(), `RELEASE push_job_operation`)
		return errors.Join(err, rollbackErr, releaseErr)
	}
	return nil
}

// Reserve 在 Store 提交成功后才发布候选；等待网络必须发生在返回之后。
func (r *JobRepository) Reserve(ctx context.Context, now time.Time, ttl time.Duration, limit int) ([]JobReservation, error) {
	if ctx == nil || r == nil || r.store == nil {
		return nil, ErrJobInput
	}
	var result []JobReservation
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		var err error
		result, err = ReserveJobsTx(ctx, tx, now, ttl, limit)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (r *JobRepository) Release(ctx context.Context, lease JobLease, now time.Time) (bool, error) {
	if ctx == nil || r == nil || r.store == nil {
		return false, ErrJobInput
	}
	var released bool
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		var err error
		released, err = ReleaseJobTx(ctx, tx, lease, now)
		return err
	})
	if err != nil {
		return false, err
	}
	return released, nil
}

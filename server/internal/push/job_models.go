// Package push 保存内部推送候选；租约预留不代表最终发送或提醒资格。
package push

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrJobInput = errors.New("push: invalid job input")
	ErrJobData  = errors.New("push: invalid job snapshot")
)

// jobSuppressed 表示原 alert 已命中 DND，后续配置或时段改变不能重新预留。
const jobSuppressed = "suppressed"

const MaxJobReservations = 128
const jobTimestamp = "2006-01-02T15:04:05.000000000Z"

// JobSnapshot 是原事件快照；不包含注册 token、消息正文或任何外部投递参数。
// SessionGeneration 在创建时从真实会话取得，不能以公开 Actor 字段替代授权。
type JobSnapshot struct {
	ID                  string
	SessionID           string
	DeviceID            string
	Purpose             string
	TargetChangeSeq     int64
	Provider            string
	RegistrationVersion int64
	SettingsVersion     int64
	DeploymentID        string
	MessageID           string
	BotID               string
	BotSeq              int64
	SessionGeneration   int64
	CreatedAt           time.Time
	ExpiresAt           time.Time
	AlertEligible       bool
}

type Job struct {
	Snapshot      JobSnapshot
	State         string
	Attempt       int64
	NextAttemptAt time.Time
}

// JobLease 的随机秘密只在进程内持有，JSON 与任意 fmt 格式均不可导出秘密。
// 句柄仅供可信内部调用方使用，不是 HTTP DTO 或最终发送凭证。
type JobLease struct {
	jobID  string
	secret [32]byte
	until  time.Time
}

func (JobLease) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[push job lease]") }
func (JobLease) MarshalJSON() ([]byte, error) { return []byte(`null`), nil }

type JobReservation struct {
	Job   Job
	Lease JobLease `json:"-"`
}

func jobTxInput(ctx context.Context, tx *sql.Tx) error {
	if ctx == nil || tx == nil {
		return ErrJobInput
	}
	return ctx.Err()
}
func jobTimeValid(t time.Time) bool {
	return !t.IsZero() && t.UnixNano() > 0 && time.Unix(0, t.UnixNano()).Equal(t)
}
func (j JobSnapshot) valid() bool {
	return j.ID != "" && j.SessionID != "" && j.DeviceID != "" && (j.Purpose == "sync" || j.Purpose == "alert") &&
		j.TargetChangeSeq > 0 && (j.Provider == "fcm" || j.Provider == "opush") && j.RegistrationVersion > 0 && j.SettingsVersion >= 0 &&
		j.DeploymentID != "" && j.MessageID != "" && j.BotID != "" && j.BotSeq > 0 && j.SessionGeneration > 0 &&
		jobTimeValid(j.CreatedAt) && jobTimeValid(j.ExpiresAt) && j.ExpiresAt.After(j.CreatedAt) &&
		(j.Purpose != "sync" || (j.Provider == "fcm" && !j.AlertEligible))
}

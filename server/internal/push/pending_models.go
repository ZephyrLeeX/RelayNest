// Package push 提供同步推送待办的内部只读投影。
package push

import (
	"errors"

	"relaynest/server/internal/changes"
)

var (
	ErrPendingInput = errors.New("push: invalid pending read input or transaction")
	ErrPendingData  = errors.New("push: invalid pending metadata")
)

// MaxPendingSessions 限制一次返回的会话候选数，设备组数不会超过此数。
const MaxPendingSessions = 200

// PendingSession 保留会话隔离；水位仅归并本会话已到期的同步待办。
// 它不是领取凭据，也不代表注册、提醒或发送资格。
type PendingSession struct {
	SessionID       string
	TargetChangeSeq changes.Sequence
}

// PendingDevice 将同一设备的会话候选分组，但不合并不同会话的身份或水位。
type PendingDevice struct {
	DeviceID string
	Sessions []PendingSession
}

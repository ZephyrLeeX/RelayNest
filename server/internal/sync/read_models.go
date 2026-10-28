// Package sync 提供共享已读的受信事务仓储，不提供认证或 HTTP 投影。
package sync

import (
	"errors"
	"time"
)

// 以下仅为内部仓储错误分类，不定义 HTTP 错误码。
var (
	ErrInvalidReadSequence  = errors.New("sync: read sequence outside bot range")
	ErrReadStateUnavailable = errors.New("sync: read state unavailable")
)

// ReadState 中的序号属于 Bot 内容水位，不能用 change_seq 代替。
// 返回状态在调用方事务提交成功前仅为候选，不得发布。
type ReadState struct {
	BotID        string
	ReadSeq      int64
	StateVersion int64
	UpdatedAt    time.Time
	UnreadCount  int64
}

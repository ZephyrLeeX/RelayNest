// Package messages 提供消息创建事务使用的持久数据，不提供 HTTP/同步投影。
package messages

import (
	"encoding/json"
	"time"

	"relaynest/server/internal/changes"
)

type Sender string

const (
	Application   Sender = "application"
	Administrator Sender = "administrator"
)

// Message 是当前持久内容。BotSeq 为 SQLite 正整数，不能直接作为 JSON 数字输出；
// 对外 Sequence 必须转为规范十进制字符串，Revision 仍为整数版本。
// CreatedChangeSeq 为 nil 只用于创建事务内，S03 必须回填同事务 Append 的真实结果后提交。
// OriginalReplyToMessageID 是对外引用 ID；ReplyToMessageID 可随源消息清理变为 nil。
type Message struct {
	ID                       string
	BotID                    string
	BotSeq                   int64
	CreatedChangeSeq         *changes.Sequence
	Sender                   Sender
	Revision                 int64
	Format                   string
	Text                     string
	ButtonsJSON              json.RawMessage
	ReplyToMessageID         *string
	OriginalReplyToMessageID *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// PendingDelivery 仅供 S03 同事务排队；PayloadJSON 是已验证的完整
// MessageDeliveryEvent 不可变快照，不是 Message 当前内容的引用，也不是同步事件。
// 初始 State/StateVersion/Attempt/RetryGeneration 由 SQL 默认 pending/1/0/0 写入。
type PendingDelivery struct {
	DeliveryID    string
	EventID       string
	BotID         string
	PayloadJSON   json.RawMessage
	EnqueueSeq    changes.Sequence
	NextAttemptAt time.Time
}

// PendingPush 只携带同步目标水位；正式注册枚举/偏好校验及发送由后续任务接线。
// S03 还须同事务验证 Android 会话有效性、代次和过期，不以外键存在代替授权。
type PendingPush struct {
	ID              string
	SessionID       string
	DeviceID        string
	TargetChangeSeq changes.Sequence
	NextAttemptAt   time.Time
}

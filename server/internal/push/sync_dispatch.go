package push

import (
	"context"
	"time"
)

// SyncEnvelope 仅供内部框架验证；不是生产平台请求或注册凭据桥接合同。
// 值拷贝只带稳定 job 键及静默同步元数据，不能访问业务对象或尝试票据。
type SyncEnvelope struct {
	JobKey          string
	Purpose         string
	Provider        string
	TargetChangeSeq int64
	ExpiresAt       time.Time
}

// SyncDispatch 的实现由可信内部调用方提供；本包不提供平台或默认成功实现。
// 调用可能忽略取消而仍在途，返回 accepted 只表示受理，不表示展示或已读。
type SyncDispatch interface {
	Dispatch(context.Context, SyncEnvelope) (SyncOutcome, error)
}

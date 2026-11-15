package push

import (
	"context"
	"time"
)

// OPushAlertEnvelope 仅用于内部假 adapter 框架验证，不是生产平台或凭据桥接合同。
// 只传稳定 job 键、用途、provider 和有效期限，不暴露身份、正文或尝试票据。
type OPushAlertEnvelope struct {
	JobKey    string
	Purpose   string
	Provider  string
	ExpiresAt time.Time
}

// OPushAlertDispatch 由可信内部测试调用方提供；本包不装配平台或默认成功实现。
// accepted 仅表示受理；取消无法撤回忽略 context 的在途调用。
type OPushAlertDispatch interface {
	Dispatch(context.Context, OPushAlertEnvelope) (AlertOutcome, error)
}

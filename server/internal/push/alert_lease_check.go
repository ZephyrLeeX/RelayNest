package push

import (
	"context"
	"database/sql"
	"time"
)

// AlertLeaseSnapshot 是当前可信事务中仍匹配原 alert 租约的脱敏原快照。
// 不含租约秘密、注册 token 或消息内容；不是最终发送许可，不可跨事务缓存为授权。
type AlertLeaseSnapshot struct {
	Job Job
}

// CheckAlertLeaseTx 仅读复核原 alert 资格、租约 fence 和当前身份/版本。
// 无当前资格（含非原 TEXT Android 类型）返回 nil,nil；输入、SQL 或显式坏数据返回错误。
// 只绑定原提供方注册；OPPO 注册存在不代表厂商模式已激活或具备平台发送资格。
// 不检查策略、DND、已读、源存在性或平台 readiness，不领取、续租或写入任何状态。
func CheckAlertLeaseTx(ctx context.Context, tx *sql.Tx, lease JobLease, now time.Time) (*AlertLeaseSnapshot, error) {
	j, err := checkLeaseTx(ctx, tx, lease, now, alertLeaseScope)
	if err != nil || j == nil {
		return nil, err
	}
	return &AlertLeaseSnapshot{Job: *j}, nil
}

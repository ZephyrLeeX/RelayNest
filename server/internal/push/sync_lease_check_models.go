package push

// SyncLeaseSnapshot 是当前可信事务中仍匹配原租约的脱敏快照。
// 它不包含租约秘密或注册 token，也不是网络发送资格；调用方不能跨事务
// 缓存本次检查为永久许可，最终发送策略和结果 fencing 仍由后续组合负责。
type SyncLeaseSnapshot struct {
	Job Job
}

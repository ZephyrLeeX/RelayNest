package messages

// MaxHistoryLimit 是仓储单次物化的内部资源上限，不是 HTTP 分页契约。
const MaxHistoryLimit = 200

// HistoryQuery 的边界是排他的 Bot 序号；nil 表示首屏，不使用 offset。
// 它不负责验证公开游标的签名、身份或 Bot 绑定。
type HistoryQuery struct {
	BeforeBotSeq *int64
	Limit        int
}

// HistoryPage 只含当前已有的持久字段，不包含附件投影或引用正文。
// 返回值在外层授权和事务最终成功前均为候选，调用方失败时必须丢弃。
type HistoryPage struct {
	Items   []Message
	HasMore bool
}

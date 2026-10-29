package messages

import (
	"errors"

	"relaynest/server/internal/changes"
)

var (
	ErrRevisionConflict  = errors.New("messages: revision conflict")
	ErrRevisionExhausted = errors.New("messages: revision exhausted")
)

// EditTextButtonsInput仅用于内部可信事务，不对应公开EditMessageRequest。
// 回复和附件没有输入字段，保持原关系；正文及按钮完整替换而不合并。
type EditTextButtonsInput struct {
	BotID            string
	MessageID        string
	ExpectedRevision int64
	Format           string
	Text             string
	Buttons          [][]Button
}

// Edited始终是事务内候选。事务拥有者须传播任何错误，且仅在真实COMMIT成功后
// 才能发布；失败必须丢弃候选，不能把此结果当作持久成功回执。
type Edited struct {
	Message       Message
	LastChangeSeq changes.Sequence
}

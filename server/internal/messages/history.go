package messages

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"relaynest/server/internal/changes"
)

var ErrHistoryNotFound = errors.New("messages: history resource not found")

const historyColumns = `id,bot_id,bot_seq,created_change_seq,sender,revision,format,text,buttons_json,reply_to_message_id,original_reply_to_message_id,created_at,updated_at`

// ListHistoryTx 仅供受信调用方在已授权的事务内读取。它不认证 Actor、不开事务，
// 也不检查 Bot 启用状态：管理员仍可查看停用 Bot 的历史。
func (r *Repository) ListHistoryTx(ctx context.Context, tx *sql.Tx, botID string, query HistoryQuery) (HistoryPage, error) {
	if tx == nil || botID == "" || query.Limit < 1 || query.Limit > MaxHistoryLimit || (query.BeforeBotSeq != nil && *query.BeforeBotSeq < 1) {
		return HistoryPage{}, ErrInvalid
	}
	if err := historyBotTx(ctx, tx, botID); err != nil {
		return HistoryPage{}, err
	}
	statement := `SELECT ` + historyColumns + ` FROM messages WHERE bot_id=?`
	args := []any{botID}
	if query.BeforeBotSeq != nil {
		statement += ` AND bot_seq<?`
		args = append(args, *query.BeforeBotSeq)
	}
	statement += ` ORDER BY bot_seq DESC LIMIT ?`
	args = append(args, query.Limit+1)
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := make([]Message, 0, query.Limit+1)
	for rows.Next() {
		message, err := scanHistoryMessage(rows)
		if err != nil {
			return HistoryPage{}, err
		}
		items = append(items, message)
	}
	if err := rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	if err := rows.Close(); err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Items: items, HasMore: len(items) > query.Limit}
	if page.HasMore {
		page.Items = items[:query.Limit:query.Limit]
	}
	for i := range page.Items {
		if err := historyAttachmentRefsTx(ctx, tx, &page.Items[i]); err != nil {
			return HistoryPage{}, err
		}
	}
	return page, nil
}

// GetHistoryMessageTx 按明确的 Bot 域读取当前详情，不拼接可能已自然清理的引用正文。
func (r *Repository) GetHistoryMessageTx(ctx context.Context, tx *sql.Tx, botID, messageID string) (Message, error) {
	if tx == nil || botID == "" || messageID == "" {
		return Message{}, ErrInvalid
	}
	if err := historyBotTx(ctx, tx, botID); err != nil {
		return Message{}, err
	}
	message, err := scanHistoryMessage(tx.QueryRowContext(ctx, `SELECT `+historyColumns+` FROM messages WHERE bot_id=? AND id=?`, botID, messageID))
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrHistoryNotFound
	}
	if err != nil {
		return Message{}, err
	}
	if err := historyAttachmentRefsTx(ctx, tx, &message); err != nil {
		return Message{}, err
	}
	return message, nil
}

// GetHistoryMessageByIDTx 仅供已授权读事务使用；Bot 解析与详情必须共享同一事务，
// 不向 HTTP 层暴露未经授权的 Bot 选择器，也不预先读取候选正文。
func (r *Repository) GetHistoryMessageByIDTx(ctx context.Context, tx *sql.Tx, messageID string) (Message, error) {
	if tx == nil || messageID == "" {
		return Message{}, ErrInvalid
	}
	var botID string
	if err := tx.QueryRowContext(ctx, `SELECT bot_id FROM messages WHERE id=?`, messageID).Scan(&botID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Message{}, ErrHistoryNotFound
		}
		return Message{}, err
	}
	return r.GetHistoryMessageTx(ctx, tx, botID, messageID)
}

func historyBotTx(ctx context.Context, tx *sql.Tx, botID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM bots WHERE id=?`, botID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHistoryNotFound
	}
	return err
}

// scanHistoryMessage 同时服务行集和详情；解析失败不向调用方返回部分正文。
func scanHistoryMessage(row interface{ Scan(...any) error }) (Message, error) {
	var m Message
	var sequence sql.NullInt64
	var reply, original sql.NullString
	var buttons, created, updated string
	if err := row.Scan(&m.ID, &m.BotID, &m.BotSeq, &sequence, &m.Sender, &m.Revision, &m.Format, &m.Text, &buttons, &reply, &original, &created, &updated); err != nil {
		return Message{}, err
	}
	var err error
	if m.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return Message{}, err
	}
	if m.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return Message{}, err
	}
	m.ButtonsJSON = []byte(buttons)
	if sequence.Valid {
		v := changes.Sequence(sequence.Int64)
		m.CreatedChangeSeq = &v
	}
	if reply.Valid {
		m.ReplyToMessageID = &reply.String
	}
	if original.Valid {
		m.OriginalReplyToMessageID = &original.String
	}
	return m, nil
}

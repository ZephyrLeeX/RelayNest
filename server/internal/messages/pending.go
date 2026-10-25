package messages

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"relaynest/server/internal/changes"
)

// 快照按冻结 MessageDeliveryEvent 写入全部 Message 键；内部模型不是 HTTP DTO。
// 有限文本路径固定空附件/按钮和 null 引用，序号保持十进制字符串。
type deliveryMessage struct {
	ID               string     `json:"id"`
	BotID            string     `json:"bot_id"`
	BotSeq           string     `json:"bot_seq"`
	Revision         int64      `json:"revision"`
	Sender           Sender     `json:"sender"`
	Format           string     `json:"format"`
	Text             string     `json:"text"`
	Attachments      []string   `json:"attachments"`
	ReplyToMessageID *string    `json:"reply_to_message_id"`
	Buttons          [][]string `json:"buttons"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (r *Repository) enqueueDelivery(ctx context.Context, tx *sql.Tx, m Message, last *changes.Sequence) (*PendingDelivery, error) {
	did, err := r.uuid()
	if err != nil {
		return nil, err
	}
	eid, err := r.uuid()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(struct {
		Type    string          `json:"type"`
		Message deliveryMessage `json:"message"`
	}{"message.created", deliveryMessage{m.ID, m.BotID, strconv.FormatInt(m.BotSeq, 10), m.Revision, m.Sender, m.Format, m.Text, []string{}, nil, [][]string{}, m.CreatedAt, m.UpdatedAt}})
	if err != nil {
		return nil, err
	}
	// 排队变化与消息创建是两个事件；created_change_seq 只能指向消息创建的 MessageUpsert。
	seq, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.DeliveryUpsert, EntityID: did, Revision: 1, BotID: &m.BotID, CreatedAt: m.CreatedAt})
	if err != nil {
		return nil, err
	}
	if err = requireOne(tx.ExecContext(ctx, `INSERT INTO deliveries(delivery_id,event_id,bot_id,payload_json,enqueue_seq,next_attempt_at) VALUES(?,?,?,?,?,?)`, did, eid, m.BotID, string(payload), seq, m.CreatedAt.Format(time.RFC3339Nano))); err != nil {
		return nil, err
	}
	*last = seq
	return &PendingDelivery{DeliveryID: did, EventID: eid, BotID: m.BotID, PayloadJSON: payload, EnqueueSeq: seq, NextAttemptAt: m.CreatedAt}, nil
}

func (r *Repository) enqueuePushes(ctx context.Context, tx *sql.Tx, targets []PushTarget, now time.Time, seq changes.Sequence) ([]PendingPush, error) {
	jobs := make([]PendingPush, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		if target.SessionID == "" || target.DeviceID == "" || seen[target.SessionID] {
			return nil, ErrPushTarget
		}
		seen[target.SessionID] = true
		// 会话表是当前唯一绑定来源。FK 不能代替撤销、期限与管理员代次复核；
		// 不在这里发明推送注册/偏好选择规则，后续发送时仍须再次校验。
		var valid int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM sessions s JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation WHERE s.id=? AND s.device_id=? AND s.client_type='android' AND s.revoked=0 AND s.idle_expires_at>? AND s.absolute_expires_at>?`, target.SessionID, target.DeviceID, now.Unix(), now.Unix()).Scan(&valid)
		if err == sql.ErrNoRows {
			return nil, ErrPushTarget
		}
		if err != nil {
			return nil, err
		}
		id, err := r.uuid()
		if err != nil {
			return nil, err
		}
		if err = requireOne(tx.ExecContext(ctx, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES(?,?,?,?,?)`, id, target.SessionID, target.DeviceID, seq, now.Format(time.RFC3339Nano))); err != nil {
			return nil, err
		}
		jobs = append(jobs, PendingPush{ID: id, SessionID: target.SessionID, DeviceID: target.DeviceID, TargetChangeSeq: seq, NextAttemptAt: now})
	}
	return jobs, nil
}

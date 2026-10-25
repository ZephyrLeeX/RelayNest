package messages

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	"time"
	"unicode/utf8"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
)

var (
	ErrInvalid           = errors.New("messages: invalid input or transaction")
	ErrUnsupported       = errors.New("messages: attachments and references not supported")
	ErrSequenceExhausted = errors.New("messages: bot sequence exhausted")
	ErrAffectedRows      = errors.New("messages: unexpected affected rows")
	ErrPushTarget        = errors.New("messages: invalid push target")
)

// TextInput 仅承接有限文本路径；正式业务校验、幂等及附件/引用由后续任务提供。
// PushTargets 是调用方已选出的有界目标，不代表订阅或推送注册枚举结果。
type TextInput struct {
	BotID            string
	Format           string
	Text             string
	Attachments      []string
	ReplyToMessageID *string
	PushTargets      []PushTarget
}

type PushTarget struct{ SessionID, DeviceID string }

// Created 在事务 primitive 返回时仍是候选，只有外层提交成功后才能对外发布。
type Created struct {
	Message       Message
	Delivery      *PendingDelivery
	Pushes        []PendingPush
	LastChangeSeq changes.Sequence
}

type Repository struct {
	store  *storage.Store
	now    func() time.Time
	random io.Reader
}

func New(store *storage.Store) *Repository {
	return &Repository{store: store, now: time.Now, random: rand.Reader}
}

func (r *Repository) uuid() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(r.random, b[:]); err != nil {
		return "", fmt.Errorf("messages UUID: %w", err)
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// CreateApplication 接受真实 Authenticate 得到的身份；排队后在唯一写事务复核。
// 提交失败不返回候选 UUID、序号或待办。
func (r *Repository) CreateApplication(ctx context.Context, identity bots.Identity, in TextInput) (Created, error) {
	var candidate Created
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		var err error
		candidate, err = r.CreateApplicationTx(ctx, tx, identity, in)
		return err
	})
	if err != nil {
		return Created{}, err
	}
	return candidate, nil
}

// CreateApplicationTx 供幂等业务复用既有写事务。调用方必须传播错误并负责提交；
// 不得自行提交/保留 tx，不得在提交前发布返回的候选结果。
func (r *Repository) CreateApplicationTx(ctx context.Context, tx *sql.Tx, identity bots.Identity, in TextInput) (Created, error) {
	if tx == nil {
		return Created{}, ErrInvalid
	}
	if err := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Send); err != nil {
		return Created{}, err
	}
	return r.createTx(ctx, tx, in, Application)
}

// CreateAdministratorTx 仅供受信业务在 auth.SessionStore.WithAuthorizedWrite 的同一
// 事务内调用。此 primitive 不认证管理员，不是公开请求入口；调用方仍须完成 G1。
// sender 固定，不允许请求指定。任何错误必须使外层事务回滚，候选只在提交后发布。
func (r *Repository) CreateAdministratorTx(ctx context.Context, tx *sql.Tx, in TextInput) (Created, error) {
	if tx == nil {
		return Created{}, ErrInvalid
	}
	var enabled, receive bool
	if err := tx.QueryRowContext(ctx, "SELECT enabled,can_receive FROM bots WHERE id=?", in.BotID).Scan(&enabled, &receive); err != nil {
		return Created{}, err
	}
	if !enabled || !receive {
		return Created{}, bots.ErrForbidden
	}
	return r.createTx(ctx, tx, in, Administrator)
}

func requireOne(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrAffectedRows
	}
	return nil
}

func (r *Repository) createTx(ctx context.Context, tx *sql.Tx, in TextInput, sender Sender) (Created, error) {
	if len(in.Attachments) != 0 || in.ReplyToMessageID != nil {
		return Created{}, ErrUnsupported
	}
	if in.BotID == "" || (in.Format != "plain" && in.Format != "markdown") || len(in.Text) == 0 || len(in.Text) > 64*1024 || !utf8.ValidString(in.Text) || len(in.PushTargets) > 100 {
		return Created{}, ErrInvalid
	}
	now := r.now().UTC()
	if now.IsZero() || now.Year() < 1 || now.Year() > 9999 {
		return Created{}, ErrInvalid
	}
	id, err := r.uuid()
	if err != nil {
		return Created{}, err
	}
	var seq int64
	if err = tx.QueryRowContext(ctx, "SELECT bot_seq FROM bots WHERE id=?", in.BotID).Scan(&seq); err != nil {
		return Created{}, err
	}
	if seq < 0 || seq == math.MaxInt64 {
		return Created{}, ErrSequenceExhausted
	}
	// 显式 CAS/行数检查防止计数溢出及异常触发器被当作成功；不触碰 Bot 元数据版本。
	if err = requireOne(tx.ExecContext(ctx, "UPDATE bots SET bot_seq=? WHERE id=? AND bot_seq=?", seq+1, in.BotID, seq)); err != nil {
		return Created{}, err
	}
	m := Message{ID: id, BotID: in.BotID, BotSeq: seq + 1, Sender: sender, Revision: 1, Format: in.Format, Text: in.Text, ButtonsJSON: []byte("[]"), CreatedAt: now, UpdatedAt: now}
	stamp := now.Format(time.RFC3339Nano)
	if err = requireOne(tx.ExecContext(ctx, `INSERT INTO messages(id,bot_id,bot_seq,sender,revision,format,text,buttons_json,created_at,updated_at) VALUES(?,?,?,?,1,?,?,'[]',?,?)`, id, in.BotID, m.BotSeq, sender, m.Format, m.Text, stamp, stamp)); err != nil {
		return Created{}, err
	}
	createdSeq, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.MessageUpsert, EntityID: id, Revision: 1, BotID: &m.BotID, CreatedAt: now})
	if err != nil {
		return Created{}, err
	}
	if err = requireOne(tx.ExecContext(ctx, "UPDATE messages SET created_change_seq=? WHERE id=? AND created_change_seq IS NULL", createdSeq, id)); err != nil {
		return Created{}, err
	}
	m.CreatedChangeSeq = &createdSeq
	out := Created{Message: m, LastChangeSeq: createdSeq}
	if sender == Administrator {
		out.Delivery, err = r.enqueueDelivery(ctx, tx, m, &out.LastChangeSeq)
		if err != nil {
			return Created{}, err
		}
	}
	out.Pushes, err = r.enqueuePushes(ctx, tx, in.PushTargets, now, out.LastChangeSeq)
	if err != nil {
		return Created{}, err
	}
	return out, nil
}

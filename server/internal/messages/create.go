package messages

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/idempotency"
)

// ErrTooLarge 供传输层映射 413，不能截断正文后继续创建。
var ErrTooLarge = errors.New("messages: text exceeds byte limit")

const (
	CreateBotMessage     = "createBotMessage"
	CreateWebhookMessage = "createWebhookMessage"
	CreateClientMessage  = "createClientMessage"
)

type AttachmentReference struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}
type Button struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	URL          *string `json:"url,omitempty"`
	CallbackData *string `json:"callback_data,omitempty"`
}

// CreateRequest 对应冻结创建载荷，传输层仍须拒绝未知字段及非法 null。
// 有限文本路径显式拒绝其余非空字段，不静默丢弃未来业务输入。
type CreateRequest struct {
	Format           string                `json:"format"`
	Text             string                `json:"text"`
	Attachments      []AttachmentReference `json:"attachments"`
	ReplyToMessageID *string               `json:"reply_to_message_id"`
	Buttons          [][]Button            `json:"buttons"`
}

func createDigest(botID string, in CreateRequest) ([32]byte, error) {
	// 省略、null 与空数组归一；正文和数组顺序逐字保留，不 trim Markdown。
	if in.Attachments == nil {
		in.Attachments = []AttachmentReference{}
	}
	if in.Buttons == nil {
		in.Buttons = [][]Button{}
	}
	data, err := json.Marshal(struct {
		BotID   string        `json:"bot_id"`
		Request CreateRequest `json:"request"`
	}{botID, in})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}
func validateCreate(botID, key string, in CreateRequest) error {
	if botID == "" || !idempotency.ValidKey(key) {
		return ErrInvalid
	}
	if len(in.Text) > 64*1024 {
		return ErrTooLarge
	}
	if (in.Format != "plain" && in.Format != "markdown") || !utf8.ValidString(in.Text) {
		return ErrInvalid
	}
	return nil
}

// CreateIdempotentApplication 仅在成功提交后发布结果；失败清空所有候选。
func (r *Repository) CreateIdempotentApplication(ctx context.Context, identity bots.Identity, key string, in CreateRequest, targets []PushTarget) (Created, error) {
	var candidate Created
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		var err error
		candidate, err = r.CreateIdempotentApplicationTx(ctx, tx, identity, key, in, targets)
		return err
	})
	if err != nil {
		return Created{}, err
	}
	return candidate, nil
}

// CreateIdempotentApplicationTx 与消息、changes、pending 共用事务。重放前也必须
// 复核当前凭据代次及能力，撤销 Token 不能换回结果。返回值仅是提交前候选。
func (r *Repository) CreateIdempotentApplicationTx(ctx context.Context, tx *sql.Tx, identity bots.Identity, key string, in CreateRequest, targets []PushTarget) (Created, error) {
	if tx == nil {
		return Created{}, ErrInvalid
	}
	if err := bots.AuthorizeTx(ctx, tx, identity, identity.BotID(), bots.Send); err != nil {
		return Created{}, err
	}
	op := CreateBotMessage
	if identity.Kind() == bots.WebhookKey {
		op = CreateWebhookMessage
	}
	scope := idempotency.Scope{Identity: "bot:" + identity.BotID(), Operation: op, Key: key}
	return r.createIdempotentTx(ctx, tx, scope, identity.BotID(), in, func(input TextInput) (Created, error) { return r.CreateApplicationTx(ctx, tx, identity, input) }, targets)
}

// CreateIdempotentAdministratorTx 仅供 SessionStore.WithAuthorizedWrite 所拥有的
// 同一事务调用，不认证管理员，不提供绕过会话及 G1 的公开管理员包装器。
func (r *Repository) CreateIdempotentAdministratorTx(ctx context.Context, tx *sql.Tx, botID, key string, in CreateRequest, targets []PushTarget) (Created, error) {
	if tx == nil {
		return Created{}, ErrInvalid
	}
	var enabled, receive bool
	if err := tx.QueryRowContext(ctx, `SELECT enabled,can_receive FROM bots WHERE id=?`, botID).Scan(&enabled, &receive); err != nil {
		return Created{}, err
	}
	if !enabled || !receive {
		return Created{}, bots.ErrForbidden
	}
	scope := idempotency.Scope{Identity: "administrator", Operation: CreateClientMessage, Key: key}
	return r.createIdempotentTx(ctx, tx, scope, botID, in, func(input TextInput) (Created, error) { return r.CreateAdministratorTx(ctx, tx, input) }, targets)
}

func (r *Repository) createIdempotentTx(ctx context.Context, tx *sql.Tx, scope idempotency.Scope, botID string, in CreateRequest, create func(TextInput) (Created, error), targets []PushTarget) (Created, error) {
	if err := validateCreate(botID, scope.Key, in); err != nil {
		return Created{}, err
	}
	digest, err := createDigest(botID, in)
	if err != nil {
		return Created{}, err
	}
	record, found, err := idempotency.LookupTx(ctx, tx, scope, digest)
	if err != nil {
		return Created{}, err
	}
	if found {
		var original Created
		if err = json.Unmarshal(record.Result, &original); err != nil {
			return Created{}, err
		}
		return original, nil
	}
	// 查重后才进入有限业务支持校验；同键不同有效字段必须报冲突。
	if len(in.Attachments) != 0 || len(in.Buttons) != 0 || in.ReplyToMessageID != nil {
		return Created{}, ErrUnsupported
	}
	out, err := create(TextInput{BotID: botID, Format: in.Format, Text: in.Text, PushTargets: targets})
	if err != nil {
		return Created{}, err
	}
	data, err := json.Marshal(out)
	if err != nil {
		return Created{}, err
	}
	record = idempotency.Record{Digest: digest, MessageID: out.Message.ID, Result: data}
	if out.Delivery != nil {
		record.DeliveryID = &out.Delivery.DeliveryID
	}
	if err = idempotency.InsertTx(ctx, tx, scope, record); err != nil {
		return Created{}, err
	}
	return out, nil
}

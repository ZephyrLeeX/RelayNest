// Package changes 提供全局变更日志的调用方事务基础，不组装同步实体投影。
package changes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var (
	ErrInvalidSequence    = errors.New("changes: invalid sequence")
	ErrInvalidMetadata    = errors.New("changes: invalid metadata")
	ErrInvalidTransaction = errors.New("changes: nil transaction")
	ErrInvalidEpoch       = errors.New("changes: invalid persisted epoch")
)

// Sequence 只表示全局 change_seq；零用于空日志游标，追加结果始终为正。
// 不可用于实体版本、bot_seq 或 read_seq；转换负数仍会在 JSON 边界失败。
type Sequence int64

func ParseSequence(s string) (Sequence, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != s {
		return 0, ErrInvalidSequence
	}
	return Sequence(n), nil
}

func (s Sequence) MarshalJSON() ([]byte, error) {
	if s < 0 {
		return nil, ErrInvalidSequence
	}
	return json.Marshal(strconv.FormatInt(int64(s), 10))
}

func (s *Sequence) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return ErrInvalidSequence
	}
	n, err := ParseSequence(*value)
	if err != nil {
		return err
	}
	*s = n
	return nil
}

// Type 为冻结的同步事件字面值；类型本身仅序列化为字符串。
type Type string

const (
	MessageUpsert              Type = "message.upsert"
	MessageDeleted             Type = "message.deleted"
	BotUpsert                  Type = "bot.upsert"
	BotDeleted                 Type = "bot.deleted"
	ReadUpdated                Type = "read.updated"
	DeliveryUpsert             Type = "delivery.upsert"
	DeviceNotificationsUpdated Type = "device.notifications.updated"
	SettingsUpdated            Type = "settings.updated"
)

// ProjectionKeys 仅描述 type/change_seq 外的冻结投影键，不提供投影值。
// 每次返回独立切片；未知类型返回 nil。settings.updated 的 recovery_path 固定为 /admin/settings。
func (t Type) ProjectionKeys() []string {
	switch t {
	case MessageUpsert:
		return []string{"message"}
	case MessageDeleted:
		return []string{"id", "bot_id", "bot_seq", "revision", "deleted_at"}
	case BotUpsert:
		return []string{"bot"}
	case BotDeleted:
		return []string{"id", "version", "deleted_at"}
	case ReadUpdated:
		return []string{"read"}
	case DeliveryUpsert:
		return []string{"delivery"}
	case DeviceNotificationsUpdated:
		return []string{"settings"}
	case SettingsUpdated:
		return []string{"settings_version", "recovery_path"}
	default:
		return nil
	}
}

// Metadata 仅含日志字段，不能作为完整 SyncChange；实体版本和 ID 由业务调用方提供。
// BotID=nil 写 SQL NULL；非 nil 必须非空。CreatedAt 由调用方提供并统一存为 UTC RFC3339 Z。
type Metadata struct {
	Type      Type
	EntityID  string
	Revision  int64
	BotID     *string
	CreatedAt time.Time
}

// Append 只在调用方事务插入元数据。返回序号不代表已提交；调用方必须传播错误，
// 仅在 Store.Write 成功后对外返回或广播，禁止忽略追加错误后提交业务写入。
func Append(ctx context.Context, tx *sql.Tx, m Metadata) (Sequence, error) {
	if tx == nil {
		return 0, ErrInvalidTransaction
	}
	if m.Type.ProjectionKeys() == nil || m.EntityID == "" || m.Revision < 1 ||
		(m.BotID != nil && *m.BotID == "") || m.CreatedAt.IsZero() ||
		m.CreatedAt.Year() < 1 || m.CreatedAt.Year() > 9999 ||
		m.CreatedAt.UTC().Year() < 1 || m.CreatedAt.UTC().Year() > 9999 {
		return 0, ErrInvalidMetadata
	}
	result, err := tx.ExecContext(ctx,
		"INSERT INTO changes(type, entity_id, revision, bot_id, created_at) VALUES (?, ?, ?, ?, ?)",
		string(m.Type), m.EntityID, m.Revision, m.BotID, m.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("changes append: %w", err)
	}
	n, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("changes sequence: %w", err)
	}
	if n <= 0 {
		return 0, ErrInvalidSequence
	}
	return Sequence(n), nil
}

var epochPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Epoch 只读既有 0001 初始化的 UUID v4；状态缺失/损坏时失败，不创建或修复。
func Epoch(ctx context.Context, tx *sql.Tx) (string, error) {
	if tx == nil {
		return "", ErrInvalidTransaction
	}
	var epoch string
	if err := tx.QueryRowContext(ctx, "SELECT sync_epoch FROM system_state WHERE id = 1").Scan(&epoch); err != nil {
		return "", fmt.Errorf("changes epoch: %w", err)
	}
	if !epochPattern.MatchString(epoch) {
		return "", ErrInvalidEpoch
	}
	return epoch, nil
}

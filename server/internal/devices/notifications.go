package devices

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"regexp"
	"time"
	"unicode/utf8"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/changes"
)

// NotificationConfiguration 是完整替换的设备偏好；所有时间/版本由服务端控制。
type NotificationConfiguration struct {
	Enabled        bool                      `json:"enabled"`
	BotOverrides   []NotificationBotOverride `json:"bot_overrides"`
	DND            NotificationDND           `json:"dnd"`
	Preview        bool                      `json:"preview"`
	NotifyChannel  string                    `json:"notify_channel"`
	NotifyProvider *PushProvider             `json:"notify_provider"`
}
type NotificationBotOverride struct {
	BotID   string `json:"bot_id"`
	Enabled bool   `json:"enabled"`
}
type NotificationDND struct {
	Enabled  bool   `json:"enabled"`
	Timezone string `json:"timezone"`
	Start    string `json:"start"`
	End      string `json:"end"`
}
type NotificationSettingsUpdate struct {
	ExpectedVersion int64 `json:"expected_version"`
	NotificationConfiguration
}
type NotificationSettings struct {
	DeviceID        string     `json:"device_id"`
	SettingsVersion int64      `json:"settings_version"`
	UpdatedAt       *time.Time `json:"updated_at"`
	NotificationConfiguration
}
type NotificationSettingsState struct {
	Settings   NotificationSettings `json:"settings"`
	ServerTime time.Time            `json:"server_time"`
}

func defaultNotifications(device string) NotificationSettings {
	provider := PushFCM
	return NotificationSettings{DeviceID: device, NotificationConfiguration: NotificationConfiguration{
		Enabled: true, BotOverrides: []NotificationBotOverride{}, DND: NotificationDND{Timezone: "UTC", Start: "22:00", End: "07:00"}, NotifyChannel: "fcm", NotifyProvider: &provider,
	}}
}
func readNotifications(ctx context.Context, tx *sql.Tx, device string) (NotificationSettings, error) {
	result := defaultNotifications(device)
	var stamp, configuration string
	err := tx.QueryRowContext(ctx, `SELECT settings_version,updated_at,configuration FROM device_notification_settings WHERE device_id=?`, device).Scan(&result.SettingsVersion, &stamp, &configuration)
	if err == sql.ErrNoRows {
		return result, nil
	}
	if err != nil {
		return NotificationSettings{}, auth.ErrStorage
	}
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return NotificationSettings{}, auth.ErrStorage
	}
	if err = json.Unmarshal([]byte(configuration), &result.NotificationConfiguration); err != nil {
		return NotificationSettings{}, auth.ErrStorage
	}
	result.UpdatedAt = &t
	return result, nil
}
func (r *PushRepository) GetCurrentNotifications(ctx context.Context, actor auth.AdminRequest) (NotificationSettingsState, error) {
	var result NotificationSettingsState
	err := r.authorized(ctx, actor, true, func(tx *sql.Tx, s auth.Session) error {
		var err error
		result.Settings, err = readNotifications(ctx, tx, s.DeviceID)
		result.ServerTime = r.now().UTC()
		return err
	})
	if err != nil {
		return NotificationSettingsState{}, err
	}
	return result, nil
}

var notificationClock = regexp.MustCompile(`^(?:[01][0-9]|2[0-3]):[0-5][0-9]$`)

func validateNotifications(in NotificationSettingsUpdate) error {
	if in.ExpectedVersion < 0 || in.BotOverrides == nil || len(in.BotOverrides) > 1000 {
		return ErrSettingsInput
	}
	d := in.DND
	if !utf8.ValidString(d.Timezone) || utf8.RuneCountInString(d.Timezone) < 1 || utf8.RuneCountInString(d.Timezone) > 128 || d.Timezone == "Local" || !notificationClock.MatchString(d.Start) || !notificationClock.MatchString(d.End) || d.Start == d.End {
		return ErrSettingsInput
	}
	if _, err := time.LoadLocation(d.Timezone); err != nil {
		return ErrSettingsInput
	}
	switch in.NotifyChannel {
	case "fcm":
		if in.NotifyProvider == nil || *in.NotifyProvider != PushFCM {
			return ErrSettingsInput
		}
	case "vendor":
		if in.NotifyProvider == nil || *in.NotifyProvider != PushOPush {
			return ErrSettingsInput
		}
	case "none":
		if in.NotifyProvider != nil {
			return ErrSettingsInput
		}
	default:
		return ErrSettingsInput
	}
	seen := make(map[string]bool, len(in.BotOverrides))
	for _, bot := range in.BotOverrides {
		if !sessionID.MatchString(bot.BotID) || seen[bot.BotID] {
			return ErrSettingsInput
		}
		seen[bot.BotID] = true
	}
	return nil
}
func (r *PushRepository) PutCurrentNotifications(ctx context.Context, actor auth.AdminRequest, in NotificationSettingsUpdate) (NotificationSettingsState, error) {
	var result NotificationSettingsState
	err := r.authorized(ctx, actor, false, func(tx *sql.Tx, s auth.Session) error {
		if err := validateNotifications(in); err != nil {
			return err
		}
		current, err := readNotifications(ctx, tx, s.DeviceID)
		if err != nil {
			return err
		}
		if current.SettingsVersion != in.ExpectedVersion {
			return settingsVersionError(ErrSettingsConflict, current.SettingsVersion)
		}
		if current.SettingsVersion == math.MaxInt64 {
			return settingsVersionError(ErrSettingsExhausted, current.SettingsVersion)
		}
		// 当前产品只有一个管理员；bots 中存在的实体即该身份有权且未删除的 Bot。
		for _, bot := range in.BotOverrides {
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM bots WHERE id=?)`, bot.BotID).Scan(&exists); err != nil {
				return auth.ErrStorage
			}
			if !exists {
				return ErrNotificationBotNotFound
			}
		}
		if in.NotifyChannel == "vendor" {
			registration, err := readRegistration(ctx, tx, s.ID, PushOPush)
			if err != nil {
				return err
			}
			if !registration.token.Valid || r.vendor == nil {
				return ErrProviderNotReady
			}
			ready, err := r.vendor.Ready(ctx, tx)
			if err != nil {
				return auth.ErrStorage
			}
			if !ready {
				return ErrProviderNotReady
			}
		}
		// 先序列化配置以获取独立快照，避免将调用方可变 slice/pointer 发布为持久结果。
		encoded, err := json.Marshal(in.NotificationConfiguration)
		if err != nil {
			return ErrSettingsInput
		}
		version := current.SettingsVersion + 1
		now := r.now().UTC()
		_, err = tx.ExecContext(ctx, `INSERT INTO device_notification_settings(device_id,settings_version,updated_at,configuration) VALUES(?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET settings_version=excluded.settings_version,updated_at=excluded.updated_at,configuration=excluded.configuration`, s.DeviceID, version, now.Format(time.RFC3339Nano), string(encoded))
		if err != nil {
			return auth.ErrStorage
		}
		// typed change 与偏好、服务端时间、会话复核/续期共用同一提交边界。
		if _, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.DeviceNotificationsUpdated, EntityID: s.DeviceID, Revision: version, CreatedAt: now}); err != nil {
			return auth.ErrStorage
		}
		result.Settings, err = readNotifications(ctx, tx, s.DeviceID)
		result.ServerTime = now
		return err
	})
	if err != nil {
		return NotificationSettingsState{}, err
	}
	return result, nil
}

package devices

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
)

func notificationInput() NotificationSettingsUpdate {
	return NotificationSettingsUpdate{NotificationConfiguration: defaultNotifications(pushDevice).NotificationConfiguration}
}

type readyVendor bool

func (r readyVendor) Ready(context.Context, *sql.Tx) (bool, error) { return bool(r), nil }
func TestNotificationsSharedDefaultsCASAndChanges(t *testing.T) {
	f := newPushFixture(t)
	a, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[0])
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[1])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Settings, defaultNotifications(pushDevice)) || !reflect.DeepEqual(a.Settings, b.Settings) || a.ServerTime.IsZero() {
		t.Fatal("virtual defaults")
	}
	if pushCount(t, f, "SELECT count(*) FROM device_notification_settings") != 0 || pushCount(t, f, "SELECT count(*) FROM changes") != 0 {
		t.Fatal("default GET wrote state")
	}
	in := notificationInput()
	in.Enabled = false
	in.DND = NotificationDND{true, "America/New_York", "23:00", "06:00"}
	first, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[1])
	if err != nil || !reflect.DeepEqual(first.Settings, shared.Settings) {
		t.Fatal("not shared", err)
	}
	if first.Settings.SettingsVersion != 1 || first.Settings.UpdatedAt == nil || !first.ServerTime.Equal(*first.Settings.UpdatedAt) {
		t.Fatal("version/time")
	}
	_, err = f.repo.PutCurrentNotifications(f.ctx, f.actors[1], in)
	assertPushError(t, err, ErrSettingsConflict, 1, "/devices/current/notifications")
	in.ExpectedVersion = 1
	second, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[1], in)
	if err != nil || second.Settings.SettingsVersion != 2 {
		t.Fatal("same value must increment", err)
	}
	if !second.Settings.UpdatedAt.After(*first.Settings.UpdatedAt) {
		t.Fatal("server timestamp not updated")
	}
	if pushCount(t, f, "SELECT count(*) FROM changes") != 2 {
		t.Fatal("missing changes")
	}
	if err := f.store.Read(f.ctx, func(tx *sql.Tx) error {
		var typ, id string
		var revision int64
		var bot sql.NullString
		err := tx.QueryRow(`SELECT type,entity_id,revision,bot_id FROM changes ORDER BY change_seq DESC LIMIT 1`).Scan(&typ, &id, &revision, &bot)
		if typ != string(changes.DeviceNotificationsUpdated) || id != pushDevice || revision != 2 || bot.Valid {
			t.Fatal("wrong typed change")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// 另一个真实设备的默认值保持独立。
	_, token, err := f.sessions.Create(f.ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: "00000000-0000-4000-8000-000000000099", DeviceName: "another"})
	if err != nil {
		t.Fatal(err)
	}
	f.tokens = append(f.tokens, token)
	f.authorize(t)
	other, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[2])
	if err != nil || other.Settings.SettingsVersion != 0 || !other.Settings.Enabled {
		t.Fatal("other device", err)
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	f.store, err = storage.Open(f.ctx, storage.Config{Path: f.path})
	if err != nil {
		t.Fatal(err)
	}
	f.sessions = auth.NewSessionStore(f.store)
	f.repo = NewPushRepository(f.store, f.sessions, nil)
	f.authorize(t)
	reopened, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[1])
	if err != nil || !reflect.DeepEqual(second.Settings, reopened.Settings) {
		t.Fatal("reopen preferences", err)
	}
}
func TestNotificationsConcurrentSessionCAS(t *testing.T) {
	f := newPushFixture(t)
	start := make(chan struct{})
	result := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			in := notificationInput()
			in.Preview = i == 1
			_, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[i], in)
			result <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(result)
	success, conflict := 0, 0
	for err := range result {
		if err == nil {
			success++
		} else if errors.Is(err, ErrSettingsConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 || pushCount(t, f, "SELECT count(*) FROM changes") != 1 {
		t.Fatal("shared CAS")
	}
}
func TestNotificationsValidationAndVendorReadiness(t *testing.T) {
	f := newPushFixture(t)
	cases := []func(*NotificationSettingsUpdate){
		func(in *NotificationSettingsUpdate) { in.ExpectedVersion = -1 },
		func(in *NotificationSettingsUpdate) { in.BotOverrides = nil },
		func(in *NotificationSettingsUpdate) { in.BotOverrides = make([]NotificationBotOverride, 1001) },
		func(in *NotificationSettingsUpdate) { in.DND.Timezone = "Mars/Test" },
		func(in *NotificationSettingsUpdate) { in.DND.Timezone = "Local" },
		func(in *NotificationSettingsUpdate) { in.DND.Timezone = strings.Repeat("x", 129) },
		func(in *NotificationSettingsUpdate) { in.DND.Start = "24:00" },
		func(in *NotificationSettingsUpdate) { in.DND.End = in.DND.Start },
		func(in *NotificationSettingsUpdate) { in.NotifyProvider = nil },
		func(in *NotificationSettingsUpdate) { in.NotifyChannel = "vendor" },
		func(in *NotificationSettingsUpdate) { in.NotifyChannel = "none" },
		func(in *NotificationSettingsUpdate) { in.BotOverrides = []NotificationBotOverride{{"not-uuid", true}} },
		func(in *NotificationSettingsUpdate) {
			in.BotOverrides = []NotificationBotOverride{{pushDevice, true}, {pushDevice, false}}
		},
	}
	for i, change := range cases {
		in := notificationInput()
		change(&in)
		if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in); !errors.Is(err, ErrSettingsInput) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	in := notificationInput()
	in.BotOverrides = []NotificationBotOverride{{pushDevice, true}}
	if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in); !errors.Is(err, ErrNotificationBotNotFound) {
		t.Fatal("missing Bot", err)
	}
	mustPushSQL(t, f, `INSERT INTO bots(id,name,enabled,can_send,can_receive,version) VALUES(?,'bot',1,1,1,1)`, pushDevice)
	// FCM 偏好允许没有任何注册，不等待 OPPO 平台资格。
	first, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in)
	if err != nil || first.Settings.SettingsVersion != 1 {
		t.Fatal("fcm without token", err)
	}
	vendor := PushOPush
	in.ExpectedVersion = 1
	in.NotifyChannel = "vendor"
	in.NotifyProvider = &vendor
	f.repo.vendor = readyVendor(true)
	if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in); !errors.Is(err, ErrProviderNotReady) {
		t.Fatal("vendor without token", err)
	}
	putPush(t, f, 1, PushOPush, 1, 0, "other-session-opush")
	if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in); !errors.Is(err, ErrProviderNotReady) {
		t.Fatal("other session qualified", err)
	}
	putPush(t, f, 0, PushOPush, 1, 0, "current-opush")
	for _, readiness := range []VendorReadiness{nil, readyVendor(false)} {
		f.repo.vendor = readiness
		if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in); !errors.Is(err, ErrProviderNotReady) {
			t.Fatal("unverified vendor", err)
		}
	}
	f.repo.vendor = readyVendor(true)
	saved, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in)
	if err != nil || saved.Settings.NotifyChannel != "vendor" {
		t.Fatal("ready vendor", err)
	}
	// 注册更新不改变提醒模式/配置代次。
	putPush(t, f, 0, PushFCM, 1, 0, "fcm")
	current, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[0])
	if err != nil || current.Settings.SettingsVersion != 2 || current.Settings.NotifyChannel != "vendor" {
		t.Fatal("registration changed preferences", err)
	}
	in.ExpectedVersion = 2
	in.NotifyChannel = "none"
	in.NotifyProvider = nil
	if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in); err != nil {
		t.Fatal("none mode", err)
	}
}
func TestNotificationsExhaustionAndTransactionRollback(t *testing.T) {
	for _, failure := range []string{"statement", "changes", "commit", "auth-renew"} {
		t.Run(failure, func(t *testing.T) {
			f := newPushFixture(t)
			// 缩短续期，失败后必须连会话续期也回滚。
			deadline := time.Now().Add(time.Hour).UnixNano()
			mustPushSQL(t, f, "UPDATE sessions SET idle_expires_at=?", deadline)
			switch failure {
			case "statement":
				mustPushSQL(t, f, `CREATE TRIGGER settings_fault BEFORE INSERT ON device_notification_settings BEGIN SELECT RAISE(ABORT,'private-diagnostic'); END`)
			case "changes":
				mustPushSQL(t, f, `CREATE TRIGGER changes_fault BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'private-diagnostic'); END`)
			case "commit":
				mustPushSQL(t, f, `CREATE TABLE deferred_settings_fault(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED)`)
				mustPushSQL(t, f, `CREATE TRIGGER commit_fault AFTER INSERT ON device_notification_settings BEGIN INSERT INTO deferred_settings_fault VALUES('missing-session'); END`)
			case "auth-renew":
				mustPushSQL(t, f, `CREATE TRIGGER renew_fault AFTER UPDATE OF idle_expires_at ON sessions BEGIN UPDATE administrator SET auth_generation=auth_generation+1 WHERE id=1; END`)
			}
			result, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], notificationInput())
			if err == nil || result.Settings.DeviceID != "" || strings.Contains(err.Error(), "private-diagnostic") {
				t.Fatal("failed write published", err)
			}
			if failure == "auth-renew" {
				if !errors.Is(err, auth.ErrInvalidSession) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, auth.ErrStorage) {
				t.Fatal(err)
			}
			if pushCount(t, f, "SELECT count(*) FROM device_notification_settings") != 0 || pushCount(t, f, "SELECT count(*) FROM changes") != 0 || pushCount(t, f, "SELECT idle_expires_at FROM sessions WHERE id=?", f.actors[0].Session.ID) != deadline {
				t.Fatal("partial commit")
			}
			if failure == "commit" && pushCount(t, f, "SELECT count(*) FROM deferred_settings_fault") != 0 {
				t.Fatal("commit failure left deferred row")
			}
		})
	}
	f := newPushFixture(t)
	_, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], notificationInput())
	if err != nil {
		t.Fatal(err)
	}
	mustPushSQL(t, f, `UPDATE device_notification_settings SET settings_version=?`, int64(math.MaxInt64))
	in := notificationInput()
	in.ExpectedVersion = math.MaxInt64
	_, err = f.repo.PutCurrentNotifications(f.ctx, f.actors[0], in)
	assertPushError(t, err, ErrSettingsExhausted, math.MaxInt64, "/devices/current/notifications")
	if pushCount(t, f, "SELECT count(*) FROM changes") != 1 {
		t.Fatal("exhausted append")
	}
	current, err := f.repo.GetCurrentNotifications(f.ctx, f.actors[0])
	if err != nil || current.Settings.SettingsVersion != math.MaxInt64 {
		t.Fatal("overflow", err)
	}
}

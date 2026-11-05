package devices

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestNotificationSettingsTrustedTxDefaultsAndFailures(t *testing.T) {
	f := newPushFixture(t)
	if _, e := ReadNotificationSettingsTx(f.ctx, nil, pushDevice); !errors.Is(e, ErrSettingsInput) {
		t.Fatal(e)
	}
	if e := f.store.Read(f.ctx, func(tx *sql.Tx) error {
		got, e := ReadNotificationSettingsTx(f.ctx, tx, pushDevice)
		if e != nil || !reflect.DeepEqual(got, defaultNotifications(pushDevice)) {
			t.Fatal(got, e)
		}
		for _, device := range []string{""} {
			if _, e := ReadNotificationSettingsTx(f.ctx, tx, device); !errors.Is(e, ErrSettingsInput) {
				t.Fatal(e)
			}
		}
		ctx, cancel := context.WithCancel(f.ctx)
		cancel()
		if _, e := ReadNotificationSettingsTx(ctx, tx, pushDevice); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if pushCount(t, f, "SELECT count(*) FROM device_notification_settings") != 0 {
		t.Fatal("default wrote settings")
	}
	got, e := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], notificationInput())
	if e != nil {
		t.Fatal(e)
	}
	if e := f.store.Read(f.ctx, func(tx *sql.Tx) error {
		current, e := ReadNotificationSettingsTx(f.ctx, tx, pushDevice)
		if e != nil || !reflect.DeepEqual(current, got.Settings) {
			t.Fatal(current, e)
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
}

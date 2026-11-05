package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"relaynest/server/internal/devices"
	sharedsync "relaynest/server/internal/sync"
	"testing"
	"time"
)

func alertFixture(t *testing.T) (*jobFixture, JobSnapshot, devices.NotificationConfiguration) {
	t.Helper()
	f := jobSetup(t)
	j := f.snapshot(t, f.session(t, jobDeviceA), "alert")
	jobCreate(t, f, j)
	p := devices.PushFCM
	return f, j, devices.NotificationConfiguration{Enabled: true, BotOverrides: []devices.NotificationBotOverride{}, DND: devices.NotificationDND{Timezone: "UTC", Start: "22:00", End: "07:00"}, NotifyChannel: "fcm", NotifyProvider: &p}
}
func alertSettings(t *testing.T, f *jobFixture, j JobSnapshot, c devices.NotificationConfiguration) {
	t.Helper()
	raw, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	jobSQL(t, f.store, `INSERT INTO device_notification_settings VALUES(?,1,?,?) ON CONFLICT(device_id) DO UPDATE SET configuration=excluded.configuration`, j.DeviceID, f.now.Format(time.RFC3339Nano), string(raw))
}
func alertExpect(t *testing.T, f *jobFixture, j JobSnapshot, now time.Time, want, wantError bool) error {
	t.Helper()
	before := leaseDigest(t, f.store)
	var got bool
	err := f.other.Read(jobContext, func(tx *sql.Tx) error { var e error; got, e = CheckAlertPolicyTx(jobContext, tx, j, now); return e })
	if got != want || (err != nil) != wantError {
		t.Fatalf("got %v err %v want %v error %v", got, err, want, wantError)
	}
	if before != leaseDigest(t, f.store) {
		t.Fatal("policy wrote database")
	}
	return err
}

// 故障注入先证明生产约束会拒绝，再仅在同一专用写事务关闭 CHECK，
// 模拟磁盘/旧库损坏；恢复约束后才调用被测读取，确保实际命中防御分支。
func alertCorrupt(t *testing.T, f *jobFixture, q string, args ...any) {
	t.Helper()
	if e := f.store.Write(jobContext, func(tx *sql.Tx) error { _, e := tx.Exec(q, args...); return e }); e == nil {
		t.Fatal("schema failed to reject corrupt fixture")
	} else {
		var sqlite interface{ Code() int }
		if !errors.As(e, &sqlite) || sqlite.Code() != 275 {
			t.Fatal("expected CHECK constraint", e)
		}
	}

	if e := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`PRAGMA ignore_check_constraints=ON`); e != nil {
			return e
		}
		_, err := tx.Exec(q, args...)
		_, reset := tx.Exec(`PRAGMA ignore_check_constraints=OFF`)
		if err != nil {
			return err
		}
		if reset != nil {
			return reset
		}
		var ignored int
		if e := tx.QueryRow(`PRAGMA ignore_check_constraints`).Scan(&ignored); e != nil {
			return e
		}
		if ignored != 0 {
			return fmt.Errorf("CHECK constraints were not restored")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}

func TestAlertPolicySnapshotAndSettings(t *testing.T) {
	for _, name := range []string{"default", "false", "sync", "version", "disabled", "bot-disabled", "other-bot", "none", "provider", "opush", "registration"} {
		t.Run(name, func(t *testing.T) {
			f, j, c := alertFixture(t)
			want := true
			switch name {
			case "false":
				j.AlertEligible = false
				want = false
			case "sync":
				j.Purpose = "sync"
				j.AlertEligible = false
				want = false
			case "version":
				j.SettingsVersion = 1
				want = false
			case "disabled":
				c.Enabled = false
			case "bot-disabled":
				c.BotOverrides = []devices.NotificationBotOverride{{BotID: j.BotID, Enabled: false}}
			case "other-bot":
				c.BotOverrides = []devices.NotificationBotOverride{{BotID: jobDeviceB, Enabled: false}}
			case "none":
				c.NotifyChannel = "none"
				c.NotifyProvider = nil
			case "provider":
				p := devices.PushOPush
				c.NotifyChannel = "vendor"
				c.NotifyProvider = &p
			case "opush":
				p := devices.PushOPush
				c.NotifyChannel = "vendor"
				c.NotifyProvider = &p
				j.Provider = "opush"
			case "registration":
				jobSQL(t, f.store, `UPDATE device_push_registrations SET registration_version=2`)
			}
			if name == "disabled" || name == "bot-disabled" || name == "other-bot" || name == "none" || name == "provider" || name == "opush" {
				alertSettings(t, f, j, c)
				j.SettingsVersion = 1
				want = name == "other-bot" || name == "opush"
			}
			alertExpect(t, f, j, f.now, want, false)
		})
	}
}
func TestAlertPolicyDND(t *testing.T) {
	cases := []struct {
		name, zone, start, end, at string
		want                       bool
	}{
		{"same-before", "UTC", "10:00", "12:00", "2026-01-01T09:59:59Z", true}, {"same-start", "UTC", "10:00", "12:00", "2026-01-01T10:00:00Z", false}, {"same-end", "UTC", "10:00", "12:00", "2026-01-01T12:00:00Z", true},
		{"night-start", "UTC", "22:00", "07:00", "2026-01-01T22:00:00Z", false}, {"night-midnight", "UTC", "22:00", "07:00", "2026-01-02T00:00:00Z", false}, {"night-end", "UTC", "22:00", "07:00", "2026-01-02T07:00:00Z", true},
		{"device-zone", "Asia/Shanghai", "10:00", "12:00", "2026-01-01T02:00:00Z", false}, {"dst-first", "America/New_York", "01:15", "01:45", "2026-11-01T05:30:00Z", false}, {"dst-second", "America/New_York", "01:15", "01:45", "2026-11-01T06:30:00Z", false}, {"dst-end", "America/New_York", "01:15", "01:45", "2026-11-01T06:45:00Z", true}, {"dst-gap", "America/New_York", "02:00", "03:00", "2026-03-08T07:00:00Z", true}, {"low-year", "UTC", "10:00", "12:00", "1970-01-01T00:00:00.000000001Z", true}, {"high-year", "UTC", "10:00", "12:00", "2262-04-11T23:47:16Z", true}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, j, c := alertFixture(t)
			c.DND = devices.NotificationDND{Enabled: true, Timezone: tc.zone, Start: tc.start, End: tc.end}
			alertSettings(t, f, j, c)
			j.SettingsVersion = 1
			now, e := time.Parse(time.RFC3339Nano, tc.at)
			if e != nil {
				t.Fatal(e)
			}
			alertExpect(t, f, j, now, tc.want, false)
		})
	}
}
func TestAlertPolicySourceAndRead(t *testing.T) {
	for _, name := range []string{"deleted", "wrong-bot", "wrong-seq", "equal", "above", "below", "missing", "bad-read", "blob-read", "bad-version", "bad-time", "bad-upper", "blob-source"} {
		t.Run(name, func(t *testing.T) {
			f, j, _ := alertFixture(t)
			want, wantErr := false, false
			switch name {
			case "deleted":
				jobSQL(t, f.store, `DELETE FROM messages WHERE id=?`, j.MessageID)
			case "wrong-bot":
				j.BotID = jobDeviceB
			case "wrong-seq":
				j.BotSeq++
			case "equal":
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq, j.BotID)
			case "above":
				_ = f.snapshot(t, f.session(t, jobDeviceB), "alert")
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq+1, j.BotID)
			case "below":
				j = f.snapshot(t, f.session(t, jobDeviceB), "alert")
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq-1, j.BotID)
				want = true
			case "missing":
				jobSQL(t, f.store, `DELETE FROM bot_read_states WHERE bot_id=?`, j.BotID)
				wantErr = true
			case "bad-read":
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=999 WHERE bot_id=?`, j.BotID)
				wantErr = true
			case "blob-read":
				alertCorrupt(t, f, `UPDATE bot_read_states SET read_seq=CAST('0' AS BLOB) WHERE bot_id=?`, j.BotID)
				wantErr = true
			case "bad-version":
				alertCorrupt(t, f, `UPDATE bot_read_states SET state_version=CAST('1' AS BLOB) WHERE bot_id=?`, j.BotID)
				wantErr = true
			case "bad-time":
				jobSQL(t, f.store, `UPDATE bot_read_states SET updated_at='bad' WHERE bot_id=?`, j.BotID)
				wantErr = true
			case "bad-upper":
				alertCorrupt(t, f, `UPDATE bots SET bot_seq=CAST('1' AS BLOB) WHERE id=?`, j.BotID)
				wantErr = true
			case "blob-source":
				alertCorrupt(t, f, `UPDATE messages SET bot_seq=CAST('1' AS BLOB) WHERE id=?`, j.MessageID)
				wantErr = true
			}
			e := alertExpect(t, f, j, f.now, want, wantErr)
			if name == "missing" && !errors.Is(e, sharedsync.ErrReadStateUnavailable) {
				t.Fatal(e)
			}
		})
	}
}
func TestAlertPolicyDamagedConfiguration(t *testing.T) {
	cases := map[string]string{
		"missing": `configuration=json_remove(configuration,'$.enabled')`, "null": `configuration=json_set(configuration,'$.enabled',NULL)`, "timezone": `configuration=json_set(configuration,'$.dnd.timezone','Invalid/Zone')`, "equal": `configuration=json_set(configuration,'$.dnd.end','22:00')`, "clock": `configuration=json_set(configuration,'$.dnd.start','3:00')`, "nested-missing": `configuration=json_remove(configuration,'$.dnd.enabled')`, "blob-config": `configuration=CAST(configuration AS BLOB)`, "blob-version": `settings_version=CAST('1' AS BLOB)`, "blob-time": `updated_at=CAST(updated_at AS BLOB)`, "zero-version": `settings_version=0`, "bad-time": `updated_at='bad'`, "zero-time": `updated_at='0001-01-01T00:00:00Z'`}
	for name, q := range cases {
		t.Run(name, func(t *testing.T) {
			f, j, c := alertFixture(t)
			alertSettings(t, f, j, c)
			j.SettingsVersion = 1
			if name == "blob-version" || name == "zero-version" {
				alertCorrupt(t, f, `UPDATE device_notification_settings SET `+q)
			} else {
				jobSQL(t, f.store, `UPDATE device_notification_settings SET `+q)
			}
			alertExpect(t, f, j, f.now, false, true)
		})
	}
}
func TestAlertPolicyTransactionFailuresAndSnapshot(t *testing.T) {
	f, j, c := alertFixture(t)
	if got, e := CheckAlertPolicyTx(jobContext, nil, j, f.now); got || !errors.Is(e, ErrJobInput) {
		t.Fatal(got, e)
	}
	err := f.other.Read(jobContext, func(tx *sql.Tx) error {
		ctx, cancel := context.WithCancel(jobContext)
		cancel()
		if got, e := CheckAlertPolicyTx(ctx, tx, j, f.now); got || !errors.Is(e, context.Canceled) {
			t.Fatal(got, e)
		}
		for _, bad := range []time.Time{{}, time.Date(2263, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(1969, 1, 1, 0, 0, 0, 0, time.UTC)} {
			if got, e := CheckAlertPolicyTx(jobContext, tx, j, bad); got || !errors.Is(e, ErrJobInput) {
				t.Fatal(got, e)
			}
		}
		got, e := CheckAlertPolicyTx(jobContext, tx, j, f.now)
		if !got || e != nil {
			t.Fatal(got, e)
		}
		c.Enabled = false
		alertSettings(t, f, j, c)
		got, e = CheckAlertPolicyTx(jobContext, tx, j, f.now)
		if !got || e != nil {
			t.Fatal("lost snapshot", got, e)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	alertExpect(t, f, j, f.now, false, false)
	var closed *sql.Tx
	if e := f.store.Read(jobContext, func(tx *sql.Tx) error { closed = tx; return nil }); e != nil {
		t.Fatal(e)
	}
	if got, e := CheckAlertPolicyTx(jobContext, closed, j, f.now); got || !errors.Is(e, sql.ErrTxDone) {
		t.Fatal(got, e)
	}
	f, j, _ = alertFixture(t)
	if e := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`DROP TABLE bot_read_states`); e != nil {
			return e
		}
		got, e := CheckAlertPolicyTx(jobContext, tx, j, f.now)
		if got || e == nil {
			t.Fatal(got, e)
		}
		var sqlite interface{ Code() int }
		if !errors.As(e, &sqlite) {
			t.Fatal("lost SQL type", e)
		}
		return fmt.Errorf("rollback fixture")
	}); e == nil {
		t.Fatal("expected rollback")
	}
}
func TestAlertPolicySyncNegativeControl(t *testing.T) {
	f, held := leaseFixture(t)
	j := held.Job.Snapshot
	p := devices.PushFCM
	c := devices.NotificationConfiguration{Enabled: true, BotOverrides: []devices.NotificationBotOverride{}, DND: devices.NotificationDND{Enabled: true, Timezone: "UTC", Start: f.now.Format("15:04"), End: f.now.Add(time.Hour).Format("15:04")}, NotifyChannel: "fcm", NotifyProvider: &p}
	alertSettings(t, f, j, c)
	jobSQL(t, f.store, `DELETE FROM push_jobs`)
	j = f.snapshot(t, f.session(t, jobDeviceB), "sync")
	j.DeviceID = jobDeviceA
	j.SessionID = held.Job.Snapshot.SessionID
	j.SettingsVersion = 1
	jobCreate(t, f, j)
	alert := j
	alert.Purpose = "alert"
	alert.AlertEligible = true
	alertExpect(t, f, alert, f.now, false, false)
	reservations, e := NewJobRepository(f.store).ReserveSync(jobContext, f.now, time.Minute, 1)
	if e != nil || len(reservations) != 1 {
		t.Fatal(e, len(reservations))
	}
	jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq, j.BotID)
	jobSQL(t, f.store, `DELETE FROM messages WHERE id=?`, j.MessageID)
	leaseExpect(t, f.store, reservations[0].Lease, f.now, true)
	alertExpect(t, f, j, f.now, false, false)
}

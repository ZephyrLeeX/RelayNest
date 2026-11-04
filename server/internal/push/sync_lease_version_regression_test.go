package push

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

// 可信坏库中，数值等于原快照也不能将 BLOB 版本当作有效 INTEGER。
func TestSyncLeaseCheckVersionStorageClassesRegression(t *testing.T) {
	for _, field := range []string{"registration", "settings"} {
		for _, representation := range []string{"plain", "leading-zero", "plus"} {
			t.Run(field+"/"+representation, func(t *testing.T) {
				f := jobSetup(t)
				s := f.session(t, jobDeviceA)
				version, encoded := int64(1), "1"
				if field == "settings" {
					version, encoded = 7, "7"
					jobSQL(t, f.store, `INSERT INTO device_notification_settings VALUES(?,7,?,'{}')`, s.DeviceID, f.now.Format(time.RFC3339Nano))
				}
				j := f.snapshot(t, s, "sync")
				if field == "settings" {
					j.SettingsVersion = version
				}
				jobCreate(t, f, j)
				held, err := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
				if err != nil || len(held) != 1 {
					t.Fatal("reserve", err)
				}
				leaseExpect(t, f.other, held[0].Lease, f.now, true)
				if representation == "leading-zero" {
					encoded = "00" + encoded
				} else if representation == "plus" {
					encoded = "+" + encoded
				}
				update := `UPDATE device_push_registrations SET registration_version=CAST(? AS BLOB) WHERE session_id=? AND provider='fcm'`
				query := `SELECT typeof(registration_version) FROM device_push_registrations WHERE session_id=? AND provider='fcm'`
				restore := `UPDATE device_push_registrations SET registration_version=? WHERE session_id=? AND provider='fcm'`
				key := s.ID
				if field == "settings" {
					update = `UPDATE device_notification_settings SET settings_version=CAST(? AS BLOB) WHERE device_id=?`
					query = `SELECT typeof(settings_version) FROM device_notification_settings WHERE device_id=?`
					restore = `UPDATE device_notification_settings SET settings_version=? WHERE device_id=?`
					key = s.DeviceID
				}
				// 只在坏库夹具中临时绕过 CHECK，随后立刻恢复；不修改历史约束。
				jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
				jobSQL(t, f.store, update, encoded, key)
				jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
				before := leaseDigest(t, f.store)
				if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
					var storageClass string
					if err := tx.QueryRowContext(jobContext, query, key).Scan(&storageClass); err != nil {
						return err
					}
					if storageClass != "blob" {
						t.Fatal("fixture type", storageClass)
					}
					got, err := CheckSyncLeaseTx(jobContext, tx, held[0].Lease, f.now)
					if got != nil || !errors.Is(err, ErrJobData) {
						t.Fatalf("corrupt version candidate=%v error=%v", got != nil, err)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if before != leaseDigest(t, f.store) {
					t.Fatal("version check wrote database")
				}
				// 恢复同一个合法 INTEGER 后原租约仍有效；修复不改变数值比较边界。
				jobSQL(t, f.store, restore, version, key)
				leaseExpect(t, f.other, held[0].Lease, f.now, true)
			})
		}
	}
}

package push

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

func TestSyncLeaseCheckCurrentVersionsAndIdentity(t *testing.T) {
	for _, test := range []struct{ name, query string }{
		{"registration-rotated", `UPDATE device_push_registrations SET registration_version=2 WHERE session_id=? AND provider='fcm'`},
		{"registration-tombstone", `UPDATE device_push_registrations SET registration_version=2,token=NULL WHERE session_id=? AND provider='fcm'`},
		{"registration-deleted", `DELETE FROM device_push_registrations WHERE session_id=? AND provider='fcm'`},
		{"revoked", `UPDATE sessions SET revoked=1 WHERE id=?`},
		{"session-generation", `UPDATE sessions SET auth_generation=2 WHERE id=?`},
		{"client-web", `UPDATE sessions SET client_type='web' WHERE id=?`},
		{"idle-exact", `UPDATE sessions SET idle_expires_at=0 WHERE id=?`},
		{"absolute-exact", `UPDATE sessions SET absolute_expires_at=0 WHERE id=?`},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, held := leaseFixture(t)
			leaseExpect(t, f.store, held.Lease, f.now, true)
			query := test.query
			if test.name == "idle-exact" {
				query = fmt.Sprintf("UPDATE sessions SET idle_expires_at=%d WHERE id=?", f.now.UnixNano())
			}
			if test.name == "absolute-exact" {
				query = fmt.Sprintf("UPDATE sessions SET idle_expires_at=%d,absolute_expires_at=%d WHERE id=?", f.now.UnixNano(), f.now.UnixNano())
			}
			jobSQL(t, f.store, query, held.Job.Snapshot.SessionID)
			leaseExpect(t, f.other, held.Lease, f.now, false)
		})
	}
	for _, change := range []string{
		`UPDATE administrator SET auth_generation=2 WHERE id=1`,
		`INSERT INTO device_notification_settings VALUES('11111111-1111-4111-8111-111111111111',1,'2026-01-01T00:00:00Z','{}')`,
	} {
		t.Run(change, func(t *testing.T) {
			f, held := leaseFixture(t)
			jobSQL(t, f.store, change)
			leaseExpect(t, f.other, held.Lease, f.now, false)
		})
	}
}

func TestSyncLeaseCheckChangedDeployment(t *testing.T) {
	f, held := leaseFixture(t)
	leaseExpect(t, f.store, held.Lease, f.now, true)
	jobSQL(t, f.store, `UPDATE instance_identity SET instance_id='33333333-3333-4333-8333-333333333333' WHERE id=1`)
	// 旧Store本身先失败关闭；新实例重开后仍必须拒绝原部署的job。
	called := false
	if err := f.store.Read(jobContext, func(*sql.Tx) error { called = true; return nil }); !errors.Is(err, storage.ErrInstanceUnavailable) || called {
		t.Fatal("old store crossed deployment", err)
	}
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	leaseExpect(t, reopened, held.Lease, f.now, false)
}

func TestSyncLeaseCheckSettingsVersionAndSilentPolicy(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	// 只读复核比较版本而不消费配置内容；真实HTTP偏好写入另在cmd组合验证。
	configuration := `{"enabled":false,"bot_overrides":[],"dnd":{"enabled":true,"timezone":"UTC","start":"22:00","end":"07:00"},"preview":false,"notify_channel":"none","notify_provider":null}`
	jobSQL(t, f.store, `INSERT INTO device_notification_settings VALUES(?,7,?,?)`, s.DeviceID, f.now.Format(time.RFC3339Nano), configuration)
	j := f.snapshot(t, s, "sync")
	j.SettingsVersion = 7
	jobCreate(t, f, j)
	held, err := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(held) != 1 {
		t.Fatal(err)
	}
	leaseExpect(t, f.other, held[0].Lease, f.now, true)
	jobSQL(t, f.store, `UPDATE device_notification_settings SET settings_version=8 WHERE device_id=?`, s.DeviceID)
	leaseExpect(t, f.other, held[0].Lease, f.now, false)
	jobSQL(t, f.store, `DELETE FROM device_notification_settings WHERE device_id=?`, s.DeviceID)
	leaseExpect(t, f.other, held[0].Lease, f.now, false)
	if jobRead(t, f.store, j.ID).Snapshot != j {
		t.Fatal("current defaults replaced old snapshot")
	}
}

func TestSyncLeaseCheckInputAndSQLFailures(t *testing.T) {
	f, held := leaseFixture(t)
	if got, err := CheckSyncLeaseTx(jobContext, nil, held.Lease, f.now); !errors.Is(err, ErrJobInput) || got != nil {
		t.Fatal("nil tx", err)
	}
	err := f.store.Read(jobContext, func(tx *sql.Tx) error {
		if got, err := CheckSyncLeaseTx(nil, tx, held.Lease, f.now); !errors.Is(err, ErrJobInput) || got != nil {
			t.Fatal("nil context", err)
		}
		ctx, cancel := context.WithCancel(jobContext)
		cancel()
		if got, err := CheckSyncLeaseTx(ctx, tx, held.Lease, f.now); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("canceled", err)
		}
		for _, lease := range []JobLease{{}, {jobID: held.Lease.jobID, until: held.Lease.until}, {jobID: held.Lease.jobID, secret: held.Lease.secret}} {
			if got, err := CheckSyncLeaseTx(jobContext, tx, lease, f.now); !errors.Is(err, ErrJobInput) || got != nil {
				t.Fatal("invalid handle", err)
			}
		}
		for _, now := range []time.Time{{}, time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)} {
			if got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, now); !errors.Is(err, ErrJobInput) || got != nil {
				t.Fatal("invalid now", err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"push_jobs", "instance_identity", "device_push_registrations", "device_notification_settings"} {
		t.Run(table, func(t *testing.T) {
			before := leaseDigest(t, f.store)
			rollback := errors.New("rollback SQL fault fixture")
			err := f.store.Write(jobContext, func(tx *sql.Tx) error {
				if _, err := tx.Exec(`DROP TABLE ` + table); err != nil {
					return err
				}
				if got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now); err == nil || got != nil {
					t.Fatal("SQL failure returned usable candidate")
				}
				return rollback
			})
			if !errors.Is(err, rollback) || before != leaseDigest(t, f.other) {
				t.Fatal("fault fixture leaked", err)
			}
		})
	}
	var closed *sql.Tx
	if err := f.store.Read(jobContext, func(tx *sql.Tx) error { closed = tx; return nil }); err != nil {
		t.Fatal(err)
	}
	if got, err := CheckSyncLeaseTx(jobContext, closed, held.Lease, f.now); err == nil || got != nil {
		t.Fatal("closed transaction returned candidate")
	}
}

func TestSyncLeaseCheckCorruptDataFailsClosed(t *testing.T) {
	for _, query := range []string{
		`UPDATE push_jobs SET lease_hash=X'00' WHERE id=?`,
		`UPDATE push_jobs SET lease_hash='not-a-blob' WHERE id=?`,
		`UPDATE push_jobs SET lease_until_ns=0 WHERE id=?`,
		`UPDATE push_jobs SET next_attempt_at='bad-time' WHERE id=?`,
		`UPDATE push_jobs SET attempt=-1 WHERE id=?`,
	} {
		t.Run(query, func(t *testing.T) {
			f, held := leaseFixture(t)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
			jobSQL(t, f.store, query, held.Job.Snapshot.ID)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
			before := leaseDigest(t, f.store)
			if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
				if got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now); err == nil || got != nil {
					t.Fatal("corrupt job candidate")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if before != leaseDigest(t, f.other) {
				t.Fatal("corrupt check wrote data")
			}
		})
	}
	for _, query := range []string{
		`UPDATE device_push_registrations SET token=X'00'`,
		`UPDATE sessions SET idle_expires_at='bad-time'`,
		`INSERT INTO device_notification_settings VALUES('11111111-1111-4111-8111-111111111111',-1,'invalid','{}')`,
	} {
		t.Run(query, func(t *testing.T) {
			f, held := leaseFixture(t)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
			jobSQL(t, f.store, query)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
			before := leaseDigest(t, f.store)
			if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
				if got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now); err == nil || got != nil {
					t.Fatal("corrupt current identity candidate")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if before != leaseDigest(t, f.other) {
				t.Fatal("corrupt check wrote data")
			}
		})
	}
}

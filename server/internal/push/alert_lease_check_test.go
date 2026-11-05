package push

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

// 目前没有真实 alert 生产者；这里显式创建完整 alert 快照，使用真实通用预留。
func alertLeaseFixture(t *testing.T, provider string, version int64) (*jobFixture, JobReservation) {
	t.Helper()
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	if provider == "opush" {
		jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'opush',1,'synthetic-opush-secret',?)`, s.ID, f.now.Format(jobTimestamp))
	}
	if version > 0 {
		jobSQL(t, f.store, `INSERT INTO device_notification_settings VALUES(?,?,?,'{}')`, s.DeviceID, version, f.now.Format(jobTimestamp))
	}
	j := f.snapshot(t, s, "alert")
	j.Provider, j.SettingsVersion = provider, version
	jobCreate(t, f, j)
	held, err := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(held) != 1 || held[0].Job.Snapshot != j {
		t.Fatal("reserve complete alert", err)
	}
	return f, held[0]
}

func alertLeaseExpect(t *testing.T, store *storage.Store, lease JobLease, now time.Time, valid bool) *AlertLeaseSnapshot {
	t.Helper()
	before := leaseDigest(t, store)
	var got *AlertLeaseSnapshot
	if err := store.Read(jobContext, func(tx *sql.Tx) error {
		var err error
		got, err = CheckAlertLeaseTx(jobContext, tx, lease, now)
		return err
	}); err != nil || (got != nil) != valid {
		t.Fatalf("candidate=%v want=%v error=%v", got != nil, valid, err)
	}
	if before != leaseDigest(t, store) {
		t.Fatal("check wrote database")
	}
	return got
}

func TestAlertLeaseCheckFencingAndPurpose(t *testing.T) {
	for _, provider := range []string{"fcm", "opush"} {
		t.Run(provider, func(t *testing.T) {
			f, held := alertLeaseFixture(t, provider, 0)
			checked := alertLeaseExpect(t, f.other, held.Lease, f.now, true)
			if checked.Job != held.Job {
				t.Fatal("replaced original job")
			}
			leaseExpect(t, f.other, held.Lease, f.now, false)
			raw, err := json.Marshal(checked)
			if err != nil {
				t.Fatal(err)
			}
			printed := fmt.Sprintf("%v %+v %#v", checked, checked, checked)
			for _, secret := range []string{hex.EncodeToString(held.Lease.secret[:]), "synthetic-registration-secret", "synthetic-opush-secret", "private-job-fixture"} {
				if strings.Contains(string(raw), secret) || strings.Contains(printed, secret) {
					t.Fatal("secret in candidate")
				}
			}
			wrong := held.Lease
			wrong.secret[0] ^= 1
			alertLeaseExpect(t, f.store, wrong, f.now, false)
			wrong = held.Lease
			wrong.until = wrong.until.Add(time.Nanosecond)
			alertLeaseExpect(t, f.store, wrong, f.now, false)
			wrong = held.Lease
			wrong.jobID = "missing"
			alertLeaseExpect(t, f.store, wrong, f.now, false)
			alertLeaseExpect(t, f.store, held.Lease, held.Lease.until, false)
			alertLeaseExpect(t, f.store, held.Lease, f.now.Add(-time.Nanosecond), false)
			taken, err := NewJobRepository(f.other).Reserve(jobContext, held.Lease.until, time.Minute, 1)
			if err != nil || len(taken) != 1 {
				t.Fatal("takeover", err)
			}
			alertLeaseExpect(t, f.store, held.Lease, held.Lease.until, false)
			reopened, err := storage.Open(jobContext, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			alertLeaseExpect(t, reopened, taken[0].Lease, held.Lease.until, true)
			if ok, err := NewJobRepository(f.other).Release(jobContext, taken[0].Lease, held.Lease.until); err != nil || !ok {
				t.Fatal("release", err)
			}
			alertLeaseExpect(t, reopened, taken[0].Lease, held.Lease.until, false)
			long, err := NewJobRepository(f.other).Reserve(jobContext, held.Lease.until, 2*time.Hour, 1)
			if err != nil || len(long) != 1 {
				t.Fatal("long lease", err)
			}
			alertLeaseExpect(t, reopened, long[0].Lease, held.Job.Snapshot.ExpiresAt.Add(-time.Nanosecond), true)
			alertLeaseExpect(t, reopened, long[0].Lease, held.Job.Snapshot.ExpiresAt, false)
			if jobRead(t, f.store, held.Job.Snapshot.ID).Attempt != 0 {
				t.Fatal("attempt advanced")
			}
		})
	}
	f, sync := leaseFixture(t)
	alertLeaseExpect(t, f.other, sync.Lease, f.now, false)
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('alert-legacy',?,?,1,'legacy-time')`, sync.Job.Snapshot.SessionID, sync.Job.Snapshot.DeviceID)
	alertLeaseExpect(t, f.store, JobLease{jobID: "alert-legacy", secret: [32]byte{1}, until: f.now.Add(time.Minute)}, f.now, false)
}

// 每个故障在隔离事务中注入，检查前恢复 CHECK/触发器，最终回滚原始数据库。
// 只复用一次完整夹具，避免为相同迁移/密码散列重复付出昂贵成本。
func alertLeaseFault(t *testing.T, f *jobFixture, held JobReservation, inject func(*sql.Tx) error, valid, wantError bool) {
	t.Helper()
	before := leaseDigest(t, f.store)
	rollback := errors.New("rollback alert fixture")
	err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if err := inject(tx); err != nil {
			return err
		}
		var changesBefore, changesAfter int64
		if err := tx.QueryRow(`SELECT total_changes()`).Scan(&changesBefore); err != nil {
			return err
		}
		got, err := CheckAlertLeaseTx(jobContext, tx, held.Lease, f.now)
		if (got != nil) != valid || (err != nil) != wantError {
			t.Errorf("candidate=%v error=%v want=%v/%v", got != nil, err, valid, wantError)
		}
		if err := tx.QueryRow(`SELECT total_changes()`).Scan(&changesAfter); err != nil {
			return err
		}
		if changesBefore != changesAfter {
			t.Error("check wrote transaction")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal("fault fixture", err)
	}
	if before != leaseDigest(t, f.other) {
		t.Fatal("fault fixture escaped rollback")
	}
	alertLeaseExpect(t, f.other, held.Lease, f.now, true)
}

func TestAlertLeaseCheckCurrentIdentityAndProviderIsolation(t *testing.T) {
	for _, provider := range []string{"fcm", "opush"} {
		t.Run(provider, func(t *testing.T) {
			f, held := alertLeaseFixture(t, provider, 7)
			j := held.Job.Snapshot
			// 同版本的另一提供方不能替代原注册；另一 session 也不能替代原 session。
			if provider == "fcm" {
				jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'opush',1,'other-provider-secret',?)`, j.SessionID, f.now.Format(jobTimestamp))
			}
			otherSession := f.session(t, jobDeviceB)
			jobSQL(t, f.store, `INSERT OR IGNORE INTO device_push_registrations VALUES(?,'opush',1,'other-session-secret',?)`, otherSession.ID, f.now.Format(jobTimestamp))
			cases := []struct {
				name, q string
				args    []any
				valid   bool
			}{
				{"rotation", `UPDATE device_push_registrations SET registration_version=2 WHERE session_id=? AND provider=?`, []any{j.SessionID, provider}, false},
				{"tombstone", `UPDATE device_push_registrations SET token=NULL WHERE session_id=? AND provider=?`, []any{j.SessionID, provider}, false},
				{"missing-registration", `DELETE FROM device_push_registrations WHERE session_id=? AND provider=?`, []any{j.SessionID, provider}, false},
				{"other-provider", `UPDATE device_push_registrations SET token=NULL,registration_version=2 WHERE session_id=? AND provider<>?`, []any{j.SessionID, provider}, true},
				{"revoked", `UPDATE sessions SET revoked=1 WHERE id=?`, []any{j.SessionID}, false},
				{"generation", `UPDATE sessions SET auth_generation=2 WHERE id=?`, []any{j.SessionID}, false},
				{"admin-generation", `UPDATE administrator SET auth_generation=2 WHERE id=1`, nil, false},
				{"admin-missing", `DELETE FROM administrator WHERE id=1`, nil, false},
				{"session-missing", `DELETE FROM sessions WHERE id=?`, []any{j.SessionID}, false},
				{"client-web", `UPDATE sessions SET client_type='web' WHERE id=?`, []any{j.SessionID}, false},
				{"idle-boundary", `UPDATE sessions SET idle_expires_at=? WHERE id=?`, []any{f.now.UnixNano(), j.SessionID}, false},
				{"absolute-boundary", `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=? WHERE id=?`, []any{f.now.UnixNano(), f.now.UnixNano(), j.SessionID}, false},
				{"idle-plus-ns", `UPDATE sessions SET idle_expires_at=? WHERE id=?`, []any{f.now.Add(time.Nanosecond).UnixNano(), j.SessionID}, true},
				{"settings-rotation", `UPDATE device_notification_settings SET settings_version=8 WHERE device_id=?`, []any{j.DeviceID}, false},
				{"settings-missing", `DELETE FROM device_notification_settings WHERE device_id=?`, []any{j.DeviceID}, false},
				{"configuration-not-policy", `UPDATE device_notification_settings SET configuration='{"enabled":false,"notify_channel":"none","dnd":{"enabled":true}}' WHERE device_id=?`, []any{j.DeviceID}, true},
				{"source-not-currentness", `DELETE FROM messages WHERE id=?`, []any{j.MessageID}, true},
				{"due-future", `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, []any{f.now.Add(time.Nanosecond).Format(jobTimestamp), j.ID}, false},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					alertLeaseFault(t, f, held, func(tx *sql.Tx) error { _, e := tx.Exec(c.q, c.args...); return e }, c.valid, false)
				})
			}
			// 不经过 Store 的末端身份拦截，在同一事务中验证部署不匹配分支。
			t.Run("deployment", func(t *testing.T) {
				alertLeaseFault(t, f, held, func(tx *sql.Tx) error {
					_, e := tx.Exec(`UPDATE instance_identity SET instance_id='33333333-3333-4333-8333-333333333333' WHERE id=1`)
					return e
				}, false, false)
			})
		})
	}
}

func TestAlertLeaseCheckTypedCorruption(t *testing.T) {
	for _, provider := range []string{"fcm", "opush"} {
		t.Run(provider, func(t *testing.T) {
			f, held := alertLeaseFixture(t, provider, 7)
			j := held.Job.Snapshot
			cases := []struct {
				table, column string
				wantError     bool
			}{
				{"push_jobs", "target_change_seq", true}, {"push_jobs", "attempt", true},
				{"push_jobs", "registration_version", true}, {"push_jobs", "settings_version", true},
				{"push_jobs", "bot_seq", true}, {"push_jobs", "session_generation", true},
				{"push_jobs", "created_at_ns", true}, {"push_jobs", "expires_at_ns", true},
				{"push_jobs", "alert_eligible", true}, {"push_jobs", "lease_until_ns", true},
				{"push_jobs", "state", true}, {"push_jobs", "purpose", true}, {"push_jobs", "provider", true},
				{"push_jobs", "next_attempt_at", true}, {"push_jobs", "deployment_id", true},
				{"push_jobs", "message_id", true}, {"push_jobs", "bot_id", true}, {"push_jobs", "session_id", true}, {"push_jobs", "device_id", true},
				{"sessions", "auth_generation", true}, {"sessions", "revoked", true},
				{"sessions", "idle_expires_at", true}, {"sessions", "absolute_expires_at", true},
				{"sessions", "client_type", false}, {"sessions", "device_id", true},
				{"administrator", "auth_generation", true}, {"instance_identity", "instance_id", true},
				{"device_push_registrations", "registration_version", true}, {"device_push_registrations", "token", true},
				{"device_notification_settings", "settings_version", true},
			}
			for _, assignment := range []string{"lease_hash=X'00'", "lease_hash='not-blob'", "lease_until_ns=0", "next_attempt_at='invalid-time'", "attempt=-1"} {
				t.Run(assignment, func(t *testing.T) {
					alertLeaseFault(t, f, held, func(tx *sql.Tx) error {
						if _, e := tx.Exec(`PRAGMA ignore_check_constraints=ON`); e != nil {
							return e
						}
						_, e := tx.Exec(`UPDATE push_jobs SET `+assignment+` WHERE id=?`, j.ID)
						_, reset := tx.Exec(`PRAGMA ignore_check_constraints=OFF`)
						return errors.Join(e, reset)
					}, false, true)
				})
			}
			for _, c := range cases {
				t.Run(c.table+"."+c.column, func(t *testing.T) {
					alertLeaseFault(t, f, held, func(tx *sql.Tx) error {
						var trigger string
						if e := tx.QueryRow(`SELECT sql FROM sqlite_master WHERE name='push_jobs_snapshot_immutable'`).Scan(&trigger); e != nil {
							return e
						}
						if _, e := tx.Exec(`PRAGMA defer_foreign_keys=ON; PRAGMA ignore_check_constraints=ON; DROP TRIGGER push_jobs_snapshot_immutable`); e != nil {
							return e
						}
						where := "id=?"
						args := []any{j.ID}
						switch c.table {
						case "sessions":
							args = []any{j.SessionID}
						case "administrator", "instance_identity":
							args = []any{1}
						case "device_push_registrations":
							where = "session_id=? AND provider=?"
							args = []any{j.SessionID, provider}
						case "device_notification_settings":
							where = "device_id=?"
							args = []any{j.DeviceID}
						}
						res, e := tx.Exec("UPDATE "+c.table+" SET "+c.column+"=CAST("+c.column+" AS BLOB) WHERE "+where, args...)
						if e != nil {
							return e
						}
						if n, e := res.RowsAffected(); e != nil || n != 1 {
							return fmt.Errorf("fault not hit: %d %v", n, e)
						}
						if _, e = tx.Exec(`PRAGMA ignore_check_constraints=OFF`); e != nil {
							return e
						}
						if _, e = tx.Exec(trigger); e != nil {
							return e
						}
						// session/admin 撤销触发器已清除映射，恢复原注册以真正命中身份类型复核。
						if _, e = tx.Exec(`INSERT OR IGNORE INTO device_push_registrations VALUES(?,?,1,'synthetic-registration-secret',?)`, j.SessionID, provider, f.now.Format(jobTimestamp)); e != nil {
							return e
						}
						var ignored int
						if e = tx.QueryRow(`PRAGMA ignore_check_constraints`).Scan(&ignored); e != nil {
							return e
						}
						if ignored != 0 {
							return errors.New("CHECK not restored")
						}
						return nil
					}, false, c.wantError)
				})
			}
		})
	}
}

func TestAlertLeaseCheckInputSQLAndOriginalEligibility(t *testing.T) {
	f, held := alertLeaseFixture(t, "fcm", 0)
	if got, e := CheckAlertLeaseTx(jobContext, nil, held.Lease, f.now); got != nil || !errors.Is(e, ErrJobInput) {
		t.Fatal("nil tx", e)
	}
	if e := f.store.Read(jobContext, func(tx *sql.Tx) error {
		canceled, cancel := context.WithCancel(jobContext)
		cancel()
		for _, ctx := range []context.Context{nil, canceled} {
			if got, e := CheckAlertLeaseTx(ctx, tx, held.Lease, f.now); got != nil || e == nil {
				t.Error("context accepted")
			}
		}
		for _, lease := range []JobLease{{}, {jobID: held.Lease.jobID, until: held.Lease.until}, {jobID: held.Lease.jobID, secret: held.Lease.secret}} {
			if got, e := CheckAlertLeaseTx(jobContext, tx, lease, f.now); got != nil || !errors.Is(e, ErrJobInput) {
				t.Error("bad lease accepted", e)
			}
		}
		for _, now := range []time.Time{{}, time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)} {
			if got, e := CheckAlertLeaseTx(jobContext, tx, held.Lease, now); got != nil || !errors.Is(e, ErrJobInput) {
				t.Error("bad time accepted", e)
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"push_jobs", "instance_identity", "sessions", "administrator", "device_push_registrations", "device_notification_settings"} {
		t.Run(table, func(t *testing.T) {
			alertLeaseFault(t, f, held, func(tx *sql.Tx) error {
				q := `DROP TABLE ` + table
				if table == "sessions" {
					q = `ALTER TABLE sessions RENAME TO alert_fault_sessions`
				}
				if _, e := tx.Exec(q); e != nil {
					return e
				}
				if table == "sessions" {
					var count int
					if e := tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE id=?`, held.Lease.jobID).Scan(&count); e != nil {
						return e
					}
					if count != 1 {
						return errors.New("SQL fault lost original job")
					}
					got, err := CheckAlertLeaseTx(jobContext, tx, held.Lease, f.now)
					if got != nil || err == nil || !strings.Contains(err.Error(), "no such table: sessions") {
						return fmt.Errorf("SQL fault not reached: %v", err)
					}
					t.Log("original leased job remains; checker reached missing sessions SQL table")
				}
				return nil
			}, false, true)
		})
	}
	var closed *sql.Tx
	if e := f.store.Read(jobContext, func(tx *sql.Tx) error { closed = tx; return nil }); e != nil {
		t.Fatal(e)
	}
	if got, e := CheckAlertLeaseTx(jobContext, closed, held.Lease, f.now); got != nil || !errors.Is(e, sql.ErrTxDone) {
		t.Fatal("closed tx", e)
	}
	// 原资格 false 的完整 alert 仍可由通用仓储预留，但不得由本叶提升。
	if ok, e := NewJobRepository(f.store).Release(jobContext, held.Lease, f.now); e != nil || !ok {
		t.Fatal(e)
	}
	jobSQL(t, f.store, `DELETE FROM push_jobs WHERE id=?`, held.Job.Snapshot.ID)
	j := held.Job.Snapshot
	j.ID = "original-ineligible"
	j.AlertEligible = false
	jobCreate(t, f, j)
	reserved, e := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
	if e != nil || len(reserved) != 1 {
		t.Fatal("reserve ineligible fixture", e)
	}
	alertLeaseExpect(t, f.other, reserved[0].Lease, f.now, false)
}

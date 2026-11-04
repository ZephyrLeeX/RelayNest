package push

import (
	"database/sql"
	"errors"
	"testing"
)

// 同值数字 BLOB 不能因 typed Scan 转换而重新获得完整 sync 候选资格。
func TestSyncLeaseCheckAuthoritativeIntegerStorageRegression(t *testing.T) {
	for _, c := range []struct {
		table, column string
		immutable     bool
	}{
		{"sessions", "idle_expires_at", false},
		{"sessions", "absolute_expires_at", false},
		{"push_jobs", "target_change_seq", false},
		{"push_jobs", "attempt", false},
		{"push_jobs", "registration_version", true},
		{"push_jobs", "settings_version", true},
		{"push_jobs", "bot_seq", true},
		{"push_jobs", "session_generation", true},
		{"push_jobs", "created_at_ns", true},
		{"push_jobs", "expires_at_ns", true},
		{"push_jobs", "alert_eligible", true},
	} {
		t.Run(c.table+"."+c.column, func(t *testing.T) {
			f, held := leaseFixture(t)
			leaseExpect(t, f.store, held.Lease, f.now, true)
			key := held.Job.Snapshot.SessionID
			if c.table == "push_jobs" {
				key = held.Job.Snapshot.ID
			}
			var original int64
			var trigger string
			if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
				if err := tx.QueryRowContext(jobContext, "SELECT "+c.column+" FROM "+c.table+" WHERE id=?", key).Scan(&original); err != nil {
					return err
				}
				if c.table == "push_jobs" {
					return tx.QueryRowContext(jobContext, `SELECT sql FROM sqlite_master WHERE type='trigger' AND name='push_jobs_snapshot_immutable'`).Scan(&trigger)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			// 可信坏库夹具仅临时绕过对应约束；检查前恢复原始不可变触发器。
			if c.immutable {
				jobSQL(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable`)
			}
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
			jobSQL(t, f.store, "UPDATE "+c.table+" SET "+c.column+"=CAST("+c.column+" AS BLOB) WHERE id=?", key)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
			if c.immutable {
				jobSQL(t, f.store, trigger)
			}
			before := leaseDigest(t, f.store)
			if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
				var kind string
				if err := tx.QueryRowContext(jobContext, "SELECT typeof("+c.column+") FROM "+c.table+" WHERE id=?", key).Scan(&kind); err != nil {
					return err
				}
				if kind != "blob" {
					t.Fatal("fixture storage class", kind)
				}
				got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now)
				if got != nil || !errors.Is(err, ErrJobData) {
					t.Fatalf("corrupt number candidate=%v error=%v", got != nil, err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if before != leaseDigest(t, f.store) {
				t.Fatal("integer validation changed database")
			}
			// BLOB 还原 INTEGER 会改变 SQLite 类型排序，target 水位触发器也须
			// 在此可信还原步骤临时绕过；随后完整恢复原触发器和同值正控。
			if c.table == "push_jobs" {
				jobSQL(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable`)
			}
			jobSQL(t, f.store, "UPDATE "+c.table+" SET "+c.column+"=? WHERE id=?", original, key)
			if c.table == "push_jobs" {
				jobSQL(t, f.store, trigger)
			}
			leaseExpect(t, f.other, held.Lease, f.now, true)
		})
	}
}

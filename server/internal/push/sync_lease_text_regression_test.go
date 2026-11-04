package push

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

// 完整 job 的文本存储类必须与 SQL 调度及外键谓词一致；不规范化坏库行。
func TestSyncLeaseCheckTextStorageRegression(t *testing.T) {
	for _, field := range []string{"id", "session_id", "device_id", "state", "next_attempt_at", "purpose", "provider", "deployment_id", "message_id", "bot_id"} {
		t.Run(field, func(t *testing.T) {
			f, held := leaseFixture(t)
			leaseExpect(t, f.store, held.Lease, f.now, true)
			var original, trigger string
			if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
				if err := tx.QueryRowContext(jobContext, "SELECT "+field+" FROM push_jobs WHERE id=?", held.Job.Snapshot.ID).Scan(&original); err != nil {
					return err
				}
				return tx.QueryRowContext(jobContext, `SELECT sql FROM sqlite_master WHERE type='trigger' AND name='push_jobs_snapshot_immutable'`).Scan(&trigger)
			}); err != nil {
				t.Fatal(err)
			}
			immutable := field != "state" && field != "next_attempt_at"
			originalDigest := leaseDigest(t, f.store)
			if immutable {
				jobSQL(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable`)
			}
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
			mutationErr := f.store.Write(jobContext, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(jobContext, "UPDATE push_jobs SET "+field+"=CAST("+field+" AS BLOB) WHERE id=?", held.Job.Snapshot.ID)
				return err
			})
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
			if immutable {
				jobSQL(t, f.store, trigger)
			}
			if mutationErr != nil {
				// 关系键先被原外键约束拒绝也是既有失败关闭；不为造坏库关闭外键。
				if (field != "session_id" && field != "device_id") || !strings.Contains(mutationErr.Error(), "FOREIGN KEY constraint failed") {
					t.Fatal("unexpected fixture error", mutationErr)
				}
				if originalDigest != leaseDigest(t, f.other) {
					t.Fatal("rejected foreign-key mutation changed data")
				}
				leaseExpect(t, f.other, held.Lease, f.now, true)
				t.Log("existing foreign key rejected non-TEXT", field)
				return
			}
			where := "id=?"
			if field == "id" {
				where = "id=CAST(? AS BLOB)"
			}
			var second *JobReservation
			if field == "state" {
				s := f.session(t, held.Job.Snapshot.DeviceID)
				j := f.snapshot(t, s, "sync")
				jobCreate(t, f, j)
				got, err := NewJobRepository(f.other).Reserve(jobContext, f.now, time.Minute, 1)
				if err != nil || len(got) != 1 || got[0].Job.Snapshot.ID == held.Job.Snapshot.ID || got[0].Job.Snapshot.DeviceID != held.Job.Snapshot.DeviceID {
					t.Fatal("same-device new lease", err)
				}
				second = &got[0]
			}
			before := leaseDigest(t, f.store)
			if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
				var kind string
				if err := tx.QueryRowContext(jobContext, "SELECT typeof("+field+") FROM push_jobs WHERE "+where, held.Job.Snapshot.ID).Scan(&kind); err != nil {
					return err
				}
				if kind != "blob" {
					t.Fatal("fixture type", kind)
				}
				got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now)
				if got != nil || (field != "id" && !errors.Is(err, ErrJobData)) {
					t.Fatalf("non-TEXT job candidate=%v error=%v", got != nil, err)
				}
				if second != nil {
					current, err := CheckSyncLeaseTx(jobContext, tx, second.Lease, f.now)
					if err != nil || current == nil || current.Job.Snapshot.ID != second.Job.Snapshot.ID {
						t.Fatal("valid same-device replacement rejected", err)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if before != leaseDigest(t, f.other) {
				t.Fatal("TEXT validation wrote database")
			}
			if second != nil {
				ok, err := NewJobRepository(f.store).Release(jobContext, second.Lease, f.now)
				if err != nil || !ok {
					t.Fatal("release replacement fixture", err)
				}
			}
			if immutable {
				jobSQL(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable`)
			}
			jobSQL(t, f.store, "UPDATE push_jobs SET "+field+"=? WHERE "+where, original, held.Job.Snapshot.ID)
			if immutable {
				jobSQL(t, f.store, trigger)
			}
			leaseExpect(t, f.other, held.Lease, f.now, true)
		})
	}
}

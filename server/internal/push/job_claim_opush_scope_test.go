package push

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
)

// 仓储夹具通过真实 CreateJobTx 和会话注册生成完整原快照，不冒称生产者或平台验证。
func (f *jobFixture) opushSnapshot(t *testing.T, s auth.Session) JobSnapshot {
	t.Helper()
	jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'opush',1,'synthetic-opush-registration',?) ON CONFLICT(session_id,provider) DO NOTHING`, s.ID, f.now.Format(time.RFC3339Nano))
	j := f.snapshot(t, s, "alert")
	j.Provider = "opush"
	return j
}

// 仅内部短事务路径需要直接观测暂存值或故意忽略错误后继续提交。
func TestReserveOPushAlertsInternalTransactionAtomicity(t *testing.T) {
	t.Run("savepoint", opushInternalBatchFaults)
	t.Run("cancel-after-staging", opushInternalBatchCancellation)
}

func opushInternalBatchFaults(t *testing.T) {
	for _, fault := range []string{"sql", "scan", "zero"} {

		t.Run(fault, func(t *testing.T) {
			f := jobSetup(t)
			a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
			jobCreate(t, f, f.snapshot(t, a, "alert"))
			jobCreate(t, f, f.snapshot(t, b, "alert"))
			r := NewJobRepository(f.store)
			if got, e := r.Reserve(jobContext, f.now, time.Minute, 2); e != nil || len(got) != 2 {
				t.Fatal(e)
			}
			one, two := f.opushSnapshot(t, a), f.opushSnapshot(t, b)
			jobCreate(t, f, one)
			jobCreate(t, f, two)
			jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.AddDate(-2, 0, 0).Format(jobTimestamp), one.ID)
			// 第二项更新前实查第一项已领取、两个旧 FCM alert 已回收，防止排序错误使故障提前。
			// 守卫错误刻意不同于各预期故障，下面必须核对具体错误，不能任意错误都算通过。
			orderGuard := `SELECT CASE WHEN (SELECT state FROM push_jobs WHERE id='job-003') IS NOT 'leased'
 OR (SELECT state FROM push_jobs WHERE id='job-001') IS NOT 'queued'
 OR (SELECT state FROM push_jobs WHERE id='job-002') IS NOT 'queued'
 THEN RAISE(ABORT,'scope order guard failed') END;`
			if fault == "scan" || fault == "commit" {
				jobSQL(t, f.store, `CREATE TRIGGER scope_order BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` END`)
			}
			expectFault := func(err error) {
				t.Helper()
				valid := err != nil
				switch fault {
				case "sql":
					valid = valid && strings.Contains(err.Error(), "scope batch fault")
				case "scan", "zero":
					valid = errors.Is(err, ErrJobData)
				case "commit":
					valid = valid && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
				}
				if !valid {
					t.Fatalf("wrong %s fault: %v", fault, err)
				}
			}
			switch fault {
			case "sql":
				jobSQL(t, f.store, `CREATE TRIGGER scope_fault BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` SELECT RAISE(ABORT,'scope batch fault'); END`)
			case "scan":
				// SQLite 会归一化二月三十日，因此通过原 CHECK；Go 日历解析拒绝。
				// 第一个 opush alert 的时间更早，守卫确保此错误发生在第二项读取阶段。
				jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, fmt.Sprintf("%04d-02-30T00:00:00.000000000Z", f.now.Year()-1), two.ID)
			case "zero":
				jobSQL(t, f.store, `CREATE TRIGGER scope_fault BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` SELECT RAISE(IGNORE); END`)
			case "commit":
				jobSQL(t, f.store, `CREATE TABLE scope_commit(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER scope_fault AFTER UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN INSERT INTO scope_commit VALUES('absent'); END`)
			}
			before := scopeRows(t, f.store)
			now := f.now.Add(time.Minute)

			err := f.store.Write(jobContext, func(tx *sql.Tx) error {
				got, e := reserveJobsTx(jobContext, tx, now, time.Minute, 2, jobReservationOPushAlerts)
				if e == nil || got != nil {
					t.Fatal("fault published partial batch")
				}
				expectFault(e)
				// 故意继续提交，验证保存点而非仅外层回滚提供原子性。
				_, e = tx.Exec(`CREATE TABLE scope_caller_continued(id INTEGER)`)
				return e
			})
			if err != nil {
				t.Fatal(err)
			}
			if jobCount(t, f.other, `SELECT count(*) FROM sqlite_master WHERE name='scope_caller_continued'`) != 1 {
				t.Fatal("caller did not commit")
			}

			if !reflect.DeepEqual(before, scopeRows(t, f.other)) {
				t.Fatal("partial reclaim/reservation survived")
			}
			if fault == "commit" && jobCount(t, f.store, `SELECT count(*) FROM scope_commit`) != 0 {
				t.Fatal("failed commit persisted side effect")
			}
		})

	}
}

func opushInternalBatchCancellation(t *testing.T) {
	f := jobSetup(t)
	a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
	jobCreate(t, f, f.snapshot(t, a, "alert"))
	jobCreate(t, f, f.snapshot(t, b, "alert"))
	r := NewJobRepository(f.store)
	if got, e := r.Reserve(jobContext, f.now, time.Minute, 2); e != nil || len(got) != 2 {
		t.Fatal(e)
	}
	one, two := f.opushSnapshot(t, a), f.opushSnapshot(t, b)
	jobCreate(t, f, one)
	jobCreate(t, f, two)
	before := scopeRows(t, f.store)
	ctx, cancel := context.WithCancel(jobContext)
	defer cancel()
	var staged []JobReservation
	err := f.store.Write(ctx, func(tx *sql.Tx) error {
		var e error
		staged, e = reserveJobsTx(ctx, tx, f.now.Add(time.Minute), time.Minute, 2, jobReservationOPushAlerts)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(scopeIDs(staged), []string{one.ID, two.ID}) {
			t.Fatal("batch was not fully staged")
		}
		var leased, reclaimed int
		if e = tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE provider='opush' AND state='leased'`).Scan(&leased); e != nil {
			return e
		}
		if e = tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE purpose='alert' AND provider='fcm' AND state='queued' AND lease_hash IS NULL AND lease_until_ns IS NULL`).Scan(&reclaimed); e != nil {
			return e
		}
		if leased != 2 || reclaimed != 2 {
			t.Fatal("cancel did not follow both claims and reclaims")
		}
		cancel()
		return nil
	})
	if err == nil || len(staged) != 2 {
		t.Fatal("canceled batch committed", err)
	}
	if !reflect.DeepEqual(before, scopeRows(t, f.other)) {
		t.Fatal("cancellation left half batch")
	}
}

// 未定义的 scope 始终拒绝，保留旧测试中的 2 并补充 uint8 上界。
func TestReserveOPushAlertsRejectsUnknownScope(t *testing.T) {
	f := jobSetup(t)
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if got, err := reserveJobsTx(jobContext, tx, f.now, time.Minute, 1, jobReservationScope(255)); !errors.Is(err, ErrJobInput) || got != nil {
			t.Fatal("unknown scope", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

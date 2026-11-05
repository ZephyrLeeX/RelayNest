package push

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

// 读取全部真实列，避免回收测试只比较 Job 投影而漏掉 attempt/result 等持久字段。
func scopeRows(t *testing.T, s *storage.Store) [][]any {
	t.Helper()
	var out [][]any
	err := s.Read(jobContext, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT * FROM push_jobs ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return err
		}
		for rows.Next() {
			values := make([]any, len(cols))
			targets := make([]any, len(cols))
			for i := range values {
				targets[i] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				return err
			}
			out = append(out, values)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func scopeIDs(got []JobReservation) []string {
	ids := make([]string, len(got))
	for i, j := range got {
		ids[i] = j.Job.Snapshot.ID
	}
	return ids
}

func TestReserveSyncFiltersBeforeDeviceRankAndGlobalLimit(t *testing.T) {
	f := jobSetup(t)
	a, b, c := f.session(t, jobDeviceA), f.session(t, jobDeviceB), f.session(t, "33333333-3333-4333-8333-333333333333")
	alertA, alertB := f.snapshot(t, a, "alert"), f.snapshot(t, b, "alert")
	laterA, firstA, firstB, firstC := f.snapshot(t, a, "sync"), f.snapshot(t, a, "sync"), f.snapshot(t, b, "sync"), f.snapshot(t, c, "sync")
	for _, j := range []JobSnapshot{alertA, alertB, laterA, firstA, firstB, firstC} {
		jobCreate(t, f, j)
	}
	for _, item := range []struct {
		id    string
		delay time.Duration
	}{{laterA.ID, 3 * time.Second}, {firstA.ID, time.Second}, {firstB.ID, 2 * time.Second}, {firstC.ID, 2 * time.Second}} {
		jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.Add(item.delay).Format(jobTimestamp), item.id)
	}
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy',?,?,7,'unknown')`, c.ID, c.DeviceID)
	beforeAlerts := []Job{jobRead(t, f.store, alertA.ID), jobRead(t, f.store, alertB.ID)}
	r := NewJobRepository(f.store)
	got, err := r.ReserveSync(jobContext, f.now.Add(2*time.Second), time.Minute, 2)
	if err != nil || !reflect.DeepEqual(scopeIDs(got), []string{firstA.ID, firstB.ID}) {
		t.Fatal("scope/rank/order/limit", scopeIDs(got), err)
	}
	remaining, err := r.ReserveSync(jobContext, f.now.Add(2*time.Second), time.Minute, 2)
	if err != nil || !reflect.DeepEqual(scopeIDs(remaining), []string{firstC.ID}) {
		t.Fatal("remaining device", scopeIDs(remaining), err)
	}
	if !reflect.DeepEqual(beforeAlerts, []Job{jobRead(t, f.store, alertA.ID), jobRead(t, f.store, alertB.ID)}) {
		t.Fatal("unselected alerts changed")
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE id='legacy' AND state='pending' AND purpose IS NULL AND lease_hash IS NULL`) != 1 {
		t.Fatal("legacy upgraded")
	}
	for _, j := range append(got, remaining...) {
		if ok, e := r.Release(jobContext, j.Lease, f.now.Add(2*time.Second)); e != nil || !ok {
			t.Fatal(e)
		}
	}
	all, err := r.Reserve(jobContext, f.now.Add(2*time.Second), time.Minute, 2)
	if err != nil || !reflect.DeepEqual(scopeIDs(all), []string{alertA.ID, alertB.ID}) {
		t.Fatal("generic purpose compatibility", scopeIDs(all), err)
	}
}

func TestReserveSyncAlertHolderRecoveryAndReopen(t *testing.T) {
	f := jobSetup(t)
	a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
	alert, otherAlert := f.snapshot(t, a, "alert"), f.snapshot(t, b, "alert")
	jobCreate(t, f, alert)
	jobCreate(t, f, otherAlert)
	// 非零历史列验证最小回收不清除任何结果／尝试状态。
	jobSQL(t, f.store, `UPDATE push_jobs SET attempt=3 WHERE purpose='alert'`)
	r := NewJobRepository(f.store)
	old, err := r.Reserve(jobContext, f.now, time.Minute, 2)
	if err != nil || len(old) != 2 {
		t.Fatal(err)
	}
	target := f.snapshot(t, a, "sync")
	jobCreate(t, f, target)
	before := scopeRows(t, f.store)
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	syncRepo := NewJobRepository(reopened)
	if got, e := syncRepo.ReserveSync(jobContext, f.now.Add(time.Minute-time.Nanosecond), time.Minute, 1); e != nil || len(got) != 0 {
		t.Fatal("active alert bypassed", e)
	}
	got, err := syncRepo.ReserveSync(jobContext, f.now.Add(time.Minute), time.Minute, 1)
	if err != nil || !reflect.DeepEqual(scopeIDs(got), []string{target.ID}) {
		t.Fatal("exact expired alert takeover", err)
	}
	after := scopeRows(t, f.store)
	// SELECT * 的真实列名确定唯一允许变化，其他设备即使到期也完全不动。
	if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
		rows, e := tx.Query(`SELECT * FROM push_jobs LIMIT 0`)
		if e != nil {
			return e
		}
		defer rows.Close()
		cols, e := rows.Columns()
		if e != nil {
			return e
		}
		for row := range before {
			id := before[row][0].(string)
			for col, name := range cols {
				if id == alert.ID && (name == "state" || name == "lease_hash" || name == "lease_until_ns") {
					continue
				}
				if id == target.ID && (name == "state" || name == "lease_hash" || name == "lease_until_ns") {
					continue
				}
				if !reflect.DeepEqual(before[row][col], after[row][col]) {
					t.Fatalf("unexpected mutation %s.%s", id, name)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE id=? AND state='queued' AND lease_hash IS NULL AND lease_until_ns IS NULL`, alert.ID) != 1 {
		t.Fatal("expired alert not minimally reclaimed")
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE device_id=? AND state='leased'`, jobDeviceA) != 1 {
		t.Fatal("cross purpose device uniqueness")
	}
	if changed, e := r.Release(jobContext, old[0].Lease, f.now.Add(time.Minute)); e != nil || changed {
		t.Fatal("stale alert release", e)
	}
	if changed, e := r.Release(jobContext, got[0].Lease, f.now.Add(time.Minute)); e != nil || !changed {
		t.Fatal("new sync release", e)
	}
}

func TestReserveSyncDueExpiryAndSessionBoundaries(t *testing.T) {
	for _, scenario := range []string{"future", "expiry", "idle", "absolute", "revoked", "generation", "client"} {
		t.Run(scenario, func(t *testing.T) {
			f := jobSetup(t)
			s := f.session(t, jobDeviceA)
			j := f.snapshot(t, s, "sync")
			jobCreate(t, f, j)
			now := f.now
			switch scenario {
			case "future":
				jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, now.Add(time.Nanosecond).Format(jobTimestamp), j.ID)
			case "expiry":
				now = j.ExpiresAt
			case "idle":
				jobSQL(t, f.store, `UPDATE sessions SET idle_expires_at=? WHERE id=?`, now.UnixNano(), s.ID)
			case "absolute":
				jobSQL(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=? WHERE id=?`, now.UnixNano(), now.UnixNano(), s.ID)
			case "revoked":
				jobSQL(t, f.store, `UPDATE sessions SET revoked=1 WHERE id=?`, s.ID)
			case "generation":
				jobSQL(t, f.store, `UPDATE administrator SET auth_generation=2 WHERE id=1`)
			case "client":
				jobSQL(t, f.store, `UPDATE sessions SET client_type='web' WHERE id=?`, s.ID)
			}
			before := scopeRows(t, f.store)
			r := NewJobRepository(f.store)
			if got, e := r.ReserveSync(jobContext, now, time.Minute, 1); e != nil || len(got) != 0 {
				t.Fatal("boundary accepted", e)
			}
			if !reflect.DeepEqual(before, scopeRows(t, f.store)) {
				t.Fatal("rejected candidate mutated")
			}
			if scenario == "future" {
				got, e := r.ReserveSync(jobContext, now.Add(time.Nanosecond), time.Minute, 1)
				if e != nil || len(got) != 1 {
					t.Fatal("due equality", e)
				}
			}
		})
	}
}

func TestReserveSyncCompetesWithGenericAcrossStores(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	jobCreate(t, f, f.snapshot(t, s, "alert"))
	jobCreate(t, f, f.snapshot(t, s, "sync"))
	start := make(chan struct{})
	out := make(chan []JobReservation, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var got []JobReservation
			var err error
			if i%2 == 0 {
				got, err = NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
			} else {
				got, err = NewJobRepository(f.other).ReserveSync(jobContext, f.now, time.Minute, 1)
			}
			out <- got
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(out)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	count := 0
	for got := range out {
		count += len(got)
	}
	if count != 1 || jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 1 {
		t.Fatal("duplicate device lease", count)
	}
}

// 两个设备各有到期 alert，失败发生在第二个 sync；逐列证明领取及回收同批回滚。
func TestReserveSyncBatchFaultsAndSavepoint(t *testing.T) {
	for _, fault := range []string{"sql", "scan", "zero", "commit"} {
		for _, helper := range []bool{false, true} {
			if fault == "commit" && helper {
				continue
			}
			t.Run(fmt.Sprintf("%s/helper=%v", fault, helper), func(t *testing.T) {
				f := jobSetup(t)
				a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
				jobCreate(t, f, f.snapshot(t, a, "alert"))
				jobCreate(t, f, f.snapshot(t, b, "alert"))
				r := NewJobRepository(f.store)
				if got, e := r.Reserve(jobContext, f.now, time.Minute, 2); e != nil || len(got) != 2 {
					t.Fatal(e)
				}
				one, two := f.snapshot(t, a, "sync"), f.snapshot(t, b, "sync")
				jobCreate(t, f, one)
				jobCreate(t, f, two)
				jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.AddDate(-2, 0, 0).Format(jobTimestamp), one.ID)
				// 第二项更新前实查第一项已领取、两个旧 alert 已回收，防止排序错误使故障提前。
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
					// 第一个 sync 的时间更早，守卫确保此错误发生在第二项读取阶段。
					jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, fmt.Sprintf("%04d-02-30T00:00:00.000000000Z", f.now.Year()-1), two.ID)
				case "zero":
					jobSQL(t, f.store, `CREATE TRIGGER scope_fault BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` SELECT RAISE(IGNORE); END`)
				case "commit":
					jobSQL(t, f.store, `CREATE TABLE scope_commit(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER scope_fault AFTER UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN INSERT INTO scope_commit VALUES('absent'); END`)
				}
				before := scopeRows(t, f.store)
				now := f.now.Add(time.Minute)
				if helper {
					err := f.store.Write(jobContext, func(tx *sql.Tx) error {
						got, e := reserveJobsTx(jobContext, tx, now, time.Minute, 2, jobReservationSync)
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
				} else {
					got, e := r.ReserveSync(jobContext, now, time.Minute, 2)
					if got != nil {
						t.Fatal("fault published candidates")
					}
					expectFault(e)
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
}

func TestReserveSyncCancellationAndInputs(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	jobCreate(t, f, f.snapshot(t, s, "sync"))
	r := NewJobRepository(f.other)
	before := scopeRows(t, f.store)
	canceled, cancel := context.WithCancel(jobContext)
	cancel()
	if got, e := r.ReserveSync(canceled, f.now, time.Minute, 1); !errors.Is(e, context.Canceled) || got != nil {
		t.Fatal("canceled reserve", e)
	}
	if err := f.store.Write(jobContext, func(*sql.Tx) error {
		ctx, cancel := context.WithTimeout(jobContext, 10*time.Millisecond)
		defer cancel()
		got, e := r.ReserveSync(ctx, f.now, time.Minute, 1)
		if !errors.Is(e, context.DeadlineExceeded) || got != nil {
			return fmt.Errorf("waiting cancellation: %v", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var staged []JobReservation
	ctx, stop := context.WithCancel(jobContext)
	defer stop()
	err := f.store.Write(ctx, func(tx *sql.Tx) error {
		var e error
		staged, e = reserveJobsTx(ctx, tx, f.now, time.Minute, 1, jobReservationSync)
		stop()
		return e
	})
	if err == nil || len(staged) != 1 {
		t.Fatal("cancel after staging", err)
	}
	if !reflect.DeepEqual(before, scopeRows(t, f.store)) {
		t.Fatal("cancellation persisted lease")
	}
	for _, repo := range []*JobRepository{nil, NewJobRepository(nil), r} {
		if got, e := repo.ReserveSync(nil, f.now, time.Minute, 1); !errors.Is(e, ErrJobInput) || got != nil {
			t.Fatal("nil input", e)
		}
	}
	for _, repo := range []*JobRepository{nil, NewJobRepository(nil)} {
		if got, e := repo.ReserveSync(jobContext, f.now, time.Minute, 1); !errors.Is(e, ErrJobInput) || got != nil {
			t.Fatal("nil repository", e)
		}
	}
	for _, tc := range []struct {
		now   time.Time
		ttl   time.Duration
		limit int
	}{
		{f.now, 0, 1}, {f.now, -1, 1}, {f.now, time.Minute, 0}, {f.now, time.Minute, MaxJobReservations + 1},
		{time.Time{}, time.Minute, 1}, {time.Unix(0, 1), -time.Second, 1},
		{time.Unix(0, 1<<63-1), time.Nanosecond, 1}, {time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC), time.Minute, 1},
	} {
		if got, e := r.ReserveSync(jobContext, tc.now, tc.ttl, tc.limit); !errors.Is(e, ErrJobInput) || got != nil {
			t.Fatal("parameter boundary", e)
		}
	}
	if got, e := reserveJobsTx(jobContext, nil, f.now, time.Minute, 1, jobReservationSync); !errors.Is(e, ErrJobInput) || got != nil {
		t.Fatal("nil Tx", e)
	}
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if got, e := reserveJobsTx(jobContext, tx, f.now, time.Minute, 1, jobReservationScope(2)); !errors.Is(e, ErrJobInput) || got != nil {
			t.Fatal("open scope", e)
		}
		if got, e := reserveJobsTx(nil, tx, f.now, time.Minute, 1, jobReservationSync); !errors.Is(e, ErrJobInput) || got != nil {
			t.Fatal("nil context", e)
		}
		if got, e := reserveJobsTx(canceled, tx, f.now, time.Minute, 1, jobReservationSync); !errors.Is(e, context.Canceled) || got != nil {
			t.Fatal("canceled Tx", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, scopeRows(t, f.store)) {
		t.Fatal("invalid input mutated")
	}
}

func TestReserveSyncPreservesAttemptHistoryOnTakeover(t *testing.T) {
	for _, result := range []bool{false, true} {
		t.Run(fmt.Sprint(result), func(t *testing.T) {
			f := jobSetup(t)
			s := f.session(t, jobDeviceA)
			j := f.snapshot(t, s, "sync")
			jobCreate(t, f, j)
			r := NewJobRepository(f.store)
			got, err := r.ReserveSync(jobContext, f.now, time.Minute, 1)
			if err != nil || len(got) != 1 {
				t.Fatal(err)
			}
			ticket := mustBeginSync(t, r, got[0].Lease, f.now)
			now := f.now.Add(time.Minute)
			if result {
				if changed, e := r.ApplySyncResult(jobContext, *ticket, SyncTransient, f.now.Add(time.Second)); e != nil || !changed {
					t.Fatal(e)
				}
				now = f.now.Add(11 * time.Second)
			}
			before := jobRead(t, f.store, j.ID)
			var history []any
			readHistory := func() []any {
				var hash []byte
				var until, started, stamp sql.NullInt64
				var kind sql.NullString
				if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
					return tx.QueryRow(`SELECT attempt_lease_hash,attempt_lease_until_ns,attempt_started_at_ns,result_kind,result_at_ns FROM push_jobs WHERE id=?`, j.ID).Scan(&hash, &until, &started, &kind, &stamp)
				}); err != nil {
					t.Fatal(err)
				}
				return []any{hash, until, started, kind, stamp}
			}
			history = readHistory()
			next, err := NewJobRepository(f.other).ReserveSync(jobContext, now, time.Minute, 1)
			if err != nil || len(next) != 1 {
				t.Fatal("sync takeover", err)
			}
			after := jobRead(t, f.store, j.ID)
			before.State = "leased"
			if before != after || !reflect.DeepEqual(history, readHistory()) {
				t.Fatal("reserve rewrote attempt/result/snapshot")
			}
			if changed, e := r.Release(jobContext, got[0].Lease, now); e != nil || changed {
				t.Fatal("old sync release", e)
			}
			expectSyncNoop(t, r, *ticket, now)
		})
	}
}

// 取消发生在整批暂存候选形成之后，两个设备的旧 alert 回收也必须跟随外层回滚。
func TestReserveSyncBatchCancellationAfterStaging(t *testing.T) {
	f := jobSetup(t)
	a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
	jobCreate(t, f, f.snapshot(t, a, "alert"))
	jobCreate(t, f, f.snapshot(t, b, "alert"))
	r := NewJobRepository(f.store)
	if got, e := r.Reserve(jobContext, f.now, time.Minute, 2); e != nil || len(got) != 2 {
		t.Fatal(e)
	}
	one, two := f.snapshot(t, a, "sync"), f.snapshot(t, b, "sync")
	jobCreate(t, f, one)
	jobCreate(t, f, two)
	before := scopeRows(t, f.store)
	ctx, cancel := context.WithCancel(jobContext)
	defer cancel()
	var staged []JobReservation
	err := f.store.Write(ctx, func(tx *sql.Tx) error {
		var e error
		staged, e = reserveJobsTx(ctx, tx, f.now.Add(time.Minute), time.Minute, 2, jobReservationSync)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(scopeIDs(staged), []string{one.ID, two.ID}) {
			t.Fatal("batch was not fully staged")
		}
		var leased, reclaimed int
		if e = tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE purpose='sync' AND state='leased'`).Scan(&leased); e != nil {
			return e
		}
		if e = tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE purpose='alert' AND state='queued' AND lease_hash IS NULL AND lease_until_ns IS NULL`).Scan(&reclaimed); e != nil {
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

package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

func TestJobDeviceLeaseTakeoverAndRecovery(t *testing.T) {
	f := jobSetup(t)
	a := f.session(t, jobDeviceA)
	same := f.session(t, jobDeviceA)
	b := f.session(t, jobDeviceB)
	one := f.snapshot(t, a, "sync")
	two := f.snapshot(t, same, "alert")
	three := f.snapshot(t, b, "sync")
	for _, j := range []JobSnapshot{one, two, three} {
		jobCreate(t, f, j)
	}
	r := NewJobRepository(f.store)
	other := NewJobRepository(f.other)
	first, err := r.Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(first) != 1 || first[0].Job.Snapshot.ID != one.ID {
		t.Fatal("deterministic first reservation", err)
	}
	second, err := other.Reserve(jobContext, f.now, time.Minute, MaxJobReservations)
	if err != nil || len(second) != 1 || second[0].Job.Snapshot.ID != three.ID {
		t.Fatal("same device/session isolation or independent device", err)
	}
	if none, err := r.Reserve(jobContext, f.now, time.Minute, 2); err != nil || len(none) != 0 {
		t.Fatal("live device lease bypassed", err)
	}
	// 重开后仍由原持有者占用；租约并非进程内锁。
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if none, err := NewJobRepository(reopened).Reserve(jobContext, f.now, time.Minute, 2); err != nil || len(none) != 0 {
		t.Fatal("reopen lost lease", err)
	}
	takeover, err := other.Reserve(jobContext, f.now.Add(time.Minute), 2*time.Minute, 1)
	if err != nil || len(takeover) != 1 || takeover[0].Job.Snapshot.ID != one.ID {
		t.Fatal("expired lease not taken over", err)
	}
	if first[0].Lease.secret == takeover[0].Lease.secret {
		t.Fatal("lease secret reused")
	}
	if changed, err := r.Release(jobContext, first[0].Lease, f.now.Add(time.Minute)); err != nil || changed {
		t.Fatal("old lease changed new lease", err)
	}
	preserved := jobRead(t, f.store, one.ID)
	if preserved.Snapshot != one || preserved.Attempt != 0 {
		t.Fatal("takeover reset original snapshot")
	}
	if changed, err := other.Release(jobContext, takeover[0].Lease, f.now.Add(time.Minute)); err != nil || !changed {
		t.Fatal("new lease could not release", err)
	}
	if changed, err := other.Release(jobContext, takeover[0].Lease, f.now.Add(time.Minute)); err != nil || changed {
		t.Fatal("duplicate release")
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE state='pending'`) != 0 {
		t.Fatal("full job became legacy")
	}
	// 不把内部句柄当 token 或普通回执；数据库仅有随机秘密摘要。
	var digest []byte
	if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT lease_hash FROM push_jobs WHERE id=?`, three.ID).Scan(&digest)
	}); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(second[0].Lease.secret[:])
	if !reflect.DeepEqual(digest, want[:]) || reflect.DeepEqual(digest, second[0].Lease.secret[:]) {
		t.Fatal("lease raw secret persisted")
	}
	raw, _ := json.Marshal(second[0])
	formatted := fmt.Sprintf("%v %+v %#v", second[0], second[0], second[0])
	secretHex := hex.EncodeToString(second[0].Lease.secret[:])
	if strings.Contains(string(raw), secretHex) || strings.Contains(formatted, secretHex) || strings.Contains(string(raw), "synthetic-registration-secret") || strings.Contains(formatted, "private-job-fixture") {
		t.Fatal("secret or content leaked")
	}
}

func TestJobTwelveConcurrentStoresOneDevice(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	jobCreate(t, f, j)
	start := make(chan struct{})
	type outcome struct {
		jobs []JobReservation
		err  error
	}
	results := make(chan outcome, 12)
	var wg stdsync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			store := f.store
			if i%2 == 1 {
				store = f.other
			}
			jobs, err := NewJobRepository(store).Reserve(jobContext, f.now, time.Minute, 1)
			results <- outcome{jobs, err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	n := 0
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		n += len(got.jobs)
	}
	if n != 1 || jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 1 {
		t.Fatal("duplicate concurrent device reservation", n)
	}
}

func TestJobDueExpiryAndSessionFiltering(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	jobCreate(t, f, j)
	r := NewJobRepository(f.store)
	jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.Add(time.Nanosecond).Format(jobTimestamp), j.ID)
	if got, err := r.Reserve(jobContext, f.now, time.Minute, 1); err != nil || len(got) != 0 {
		t.Fatal("future reservation", err)
	}
	exact, err := r.Reserve(jobContext, f.now.Add(time.Nanosecond), time.Minute, 1)
	if err != nil || len(exact) != 1 {
		t.Fatal("nanosecond due equality", err)
	}
	if changed, err := r.Release(jobContext, exact[0].Lease, f.now.Add(time.Nanosecond)); err != nil || !changed {
		t.Fatal(err)
	}
	// 预留不是提醒资格；过期仍必须拒绝，不能因重新预留重置原 expiry。
	if got, err := r.Reserve(jobContext, j.ExpiresAt, time.Minute, 1); err != nil || len(got) != 0 {
		t.Fatal("expired event revived", err)
	}
	jobSQL(t, f.store, `UPDATE sessions SET revoked=1 WHERE id=?`, s.ID)
	if got, err := r.Reserve(jobContext, f.now.Add(time.Second), time.Minute, 1); err != nil || len(got) != 0 {
		t.Fatal("revoked session reserved", err)
	}
	if jobRead(t, f.store, j.ID).Snapshot != j {
		t.Fatal("filter changed original qualification")
	}
}

func TestJobBatchFaultRollsBackEvenIfCallerCommits(t *testing.T) {
	f := jobSetup(t)
	a := f.session(t, jobDeviceA)
	b := f.session(t, jobDeviceB)
	one := f.snapshot(t, a, "sync")
	two := f.snapshot(t, b, "alert")
	jobCreate(t, f, one)
	jobCreate(t, f, two)
	jobSQL(t, f.store, `CREATE TRIGGER job_batch_fault BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-002' BEGIN SELECT RAISE(ABORT,'batch fault'); END`)
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		got, e := ReserveJobsTx(jobContext, tx, f.now, time.Minute, 2)
		if e == nil || got != nil {
			t.Fatal("batch partial output")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 0 || jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='queued'`) != 2 {
		t.Fatal("half batch persisted")
	}
	jobSQL(t, f.store, `DROP TRIGGER job_batch_fault`)
	// 损坏第二个候选的时间必须让整批回滚，而不是发布第一个句柄。
	jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id='job-001'`, f.now.Add(-time.Second).Format(jobTimestamp))
	jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON; UPDATE push_jobs SET next_attempt_at=? WHERE id='job-002'; PRAGMA ignore_check_constraints=OFF`, strings.TrimSuffix(f.now.Format(jobTimestamp), "Z")+"Y")
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		got, e := ReserveJobsTx(jobContext, tx, f.now, time.Minute, 2)
		if e == nil || got != nil {
			t.Fatal("corrupt candidate partial output")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 0 {
		t.Fatal("corrupt batch left lease")
	}
}

func TestJobCommitAndSQLFailuresDoNotPublish(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	jobCreate(t, f, j)
	r := NewJobRepository(f.store)
	jobSQL(t, f.store, `CREATE TABLE job_commit_fault(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER job_commit_reserve AFTER UPDATE ON push_jobs WHEN NEW.state='leased' BEGIN INSERT INTO job_commit_fault VALUES('absent'); END`)
	if got, err := r.Reserve(jobContext, f.now, time.Minute, 1); err == nil || got != nil {
		t.Fatal("COMMIT failure published lease")
	}
	if jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 0 || jobCount(t, f.other, `SELECT count(*) FROM job_commit_fault`) != 0 {
		t.Fatal("COMMIT failure persisted partial reserve")
	}
	jobSQL(t, f.store, `DROP TRIGGER job_commit_reserve`)
	got, err := r.Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(got) != 1 {
		t.Fatal(err)
	}
	jobSQL(t, f.store, `CREATE TRIGGER job_commit_release AFTER UPDATE ON push_jobs WHEN NEW.state='queued' BEGIN INSERT INTO job_commit_fault VALUES('absent'); END`)
	if changed, err := r.Release(jobContext, got[0].Lease, f.now); err == nil || changed {
		t.Fatal("COMMIT failure published release")
	}
	if jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 1 {
		t.Fatal("failed release persisted")
	}
	jobSQL(t, f.store, `DROP TRIGGER job_commit_release`)
	rollback := errors.New("rollback SQL fixture")
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`DROP TABLE push_jobs`); e != nil {
			return e
		}
		out, e := ReserveJobsTx(jobContext, tx, f.now, time.Minute, 1)
		if e == nil || out != nil {
			t.Fatal("SQL failure published")
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if jobCount(t, f.other, `SELECT count(*) FROM push_jobs`) != 1 {
		t.Fatal("SQL fixture not rolled back")
	}
}

func TestJobCancellationKeepsDatabaseAndNoReceipt(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	jobCreate(t, f, j)
	r := NewJobRepository(f.other)
	canceled, cancel := context.WithCancel(jobContext)
	cancel()
	if got, err := r.Reserve(canceled, f.now, time.Minute, 1); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("canceled reserve", err)
	}
	// 两 Store 共享写门；保留原 10ms 截止，取消不进入业务事务。
	if err := f.store.Write(jobContext, func(*sql.Tx) error {
		c, cancel := context.WithTimeout(jobContext, 10*time.Millisecond)
		defer cancel()
		got, e := r.Reserve(c, f.now, time.Minute, 1)
		if !errors.Is(e, context.DeadlineExceeded) || got != nil {
			return fmt.Errorf("blocked reservation: %v", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE state='queued'`) != 1 {
		t.Fatal("cancel persisted lease")
	}
	// 已产生事务内候选再取消，外层提交失败后不得将候选当作发布结果。
	var candidate []JobReservation
	c, cancel := context.WithCancel(jobContext)
	err := f.store.Write(c, func(tx *sql.Tx) error {
		var e error
		candidate, e = ReserveJobsTx(c, tx, f.now, time.Minute, 1)
		if e != nil {
			return e
		}
		cancel()
		return nil
	})
	if err == nil || len(candidate) != 1 || jobCount(t, f.other, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 0 {
		t.Fatal("canceled COMMIT not atomic")
	}
}

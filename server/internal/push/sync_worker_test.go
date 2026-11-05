package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type workerDispatch func(context.Context, SyncEnvelope) (SyncOutcome, error)

func (f workerDispatch) Dispatch(ctx context.Context, e SyncEnvelope) (SyncOutcome, error) {
	return f(ctx, e)
}
func workerFixture(t *testing.T) (*jobFixture, JobSnapshot) {
	t.Helper()
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	j.ExpiresAt = j.CreatedAt.Add(24 * time.Hour)
	jobCreate(t, f, j)
	return f, j
}
func workerClock(f *jobFixture, hook func(int)) func() time.Time {
	n := 0
	return func() time.Time {
		n++
		if hook != nil {
			hook(n)
		}
		return f.now.Add(time.Duration(n) * time.Nanosecond)
	}
}
func workerResultKind(t *testing.T, f *jobFixture, id string) string {
	t.Helper()
	var v string
	if e := f.store.Read(jobContext, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT COALESCE(result_kind,'') FROM push_jobs WHERE id=?`, id).Scan(&v)
	}); e != nil {
		t.Fatal(e)
	}
	return v
}

func TestSyncWorkerOutcomesAndEnvelope(t *testing.T) {
	for _, o := range []SyncOutcome{SyncAccepted, SyncTargetInvalid, SyncTransient, SyncRateLimited, SyncConfigurationError, SyncUnknownDelivery, "invalid", ""} {
		for _, adapterErr := range []bool{false, true} {
			t.Run(fmt.Sprint(o, "/error=", adapterErr), func(t *testing.T) {
				f, j := workerFixture(t)
				calls := 0
				w := NewSyncWorker(NewJobRepository(f.store), workerClock(f, nil), time.Minute, workerDispatch(func(ctx context.Context, e SyncEnvelope) (SyncOutcome, error) {
					calls++
					if e != (SyncEnvelope{JobKey: j.ID, Purpose: "sync", Provider: "fcm", TargetChangeSeq: j.TargetChangeSeq, ExpiresAt: j.ExpiresAt}) {
						t.Fatal("envelope")
					}
					// 用严格字段集合排除将来意外携带完整 snapshot、租约或业务对象。
					typ := reflect.TypeOf(e)
					want := []string{"JobKey", "Purpose", "Provider", "TargetChangeSeq", "ExpiresAt"}
					if typ.NumField() != len(want) {
						t.Fatal("envelope field count")
					}
					for i, name := range want {
						if typ.Field(i).Name != name {
							t.Fatal("unexpected field")
						}
					}
					raw, _ := json.Marshal(e)
					printed := fmt.Sprintf("%v %+v %#v", e, e, e)
					for _, secret := range []string{"synthetic-registration-secret", "private-job-fixture", j.SessionID, j.DeviceID, j.MessageID, j.DeploymentID, j.BotID, "leaseHash", "leaseUntil", "startedAt"} {
						if strings.Contains(string(raw), secret) || strings.Contains(printed, secret) {
							t.Fatal("envelope secret")
						}
					}
					if jobRead(t, f.other, j.ID).Attempt != 1 {
						t.Fatal("Begin not committed")
					}
					if adapterErr {
						return o, errors.New("synthetic-registration-secret private-job-fixture https://secret.invalid")
					}
					return o, nil
				}))
				got, e := w.Step(jobContext)
				if e != nil || got != (SyncStepResult{true, true}) || calls != 1 {
					t.Fatal(got, e, calls)
				}
				expected := o
				if adapterErr || !o.valid() {
					expected = SyncUnknownDelivery
				}
				if workerResultKind(t, f, j.ID) != string(expected) {
					t.Fatal("wrong persisted result")
				}
				after := jobRead(t, f.store, j.ID)
				if after.Snapshot != j || after.Attempt != 1 {
					t.Fatal("snapshot/attempt")
				}
				state := "degraded"
				if expected == SyncAccepted {
					state = "accepted"
				}
				if expected == SyncTransient || expected == SyncRateLimited || expected == SyncUnknownDelivery {
					state = "queued"
					if !after.NextAttemptAt.Equal(f.now.Add(4*time.Nanosecond + 10*time.Second)) {
						t.Fatal("new result clock/retry")
					}
				}
				if after.State != state {
					t.Fatal(after.State, state)
				}
				if got, e = w.Step(jobContext); e != nil || got != (SyncStepResult{}) || calls != 1 {
					t.Fatal("immediate redispatch", got, e, calls)
				}
			})
		}
	}
}

// 延迟外键保证 UPDATE 已成功且故障发生于真实 COMMIT；控制事务先证明 trigger 命中。
func workerCommitFault(t *testing.T, f *jobFixture, j JobSnapshot, column string) {
	t.Helper()
	jobSQL(t, f.store, `CREATE TABLE worker_fault_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);
 CREATE TRIGGER worker_fault AFTER UPDATE OF `+column+` ON push_jobs BEGIN INSERT INTO worker_fault_fk VALUES('missing-worker-bot'); END`)
	before := leaseDigest(t, f.store)
	reached := false
	err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		// 同值列更新仍触发，不破坏原 schema 约束。
		if _, e := tx.Exec(`UPDATE push_jobs SET `+column+`=`+column+` WHERE id=?`, j.ID); e != nil {
			return e
		}
		var n int
		if e := tx.QueryRow(`SELECT count(*) FROM worker_fault_fk`).Scan(&n); e != nil {
			return e
		}
		reached = n == 1
		return nil
	})
	var se *sqlite.Error
	if !reached || !errors.As(err, &se) || se.Code() != sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
		t.Fatalf("fault did not reach COMMIT: reached=%v err=%v", reached, err)
	}
	if before != leaseDigest(t, f.store) {
		t.Fatal("control COMMIT partially wrote")
	}
}
func TestSyncWorkerCommitFailures(t *testing.T) {
	for _, stage := range []string{"reserve", "begin", "apply"} {
		t.Run(stage, func(t *testing.T) {
			f, j := workerFixture(t)
			column := map[string]string{"reserve": "lease_hash", "begin": "attempt", "apply": "result_kind"}[stage]
			if stage != "apply" {
				workerCommitFault(t, f, j, column)
			}
			calls := 0
			w := NewSyncWorker(NewJobRepository(f.store), workerClock(f, nil), time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) {
				calls++
				if stage == "apply" {
					workerCommitFault(t, f, j, column)
				}
				return SyncAccepted, nil
			}))
			got, e := w.Step(jobContext)
			var se *sqlite.Error
			if !errors.As(e, &se) || se.Code() != sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY || got.Applied {
				t.Fatal("COMMIT failure", got, e)
			}
			wantCalls := 0
			wantAttempt := int64(0)
			wantState := "queued"
			if stage == "begin" {
				wantState = "leased"
			}
			if stage == "apply" {
				wantCalls = 1
				wantAttempt = 1
				wantState = "leased"
			}
			after := jobRead(t, f.store, j.ID)
			if calls != wantCalls || got.Dispatched != (wantCalls == 1) || after.Attempt != wantAttempt || after.State != wantState || workerResultKind(t, f, j.ID) != "" {
				t.Fatal("partial COMMIT", calls, after)
			}
			if stage != "reserve" {
				again, e := w.Step(jobContext)
				if e != nil || again != (SyncStepResult{}) || calls != wantCalls {
					t.Fatal("failure redispatched", again, e)
				}
			}
		})
	}
}

func TestSyncWorkerCheckCurrentness(t *testing.T) {
	for _, kind := range []string{"registration", "token", "revoked", "settings", "instance", "lease-expired", "job-expired", "takeover", "check-error"} {
		t.Run(kind, func(t *testing.T) {
			f, j := workerFixture(t)
			calls := 0
			stage := 0
			clock := func() time.Time {
				stage++
				now := f.now.Add(time.Duration(stage) * time.Nanosecond)
				if stage == 3 {
					switch kind {
					case "registration":
						jobSQL(t, f.other, `UPDATE device_push_registrations SET registration_version=2`)
					case "token":
						jobSQL(t, f.other, `UPDATE device_push_registrations SET token=NULL`)
					case "revoked":
						jobSQL(t, f.other, `UPDATE sessions SET revoked=1`)
					case "settings":
						jobSQL(t, f.other, `INSERT INTO device_notification_settings VALUES(?,1,'now','{}')`, jobDeviceA)
					case "instance":
						jobSQL(t, f.other, `UPDATE instance_identity SET instance_id='33333333-3333-4333-8333-333333333333'`)
					case "lease-expired":
						now = f.now.Add(time.Minute + time.Nanosecond)
					case "job-expired":
						now = j.ExpiresAt
					case "takeover":
						now = f.now.Add(time.Minute + time.Nanosecond)
						held, e := NewJobRepository(f.other).ReserveSync(jobContext, now, time.Minute, 1)
						if e != nil || len(held) != 1 {
							t.Fatal("takeover", e)
						}
					case "check-error":
						jobSQL(t, f.other, `PRAGMA ignore_check_constraints=ON;UPDATE push_jobs SET attempt_lease_hash=CAST(attempt_lease_hash AS TEXT);PRAGMA ignore_check_constraints=OFF`)
					}
				}
				return now
			}
			got, e := NewSyncWorker(NewJobRepository(f.store), clock, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) { calls++; return SyncAccepted, nil })).Step(jobContext)
			wantErr := kind == "instance" || kind == "check-error"
			if calls != 0 || got != (SyncStepResult{}) || (e != nil) != wantErr || stage != 3 {
				t.Fatal("check did not stop dispatch", got, e, calls, stage)
			}
			if kind != "instance" && jobRead(t, f.other, j.ID).Attempt != 1 {
				t.Fatal("attempt reset")
			}
		})
	}
}

func TestSyncWorkerBeginNilAndFailure(t *testing.T) {
	for _, kind := range []string{"rotation", "expiry", "exhausted", "error"} {
		t.Run(kind, func(t *testing.T) {
			f, j := workerFixture(t)
			calls := 0
			if kind == "exhausted" {
				jobSQL(t, f.store, `UPDATE push_jobs SET attempt=8 WHERE id=?`, j.ID)
			}
			clock := workerClock(f, func(n int) {
				if n == 2 {
					switch kind {
					case "rotation":
						jobSQL(t, f.other, `UPDATE device_push_registrations SET registration_version=2`)
					case "error":
						jobSQL(t, f.other, `CREATE TRIGGER worker_abort BEFORE UPDATE OF attempt ON push_jobs BEGIN SELECT RAISE(ABORT,'worker begin error');END`)
					}
				}
			})
			if kind == "expiry" {
				n := 0
				clock = func() time.Time {
					n++
					if n > 1 {
						return j.ExpiresAt
					}
					return f.now
				}
			}
			got, e := NewSyncWorker(NewJobRepository(f.store), clock, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) { calls++; return SyncAccepted, nil })).Step(jobContext)
			if got != (SyncStepResult{}) || calls != 0 || (e != nil) != (kind == "error") {
				t.Fatal(got, e, calls)
			}
			if kind == "exhausted" {
				after := jobRead(t, f.store, j.ID)
				if after.Attempt != 8 || after.State != "degraded" {
					t.Fatal(after)
				}
			}
		})
	}
}

func TestSyncWorkerBlockingDispatchAndLateFencing(t *testing.T) {
	for _, kind := range []string{"rotation", "takeover", "expiry", "unchanged"} {
		t.Run(kind, func(t *testing.T) {
			f, j := workerFixture(t)
			jobSQL(t, f.store, `CREATE TABLE worker_probe(id INTEGER)`)
			entered := make(chan struct{})
			finish := make(chan struct{})
			defer func() {
				select {
				case <-finish:
				default:
					close(finish)
				}
			}()
			done := make(chan struct {
				r SyncStepResult
				e error
			}, 1)
			now := f.now
			w := NewSyncWorker(NewJobRepository(f.store), func() time.Time { return now }, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) {
				close(entered)
				<-finish
				return SyncAccepted, nil
			}))
			go func() {
				r, e := w.Step(jobContext)
				done <- struct {
					r SyncStepResult
					e error
				}{r, e}
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("adapter not entered")
			}
			// 跨 Store 的真实写入完成才允许 adapter 返回，排除跨调用的写事务。
			jobSQL(t, f.other, `INSERT INTO worker_probe VALUES(1)`)
			// WAL 写入可与读事务共存；额外要求 checkpoint TRUNCATE 成功，排除旧读快照。
			probe, probeErr := sql.Open("sqlite", f.cfg.Path)
			if probeErr != nil {
				t.Fatal(probeErr)
			}
			defer probe.Close()
			var busy, frames, checkpointed int
			if probeErr = probe.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &frames, &checkpointed); probeErr != nil || busy != 0 || frames != 0 || checkpointed != 0 {
				t.Fatal("adapter retained read transaction", busy, frames, checkpointed, probeErr)
			}
			switch kind {
			case "rotation":
				jobSQL(t, f.other, `UPDATE device_push_registrations SET registration_version=2`)
			case "takeover":
				now = f.now.Add(time.Minute)
				held, e := NewJobRepository(f.other).ReserveSync(jobContext, now, time.Minute, 1)
				if e != nil || len(held) != 1 {
					t.Fatal(e)
				}
				mustBeginSync(t, NewJobRepository(f.other), held[0].Lease, now)
			case "expiry":
				now = j.ExpiresAt
			}
			close(finish)
			select {
			case v := <-done:
				if v.e != nil || !v.r.Dispatched || v.r.Applied != (kind == "unchanged") {
					t.Fatal(v)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Step stuck")
			}
			if jobCount(t, f.other, `SELECT count(*) FROM worker_probe`) != 1 {
				t.Fatal("other Store write")
			}
			if kind != "unchanged" && workerResultKind(t, f, j.ID) != "" {
				t.Fatal("late result applied")
			}
		})
	}
}

func TestSyncWorkerCancellation(t *testing.T) {
	for _, phase := range []string{"before", "before-check", "adapter", "ignore-cancel"} {
		t.Run(phase, func(t *testing.T) {
			f, j := workerFixture(t)
			ctx, cancel := context.WithCancel(jobContext)
			defer cancel()
			calls := 0
			if phase == "before" {
				cancel()
			}
			clock := workerClock(f, func(n int) {
				if n == 3 && phase == "before-check" {
					cancel()
				}
			})
			w := NewSyncWorker(NewJobRepository(f.store), clock, time.Minute, workerDispatch(func(ctx context.Context, _ SyncEnvelope) (SyncOutcome, error) {
				calls++
				cancel()
				if phase == "ignore-cancel" {
					return SyncAccepted, nil
				}
				return "", ctx.Err()
			}))
			got, e := w.Step(ctx)
			if !errors.Is(e, context.Canceled) || got.Applied {
				t.Fatal(got, e)
			}
			expected := 0
			if phase == "adapter" || phase == "ignore-cancel" {
				expected = 1
			}
			if calls != expected || got.Dispatched != (expected == 1) || workerResultKind(t, f, j.ID) != "" {
				t.Fatal("cancel wrote result", got, calls)
			}
			if phase != "before" {
				got, e = w.Step(jobContext)
				if e != nil || got != (SyncStepResult{}) || calls != expected {
					t.Fatal("cancel released or redispatched", got, e)
				}
			}
		})
	}
}

func TestSyncWorkerClockTTLAndInput(t *testing.T) {
	for _, phase := range []int{1, 2, 3, 4} {
		for _, bad := range []string{"zero", "backward", "overflow"} {
			t.Run(fmt.Sprint(phase, "/", bad), func(t *testing.T) {
				f, _ := workerFixture(t)
				calls := 0
				n := 0
				clock := func() time.Time {
					n++
					if n == phase {
						switch bad {
						case "zero":
							return time.Time{}
						case "backward":
							return f.now.Add(-time.Second)
						case "overflow":
							return time.Unix(0, math.MaxInt64).Add(time.Nanosecond)
						}
					}
					return f.now
				}
				got, e := NewSyncWorker(NewJobRepository(f.store), clock, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) { calls++; return SyncAccepted, nil })).Step(jobContext)
				// 首个时刻没有前值可比较；仍早于 job 创建，因此不能取得候选。
				if phase == 1 && bad == "backward" {
					if e != nil || calls != 0 || got != (SyncStepResult{}) {
						t.Fatal(got, e)
					}
					return
				}
				expected := 0
				if phase == 4 {
					expected = 1
				}
				if !errors.Is(e, ErrJobInput) || calls != expected || got.Applied {
					t.Fatal(got, e, calls)
				}
			})
		}
	}
	for _, ttl := range []time.Duration{0, -1, time.Duration(math.MaxInt64)} {
		t.Run(fmt.Sprint("ttl/", ttl), func(t *testing.T) {
			f, _ := workerFixture(t)
			before := leaseDigest(t, f.store)
			calls := 0
			got, e := NewSyncWorker(NewJobRepository(f.store), func() time.Time { return f.now }, ttl, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) { calls++; return SyncAccepted, nil })).Step(jobContext)
			if !errors.Is(e, ErrJobInput) || got != (SyncStepResult{}) || calls != 0 || before != leaseDigest(t, f.store) {
				t.Fatal(got, e, calls)
			}
		})
	}
	var w *SyncWorker
	if _, e := w.Step(jobContext); !errors.Is(e, ErrJobInput) {
		t.Fatal(e)
	}
	f, _ := workerFixture(t)
	w = NewSyncWorker(NewJobRepository(f.store), func() time.Time { return f.now }, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) {
		t.Fatal("invalid input dispatch")
		return "", nil
	}))
	if _, e := w.Step(nil); !errors.Is(e, ErrJobInput) {
		t.Fatal(e)
	}
}

func TestSyncWorkerEightAttemptsOriginalExpiryAndMixed(t *testing.T) {
	f, j := workerFixture(t)
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('worker-legacy',?,?,1,'legacy-original')`, j.SessionID, j.DeviceID)
	alert := j
	alert.ID = "000-alert"
	alert.Purpose = "alert"
	alert.AlertEligible = true
	jobCreate(t, f, alert)
	now := f.now
	calls := 0
	w := NewSyncWorker(NewJobRepository(f.store), func() time.Time { return now }, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) { calls++; return SyncUnknownDelivery, nil }))
	for n := int64(1); n <= 8; n++ {
		got, e := w.Step(jobContext)
		if e != nil || got != (SyncStepResult{true, true}) {
			t.Fatal(n, got, e)
		}
		after := jobRead(t, f.store, j.ID)
		if after.Attempt != n || after.Snapshot != j {
			t.Fatal("budget/expiry reset")
		}
		if n < 8 {
			if !after.NextAttemptAt.After(now) || !after.NextAttemptAt.Before(j.ExpiresAt) {
				t.Fatal("retry bounds")
			}
			now = after.NextAttemptAt
		} else if after.State != "degraded" {
			t.Fatal("not exhausted")
		}
	}
	if got, e := w.Step(jobContext); e != nil || got != (SyncStepResult{}) || calls != 8 {
		t.Fatal("ninth", got, e, calls)
	}
	if jobRead(t, f.store, alert.ID) != (Job{Snapshot: alert, State: "queued", NextAttemptAt: alert.CreatedAt}) {
		t.Fatal("alert changed")
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE id='worker-legacy' AND state='pending' AND attempt=0 AND next_attempt_at='legacy-original' AND purpose IS NULL AND result_kind IS NULL`) != 1 {
		t.Fatal("legacy changed")
	}
}

func TestSyncWorkerExpiryDuringDispatchAndPanic(t *testing.T) {
	for _, kind := range []string{"expiry", "panic"} {
		t.Run(kind, func(t *testing.T) {
			f, j := workerFixture(t)
			now := f.now
			calls := 0
			w := NewSyncWorker(NewJobRepository(f.store), func() time.Time { return now }, time.Minute, workerDispatch(func(context.Context, SyncEnvelope) (SyncOutcome, error) {
				calls++
				if kind == "panic" {
					panic("synthetic adapter panic")
				}
				now = j.ExpiresAt
				return SyncAccepted, nil
			}))
			if kind == "panic" {
				func() {
					defer func() {
						if recover() == nil {
							t.Fatal("panic swallowed")
						}
					}()
					_, _ = w.Step(jobContext)
				}()
			} else {
				got, e := w.Step(jobContext)
				if e != nil || got != (SyncStepResult{true, false}) {
					t.Fatal(got, e)
				}
			}
			if calls != 1 || workerResultKind(t, f, j.ID) != "" {
				t.Fatal("fabricated result")
			}
			if got, e := w.Step(jobContext); e != nil || got != (SyncStepResult{}) || calls != 1 {
				t.Fatal("panic/expiry retransmitted", got, e)
			}
		})
	}
}

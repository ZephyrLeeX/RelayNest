package push

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

func attemptCheckExpect(t *testing.T, r *JobRepository, lease JobLease, ticket SyncAttempt, now time.Time, want bool, wantErr bool) {
	t.Helper()
	before := leaseDigest(t, r.store)
	got, err := r.CheckSyncAttempt(jobContext, lease, ticket, now)
	if got != want || (err != nil) != wantErr {
		t.Fatalf("check got=%v error=%v want=%v error=%v", got, err, want, wantErr)
	}
	if before != leaseDigest(t, r.store) {
		t.Fatal("check wrote database")
	}
}

func TestSyncAttemptCheckBindingAndReopen(t *testing.T) {
	f, held := leaseFixture(t)
	r := NewJobRepository(f.store)
	ticket := *mustBeginSync(t, r, held.Lease, f.now)
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, s := range []*storage.Store{f.store, f.other, reopened} {
		attemptCheckExpect(t, NewJobRepository(s), held.Lease, ticket, f.now.Add(time.Nanosecond), true, false)
		// 只读复核不提供一次性消费语义；重复检查当前票据仍有效。
		attemptCheckExpect(t, NewJobRepository(s), held.Lease, ticket, f.now, true, false)
	}
	// 逐字段变异完整原 snapshot，防止遗漏新字段。
	snapshot := reflect.ValueOf(&ticket.snapshot).Elem()
	for i := 0; i < snapshot.NumField(); i++ {
		t.Run("snapshot-"+snapshot.Type().Field(i).Name, func(t *testing.T) {
			bad := ticket
			field := reflect.ValueOf(&bad.snapshot).Elem().Field(i)
			switch field.Kind() {
			case reflect.String:
				field.SetString(field.String() + "x")
			case reflect.Int64:
				field.SetInt(field.Int() + 1)
			case reflect.Bool:
				field.SetBool(!field.Bool())
			case reflect.Struct:
				field.Set(reflect.ValueOf(field.Interface().(time.Time).Add(time.Nanosecond)))
			}
			before := leaseDigest(t, f.store)
			if ok, _ := r.CheckSyncAttempt(jobContext, held.Lease, bad, f.now); ok {
				t.Fatal("tampered snapshot accepted")
			}
			if before != leaseDigest(t, f.store) {
				t.Fatal("tamper wrote")
			}
		})
	}
	for _, change := range []func(*SyncAttempt){func(v *SyncAttempt) { v.attempt++ }, func(v *SyncAttempt) { v.leaseHash[0] ^= 1 }, func(v *SyncAttempt) { v.leaseUntil = v.leaseUntil.Add(time.Nanosecond) }, func(v *SyncAttempt) { v.startedAt = v.startedAt.Add(time.Nanosecond) }} {
		bad := ticket
		change(&bad)
		attemptCheckExpect(t, r, held.Lease, bad, f.now.Add(time.Second), false, false)
	}
	for _, change := range []func(*JobLease){func(v *JobLease) { v.jobID = "absent" }, func(v *JobLease) { v.secret[0] ^= 1 }, func(v *JobLease) { v.until = v.until.Add(time.Nanosecond) }} {
		bad := held.Lease
		change(&bad)
		attemptCheckExpect(t, r, bad, ticket, f.now, false, false)
	}
	attemptCheckExpect(t, r, held.Lease, ticket, f.now.Add(-time.Nanosecond), false, false)
	attemptCheckExpect(t, r, held.Lease, ticket, held.Lease.until, false, false)
	if ok, e := r.ApplySyncResult(jobContext, ticket, SyncAccepted, f.now); e != nil || !ok {
		t.Fatal(e)
	}
	attemptCheckExpect(t, r, held.Lease, ticket, f.now, false, false)
}

func TestSyncAttemptCheckCurrentState(t *testing.T) {
	for _, q := range []string{
		`UPDATE device_push_registrations SET registration_version=2`, `UPDATE device_push_registrations SET token=NULL`,
		`DELETE FROM device_push_registrations`, `UPDATE sessions SET revoked=1`, `UPDATE sessions SET auth_generation=2`,
		`UPDATE administrator SET auth_generation=2`, `UPDATE sessions SET client_type='web'`,
		`UPDATE sessions SET idle_expires_at=1`, `UPDATE sessions SET idle_expires_at=1,absolute_expires_at=1`,
		`INSERT INTO device_notification_settings VALUES('11111111-1111-4111-8111-111111111111',1,'now','{}')`,
	} {
		t.Run(q, func(t *testing.T) {
			f, h := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := *mustBeginSync(t, r, h.Lease, f.now)
			jobSQL(t, f.store, q)
			attemptCheckExpect(t, NewJobRepository(f.other), h.Lease, ticket, f.now, false, false)
		})
	}
	for _, release := range []bool{false, true} {
		t.Run(fmt.Sprint("takeover-", release), func(t *testing.T) {
			f, h := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := *mustBeginSync(t, r, h.Lease, f.now)
			now := h.Lease.until
			if release {
				now = f.now
				if ok, e := r.Release(jobContext, h.Lease, now); e != nil || !ok {
					t.Fatal(e)
				}
			}
			attemptCheckExpect(t, r, h.Lease, ticket, now, false, false)
			next, e := NewJobRepository(f.other).Reserve(jobContext, now, time.Minute, 1)
			if e != nil || len(next) != 1 {
				t.Fatal(e)
			}
			fresh := *mustBeginSync(t, r, next[0].Lease, now)
			attemptCheckExpect(t, r, h.Lease, ticket, now, false, false)
			attemptCheckExpect(t, r, next[0].Lease, ticket, now, false, false)
			attemptCheckExpect(t, r, next[0].Lease, fresh, now, true, false)
		})
	}
	t.Run("instance", func(t *testing.T) {
		f, h := leaseFixture(t)
		r := NewJobRepository(f.store)
		ticket := *mustBeginSync(t, r, h.Lease, f.now)
		jobSQL(t, f.store, `UPDATE instance_identity SET instance_id='33333333-3333-4333-8333-333333333333'`)
		if ok, e := r.CheckSyncAttempt(jobContext, h.Lease, ticket, f.now); ok || e == nil {
			t.Fatal("old Store", e)
		}
		s, e := storage.Open(jobContext, f.cfg)
		if e != nil {
			t.Fatal(e)
		}
		defer s.Close()
		attemptCheckExpect(t, NewJobRepository(s), h.Lease, ticket, f.now, false, false)
	})
}

func TestSyncAttemptCheckCorruptionAndErrors(t *testing.T) {
	for _, q := range []string{
		`UPDATE push_jobs SET attempt_lease_hash=CAST(attempt_lease_hash AS TEXT)`,
		`UPDATE push_jobs SET attempt_lease_until_ns=CAST(attempt_lease_until_ns AS BLOB)`,
		`UPDATE push_jobs SET attempt_started_at_ns=CAST(attempt_started_at_ns AS BLOB)`,
		`UPDATE push_jobs SET attempt_started_at_ns=NULL`,
		`UPDATE push_jobs SET result_kind='accepted',result_at_ns=attempt_started_at_ns`,
		`UPDATE push_jobs SET result_kind=CAST('accepted' AS BLOB),result_at_ns=attempt_started_at_ns`,
		`UPDATE push_jobs SET result_at_ns=attempt_started_at_ns`,
		`UPDATE push_jobs SET attempt=CAST(attempt AS BLOB)`,
		`UPDATE push_jobs SET lease_hash=CAST(lease_hash AS TEXT)`,
		`UPDATE push_jobs SET lease_until_ns=CAST(lease_until_ns AS BLOB)`,
	} {
		t.Run(q, func(t *testing.T) {
			f, h := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := *mustBeginSync(t, r, h.Lease, f.now)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
			jobSQL(t, f.store, q)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
			before := leaseDigest(t, f.store)
			ok, e := r.CheckSyncAttempt(jobContext, h.Lease, ticket, f.now)
			if ok {
				t.Fatal("corruption accepted")
			}
			if q != `UPDATE push_jobs SET result_kind='accepted',result_at_ns=attempt_started_at_ns` && e == nil {
				t.Fatal("missing corruption error")
			}
			if before != leaseDigest(t, f.store) {
				t.Fatal("corruption changed")
			}
		})
	}
	f, h := leaseFixture(t)
	r := NewJobRepository(f.store)
	ticket := *mustBeginSync(t, r, h.Lease, f.now)
	ctx, cancel := context.WithCancel(jobContext)
	cancel()
	if ok, e := r.CheckSyncAttempt(ctx, h.Lease, ticket, f.now); ok || e == nil {
		t.Fatal("canceled", e)
	}
	if ok, e := r.CheckSyncAttempt(nil, h.Lease, ticket, f.now); ok || e == nil {
		t.Fatal("nil context", e)
	}
	attemptCheckExpect(t, r, h.Lease, SyncAttempt{}, f.now, false, true)
	jobSQL(t, f.store, `DROP TABLE push_jobs`)
	if ok, e := r.CheckSyncAttempt(jobContext, h.Lease, ticket, f.now); ok || e == nil {
		t.Fatal("SQL failure", e)
	}
}

// 在回调暂存通过后的最后 Err 调用处取消真实事务上下文；本次仍返回
// nil，使 Store 必须处理事务结束错误，不能凭回调暂存值返回 true。
type attemptCheckEndContext struct {
	context.Context
	cancel  context.CancelFunc
	once    sync.Once
	reached bool
}

func (c *attemptCheckEndContext) Err() error {
	pc, _, _, _ := runtime.Caller(1)
	if fn := runtime.FuncForPC(pc); fn != nil && strings.HasSuffix(fn.Name(), ".(*JobRepository).CheckSyncAttempt.func1") {
		c.once.Do(func() { c.reached = true; c.cancel() })
		return nil
	}
	return c.Context.Err()
}
func TestSyncAttemptCheckTransactionEndFailure(t *testing.T) {
	f, h := leaseFixture(t)
	r := NewJobRepository(f.store)
	ticket := *mustBeginSync(t, r, h.Lease, f.now)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &attemptCheckEndContext{Context: base, cancel: cancel}
	before := leaseDigest(t, f.store)
	ok, err := r.CheckSyncAttempt(ctx, h.Lease, ticket, f.now)
	if !ctx.reached || ok || err == nil {
		t.Fatal("transaction end did not fail closed", ctx.reached, ok, err)
	}
	if before != leaseDigest(t, f.store) {
		t.Fatal("failed transaction wrote")
	}
	attemptCheckExpect(t, r, h.Lease, ticket, f.now, true, false)
}

func TestSyncAttemptCheckOriginalExpiryAndPersistedBinding(t *testing.T) {
	t.Run("original-expiry", func(t *testing.T) {
		f := jobSetup(t)
		s := f.session(t, jobDeviceA)
		j := f.snapshot(t, s, "sync")
		j.ExpiresAt = f.now.Add(2 * time.Second)
		jobCreate(t, f, j)
		r := NewJobRepository(f.store)
		held, e := r.Reserve(jobContext, f.now, time.Minute, 1)
		if e != nil || len(held) != 1 {
			t.Fatal(e)
		}
		ticket := *mustBeginSync(t, r, held[0].Lease, f.now)
		attemptCheckExpect(t, r, held[0].Lease, ticket, j.ExpiresAt.Add(-time.Nanosecond), true, false)
		attemptCheckExpect(t, r, held[0].Lease, ticket, j.ExpiresAt, false, false)
	})
	for _, q := range []string{
		`UPDATE push_jobs SET attempt_lease_hash=zeroblob(32)`,
		`UPDATE push_jobs SET attempt_lease_until_ns=attempt_lease_until_ns+1`,
		`UPDATE push_jobs SET attempt_started_at_ns=attempt_started_at_ns+1`,
		`UPDATE push_jobs SET attempt=attempt+1`,
		`UPDATE push_jobs SET attempt_lease_hash=NULL,attempt_lease_until_ns=NULL,attempt_started_at_ns=NULL`,
	} {
		t.Run(q, func(t *testing.T) {
			f, h := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := *mustBeginSync(t, r, h.Lease, f.now)
			jobSQL(t, f.store, q)
			attemptCheckExpect(t, r, h.Lease, ticket, f.now.Add(time.Second), false, false)
		})
	}
}

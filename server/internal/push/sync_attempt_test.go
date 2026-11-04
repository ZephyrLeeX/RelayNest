package push

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

func mustBeginSync(t *testing.T, r *JobRepository, l JobLease, now time.Time) *SyncAttempt {
	t.Helper()
	ticket, err := r.BeginSync(jobContext, l, now)
	if err != nil || ticket == nil {
		t.Fatal("begin", ticket != nil, err)
	}
	return ticket
}
func expectSyncNoop(t *testing.T, r *JobRepository, ticket SyncAttempt, now time.Time) {
	t.Helper()
	before := leaseDigest(t, r.store)
	if changed, err := r.ApplySyncResult(jobContext, ticket, SyncAccepted, now); err != nil || changed {
		t.Fatal("stale result", changed, err)
	}
	if before != leaseDigest(t, r.store) {
		t.Fatal("no-op changed database")
	}
}

func TestSyncAttemptOnceConcurrentAndReopen(t *testing.T) {
	f, held := leaseFixture(t)
	repos := []*JobRepository{NewJobRepository(f.store), NewJobRepository(f.other)}
	type answer struct {
		ticket *SyncAttempt
		err    error
	}
	answers := make(chan answer, 2)
	var wg sync.WaitGroup
	for _, r := range repos {
		wg.Add(1)
		go func(r *JobRepository) {
			defer wg.Done()
			v, e := r.BeginSync(jobContext, held.Lease, f.now)
			answers <- answer{v, e}
		}(r)
	}
	wg.Wait()
	close(answers)
	var ticket *SyncAttempt
	for a := range answers {
		if a.err != nil {
			t.Fatal(a.err)
		}
		if a.ticket != nil {
			if ticket != nil {
				t.Fatal("duplicate ticket")
			}
			ticket = a.ticket
		}
	}
	if ticket == nil || jobRead(t, f.other, held.Job.Snapshot.ID).Attempt != 1 {
		t.Fatal("missing attempt")
	}
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	before := leaseDigest(t, f.store)
	if again, err := NewJobRepository(reopened).BeginSync(jobContext, held.Lease, f.now); err != nil || again != nil {
		t.Fatal("reopen duplicated", err)
	}
	if before != leaseDigest(t, f.store) {
		t.Fatal("duplicate Begin wrote state")
	}
	raw, err := json.Marshal(ticket)
	if err != nil || string(raw) != "null" || strings.Contains(fmt.Sprintf("%+v", *ticket), held.Job.Snapshot.ID) {
		t.Fatal("ticket export", err)
	}
	if ticket.snapshot != held.Job.Snapshot || ticket.attempt != 1 {
		t.Fatal("ticket changed snapshot")
	}
}

func TestSyncAttemptTypedOutcomesAndBoundedRetry(t *testing.T) {
	for _, outcome := range []SyncOutcome{SyncAccepted, SyncTargetInvalid, SyncTransient, SyncRateLimited, SyncConfigurationError, SyncUnknownDelivery} {
		t.Run(string(outcome), func(t *testing.T) {
			f, held := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := mustBeginSync(t, r, held.Lease, f.now)
			if changed, err := r.ApplySyncResult(jobContext, *ticket, outcome, f.now.Add(time.Second)); err != nil || !changed {
				t.Fatal("result", changed, err)
			}
			got := jobRead(t, f.other, held.Job.Snapshot.ID)
			retry := outcome == SyncTransient || outcome == SyncRateLimited || outcome == SyncUnknownDelivery
			wantState := "degraded"
			if retry {
				wantState = "queued"
			}
			if outcome == SyncAccepted {
				wantState = "accepted"
			}
			if got.State != wantState || got.Attempt != 1 || got.Snapshot != held.Job.Snapshot {
				t.Fatal("outcome snapshot", got)
			}
			if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE id=? AND result_kind=? AND result_at_ns=? AND attempt_lease_hash IS NOT NULL AND lease_hash IS NULL AND lease_until_ns IS NULL`, got.Snapshot.ID, string(outcome), f.now.Add(time.Second).UnixNano()) != 1 {
				t.Fatal("typed result not persisted")
			}
			if jobCount(t, f.store, `SELECT count(*) FROM device_push_registrations WHERE registration_version=1 AND token IS NOT NULL`) != 1 {
				t.Fatal("outcome removed registration")
			}
			expectSyncNoop(t, r, *ticket, f.now.Add(2*time.Second))
			if retry {
				if !got.NextAttemptAt.Equal(f.now.Add(11 * time.Second)) {
					t.Fatal("retry delay", got.NextAttemptAt)
				}
				early, err := r.Reserve(jobContext, got.NextAttemptAt.Add(-time.Nanosecond), time.Minute, 1)
				if err != nil || len(early) != 0 {
					t.Fatal("early retry", err)
				}
				next, err := r.Reserve(jobContext, got.NextAttemptAt, time.Minute, 1)
				if err != nil || len(next) != 1 {
					t.Fatal("due retry", err)
				}
				expectSyncNoop(t, r, *ticket, got.NextAttemptAt)
				second := mustBeginSync(t, r, next[0].Lease, got.NextAttemptAt)
				if second.attempt != 2 || second.snapshot != ticket.snapshot {
					t.Fatal("retry reset snapshot")
				}
			} else {
				got, err := r.Reserve(jobContext, f.now.Add(2*time.Minute), time.Minute, 1)
				if err != nil || len(got) != 0 {
					t.Fatal("terminal reclaimed", err)
				}
			}
		})
	}
}

func TestSyncAttemptResultLeaseAndVersionFencing(t *testing.T) {
	for _, mutation := range []struct {
		name, q string
		session bool
	}{
		{"registration-rotate", `UPDATE device_push_registrations SET registration_version=2 WHERE session_id=?`, true},
		{"registration-delete", `DELETE FROM device_push_registrations WHERE session_id=?`, true},
		{"registration-tombstone", `UPDATE device_push_registrations SET token=NULL WHERE session_id=?`, true},
		{"revoked", `UPDATE sessions SET revoked=1 WHERE id=?`, true},
		{"session-generation", `UPDATE sessions SET auth_generation=2 WHERE id=?`, true},
		{"client", `UPDATE sessions SET client_type='web' WHERE id=?`, true},
		{"admin-generation", `UPDATE administrator SET auth_generation=2 WHERE id=1`, false},
		{"settings", `INSERT INTO device_notification_settings VALUES('11111111-1111-4111-8111-111111111111',1,'now','{}')`, false},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			f, held := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := mustBeginSync(t, r, held.Lease, f.now)
			if mutation.session {
				jobSQL(t, f.store, mutation.q, held.Job.Snapshot.SessionID)
			} else {
				jobSQL(t, f.store, mutation.q)
			}
			expectSyncNoop(t, NewJobRepository(f.other), *ticket, f.now)
			before := leaseDigest(t, f.store)
			if v, e := r.BeginSync(jobContext, held.Lease, f.now); e != nil || v != nil {
				t.Fatal("stale begin", e)
			}
			if before != leaseDigest(t, f.store) {
				t.Fatal("stale begin changed data")
			}
		})
	}
	for _, release := range []bool{false, true} {
		t.Run(fmt.Sprintf("release=%v", release), func(t *testing.T) {
			f, held := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := mustBeginSync(t, r, held.Lease, f.now)
			now := f.now.Add(time.Minute)
			if release {
				now = f.now
				if ok, e := r.Release(jobContext, held.Lease, now); e != nil || !ok {
					t.Fatal("release", e)
				}
			}
			expectSyncNoop(t, r, *ticket, now)
			acquired, e := NewJobRepository(f.other).Reserve(jobContext, now, time.Minute, 1)
			if e != nil || len(acquired) != 1 {
				t.Fatal("takeover", e)
			}
			fresh := mustBeginSync(t, r, acquired[0].Lease, now)
			if fresh.attempt != 2 {
				t.Fatal("takeover reset attempt")
			}
			before := leaseDigest(t, f.store)
			if v, e := r.BeginSync(jobContext, held.Lease, now); e != nil || v != nil {
				t.Fatal("old Begin", e)
			}
			expectSyncNoop(t, r, *ticket, now)
			if before != leaseDigest(t, f.store) {
				t.Fatal("old lease touched new lease")
			}
		})
	}
	t.Run("deployment", func(t *testing.T) {
		f, held := leaseFixture(t)
		ticket := mustBeginSync(t, NewJobRepository(f.store), held.Lease, f.now)
		jobSQL(t, f.store, `UPDATE instance_identity SET instance_id='33333333-3333-4333-8333-333333333333' WHERE id=1`)
		reopened, e := storage.Open(jobContext, f.cfg)
		if e != nil {
			t.Fatal(e)
		}
		defer reopened.Close()
		expectSyncNoop(t, NewJobRepository(reopened), *ticket, f.now)
	})
}

func TestSyncAttemptFaultsRollbackAndNoTicket(t *testing.T) {
	for _, phase := range []string{"begin", "result"} {
		for _, fault := range []string{"abort", "ignore", "commit", "post-write"} {
			t.Run(phase+"/"+fault, func(t *testing.T) {
				f, held := leaseFixture(t)
				r := NewJobRepository(f.store)
				var ticket *SyncAttempt
				column := "attempt"
				if phase == "result" {
					ticket = mustBeginSync(t, r, held.Lease, f.now)
					column = "result_kind"
				}
				q := `CREATE TRIGGER attempt_fault BEFORE UPDATE OF ` + column + ` ON push_jobs BEGIN SELECT RAISE(ABORT,'synthetic-fault');END`
				if fault == "ignore" {
					q = strings.ReplaceAll(q, "ABORT,'synthetic-fault'", "IGNORE")
				}
				if fault == "commit" {
					q = `CREATE TABLE attempt_fault_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);
 CREATE TRIGGER attempt_fault AFTER UPDATE OF ` + column + ` ON push_jobs BEGIN INSERT INTO attempt_fault_fk VALUES('missing-bot');END`
				}
				if fault == "post-write" {
					q = `CREATE TRIGGER attempt_fault AFTER UPDATE OF ` + column + ` ON push_jobs BEGIN UPDATE push_jobs SET state='queued',lease_hash=NULL,lease_until_ns=NULL WHERE id=NEW.id;END`
					if phase == "result" {
						q = `CREATE TRIGGER attempt_fault AFTER UPDATE OF result_kind ON push_jobs BEGIN UPDATE push_jobs SET result_kind='transient' WHERE id=NEW.id;END`
					}
				}
				jobSQL(t, f.store, q)
				before := leaseDigest(t, f.store)
				if phase == "begin" {
					if v, e := r.BeginSync(jobContext, held.Lease, f.now); e == nil || v != nil {
						t.Fatal("failed begin published", e)
					}
				} else {
					if changed, e := r.ApplySyncResult(jobContext, *ticket, SyncAccepted, f.now); e == nil || changed {
						t.Fatal("failed result published", e)
					}
				}
				if before != leaseDigest(t, f.other) {
					t.Fatal("partial fault write")
				}
				jobSQL(t, f.store, `DROP TRIGGER attempt_fault`)
				if phase == "begin" {
					mustBeginSync(t, r, held.Lease, f.now)
				} else {
					if ok, e := r.ApplySyncResult(jobContext, *ticket, SyncAccepted, f.now); e != nil || !ok {
						t.Fatal("retry after rollback", e)
					}
				}
			})
		}
	}
}

func TestSyncAttemptEightAttemptsTTLAndOverflow(t *testing.T) {
	t.Run("eight", func(t *testing.T) {
		f, held := leaseFixture(t)
		r := NewJobRepository(f.store)
		now := f.now
		for n := int64(1); n <= 8; n++ {
			ticket := mustBeginSync(t, r, held.Lease, now)
			if ticket.attempt != n {
				t.Fatal("attempt", ticket.attempt, n)
			}
			if ok, e := r.ApplySyncResult(jobContext, *ticket, SyncUnknownDelivery, now); e != nil || !ok {
				t.Fatal(e)
			}
			got := jobRead(t, f.store, held.Job.Snapshot.ID)
			if got.Snapshot != held.Job.Snapshot || got.Attempt != n || got.NextAttemptAt.After(got.Snapshot.ExpiresAt) {
				t.Fatal("bounds changed")
			}
			if n < 8 {
				now = got.NextAttemptAt
				next, e := r.Reserve(jobContext, now, time.Minute, 1)
				if e != nil || len(next) != 1 {
					t.Fatal("retry", e)
				}
				held = next[0]
			} else if got.State != "degraded" {
				t.Fatal("not degraded")
			}
		}
	})
	t.Run("crashed-eighth", func(t *testing.T) {
		f, held := leaseFixture(t)
		r := NewJobRepository(f.store)
		jobSQL(t, f.store, `UPDATE push_jobs SET attempt=7 WHERE id=?`, held.Job.Snapshot.ID)
		mustBeginSync(t, r, held.Lease, f.now)
		now := f.now.Add(time.Minute)
		next, e := r.Reserve(jobContext, now, time.Minute, 1)
		if e != nil || len(next) != 1 {
			t.Fatal(e)
		}
		if v, e := r.BeginSync(jobContext, next[0].Lease, now); e != nil || v != nil {
			t.Fatal("ninth ticket", e)
		}
		if got := jobRead(t, f.store, held.Job.Snapshot.ID); got.Attempt != 8 || got.State != "degraded" {
			t.Fatal("crash not bounded")
		}
	})
	t.Run("overflow-attempt", func(t *testing.T) {
		f, held := leaseFixture(t)
		jobSQL(t, f.store, `UPDATE push_jobs SET attempt=? WHERE id=?`, int64(math.MaxInt64), held.Job.Snapshot.ID)
		before := leaseDigest(t, f.store)
		if v, e := NewJobRepository(f.store).BeginSync(jobContext, held.Lease, f.now); e == nil || v != nil {
			t.Fatal("overflow issued", e)
		}
		if before != leaseDigest(t, f.store) {
			t.Fatal("overflow partial")
		}
	})
	for _, overflow := range []bool{false, true} {
		t.Run(fmt.Sprintf("expiry-overflow=%v", overflow), func(t *testing.T) {
			f := jobSetup(t)
			s := f.session(t, jobDeviceA)
			j := f.snapshot(t, s, "sync")
			now := f.now
			if overflow {
				now = time.Unix(0, math.MaxInt64-int64(2*time.Second)).UTC()
				j.CreatedAt = now
				j.ExpiresAt = time.Unix(0, math.MaxInt64).UTC()
				jobSQL(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=? WHERE id=?`, int64(math.MaxInt64), int64(math.MaxInt64), s.ID)
			} else {
				j.ExpiresAt = now.Add(2 * time.Second)
			}
			jobCreate(t, f, j)
			r := NewJobRepository(f.store)
			held, e := r.Reserve(jobContext, now, time.Second, 1)
			if e != nil || len(held) != 1 {
				t.Fatal(e)
			}
			ticket := mustBeginSync(t, r, held[0].Lease, now)
			if ok, e := r.ApplySyncResult(jobContext, *ticket, SyncTransient, now); e != nil || !ok {
				t.Fatal(e)
			}
			got := jobRead(t, f.store, j.ID)
			if got.State != "degraded" || got.NextAttemptAt.After(j.ExpiresAt) {
				t.Fatal("retry crossed expiry")
			}
		})
	}
	t.Run("exact-ttl", func(t *testing.T) {
		f := jobSetup(t)
		s := f.session(t, jobDeviceA)
		j := f.snapshot(t, s, "sync")
		j.ExpiresAt = f.now.Add(time.Second)
		jobCreate(t, f, j)
		r := NewJobRepository(f.store)
		held, e := r.Reserve(jobContext, f.now, time.Minute, 1)
		if e != nil || len(held) != 1 {
			t.Fatal(e)
		}
		ticket := mustBeginSync(t, r, held[0].Lease, f.now)
		expectSyncNoop(t, r, *ticket, j.ExpiresAt)
		before := leaseDigest(t, f.store)
		if v, e := r.BeginSync(jobContext, held[0].Lease, j.ExpiresAt); e != nil || v != nil {
			t.Fatal("ttl Begin", e)
		}
		if before != leaseDigest(t, f.store) {
			t.Fatal("ttl changed")
		}
	})
}

func TestSyncAttemptStrictMetadataAndInputs(t *testing.T) {
	for _, field := range []string{"attempt_lease_hash", "attempt_lease_until_ns", "attempt_started_at_ns", "result_kind", "result_at_ns"} {
		t.Run(field, func(t *testing.T) {
			f, held := leaseFixture(t)
			r := NewJobRepository(f.store)
			ticket := mustBeginSync(t, r, held.Lease, f.now)
			// 只在损坏库夹具临时绕过 CHECK，读写 API 必须独立拒绝原始 BLOB/REAL 类。
			value := `CAST(` + field + ` AS BLOB)`
			if field == "attempt_lease_hash" {
				value = `CAST(attempt_lease_hash AS TEXT)`
			}
			if field == "result_kind" {
				value = `CAST('transient' AS BLOB)`
			}
			if field == "result_at_ns" {
				value = fmt.Sprintf("CAST(%d AS BLOB)", f.now.UnixNano())
			}
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
			jobSQL(t, f.store, `UPDATE push_jobs SET `+field+`=`+value+` WHERE id=?`, held.Job.Snapshot.ID)
			jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
			before := leaseDigest(t, f.store)
			if v, e := r.BeginSync(jobContext, held.Lease, f.now); e == nil || v != nil {
				t.Fatal("bad metadata Begin", e)
			}
			if ok, e := r.ApplySyncResult(jobContext, *ticket, SyncAccepted, f.now); e == nil || ok {
				t.Fatal("bad metadata result", e)
			}
			if before != leaseDigest(t, f.store) {
				t.Fatal("bad metadata changed")
			}
		})
	}
	f, held := leaseFixture(t)
	r := NewJobRepository(f.store)
	ticket := mustBeginSync(t, r, held.Lease, f.now)
	for _, bad := range []SyncAttempt{{}, func() SyncAttempt { v := *ticket; v.snapshot.TargetChangeSeq++; return v }(), func() SyncAttempt { v := *ticket; v.attempt++; return v }()} {
		before := leaseDigest(t, f.store)
		ok, e := r.ApplySyncResult(jobContext, bad, SyncAccepted, f.now)
		if ok || (reflect.DeepEqual(bad, SyncAttempt{}) && e == nil) {
			t.Fatal("bad ticket", e)
		}
		if before != leaseDigest(t, f.store) {
			t.Fatal("bad ticket wrote")
		}
	}
	canceled, cancel := context.WithCancel(jobContext)
	cancel()
	for _, ctx := range []context.Context{nil, canceled} {
		if v, e := r.BeginSync(ctx, held.Lease, f.now); e == nil || v != nil {
			t.Fatal("context Begin", e)
		}
		if ok, e := r.ApplySyncResult(ctx, *ticket, SyncAccepted, f.now); e == nil || ok {
			t.Fatal("context result", e)
		}
	}
	if ok, e := r.ApplySyncResult(jobContext, *ticket, SyncOutcome("invented"), f.now); e == nil || ok {
		t.Fatal("unknown enum")
	}
	if ok, e := r.ApplySyncResult(jobContext, *ticket, SyncAccepted, f.now.Add(-time.Nanosecond)); e == nil || ok {
		t.Fatal("time reversal")
	}
	if v, e := r.BeginSync(jobContext, JobLease{}, f.now); e == nil || v != nil {
		t.Fatal("bad lease")
	}
	if v, e := r.BeginSync(jobContext, held.Lease, time.Time{}); e == nil || v != nil {
		t.Fatal("bad now")
	}
}

func TestSyncAttemptConcurrentResultsAndAlertIsolation(t *testing.T) {
	f, held := leaseFixture(t)
	r := NewJobRepository(f.store)
	ticket := mustBeginSync(t, r, held.Lease, f.now.In(time.FixedZone("fixture", 8*60*60)))
	results := make(chan bool, 2)
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for i, outcome := range []SyncOutcome{SyncAccepted, SyncTransient} {
		repo := r
		if i == 1 {
			repo = NewJobRepository(f.other)
		}
		wg.Add(1)
		go func(repo *JobRepository, outcome SyncOutcome) {
			defer wg.Done()
			changed, e := repo.ApplySyncResult(jobContext, *ticket, outcome, f.now)
			results <- changed
			failures <- e
		}(repo, outcome)
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	applied := 0
	for changed := range results {
		if changed {
			applied++
		}
	}
	if applied != 1 {
		t.Fatal("concurrent outcomes applied", applied)
	}
	got := jobRead(t, f.store, held.Job.Snapshot.ID)
	if got.Attempt != 1 || got.Snapshot != held.Job.Snapshot || (got.State != "queued" && got.State != "accepted") {
		t.Fatal("concurrent result snapshot")
	}
	// alert 可由原 Reserve 接口预留，本叶 Begin 必须保持只读拒绝。
	g := jobSetup(t)
	s := g.session(t, jobDeviceB)
	alert := g.snapshot(t, s, "alert")
	jobCreate(t, g, alert)
	repo := NewJobRepository(g.store)
	reserved, e := repo.Reserve(jobContext, g.now, time.Minute, 1)
	if e != nil || len(reserved) != 1 {
		t.Fatal("alert reserve", e)
	}
	before := leaseDigest(t, g.store)
	if v, e := repo.BeginSync(jobContext, reserved[0].Lease, g.now); e != nil || v != nil {
		t.Fatal("alert Begin", e)
	}
	if before != leaseDigest(t, g.store) {
		t.Fatal("alert promoted")
	}
}

func TestSyncAttemptResultBindsEveryTicketField(t *testing.T) {
	f, held := leaseFixture(t)
	r := NewJobRepository(f.store)
	ticket := mustBeginSync(t, r, held.Lease, f.now)
	for _, change := range []struct {
		name   string
		mutate func(*SyncAttempt)
	}{
		{"job", func(v *SyncAttempt) { v.snapshot.ID = "missing-job" }},
		{"session", func(v *SyncAttempt) { v.snapshot.SessionID = "other" }},
		{"device", func(v *SyncAttempt) { v.snapshot.DeviceID = "other" }},
		{"registration", func(v *SyncAttempt) { v.snapshot.RegistrationVersion++ }},
		{"settings", func(v *SyncAttempt) { v.snapshot.SettingsVersion++ }},
		{"deployment", func(v *SyncAttempt) { v.snapshot.DeploymentID = "other" }},
		{"generation", func(v *SyncAttempt) { v.snapshot.SessionGeneration++ }},
		{"created", func(v *SyncAttempt) { v.snapshot.CreatedAt = v.snapshot.CreatedAt.Add(time.Nanosecond) }},
		{"expiry", func(v *SyncAttempt) { v.snapshot.ExpiresAt = v.snapshot.ExpiresAt.Add(time.Nanosecond) }},
		{"watermark", func(v *SyncAttempt) { v.snapshot.TargetChangeSeq++ }},
		{"message", func(v *SyncAttempt) { v.snapshot.MessageID = "other" }},
		{"bot", func(v *SyncAttempt) { v.snapshot.BotID = "other" }},
		{"botseq", func(v *SyncAttempt) { v.snapshot.BotSeq++ }},
		{"attempt", func(v *SyncAttempt) { v.attempt++ }},
		{"hash", func(v *SyncAttempt) { v.leaseHash[0] ^= 1 }},
		{"until", func(v *SyncAttempt) { v.leaseUntil = v.leaseUntil.Add(time.Nanosecond) }},
		{"started", func(v *SyncAttempt) { v.startedAt = v.startedAt.Add(time.Nanosecond) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			bad := *ticket
			change.mutate(&bad)
			expectSyncNoop(t, r, bad, f.now.Add(time.Second))
		})
	}
}

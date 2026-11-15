package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

type opushDispatch func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error)

func (d opushDispatch) Dispatch(c context.Context, e push.OPushAlertEnvelope) (push.AlertOutcome, error) {
	return d(c, e)
}

func opushSource(t *testing.T) (*fixture, push.JobSnapshot, time.Time) {
	t.Helper()
	f := setup(t)
	if _, e := f.create(ctx); e != nil {
		t.Fatal(e)
	}
	all := rows(t, f.store)
	if len(all) != 1 {
		t.Fatal("producer did not create exactly one job", len(all))
	}
	for id := range all {
		return f, read(t, f, id).Snapshot, time.Now().UTC()
	}
	panic("unreachable")
}
func opushStep(f *fixture, clock func() time.Time, d opushDispatch) *push.OPushAlertWorker {
	return push.NewOPushAlertWorker(push.NewJobRepository(f.store), clock, time.Minute, d)
}
func opushOther(t *testing.T, f *fixture) *storage.Store {
	t.Helper()
	s, e := storage.Open(ctx, f.cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func opushKind(t *testing.T, f *fixture, id string) string {
	t.Helper()
	var s sql.NullString
	if e := f.store.Read(ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT result_kind FROM push_jobs WHERE id=?`, id).Scan(&s)
	}); e != nil {
		t.Fatal(e)
	}
	return s.String
}

// 整库投影只排除 push_jobs；每张业务表、注册及设置保持原值。
func opushBusiness(t *testing.T, f *fixture) string {
	t.Helper()
	s := database(t, f.store)
	a, b := strings.Index(s, "push_jobs"), strings.Index(s, "schema_migrations")
	if a < 0 || b < a {
		t.Fatal("unexpected database projection")
	}
	return s[:a] + s[b:]
}
func TestOPushAlertWorkerProducerOutcomes(t *testing.T) {
	for _, out := range []push.AlertOutcome{push.AlertAccepted, push.AlertTargetInvalid, push.AlertTransient, push.AlertRateLimited, push.AlertConfigurationError, push.AlertUnknownDelivery, "", "bad", "error"} {
		t.Run(string(out), func(t *testing.T) {
			f, j, n := opushSource(t)
			other := opushOther(t, f)
			business := opushBusiness(t, f)
			calls, clocks := 0, 0
			clock := func() time.Time {
				v := n.Add(time.Duration(clocks) * time.Second)
				clocks++
				return v.In(time.FixedZone("test", 3600))
			}
			w := opushStep(f, clock, func(c context.Context, en push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				var state string
				var attempt int64
				var started int64
				if e := other.Read(c, func(tx *sql.Tx) error {
					return tx.QueryRow(`SELECT state,attempt,attempt_started_at_ns FROM push_jobs WHERE id=?`, j.ID).Scan(&state, &attempt, &started)
				}); e != nil {
					t.Fatal(e)
				}
				if state != "leased" || attempt != 1 || started != n.Add(time.Second).UnixNano() {
					t.Fatal("Begin not committed", state, attempt, started)
				}
				if reflect.TypeOf(en).NumField() != 4 || en.JobKey != j.ID || en.Purpose != "alert" || en.Provider != "opush" || en.ExpiresAt != j.ExpiresAt {
					t.Fatal("envelope contract", en)
				}
				encoded, e := json.Marshal(en)
				if e != nil {
					t.Fatal(e)
				}
				var fields map[string]any
				if e = json.Unmarshal(encoded, &fields); e != nil || len(fields) != 4 {
					t.Fatal("JSON contract", e, string(encoded))
				}
				raw := string(encoded) + fmt.Sprintf("%+v %#v", en, en)
				for _, secret := range []string{"synthetic-secret", "atomic", j.SessionID, j.DeviceID, j.MessageID, j.BotID, j.DeploymentID, "lease", "ticket", "https://"} {
					if secret != "" && strings.Contains(raw, secret) {
						t.Fatal("envelope leak", secret)
					}
				}
				if out == "error" {
					return push.AlertAccepted, errors.New("synthetic-secret https://private.invalid/atomic")
				}
				return out, nil
			})
			got, e := w.Step(ctx)
			if e != nil || !got.Dispatched || !got.Applied || calls != 1 || clocks != 4 {
				t.Fatal("Step", got, e, calls, clocks)
			}
			wantOut := out
			if out == "" || out == "bad" || out == "error" {
				wantOut = push.AlertUnknownDelivery
			}
			want := "degraded"
			if wantOut == push.AlertAccepted {
				want = "accepted"
			}
			if wantOut == push.AlertTransient || wantOut == push.AlertRateLimited || wantOut == push.AlertUnknownDelivery {
				want = "queued"
			}
			after := read(t, f, j.ID)
			if after.State != want || after.Attempt != 1 || after.Snapshot != j || opushKind(t, f, j.ID) != string(wantOut) {
				t.Fatal("result projection", after)
			}
			if want == "queued" && !after.NextAttemptAt.Equal(n.Add(13*time.Second)) {
				t.Fatal("fresh Apply clock/backoff", after.NextAttemptAt)
			}
			if opushBusiness(t, f) != business {
				t.Fatal("business side effect")
			}
			if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || calls != 1 {
				t.Fatal("immediate redispatch", again, e, calls)
			}
		})
	}
}
func TestOPushAlertWorkerEightAttempts(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprint(crash), func(t *testing.T) {
			f, j, n := opushSource(t)
			calls := 0
			w := opushStep(f, func() time.Time { return n }, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				if crash && calls == 8 {
					panic("synthetic crash")
				}
				return push.AlertUnknownDelivery, nil
			})
			for a := int64(1); a <= 8; a++ {
				var got push.OPushAlertStepResult
				var e error
				if crash && a == 8 {
					func() {
						defer func() {
							if recover() != "synthetic crash" {
								t.Fatal("panic not propagated")
							}
						}()
						_, _ = w.Step(ctx)
						t.Fatal("panic swallowed")
					}()
				} else {
					got, e = w.Step(ctx)
					if e != nil || !got.Dispatched || !got.Applied {
						t.Fatal(a, got, e)
					}
				}
				after := read(t, f, j.ID)
				if after.Attempt != a || after.Snapshot != j || calls != int(a) {
					t.Fatal("attempt/key/expiry changed", after, calls)
				}
				if a < 8 {
					delay := 10 * time.Second * time.Duration(int64(1)<<(a-1))
					if delay > 10*time.Minute {
						delay = 10 * time.Minute
					}
					if !after.NextAttemptAt.Equal(n.Add(delay)) {
						t.Fatal("backoff", a, after.NextAttemptAt)
					}
					n = after.NextAttemptAt
				}
			}
			if crash {
				n = n.Add(time.Minute)
			}
			if got, e := w.Step(ctx); e != nil || got != (push.OPushAlertStepResult{}) || calls != 8 {
				t.Fatal("ninth dispatch", got, e, calls)
			}
			if after := read(t, f, j.ID); after.State != "degraded" || after.Attempt != 8 {
				t.Fatal("not exhausted", after)
			}
		})
	}
}
func TestOPushAlertWorkerEffectiveExpiry(t *testing.T) {
	for _, mode := range []string{"before", "begin-boundary", "check-boundary", "apply-boundary", "retry-boundary"} {
		t.Run(mode, func(t *testing.T) {
			f, j, n := opushSource(t)
			exec(t, f.store, `DELETE FROM push_jobs`)
			j.ExpiresAt = j.CreatedAt.Add(48 * time.Hour)
			exec(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=?`, j.CreatedAt.Add(72*time.Hour).UnixNano(), j.CreatedAt.Add(72*time.Hour).UnixNano())
			if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
				t.Fatal(e)
			}
			limit := j.CreatedAt.Add(24 * time.Hour)
			n = limit.Add(-time.Second)
			if mode == "retry-boundary" {
				n = limit.Add(-10 * time.Second)
			}
			phase, calls := 0, 0
			clock := func() time.Time {
				phase++
				if (mode == "begin-boundary" && phase >= 2) || (mode == "check-boundary" && phase >= 3) || (mode == "apply-boundary" && phase >= 4) {
					return limit
				}
				return n
			}
			w := opushStep(f, clock, func(_ context.Context, en push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				if !en.ExpiresAt.Equal(limit) {
					t.Fatal("effective expiry", en)
				}
				if mode == "retry-boundary" {
					return push.AlertTransient, nil
				}
				return push.AlertAccepted, nil
			})
			got, e := w.Step(ctx)
			wantCall := mode == "before" || mode == "apply-boundary" || mode == "retry-boundary"
			wantApply := mode == "before" || mode == "retry-boundary"
			if e != nil || got.Dispatched != wantCall || got.Applied != wantApply || calls != map[bool]int{true: 1, false: 0}[wantCall] {
				t.Fatal(mode, got, e, calls)
			}
			after := read(t, f, j.ID)
			if after.Snapshot != j {
				t.Fatal("original expiry rewritten")
			}
			if mode == "retry-boundary" && after.State != "degraded" {
				t.Fatal("retry past effective expiry", after)
			}
		})
	}
}
func TestOPushAlertWorkerMixedScopeAndSyncHolder(t *testing.T) {
	for _, holder := range []bool{false, true} {
		t.Run(fmt.Sprint(holder), func(t *testing.T) {
			f, j, n := opushSource(t)
			exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, n.Format(stamp))
			for _, mode := range []string{"fcm-alert", "ineligible", "sync", "legacy"} {
				k := j
				k.ID = "000-earlier-" + mode
				if mode == "legacy" {
					exec(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)VALUES(?,?,?,1,?)`, k.ID, k.SessionID, k.DeviceID, j.CreatedAt.Format(stamp))
					continue
				}
				if mode == "fcm-alert" {
					k.Provider = "fcm"
				}
				if mode == "ineligible" {
					k.AlertEligible = false
				}
				if mode == "sync" {
					k.Purpose = "sync"
					k.Provider = "fcm"
					k.AlertEligible = false
				}
				if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, k, j.CreatedAt) }); e != nil {
					t.Fatal(e)
				}
			}
			other := opushOther(t, f)
			if holder {
				h, e := push.NewJobRepository(other).ReserveSync(ctx, n, time.Minute, 1)
				if e != nil || len(h) != 1 {
					t.Fatal("sync holder", len(h), e)
				}
			}
			// 四个跨 scope 行在真实 ORDER BY next_attempt_at,id 下均早于原 eligible 行。
			var earlier int
			if e := f.store.Read(ctx, func(tx *sql.Tx) error {
				return tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE id<>? AND
 (next_attempt_at<? OR (next_attempt_at=? AND id<?))`, j.ID, j.CreatedAt.Format(stamp), j.CreatedAt.Format(stamp), j.ID).Scan(&earlier)
			}); e != nil || earlier != 4 {
				t.Fatal("cross-scope rows are not earlier", earlier, e)
			}
			before := rows(t, f.store)
			calls := 0
			w := opushStep(f, func() time.Time { return n }, func(_ context.Context, en push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				if en.JobKey != j.ID {
					t.Fatal("scope leak")
				}
				return push.AlertAccepted, nil
			})
			got, e := w.Step(ctx)
			if e != nil || got.Dispatched == holder || got.Applied == holder || calls != map[bool]int{false: 1, true: 0}[holder] {
				t.Fatal(got, e, calls)
			}
			after := rows(t, f.store)
			for id, row := range before {
				if id != j.ID && !reflect.DeepEqual(after[id], row) {
					t.Fatal("other scope consumed", id)
				}
			}
			if !holder {
				if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || calls != 1 {
					t.Fatal("negative scope consumed", again, e)
				}
			}
		})
	}
}

// 两个真实 Store 同时起步；首个调用阻塞期间另一 Step 必须无派发完成。
func TestOPushAlertWorkerConcurrentStores(t *testing.T) {
	f, _, n := opushSource(t)
	if _, e := f.create(ctx); e != nil {
		t.Fatal(e)
	}
	other := opushOther(t, f)
	start := make(chan struct{})
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	d := opushDispatch(func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return push.AlertAccepted, nil
	})
	type response struct {
		r push.OPushAlertStepResult
		e error
	}
	done := make(chan response, 2)
	for _, s := range []*storage.Store{f.store, other} {
		go func(s *storage.Store) {
			<-start
			r, e := push.NewOPushAlertWorker(push.NewJobRepository(s), func() time.Time { return n }, time.Minute, d).Step(ctx)
			done <- response{r, e}
		}(s)
	}
	close(start)
	<-entered
	loser := <-done
	if loser.e != nil || loser.r != (push.OPushAlertStepResult{}) || calls.Load() != 1 {
		t.Fatal("same-device mutex", loser, calls.Load())
	}
	// 放行一次调用后回收另一个 goroutine；关闭专用释放通道一次。
	release <- struct{}{}
	winner := <-done
	if winner.e != nil || !winner.r.Dispatched || !winner.r.Applied || calls.Load() != 1 {
		t.Fatal("winner", winner, calls.Load())
	}
}

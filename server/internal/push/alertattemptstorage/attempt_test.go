package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

func TestProducerCommitCheckAndSixResults(t *testing.T) {
	for _, out := range []push.AlertOutcome{push.AlertAccepted, push.AlertTargetInvalid, push.AlertTransient, push.AlertRateLimited, push.AlertConfigurationError, push.AlertUnknownDelivery} {
		t.Run(string(out), func(t *testing.T) {
			f, h, n := prepared(t)
			r := push.NewJobRepository(f.store)
			ticket := begin(t, f, h, n)
			if again, e := r.BeginAlert(ctx, h.Lease, n); e != nil || again != nil {
				t.Fatal("same lease", again, e)
			}
			if read(t, f, h.Job.Snapshot.ID).Attempt != 1 {
				t.Fatal("double count")
			}
			before := database(t, f.store)
			for i := 0; i < 2; i++ {
				if ok, e := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); e != nil || !ok {
					t.Fatal("check", ok, e)
				}
			}
			if database(t, f.store) != before {
				t.Fatal("check wrote")
			}
			encoded, err := json.Marshal(ticket)
			if err != nil || string(encoded) != "null" {
				t.Fatal("marshal", err, string(encoded))
			}
			raw := fmt.Sprintf("%+v %#v %s", ticket, ticket, ticket)
			if raw != "[push alert attempt] [push alert attempt] [push alert attempt]" {
				t.Fatal("ticket leak", raw)
			}
			changed, e := r.ApplyAlertResult(ctx, *ticket, out, n.Add(time.Second))
			if e != nil || !changed {
				t.Fatal("result", changed, e)
			}
			j := read(t, f, h.Job.Snapshot.ID)
			want := "degraded"
			if out == push.AlertAccepted {
				want = "accepted"
			}
			if out == push.AlertTransient || out == push.AlertRateLimited || out == push.AlertUnknownDelivery {
				want = "queued"
				if !j.NextAttemptAt.Equal(n.Add(11 * time.Second)) {
					t.Fatal("retry", j.NextAttemptAt)
				}
			}
			if j.State != want || j.Attempt != 1 || j.Snapshot != h.Job.Snapshot {
				t.Fatal("projection", j)
			}
			before = database(t, f.store)
			if changed, e = r.ApplyAlertResult(ctx, *ticket, out, n.Add(time.Second)); e != nil || changed {
				t.Fatal("duplicate", e)
			}
			if ok, e := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); e != nil || ok {
				t.Fatal("terminal check", e)
			}
			if database(t, f.store) != before {
				t.Fatal("late write")
			}
		})
	}
}

func TestEightAttemptsAndCrashExhaustion(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprint(crash), func(t *testing.T) {
			f, h, n := prepared(t)
			r := push.NewJobRepository(f.store)
			for attempt := int64(1); attempt <= 8; attempt++ {
				ticket := begin(t, f, h, n)
				if read(t, f, h.Job.Snapshot.ID).Attempt != attempt {
					t.Fatal("attempt", attempt)
				}
				if crash && attempt == 8 {
					n = n.Add(2 * time.Minute)
					held, e := r.ReserveOPushAlerts(ctx, n, time.Minute, 1)
					if e != nil || len(held) != 1 {
						t.Fatal(e, len(held))
					}
					if x, e := r.BeginAlert(ctx, held[0].Lease, n); e != nil || x != nil {
						t.Fatal("ninth", x, e)
					}
					break
				}
				if ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertUnknownDelivery, n); e != nil || !ok {
					t.Fatal(e)
				}
				j := read(t, f, h.Job.Snapshot.ID)
				if attempt == 8 {
					break
				}
				delay := 10 * time.Second * time.Duration(int64(1)<<(attempt-1))
				if delay > 10*time.Minute {
					delay = 10 * time.Minute
				}
				if !j.NextAttemptAt.Equal(n.Add(delay)) {
					t.Fatal("backoff", attempt, j.NextAttemptAt, n)
				}
				n = j.NextAttemptAt
				held, e := r.ReserveOPushAlerts(ctx, n, time.Minute, 1)
				if e != nil || len(held) != 1 {
					t.Fatal(e, len(held))
				}
				h = held[0]
			}
			if j := read(t, f, h.Job.Snapshot.ID); j.State != "degraded" || j.Attempt != 8 {
				t.Fatal(j)
			}
		})
	}
}

func TestPolicyDNDAndOtherDenied(t *testing.T) {
	for _, mode := range []string{"dnd", "read", "source", "bot", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			f, h, n := prepared(t)
			r := push.NewJobRepository(f.store)
			switch mode {
			case "dnd":
				f.configure(t, strings.Replace(configuration(), `"enabled":false`, `"enabled":true`, 1))
				n = time.Now().UTC()
				f.configure(t, strings.ReplaceAll(strings.Replace(configuration(), `"enabled":false`, `"enabled":true`, 1), `"22:00","end":"07:00"`, fmt.Sprintf(`"%s","end":"%s"`, n.Format("15:04"), n.Add(time.Hour).Format("15:04"))))
			case "read":
				exec(t, f.store, `INSERT INTO bot_read_states(bot_id,read_seq,updated_at) VALUES(?,?,?) ON CONFLICT(bot_id) DO UPDATE SET read_seq=excluded.read_seq`, h.Job.Snapshot.BotID, h.Job.Snapshot.BotSeq, n.Format(stamp))
			case "source":
				exec(t, f.store, `DELETE FROM messages`)
			case "bot":
				f.configure(t, strings.Replace(configuration(), `"bot_overrides":[]`, fmt.Sprintf(`"bot_overrides":[{"bot_id":"%s","enabled":false}]`, h.Job.Snapshot.BotID), 1))
			case "disabled":
				f.configure(t, strings.Replace(configuration(), `"enabled":true`, `"enabled":false`, 1))
			}
			before := database(t, f.store)
			x, e := r.BeginAlert(ctx, h.Lease, n)
			if e != nil || x != nil {
				t.Fatal(x, e)
			}
			if mode != "dnd" {
				if database(t, f.store) != before {
					t.Fatal("other denied wrote")
				}
				return
			}
			j := read(t, f, h.Job.Snapshot.ID)
			if j.State != "suppressed" || j.Attempt != 0 {
				t.Fatal(j)
			}
			f.configure(t, configuration())
			if x, e = r.BeginAlert(ctx, h.Lease, n); e != nil || x != nil {
				t.Fatal(x, e)
			}
			if e = f.store.Write(ctx, func(tx *sql.Tx) error { _, e := tx.Exec(`UPDATE push_jobs SET state='queued'`); return e }); e == nil {
				t.Fatal("revived")
			}
		})
	}
}

func TestFencesAndPolicyResultSeparation(t *testing.T) {
	for _, mode := range []string{"release", "takeover", "lease-expiry", "expiry", "registration", "settings", "session", "instance", "dnd", "read"} {
		t.Run(mode, func(t *testing.T) {
			f, h, n := prepared(t)
			r := push.NewJobRepository(f.store)
			ticket := begin(t, f, h, n)
			switch mode {
			case "release":
				if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
					t.Fatal(e)
				}
			case "takeover":
				held, e := r.ReserveOPushAlerts(ctx, n.Add(2*time.Minute), time.Minute, 1)
				if e != nil || len(held) != 1 {
					t.Fatal(e)
				}
			case "lease-expiry":
				n = n.Add(time.Minute)
			case "expiry":
				n = h.Job.Snapshot.ExpiresAt
			case "registration":
				exec(t, f.store, `UPDATE device_push_registrations SET registration_version=2`)
			case "settings":
				exec(t, f.store, `UPDATE device_notification_settings SET settings_version=2`)
			case "session":
				exec(t, f.store, `UPDATE sessions SET revoked=1`)
			case "instance":
				exec(t, f.store, `UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'`)
				// Store 绑定启动实例；重开新实例后旧票据仍必须失效。
				f.store.Close()
				var err error
				f.store, err = storage.Open(ctx, f.cfg)
				if err != nil {
					t.Fatal(err)
				}
				r = push.NewJobRepository(f.store)
			case "dnd":
				f.configure(t, strings.ReplaceAll(strings.Replace(configuration(), `"enabled":false`, `"enabled":true`, 1), `"22:00","end":"07:00"`, fmt.Sprintf(`"%s","end":"%s"`, n.Format("15:04"), n.Add(time.Hour).Format("15:04"))))
			case "read":
				exec(t, f.store, `INSERT INTO bot_read_states(bot_id,read_seq,updated_at) VALUES(?,?,?) ON CONFLICT(bot_id) DO UPDATE SET read_seq=excluded.read_seq`, h.Job.Snapshot.BotID, h.Job.Snapshot.BotSeq, n.Format(stamp))
			}
			before := database(t, f.store)
			if ok, e := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); e != nil || ok {
				t.Fatal("check", ok, e)
			}
			if database(t, f.store) != before {
				t.Fatal("check wrote")
			}
			ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n)
			want := mode == "dnd" || mode == "read"
			if e != nil || ok != want {
				t.Fatal("result", ok, e, want)
			}
			if !want && database(t, f.store) != before {
				t.Fatal("fence wrote")
			}
		})
	}
}

func TestCrossStoreSyncHolderAndReopen(t *testing.T) {
	f, h, n := prepared(t)
	r := push.NewJobRepository(f.store)
	ticket := begin(t, f, h, n)
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, n.Format(stamp))
	j := h.Job.Snapshot
	j.ID = "sync-holder"
	j.Purpose = "sync"
	j.Provider = "fcm"
	j.AlertEligible = false
	if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
		t.Fatal(e)
	}
	other, e := storage.Open(ctx, f.cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	r2 := push.NewJobRepository(other)
	if held, e := r2.ReserveSync(ctx, n, time.Minute, 1); e != nil || len(held) != 0 {
		t.Fatal("mutex", e, len(held))
	}
	if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
		t.Fatal(e)
	}
	held, e := r2.ReserveSync(ctx, n, time.Minute, 1)
	if e != nil || len(held) != 1 {
		t.Fatal(e, len(held))
	}
	if ok, e := r.Release(ctx, h.Lease, n); e != nil || ok {
		t.Fatal("old release", e)
	}
	if ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n); e != nil || ok {
		t.Fatal("late result", e)
	}
	before := database(t, f.store)
	f.store.Close()
	f.store, e = storage.Open(ctx, f.cfg)
	if e != nil {
		t.Fatal(e)
	}
	if before != database(t, f.store) {
		t.Fatal("reopen changed")
	}
	if held, e := push.NewJobRepository(f.store).ReserveOPushAlerts(ctx, n, time.Minute, 1); e != nil || len(held) != 0 {
		t.Fatal("sync holder", e)
	}
}

func TestCancelledAndNegativeInputs(t *testing.T) {
	f, h, n := prepared(t)
	r := push.NewJobRepository(f.store)
	before := database(t, f.store)
	c, cancel := context.WithCancel(ctx)
	cancel()
	if x, e := r.BeginAlert(c, h.Lease, n); e == nil || x != nil {
		t.Fatal(x, e)
	}
	if x, e := r.BeginAlert(nil, h.Lease, n); !errors.Is(e, push.ErrJobInput) || x != nil {
		t.Fatal(x, e)
	}
	if !reflect.DeepEqual(before, database(t, f.store)) {
		t.Fatal("cancel wrote")
	}
}

// 两个 Store 的同 lease 竞争只能发布一张票据，不依赖进程内 repo 锁。
func TestConcurrentBeginAcrossStores(t *testing.T) {
	f, h, n := prepared(t)
	other, e := storage.Open(ctx, f.cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan *push.AlertAttempt, 2)
	errs := make(chan error, 2)
	for _, s := range []*storage.Store{f.store, other} {
		wg.Add(1)
		go func(s *storage.Store) {
			defer wg.Done()
			<-start
			x, e := push.NewJobRepository(s).BeginAlert(ctx, h.Lease, n)
			results <- x
			errs <- e
		}(s)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	count := 0
	for x := range results {
		if x != nil {
			count++
		}
	}
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if count != 1 || read(t, f, h.Job.Snapshot.ID).Attempt != 1 {
		t.Fatal("duplicate ticket", count)
	}
}

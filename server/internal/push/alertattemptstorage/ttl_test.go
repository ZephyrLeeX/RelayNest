package alertattemptstorage_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

// 长原期限、长租约和长会话排除其他 guard 的假阴性，快照原期限不得被重写。
func TestLongOriginalExpiryStillHas24HourAttemptLimit(t *testing.T) {
	long := func(t *testing.T) (*fixture, push.JobReservation, time.Time) {
		t.Helper()
		f, h, n := prepared(t)
		r := push.NewJobRepository(f.store)
		if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
			t.Fatal(e)
		}
		exec(t, f.store, `DELETE FROM push_jobs`)
		j := h.Job.Snapshot
		j.ID = "long-ttl"
		j.ExpiresAt = j.CreatedAt.Add(48 * time.Hour)
		exec(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=?`, j.CreatedAt.Add(72*time.Hour).UnixNano(), j.CreatedAt.Add(72*time.Hour).UnixNano())
		if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
			t.Fatal(e)
		}
		held, e := r.ReserveOPushAlerts(ctx, n, 49*time.Hour, 1)
		if e != nil || len(held) != 1 {
			t.Fatal(e)
		}
		return f, held[0], j.CreatedAt.Add(24 * time.Hour)
	}
	guards := func(t *testing.T, f *fixture, h push.JobReservation, n time.Time) {
		t.Helper()
		if e := f.store.Read(ctx, func(tx *sql.Tx) error {
			checked, e := push.CheckAlertLeaseTx(ctx, tx, h.Lease, n)
			if e != nil {
				return e
			}
			if checked == nil {
				return fmt.Errorf("lease guard false")
			}
			ok, e := push.CheckAlertPolicyTx(ctx, tx, h.Job.Snapshot, n)
			if e != nil {
				return e
			}
			if !ok {
				return fmt.Errorf("policy guard false")
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	t.Run("before", func(t *testing.T) {
		f, h, limit := long(t)
		r := push.NewJobRepository(f.store)
		n := limit.Add(-time.Second)
		guards(t, f, h, n)
		ticket := begin(t, f, h, n)
		if ok, e := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); e != nil || !ok {
			t.Fatal(ok, e)
		}
		if ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n); e != nil || !ok {
			t.Fatal(ok, e)
		}
		if read(t, f, h.Job.Snapshot.ID).Snapshot != h.Job.Snapshot {
			t.Fatal("original expiry rewritten")
		}
	})
	for _, offset := range []time.Duration{0, time.Nanosecond} {
		t.Run(fmt.Sprint("begin-boundary-", offset), func(t *testing.T) {
			f, h, limit := long(t)
			n := limit.Add(offset)
			guards(t, f, h, n)
			before := database(t, f.store)
			if x, e := push.NewJobRepository(f.store).BeginAlert(ctx, h.Lease, n); e != nil || x != nil {
				t.Fatal(x, e)
			}
			if database(t, f.store) != before || read(t, f, h.Job.Snapshot.ID).Attempt != 0 {
				t.Fatal("late begin wrote")
			}
		})
		t.Run(fmt.Sprint("ticket-boundary-", offset), func(t *testing.T) {
			f, h, limit := long(t)
			r := push.NewJobRepository(f.store)
			ticket := begin(t, f, h, limit.Add(-time.Second))
			n := limit.Add(offset)
			guards(t, f, h, n)
			before := database(t, f.store)
			if ok, e := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); e != nil || ok {
				t.Fatal(ok, e)
			}
			if ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n); e != nil || ok {
				t.Fatal(ok, e)
			}
			if database(t, f.store) != before {
				t.Fatal("expired ticket wrote")
			}
		})
	}
	for _, out := range []push.AlertOutcome{push.AlertTransient, push.AlertRateLimited, push.AlertUnknownDelivery} {
		for _, seconds := range []int{10, 9} {
			t.Run(fmt.Sprint(out, "/", seconds), func(t *testing.T) {
				f, h, limit := long(t)
				n := limit.Add(-time.Duration(seconds) * time.Second)
				guards(t, f, h, n)
				ticket := begin(t, f, h, n)
				r := push.NewJobRepository(f.store)
				if ok, e := r.ApplyAlertResult(ctx, *ticket, out, n); e != nil || !ok {
					t.Fatal(ok, e)
				}
				if got := read(t, f, h.Job.Snapshot.ID); got.State != "degraded" || got.Snapshot != h.Job.Snapshot {
					t.Fatal("retry passed 24h", got)
				}
			})
		}
	}
}

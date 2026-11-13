package alertattemptstorage_test

import (
	"database/sql"
	"math"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

func TestAttemptAndNanosecondOverflowFailClosed(t *testing.T) {
	t.Run("attempt", func(t *testing.T) {
		f, h, n := prepared(t)
		exec(t, f.store, `UPDATE push_jobs SET attempt=?`, int64(math.MaxInt64))
		before := database(t, f.store)
		if x, e := push.NewJobRepository(f.store).BeginAlert(ctx, h.Lease, n); e == nil || x != nil {
			t.Fatal("overflow ticket", x, e)
		}
		if database(t, f.store) != before {
			t.Fatal("overflow wrote")
		}
	})
	t.Run("retry", func(t *testing.T) {
		f, h, n := prepared(t)
		r := push.NewJobRepository(f.store)
		if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
			t.Fatal(e)
		}
		exec(t, f.store, `DELETE FROM push_jobs`)
		n = time.Unix(0, math.MaxInt64-int64(2*time.Second)).UTC()
		j := h.Job.Snapshot
		j.ID = "overflow"
		j.CreatedAt = n
		j.ExpiresAt = time.Unix(0, math.MaxInt64).UTC()
		exec(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=?`, int64(math.MaxInt64), int64(math.MaxInt64))
		if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
			t.Fatal(e)
		}
		held, e := r.ReserveOPushAlerts(ctx, n, time.Second, 1)
		if e != nil || len(held) != 1 {
			t.Fatal(e)
		}
		ticket := begin(t, f, held[0], n)
		if ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertTransient, n); e != nil || !ok {
			t.Fatal(e)
		}
		if got := read(t, f, j.ID); got.State != "degraded" || !got.NextAttemptAt.Equal(n) {
			t.Fatal("overflow retry", got)
		}
	})
}

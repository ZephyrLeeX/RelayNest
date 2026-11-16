package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/messages"
	"relaynest/server/internal/push"
)

type schedulerSyncDispatch func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error)

func (d schedulerSyncDispatch) Dispatch(c context.Context, e push.SyncEnvelope) (push.SyncOutcome, error) {
	return d(c, e)
}

// 复用原消息仓储的首次创建事务，同时生成两个真实用途的待办。
func schedulerProduce(t *testing.T, f *fixture) (syncID, alertID string) {
	t.Helper()
	f.serial++
	m, err := f.repository(func(c context.Context, tx *sql.Tx, m messages.Created) error {
		if err := push.EnqueueSyncTx(c, tx, event(m)); err != nil {
			return err
		}
		return push.EnqueueAlertTx(c, tx, event(m))
	}).CreateIdempotentApplication(ctx, f.identity, fmt.Sprintf("00000000-0000-4000-8000-%012d", f.serial), messages.CreateRequest{Format: "plain", Text: "atomic"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for id := range rows(t, f.store) {
		j := read(t, f, id)
		if j.Snapshot.MessageID != m.Message.ID {
			continue
		}
		switch j.Snapshot.Purpose {
		case "sync":
			syncID = id
		case "alert":
			alertID = id
		}
	}
	if syncID == "" || alertID == "" {
		t.Fatal("producer did not create both purposes", syncID, alertID)
	}
	return
}
func schedulerFixture(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, time.Now().UTC().Format(stamp))
	return f
}
func schedulerWorkers(f *fixture, sc, ac func() time.Time, sd schedulerSyncDispatch, ad opushDispatch) (*push.SyncWorker, *push.OPushAlertWorker) {
	r := push.NewJobRepository(f.store)
	return push.NewSyncWorker(r, sc, time.Minute, sd), push.NewOPushAlertWorker(r, ac, time.Minute, ad)
}
func schedulerReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("scheduler did not reach expected boundary")
		var zero T
		return zero
	}
}
func schedulerDone(t *testing.T, ch <-chan error) {
	t.Helper()
	if err := schedulerReceive(t, ch); !errors.Is(err, context.Canceled) {
		t.Fatal("Run cancellation", err)
	}
}

func TestPushSchedulerSerialApplyAndFullInterval(t *testing.T) {
	f := schedulerFixture(t)
	sid, aid := schedulerProduce(t, f)
	now := time.Now().UTC()
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	var order []string
	clocks := 0
	const interval = 20 * time.Millisecond
	var workEnded, nextCycle time.Time
	s, a := schedulerWorkers(f, func() time.Time {
		clocks++
		if clocks == 5 {
			nextCycle = time.Now()
			cancel()
		}
		return now
	}, func() time.Time { return now }, func(_ context.Context, e push.SyncEnvelope) (push.SyncOutcome, error) {
		order = append(order, e.Purpose)
		if e.JobKey != sid {
			t.Fatal("wrong sync candidate")
		}
		return push.SyncAccepted, nil
	}, func(_ context.Context, e push.OPushAlertEnvelope) (push.AlertOutcome, error) {
		order = append(order, e.Purpose)
		if e.JobKey != aid || read(t, f, sid).State != "accepted" {
			t.Fatal("opush started before sync Apply")
		}
		// 故意让工作超过一个 interval，验证使用完成后的完整间隔而不是积压 tick。
		time.Sleep(2 * interval)
		workEnded = time.Now()
		return push.AlertAccepted, nil
	})
	if err := push.NewPushScheduler(s, a, interval).Run(c); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"sync", "alert"}) || nextCycle.Sub(workEnded) < interval {
		t.Fatal("ordering or complete interval", order, nextCycle.Sub(workEnded))
	}
	for _, id := range []string{sid, aid} {
		j := read(t, f, id)
		if j.State != "accepted" || j.Attempt != 1 || opushKind(t, f, id) != "accepted" {
			t.Fatal("real Apply not committed", j)
		}
	}
}

func TestPushSchedulerDiscoversDatabaseAfterEmptyCycle(t *testing.T) {
	f := schedulerFixture(t)
	other := opushOther(t, f)
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	cycle, proceed := make(chan struct{}), make(chan struct{})
	var clocks int
	var syncCalls, alertCalls atomic.Int32
	now := time.Now().UTC()
	s, a := schedulerWorkers(f, func() time.Time {
		clocks++
		if clocks == 2 {
			close(cycle)
			<-proceed
		}
		if clocks == 6 {
			cancel()
		}
		return now
	}, func() time.Time { return now }, func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
		syncCalls.Add(1)
		return push.SyncAccepted, nil
	}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
		alertCalls.Add(1)
		return push.AlertAccepted, nil
	})
	done := make(chan error, 1)
	go func() { done <- push.NewPushScheduler(s, a, time.Millisecond).Run(c) }()
	schedulerReceive(t, cycle) // 第二周期已开始，第一周期没有内存待办。
	copy := *f
	copy.store = other
	sid, aid := schedulerProduce(t, &copy)
	now = time.Now().UTC() // clock 被 proceed 隔开，避免并发改时钟。
	close(proceed)
	schedulerDone(t, done)
	if syncCalls.Load() != 1 || alertCalls.Load() != 1 || read(t, f, sid).State != "accepted" || read(t, f, aid).State != "accepted" {
		t.Fatal("later database work not discovered", syncCalls.Load(), alertCalls.Load())
	}
}

func TestPushSchedulerBlockedCancellationAndReentry(t *testing.T) {
	for _, cooperative := range []bool{false, true} {
		t.Run(fmt.Sprint(cooperative), func(t *testing.T) {
			f := schedulerFixture(t)
			sid, aid := schedulerProduce(t, f)
			other := opushOther(t, f)
			now := time.Now().UTC()
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			var syncCalls, alertCalls atomic.Int32
			s, a := schedulerWorkers(f, func() time.Time { return now }, func() time.Time { return now }, func(c context.Context, _ push.SyncEnvelope) (push.SyncOutcome, error) {
				syncCalls.Add(1)
				close(entered)
				if cooperative {
					<-c.Done()
				} else {
					<-release
				}
				return push.SyncAccepted, nil
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				alertCalls.Add(1)
				return push.AlertAccepted, nil
			})
			scheduler := push.NewPushScheduler(s, a, time.Hour)
			done := make(chan error, 1)
			go func() { done <- scheduler.Run(c) }()
			schedulerReceive(t, entered)
			if err := scheduler.Run(ctx); !errors.Is(err, push.ErrJobInput) {
				t.Fatal("concurrent Run accepted", err)
			}
			// 真写事务证明 adapter 没有占用数据库写锁；原 holder 仍排斥另一用途。
			exec(t, other, `UPDATE device_push_registrations SET token='rotated-synthetic'`)
			if held, err := push.NewJobRepository(other).ReserveOPushAlerts(ctx, now, time.Minute, 1); err != nil || len(held) != 0 {
				t.Fatal("same-device holder bypass", held, err)
			}
			before := rows(t, other)
			cancel()
			if !cooperative {
				select {
				case err := <-done:
					t.Fatal("Run returned while adapter remained in flight", err)
				case <-time.After(20 * time.Millisecond):
				}
				close(release)
			}
			schedulerDone(t, done)
			if syncCalls.Load() != 1 || alertCalls.Load() != 0 || !reflect.DeepEqual(before, rows(t, other)) || read(t, f, sid).State != "leased" || read(t, f, aid).State != "queued" {
				t.Fatal("cancel retried, applied or changed holder")
			}
			// 取消退出后的再次启动不应被遗留 running 标记挡住。
			again, stop := context.WithCancel(ctx)
			stop()
			if err := scheduler.Run(again); !errors.Is(err, context.Canceled) {
				t.Fatal("running slot leaked", err)
			}
		})
	}
}

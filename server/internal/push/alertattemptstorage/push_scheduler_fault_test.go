package alertattemptstorage_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

func TestPushSchedulerInputAndCancellationBoundaries(t *testing.T) {
	f := schedulerFixture(t)
	schedulerProduce(t, f)
	now := time.Now().UTC()
	var clocks, calls atomic.Int32
	clock := func() time.Time { clocks.Add(1); return now }
	sd := schedulerSyncDispatch(func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
		calls.Add(1)
		return push.SyncAccepted, nil
	})
	ad := opushDispatch(func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
		calls.Add(1)
		return push.AlertAccepted, nil
	})
	s, a := schedulerWorkers(f, clock, clock, sd, ad)
	r := push.NewJobRepository(f.store)
	for _, tc := range []struct {
		name string
		s    *push.PushScheduler
	}{
		{"nil", nil}, {"nil-sync", push.NewPushScheduler(nil, a, time.Second)},
		{"nil-opush", push.NewPushScheduler(s, nil, time.Second)},
		{"zero-interval", push.NewPushScheduler(s, a, 0)}, {"negative-interval", push.NewPushScheduler(s, a, -time.Second)},
		{"empty-sync", push.NewPushScheduler(&push.SyncWorker{}, a, time.Second)},
		{"empty-opush", push.NewPushScheduler(s, &push.OPushAlertWorker{}, time.Second)},
		{"missing-sync-repository", push.NewPushScheduler(push.NewSyncWorker(nil, clock, time.Minute, sd), a, time.Second)},
		{"missing-opush-clock", push.NewPushScheduler(s, push.NewOPushAlertWorker(r, nil, time.Minute, ad), time.Second)},
		{"missing-opush-adapter", push.NewPushScheduler(s, push.NewOPushAlertWorker(r, clock, time.Minute, nil), time.Second)},
		{"invalid-opush-ttl", push.NewPushScheduler(s, push.NewOPushAlertWorker(r, clock, 0, ad), time.Second)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := database(t, f.store)
			if err := tc.s.Run(ctx); !errors.Is(err, push.ErrJobInput) || clocks.Load() != 0 || calls.Load() != 0 || before != database(t, f.store) {
				t.Fatal("invalid scheduler had effects", err, clocks.Load(), calls.Load())
			}
		})
	}
	scheduler := push.NewPushScheduler(s, a, time.Hour)
	if err := scheduler.Run(nil); !errors.Is(err, push.ErrJobInput) || clocks.Load() != 0 || calls.Load() != 0 {
		t.Fatal("nil context", err)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if err := scheduler.Run(c); !errors.Is(err, context.Canceled) || clocks.Load() != 0 || calls.Load() != 0 {
		t.Fatal("canceled start", err)
	}

	t.Run("between-steps", func(t *testing.T) {
		f := schedulerFixture(t)
		sid, aid := schedulerProduce(t, f)
		now := time.Now().UTC()
		c, cancel := context.WithCancel(ctx)
		defer cancel()
		n, afterCalls := 0, 0
		s, a := schedulerWorkers(f, func() time.Time {
			n++
			if n == 4 {
				cancel() // sync adapter 已返回，但取消阻止 Apply 及后一 Step。
			}
			return now
		}, func() time.Time { afterCalls++; return now }, func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
			return push.SyncAccepted, nil
		}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
			t.Fatal("opush started after cancellation")
			return push.AlertAccepted, nil
		})
		if err := push.NewPushScheduler(s, a, time.Hour).Run(c); !errors.Is(err, context.Canceled) || afterCalls != 0 || read(t, f, sid).State != "leased" || read(t, f, aid).Attempt != 0 {
			t.Fatal("cancellation boundary", err, afterCalls)
		}
	})

	t.Run("waiting", func(t *testing.T) {
		f := schedulerFixture(t)
		c, cancel := context.WithCancel(ctx)
		defer cancel()
		last := make(chan struct{})
		var sc, ac atomic.Int32
		now := time.Now().UTC()
		s, a := schedulerWorkers(f, func() time.Time { sc.Add(1); return now }, func() time.Time {
			ac.Add(1)
			close(last)
			return now
		}, sd, ad)
		scheduler := push.NewPushScheduler(s, a, time.Hour)
		done := make(chan error, 1)
		go func() { done <- scheduler.Run(c) }()
		schedulerReceive(t, last)
		// 让空队列的末步完成并进入长间隔；取消不能等待完整一小时。
		time.Sleep(20 * time.Millisecond)
		cancel()
		schedulerDone(t, done)
		if sc.Load() != 1 || ac.Load() != 1 {
			t.Fatal("extra cycle during cancel", sc.Load(), ac.Load())
		}
	})
}

func TestPushSchedulerStorageErrorStopsAndRestarts(t *testing.T) {
	for _, scope := range []string{"sync", "opush"} {
		t.Run(scope, func(t *testing.T) {
			f := schedulerFixture(t)
			sid, aid := schedulerProduce(t, f)
			id := sid
			if scope == "opush" {
				id = aid
			}
			// 真正 Reserve 写语句失败；不以 adapter 错误冒充 Step 错误。
			exec(t, f.store, fmt.Sprintf(`CREATE TRIGGER scheduler_fault BEFORE UPDATE OF lease_hash ON push_jobs WHEN NEW.id='%s' BEGIN SELECT RAISE(ABORT,'scheduler storage fault'); END`, id))
			now := time.Now().UTC()
			sc, ac := 0, 0
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			s, a := schedulerWorkers(f, func() time.Time {
				if ac != 0 {
					cancel()
				}
				return now
			}, func() time.Time { return now }, func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
				sc++
				return push.SyncAccepted, nil
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				ac++
				return push.AlertAccepted, nil
			})
			scheduler := push.NewPushScheduler(s, a, time.Millisecond)
			err := scheduler.Run(ctx)
			if err == nil || !strings.Contains(err.Error(), "scheduler storage fault") || ac != 0 || sc != map[string]int{"sync": 0, "opush": 1}[scope] {
				t.Fatal("storage error did not stop sequence", err, sc, ac)
			}
			if read(t, f, id).State != "queued" || read(t, f, id).Attempt != 0 {
				t.Fatal("failed Reserve partially committed")
			}
			exec(t, f.store, `DROP TRIGGER scheduler_fault`)
			if err := scheduler.Run(c); !errors.Is(err, context.Canceled) || sc != 1 || ac != 1 || read(t, f, sid).State != "accepted" || read(t, f, aid).State != "accepted" {
				t.Fatal("error occupancy or database recovery", err, sc, ac)
			}
		})
	}
}

func TestPushSchedulerPanicReleasesRun(t *testing.T) {
	for _, scope := range []string{"sync", "opush"} {
		t.Run(scope, func(t *testing.T) {
			f := schedulerFixture(t)
			schedulerProduce(t, f)
			now := time.Now().UTC()
			sc, ac := 0, 0
			s, a := schedulerWorkers(f, func() time.Time { return now }, func() time.Time { return now }, func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
				sc++
				if scope == "sync" {
					panic("synthetic scheduler panic")
				}
				return push.SyncAccepted, nil
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				ac++
				panic("synthetic scheduler panic")
			})
			scheduler := push.NewPushScheduler(s, a, time.Hour)
			func() {
				defer func() {
					if recover() != "synthetic scheduler panic" {
						t.Fatal("panic not propagated")
					}
				}()
				_ = scheduler.Run(ctx)
				t.Fatal("panic swallowed")
			}()
			if sc != 1 || ac != map[string]int{"sync": 0, "opush": 1}[scope] {
				t.Fatal("extra dispatch after panic", sc, ac)
			}
			before := database(t, f.store)
			c, cancel := context.WithCancel(ctx)
			cancel()
			if err := scheduler.Run(c); !errors.Is(err, context.Canceled) || before != database(t, f.store) {
				t.Fatal("panic left running occupancy or changed recovery", err)
			}
		})
	}
}

func TestPushSchedulerCheckDenialKeepsExistingRecovery(t *testing.T) {
	for _, mode := range []string{"dnd", "read"} {
		t.Run(mode, func(t *testing.T) {
			f, j, now := opushSource(t)
			other := opushOther(t, f)
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			phases, sc, ac := 0, 0, 0
			var before string
			s, a := schedulerWorkers(f, func() time.Time {
				sc++
				if sc == 2 {
					cancel()
				}
				return now
			}, func() time.Time {
				phases++
				if phases == 3 {
					if mode == "dnd" {
						opushDND(t, f, now)
					} else {
						exec(t, other, `INSERT INTO bot_read_states(bot_id,read_seq,updated_at)VALUES(?,?,?) ON CONFLICT(bot_id) DO UPDATE SET read_seq=excluded.read_seq`, j.BotID, j.BotSeq, now.Format(stamp))
					}
					before = database(t, f.store)
				}
				return now
			}, func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
				t.Fatal("unexpected sync candidate")
				return push.SyncAccepted, nil
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				ac++
				return push.AlertAccepted, nil
			})
			if err := push.NewPushScheduler(s, a, time.Millisecond).Run(c); !errors.Is(err, context.Canceled) || ac != 0 || before != database(t, f.store) {
				t.Fatal("scheduler changed readonly refusal", err, ac)
			}
			held := read(t, f, j.ID)
			if held.State != "leased" || held.Attempt != 1 || held.Snapshot != j {
				t.Fatal("Check denial performed extra Drop", held)
			}
			if mode == "dnd" {
				f.configure(t, configuration())
			} else {
				exec(t, other, `UPDATE bot_read_states SET read_seq=0`)
			}
			now = now.Add(time.Minute)
			c, cancel = context.WithCancel(ctx)
			defer cancel()
			syncClocks := 0
			s, a = schedulerWorkers(f, func() time.Time {
				syncClocks++
				if syncClocks == 2 {
					cancel()
				}
				return now
			}, func() time.Time { return now }, func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error) {
				t.Fatal("unexpected sync candidate")
				return push.SyncAccepted, nil
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				ac++
				return push.AlertAccepted, nil
			})
			if err := push.NewPushScheduler(s, a, time.Millisecond).Run(c); !errors.Is(err, context.Canceled) || ac != 1 || read(t, f, j.ID).State != "accepted" || !reflect.DeepEqual(read(t, f, j.ID).Snapshot, j) {
				t.Fatal("existing expiry recovery changed", err, ac)
			}
		})
	}
}

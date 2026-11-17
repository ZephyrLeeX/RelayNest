package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

// 经过真实 Producer／Reserve／Begin／Drop，只允许目标三列发生永久终止变化。
func TestOPushAlertWorkerSourceReadPersistent(t *testing.T) {
	for _, reason := range []string{"missing", "read", "above"} {
		t.Run(reason, func(t *testing.T) {
			f, j, n := opushSource(t)
			other := opushOther(t, f)
			if reason == "missing" {
				exec(t, other, `CREATE TABLE saved_source AS SELECT * FROM messages`)
			}
			if reason == "above" {
				f.source(t)
			}
			clocks, calls := 0, 0
			var before string
			var original map[string][]any
			w := opushStep(f, func() time.Time {
				clocks++
				if clocks == 2 {
					if reason == "missing" {
						exec(t, other, `DELETE FROM messages WHERE id=?`, j.MessageID)
					} else {
						seq := j.BotSeq
						if reason == "above" {
							seq++
						}
						sourceReadAdvance(t, f, seq)
					}
				}
				if clocks == 3 {
					before, original = database(t, other), rows(t, other)
				}
				return n.Add(time.Duration(clocks-1) * time.Second).In(time.FixedZone("source-read", 8*3600))
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				return push.AlertAccepted, nil
			})
			got, err := w.Step(ctx)
			if err != nil || got != (push.OPushAlertStepResult{}) || clocks != 3 || calls != 0 {
				t.Fatal("termination", got, err, clocks, calls)
			}
			oldRow := fmt.Sprintf("%#v", original[j.ID])
			sourceReadProjection(t, f, j.ID, original)
			want := strings.Replace(before, oldRow, fmt.Sprintf("%#v", original[j.ID]), 1)
			if want == before || database(t, other) != want || read(t, f, j.ID).Attempt != 0 {
				t.Fatal("unexpected persistent write")
			}
			// 人工恢复源／未读并超过原租约；终止标记不能因此复活。
			if reason == "missing" {
				exec(t, other, `INSERT INTO messages SELECT * FROM saved_source`)
			} else {
				exec(t, other, `UPDATE bot_read_states SET read_seq=0`)
			}
			n = n.Add(2 * time.Minute)
			before = database(t, other)
			for range 2 {
				if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || calls != 0 {
					t.Fatal("revived worker", again, e, calls)
				}
				if held, e := push.NewJobRepository(other).ReserveOPushAlerts(ctx, n, time.Minute, 1); e != nil || len(held) != 0 {
					t.Fatal("revived reservation", len(held), e)
				}
			}
			if database(t, other) != before {
				t.Fatal("terminal retry wrote")
			}
		})
	}
}

// 第三次 clock 才注入变化，证明 Drop 不缓存 Begin 的已读原因或旧快照。
func TestOPushAlertWorkerSourceReadRechecks(t *testing.T) {
	for _, mode := range []string{"unread", "mismatch", "expected", "registration", "settings", "revoked", "generation", "client", "lease", "takeover", "lease-expiry", "expiry", "effective24h", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			f, j, n := opushSource(t)
			other := opushOther(t, f)
			ttl := time.Minute
			if mode == "effective24h" {
				ttl = 49 * time.Hour
				exec(t, other, `DELETE FROM push_jobs`)
				j.ExpiresAt = j.CreatedAt.Add(48 * time.Hour)
				if err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); err != nil {
					t.Fatal(err)
				}
				exec(t, other, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=?`, j.CreatedAt.Add(72*time.Hour).UnixNano(), j.CreatedAt.Add(72*time.Hour).UnixNano())
			}
			clocks, calls := 0, 0
			var before string
			w := push.NewOPushAlertWorker(push.NewJobRepository(f.store), func() time.Time {
				clocks++
				if clocks == 2 {
					if mode == "disabled" {
						f.configure(t, strings.Replace(configuration(), `"enabled":true`, `"enabled":false`, 1))
					} else {
						sourceReadAdvance(t, f, j.BotSeq)
					}
				}
				if clocks == 3 {
					switch mode {
					case "unread":
						exec(t, other, `UPDATE bot_read_states SET read_seq=0`)
					case "mismatch":
						exec(t, other, `UPDATE messages SET bot_seq=bot_seq+1`)
					case "expected":
						// 仅本对抗 fixture 绕过不可变触发器，核完整 expected 的独立守卫。
						exec(t, other, `DROP TRIGGER push_jobs_snapshot_immutable`)
						exec(t, other, `UPDATE push_jobs SET target_change_seq=target_change_seq+1`)
					case "registration":
						exec(t, other, `UPDATE device_push_registrations SET registration_version=2`)
					case "settings":
						exec(t, other, `UPDATE device_notification_settings SET settings_version=2`)
					case "revoked":
						exec(t, other, `UPDATE sessions SET revoked=1`)
					case "generation":
						exec(t, other, `UPDATE administrator SET auth_generation=2`)
					case "client":
						exec(t, other, `UPDATE sessions SET client_type='web'`)
					case "lease":
						exec(t, other, `UPDATE push_jobs SET lease_hash=zeroblob(32)`)
					case "takeover":
						n = n.Add(time.Minute)
						h, e := push.NewJobRepository(other).ReserveOPushAlerts(ctx, n, time.Minute, 1)
						if e != nil || len(h) != 1 {
							t.Fatal("takeover", len(h), e)
						}
					case "lease-expiry":
						n = n.Add(time.Minute)
					case "expiry":
						n = j.ExpiresAt
					case "effective24h":
						n = j.CreatedAt.Add(24 * time.Hour)
					}
					before = database(t, other)
				}
				return n
			}, ttl, opushDispatch(func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				return push.AlertAccepted, nil
			}))
			got, err := w.Step(ctx)
			if err != nil || got != (push.OPushAlertStepResult{}) || calls != 0 || clocks != 3 || database(t, other) != before {
				t.Fatal("false Drop wrote or dispatched", got, err, clocks, calls)
			}
			if after := read(t, f, j.ID); after.State != "leased" || after.Attempt != 0 {
				t.Fatal("false Drop changed attempt/lease", after)
			}
			if mode == "unread" || mode == "disabled" {
				f.configure(t, configuration())
				n = n.Add(time.Minute)
				if again, e := w.Step(ctx); e != nil || !again.Dispatched || !again.Applied || calls != 1 {
					t.Fatal("false Drop lost recovery", again, e, calls)
				}
			}
		})
	}
}

// 故障必须发生在 Worker 真正到达的 Drop 写事务内，且错误不能被吞掉。
func TestOPushAlertWorkerSourceReadAtomicFaults(t *testing.T) {
	for _, fault := range []string{"abort", "ignore", "commit", "cancel", "sql"} {
		t.Run(fault, func(t *testing.T) {
			f, j, n := opushSource(t)
			other := opushOther(t, f)
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			clocks, calls := 0, 0
			var before string
			w := opushStep(f, func() time.Time {
				clocks++
				if clocks == 2 {
					sourceReadAdvance(t, f, j.BotSeq)
				}
				if clocks == 3 {
					if fault == "sql" {
						exec(t, other, `ALTER TABLE messages RENAME TO unavailable_worker_source`)
					} else {
						op, body := "AFTER", ""
						switch fault {
						case "abort":
							op, body = "BEFORE", `SELECT RAISE(ABORT,'worker source read fault');`
						case "ignore":
							op, body = "BEFORE", `SELECT RAISE(IGNORE);`
						case "commit":
							exec(t, other, `CREATE TABLE worker_source_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
							body = `INSERT INTO worker_source_fk VALUES('missing-worker-source-bot');`
						}
						exec(t, other, `CREATE TRIGGER worker_source_fault `+op+` UPDATE OF state ON push_jobs WHEN NEW.state='suppressed' BEGIN SELECT relaynest_alert_attempt_cancel(); `+body+` END`)
						cancelState.Lock()
						cancelState.reached, cancelState.cancel = false, nil
						if fault == "cancel" {
							cancelState.cancel = cancel
						}
						cancelState.Unlock()
					}
					before = database(t, other)
				}
				return n
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				return push.AlertAccepted, nil
			})
			defer func() { cancelState.Lock(); cancelState.cancel = nil; cancelState.Unlock() }()
			got, err := w.Step(c)
			cancelState.Lock()
			reached := cancelState.reached
			cancelState.cancel = nil
			cancelState.Unlock()
			if err == nil || got != (push.OPushAlertStepResult{}) || clocks != 3 || calls != 0 || (fault != "sql" && !reached) {
				t.Fatal("Drop fault not reached/propagated", reached, got, err, clocks, calls)
			}
			if fault == "commit" || fault == "sql" || fault == "abort" {
				var code interface{ Code() int }
				want := map[string]int{"commit": 787, "sql": 1, "abort": 1811}[fault]
				if !errors.As(err, &code) || code.Code() != want {
					t.Fatal("actual SQLite error", err)
				}
			}
			if fault == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation changed", err)
			}
			if fault == "ignore" && !errors.Is(err, push.ErrJobData) {
				t.Fatal("IGNORE row guard changed", err)
			}
			if database(t, other) != before || read(t, f, j.ID).Attempt != 0 || read(t, f, j.ID).State != "leased" {
				t.Fatal("Drop fault partially committed")
			}
			if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || calls != 0 || database(t, other) != before {
				t.Fatal("Drop fault immediately retried", again, e, calls)
			}
			if fault == "sql" {
				exec(t, other, `ALTER TABLE unavailable_worker_source RENAME TO messages`)
			} else {
				exec(t, other, `DROP TRIGGER worker_source_fault`)
			}
			// 原租约依旧占用：到期后由新的 Worker 正常终止，作为同路径阴性对照。
			n = n.Add(time.Minute)
			clocks = 3
			if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || calls != 0 || read(t, f, j.ID).State != "suppressed" {
				t.Fatal("normal control after fault", again, e, calls)
			}
		})
	}
}

func TestOPushAlertWorkerSourceReadThirdClock(t *testing.T) {
	for _, mode := range []string{"equal", "fresh", "zero", "overflow", "backward", "cancel", "panic"} {
		t.Run(mode, func(t *testing.T) {
			f, j, n := opushSource(t)
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			clocks, calls := 0, 0
			var before string
			w := opushStep(f, func() time.Time {
				clocks++
				if clocks == 2 {
					sourceReadAdvance(t, f, j.BotSeq)
				}
				if clocks == 3 {
					before = database(t, f.store)
					if mode == "fresh" {
						return n.Add(time.Second).In(time.FixedZone("clock", -7*3600))
					}
					switch mode {
					case "zero":
						return time.Time{}
					case "overflow":
						return time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
					case "backward":
						return n.Add(-time.Nanosecond)
					case "cancel":
						cancel()
					case "panic":
						panic("source read clock panic")
					}
				}
				return n
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				return push.AlertAccepted, nil
			})
			var got push.OPushAlertStepResult
			var err error
			func() {
				if mode == "panic" {
					defer func() {
						if recover() != "source read clock panic" {
							t.Fatal("clock panic changed")
						}
					}()
				}
				got, err = w.Step(c)
				if mode == "panic" {
					t.Fatal("panic swallowed")
				}
			}()
			var want error
			if mode == "zero" || mode == "overflow" || mode == "backward" {
				want = push.ErrJobInput
			}
			if mode == "cancel" {
				want = context.Canceled
			}
			if !errors.Is(err, want) || got != (push.OPushAlertStepResult{}) || clocks != 3 || calls != 0 {
				t.Fatal("third clock", got, err, clocks, calls)
			}
			if mode == "equal" || mode == "fresh" {
				if read(t, f, j.ID).State != "suppressed" {
					t.Fatal("valid clock failed to terminate")
				}
			} else if database(t, f.store) != before {
				t.Fatal("bad clock wrote")
			}
		})
	}
}

// 旧调用者持有的真实租约和已提交票据不能复活 Worker 终止的提醒。
func TestOPushAlertWorkerSourceReadOldFence(t *testing.T) {
	f, h, n := prepared(t)
	r := push.NewJobRepository(f.store)
	ticket := begin(t, f, h, n)
	if ok, err := r.Release(ctx, h.Lease, n); err != nil || !ok {
		t.Fatal("release preparation", ok, err)
	}
	clocks := 0
	w := opushStep(f, func() time.Time {
		clocks++
		if clocks == 2 {
			sourceReadAdvance(t, f, h.Job.Snapshot.BotSeq)
		}
		return n
	}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
		t.Fatal("old fence dispatched")
		return push.AlertAccepted, nil
	})
	if got, err := w.Step(ctx); err != nil || got != (push.OPushAlertStepResult{}) || read(t, f, h.Job.Snapshot.ID).State != "suppressed" {
		t.Fatal("termination with old attempt", got, err)
	}
	before := database(t, f.store)
	if ok, err := r.Release(ctx, h.Lease, n); err != nil || ok {
		t.Fatal("old release", ok, err)
	}
	if ok, err := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); err != nil || ok {
		t.Fatal("old check", ok, err)
	}
	if ok, err := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n); err != nil || ok {
		t.Fatal("late result", ok, err)
	}
	if !reflect.DeepEqual(before, database(t, f.store)) {
		t.Fatal("old fence wrote")
	}
}

// nil 不是额外 DND 补偿的许可；耗尽分支也必须读第三个 clock 后按原租约 no-op。
func TestOPushAlertWorkerSourceReadOtherNilContracts(t *testing.T) {
	for _, mode := range []string{"exhausted", "dnd-after-disabled"} {
		t.Run(mode, func(t *testing.T) {
			f, j, n := opushSource(t)
			if mode == "exhausted" {
				exec(t, f.store, `UPDATE push_jobs SET attempt=8`)
			}
			clocks := 0
			var before string
			w := opushStep(f, func() time.Time {
				clocks++
				if clocks == 2 && mode == "dnd-after-disabled" {
					f.configure(t, strings.Replace(configuration(), `"enabled":true`, `"enabled":false`, 1))
				}
				if clocks == 3 {
					if mode == "dnd-after-disabled" {
						opushDND(t, f, n)
					}
					before = database(t, f.store)
				}
				return n
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				t.Fatal("nil branch dispatched")
				return push.AlertAccepted, nil
			})
			if got, err := w.Step(ctx); err != nil || got != (push.OPushAlertStepResult{}) || clocks != 3 || database(t, f.store) != before {
				t.Fatal("nil contract", got, err, clocks)
			}
			wantState, wantAttempt := "leased", int64(0)
			if mode == "exhausted" {
				wantState, wantAttempt = "degraded", 8
			}
			if after := read(t, f, j.ID); after.State != wantState || after.Attempt != wantAttempt {
				t.Fatal("nil branch changed prior contract", after)
			}
		})
	}
}

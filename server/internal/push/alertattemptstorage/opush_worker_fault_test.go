package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"modernc.org/sqlite"
	"relaynest/server/internal/push"
)

// 同值 UPDATE 先证明 trigger 命中且真正 COMMIT 返回延迟外键错误，不能用回调报错冒充。
func opushCommitFault(t *testing.T, f *fixture, id, column string) {
	t.Helper()
	exec(t, f.store, `CREATE TABLE opush_worker_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
	exec(t, f.store, `CREATE TRIGGER opush_worker_fault AFTER UPDATE OF `+column+` ON push_jobs BEGIN INSERT INTO opush_worker_fk VALUES('missing-opush-worker-bot'); END`)
	before := database(t, f.store)
	reached := false
	e := f.store.Write(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`UPDATE push_jobs SET `+column+`=`+column+` WHERE id=?`, id); e != nil {
			return e
		}
		var n int
		e := tx.QueryRow(`SELECT count(*) FROM opush_worker_fk`).Scan(&n)
		reached = n == 1
		return e
	})
	var se *sqlite.Error
	if !reached || !errors.As(e, &se) || se.Code() != 787 || before != database(t, f.store) {
		t.Fatal("real COMMIT control", reached, e)
	}
}
func TestOPushAlertWorkerCommitFailures(t *testing.T) {
	for _, stage := range []string{"reserve", "begin", "apply"} {
		t.Run(stage, func(t *testing.T) {
			f, j, n := opushSource(t)
			column := map[string]string{"reserve": "lease_hash", "begin": "attempt", "apply": "result_kind"}[stage]
			if stage != "apply" {
				opushCommitFault(t, f, j.ID, column)
			}
			calls := 0
			w := opushStep(f, func() time.Time { return n }, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				opushCommitFault(t, f, j.ID, column)
				return push.AlertAccepted, nil
			})
			got, e := w.Step(ctx)
			var se *sqlite.Error
			if !errors.As(e, &se) || se.Code() != 787 || got.Applied || got.Dispatched != (stage == "apply") {
				t.Fatal("commit failure", got, e)
			}
			after := read(t, f, j.ID)
			wantState := "leased"
			if stage == "reserve" {
				wantState = "queued"
			}
			wantAttempt := int64(0)
			if stage == "apply" {
				wantAttempt = 1
			}
			if after.State != wantState || after.Attempt != wantAttempt || opushKind(t, f, j.ID) != "" || calls != map[bool]int{false: 0, true: 1}[stage == "apply"] {
				t.Fatal("partial commit", after, calls)
			}
			if stage != "reserve" {
				before := database(t, f.store)
				if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || before != database(t, f.store) {
					t.Fatal("immediate retry after fault", again, e)
				}
			}
		})
	}
}
func opushDND(t *testing.T, f *fixture, n time.Time) {
	t.Helper()
	f.configure(t, strings.ReplaceAll(strings.Replace(configuration(), `"enabled":false`, `"enabled":true`, 1), `"22:00","end":"07:00"`, fmt.Sprintf(`"%s","end":"%s"`, n.Format("15:04"), n.Add(time.Hour).Format("15:04"))))
}
func TestOPushAlertWorkerBeginAndCheckDenied(t *testing.T) {
	for _, phase := range []int{2, 3} {
		for _, mode := range []string{"dnd", "read", "disabled", "registration", "settings", "revoked", "takeover", "source", "expiry", "error"} {
			t.Run(fmt.Sprintf("%d/%s", phase, mode), func(t *testing.T) {
				f, j, n := opushSource(t)
				other := opushOther(t, f)
				clocks, calls := 0, 0
				var before string
				clock := func() time.Time {
					clocks++
					if clocks == phase {
						switch mode {
						case "dnd":
							opushDND(t, f, n)
						case "read":
							exec(t, other, `INSERT INTO bot_read_states(bot_id,read_seq,updated_at)VALUES(?,?,?) ON CONFLICT(bot_id) DO UPDATE SET read_seq=excluded.read_seq`, j.BotID, j.BotSeq, n.Format(stamp))
						case "disabled":
							f.configure(t, strings.Replace(configuration(), `"enabled":true`, `"enabled":false`, 1))
						case "registration":
							exec(t, other, `UPDATE device_push_registrations SET registration_version=2`)
						case "settings":
							exec(t, other, `UPDATE device_notification_settings SET settings_version=2`)
						case "revoked":
							exec(t, other, `UPDATE sessions SET revoked=1`)
						case "source":
							exec(t, other, `DELETE FROM messages`)
						case "expiry":
							n = j.ExpiresAt
						case "takeover":
							n = n.Add(time.Minute)
							h, e := push.NewJobRepository(other).ReserveOPushAlerts(ctx, n, time.Minute, 1)
							if e != nil || len(h) != 1 {
								t.Fatal("takeover", len(h), e)
							}
						case "error":
							if e := f.store.Close(); e != nil {
								t.Fatal(e)
							}
						}
						if mode != "error" {
							before = database(t, f.store)
						}
					}
					return n
				}
				w := opushStep(f, clock, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
					calls++
					return push.AlertAccepted, nil
				})
				got, e := w.Step(ctx)
				if (e != nil) != (mode == "error") || got != (push.OPushAlertStepResult{}) || calls != 0 || clocks != phase {
					t.Fatal("denial", got, e, calls, clocks)
				}
				if mode == "error" {
					return
				}
				if phase == 2 && mode == "dnd" {
					after := read(t, f, j.ID)
					if after.State != "suppressed" || after.Attempt != 0 {
						t.Fatal("Begin DND not permanently dropped", after)
					}
					f.configure(t, configuration())
					n = n.Add(time.Minute)
					if again, e := w.Step(ctx); e != nil || again != (push.OPushAlertStepResult{}) || calls != 0 {
						t.Fatal("DND revived", again, e)
					}
					return
				}
				if before != database(t, f.store) {
					t.Fatal("Check/denied Begin wrote")
				}
				// Begin 后的策略拒绝不补做永久 Drop：仍持原 lease，恢复期后可能再次取得资格。
				if phase == 3 && (mode == "dnd" || mode == "read") {
					after := read(t, f, j.ID)
					if after.State != "leased" || after.Attempt != 1 {
						t.Fatal("readonly refusal lost lease", after)
					}
					if mode == "dnd" {
						f.configure(t, configuration())
					} else {
						// 只在 fixture 人工还原已有状态，验证只读 Check 未写永久终止标记。
						exec(t, other, `UPDATE bot_read_states SET read_seq=0`)
					}
					n = n.Add(time.Minute)
					if again, e := w.Step(ctx); e != nil || !again.Dispatched || !again.Applied || calls != 1 {
						t.Fatal("existing recovery rule", again, e, calls)
					}
				}
			})
		}
	}
}

// adapter 阻塞期间另一 Store 的写入必须完成；迟到结果只能被 fencing 拒绝。
func TestOPushAlertWorkerBlockingAndLateFence(t *testing.T) {
	for _, mode := range []string{"registration", "settings", "revoked", "takeover", "lease-expiry", "expiry", "cancel", "normal"} {
		t.Run(mode, func(t *testing.T) {
			f, j, n := opushSource(t)
			other := opushOther(t, f)
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			calls := 0
			w := opushStep(f, func() time.Time { return n }, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				close(entered)
				<-release
				return push.AlertAccepted, nil
			})
			type response struct {
				r push.OPushAlertStepResult
				e error
			}
			done := make(chan response, 1)
			go func() { r, e := w.Step(c); done <- response{r, e} }()
			<-entered
			// 这次真实写事务在 adapter 返回前完成；不能靠读 WAL 快照冒充无写锁。
			exec(t, other, `UPDATE device_push_registrations SET token='rotated-synthetic'`)
			switch mode {
			case "registration":
				exec(t, other, `UPDATE device_push_registrations SET registration_version=2`)
			case "settings":
				exec(t, other, `UPDATE device_notification_settings SET settings_version=2`)
			case "revoked":
				exec(t, other, `UPDATE sessions SET revoked=1`)
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
			case "cancel":
				cancel()
			}
			before := database(t, f.store)
			release <- struct{}{}
			got := <-done
			if !got.r.Dispatched || got.r.Applied != (mode == "normal") || (got.e != nil) != (mode == "cancel") || calls != 1 {
				t.Fatal("late result", got, calls)
			}
			if mode != "normal" && before != database(t, f.store) {
				t.Fatal("stale result wrote")
			}
			// 使用原时刻证明没有主动释放／立即补发；新租约接管也仍占用设备。
			if mode != "expiry" && mode != "lease-expiry" {
				if again, e := w.Step(c); (e != nil) != (mode == "cancel") || again != (push.OPushAlertStepResult{}) || calls != 1 {
					t.Fatal("immediate redispatch", again, e, calls)
				}
			}
		})
	}
}
func TestOPushAlertWorkerCancellation(t *testing.T) {
	for phase := 1; phase <= 4; phase++ {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			f, j, n := opushSource(t)
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			clocks, calls := 0, 0
			w := opushStep(f, func() time.Time {
				clocks++
				if clocks == phase {
					cancel()
				}
				return n
			}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				return push.AlertAccepted, nil
			})
			got, e := w.Step(c)
			if !errors.Is(e, context.Canceled) || got.Applied || got.Dispatched != (phase == 4) || calls != map[bool]int{true: 1, false: 0}[phase == 4] {
				t.Fatal("cancel stage", got, e, calls)
			}
			after := read(t, f, j.ID)
			if after.Attempt != map[bool]int64{true: 1, false: 0}[phase >= 3] || after.State != map[bool]string{true: "queued", false: "leased"}[phase == 1] {
				t.Fatal("cancel state", after)
			}
		})
	}
}
func TestOPushAlertWorkerClockAndInput(t *testing.T) {
	for _, mode := range []string{"nil-worker", "nil-repo", "empty-repo", "nil-clock", "nil-dispatch", "nil-context", "cancelled", "zero-ttl", "negative-ttl", "overflow-ttl", "nanosecond-overflow"} {
		t.Run(mode, func(t *testing.T) {
			f, _, n := opushSource(t)
			calls := 0
			clock := func() time.Time { return n }
			d := opushDispatch(func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
				calls++
				return push.AlertAccepted, nil
			})
			var dispatch push.OPushAlertDispatch = d
			r := push.NewJobRepository(f.store)
			ttl := time.Minute
			c := ctx
			switch mode {
			case "nil-repo":
				r = nil
			case "empty-repo":
				r = &push.JobRepository{}
			case "nil-clock":
				clock = nil
			case "nil-dispatch":
				dispatch = nil
			case "nil-context":
				c = nil
			case "cancelled":
				var cancel context.CancelFunc
				c, cancel = context.WithCancel(ctx)
				cancel()
			case "zero-ttl":
				ttl = 0
			case "negative-ttl":
				ttl = -1
			case "overflow-ttl":
				ttl = time.Duration(math.MaxInt64)
			case "nanosecond-overflow":
				n = time.Unix(0, math.MaxInt64).UTC()
			}
			w := push.NewOPushAlertWorker(r, clock, ttl, dispatch)
			if mode == "nil-worker" {
				w = nil
			}
			before := database(t, f.store)
			got, e := w.Step(c)
			want := push.ErrJobInput
			if mode == "cancelled" {
				want = context.Canceled
			}
			if !errors.Is(e, want) || got != (push.OPushAlertStepResult{}) || calls != 0 || before != database(t, f.store) {
				t.Fatal("input guard", got, e, calls)
			}
		})
	}
	for phase := 1; phase <= 4; phase++ {
		for _, mode := range []string{"zero", "outside-nanoseconds", "backward"} {
			if phase == 1 && mode == "backward" {
				continue
			}
			t.Run(fmt.Sprintf("clock-%d-%s", phase, mode), func(t *testing.T) {
				f, _, n := opushSource(t)
				clocks, calls := 0, 0
				w := opushStep(f, func() time.Time {
					clocks++
					if clocks == phase {
						switch mode {
						case "zero":
							return time.Time{}
						case "outside-nanoseconds":
							return time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
						case "backward":
							return n.Add(-time.Nanosecond)
						}
					}
					return n
				}, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
					calls++
					return push.AlertAccepted, nil
				})
				got, e := w.Step(ctx)
				if !errors.Is(e, push.ErrJobInput) || got.Applied || got.Dispatched != (phase == 4) || calls != map[bool]int{true: 1, false: 0}[phase == 4] {
					t.Fatal("clock guard", got, e, calls)
				}
			})
		}
	}
}
func TestOPushAlertWorkerPanicKeepsLease(t *testing.T) {
	f, j, n := opushSource(t)
	calls := 0
	w := opushStep(f, func() time.Time { return n }, func(context.Context, push.OPushAlertEnvelope) (push.AlertOutcome, error) {
		calls++
		panic("synthetic panic")
	})
	func() {
		defer func() {
			if recover() != "synthetic panic" {
				t.Fatal("panic changed")
			}
		}()
		_, _ = w.Step(ctx)
		t.Fatal("panic swallowed")
	}()
	after := read(t, f, j.ID)
	if after.State != "leased" || after.Attempt != 1 || opushKind(t, f, j.ID) != "" {
		t.Fatal("panic modified state", after)
	}
	before := database(t, f.store)
	if got, e := w.Step(ctx); e != nil || !reflect.DeepEqual(got, push.OPushAlertStepResult{}) || calls != 1 || before != database(t, f.store) {
		t.Fatal("panic retried", got, e, calls)
	}
}

package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
	"relaynest/server/internal/push"
)

var cancelState struct {
	sync.Mutex
	cancel  context.CancelFunc
	reached bool
}

func init() {
	sqlite.MustRegisterScalarFunction("relaynest_alert_attempt_cancel", 0, func(_ *sqlite.FunctionContext, _ []driver.Value) (driver.Value, error) {
		cancelState.Lock()
		defer cancelState.Unlock()
		cancelState.reached = true
		if cancelState.cancel != nil {
			cancelState.cancel()
		}
		return int64(1), nil
	})
}

// 每条写路径证明故障实际到达，再检查整库回滚与正常阴性对照。
func TestEveryWritePathAtomicFaults(t *testing.T) {
	for _, path := range []string{"begin", "result", "exhaust", "dnd"} {
		for _, fault := range []string{"abort", "ignore", "commit", "ctx", "identity", "registration", "settings", "instance", "row", "storage", "policy"} {
			if path == "result" && fault == "policy" {
				continue
			} // 已发生结果不要求当前策略许可。
			t.Run(path+"/"+fault, func(t *testing.T) {
				f, h, n := prepared(t)
				r := push.NewJobRepository(f.store)
				var ticket *push.AlertAttempt
				if path == "result" {
					ticket = begin(t, f, h, n)
				}
				if path == "exhaust" {
					exec(t, f.store, `UPDATE push_jobs SET attempt=8`)
				}
				if path == "dnd" {
					f.configure(t, strings.ReplaceAll(strings.Replace(configuration(), `"enabled":false`, `"enabled":true`, 1), `"22:00","end":"07:00"`, fmt.Sprintf(`"%s","end":"%s"`, n.Format("15:04"), n.Add(time.Hour).Format("15:04"))))
				}
				column := "state"
				if path == "begin" {
					column = "attempt"
				}
				op := "AFTER"
				body := ""
				condition := "1"
				switch fault {
				case "abort":
					op = "BEFORE"
					body = `SELECT RAISE(ABORT,'attempt fault');`
				case "ignore":
					op = "BEFORE"
					body = `SELECT RAISE(IGNORE);`
				case "commit":
					exec(t, f.store, `CREATE TABLE attempt_fault_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
					body = `INSERT INTO attempt_fault_fk VALUES('missing-bot');`
					condition = `(SELECT count(*) FROM attempt_fault_fk)=1`
				case "ctx":
					body = `SELECT relaynest_alert_attempt_cancel();`
				case "identity":
					body = `UPDATE sessions SET revoked=1;`
					condition = `(SELECT revoked FROM sessions LIMIT 1)=1`
				case "registration":
					body = `UPDATE device_push_registrations SET registration_version=2;`
					condition = `(SELECT registration_version FROM device_push_registrations LIMIT 1)=2`
				case "settings":
					body = `UPDATE device_notification_settings SET settings_version=2;`
					condition = `(SELECT settings_version FROM device_notification_settings LIMIT 1)=2`
				case "instance":
					body = `UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222';`
					condition = `(SELECT instance_id FROM instance_identity)='22222222-2222-4222-8222-222222222222'`
				case "row":
					body = `UPDATE push_jobs SET next_attempt_at='2099-01-01T00:00:00.000000000Z' WHERE id=NEW.id;`
					condition = `next_attempt_at='2099-01-01T00:00:00.000000000Z'`
				case "storage":
					body = `UPDATE push_jobs SET next_attempt_at=CAST(next_attempt_at AS BLOB) WHERE id=NEW.id;`
					condition = `typeof(next_attempt_at)='blob'`
				case "policy":
					body = `UPDATE device_notification_settings SET configuration=json_set(configuration,'$.enabled',json('false'));`
					condition = `(SELECT json_extract(configuration,'$.enabled') FROM device_notification_settings LIMIT 1)=0`
				}
				exec(t, f.store, `CREATE TRIGGER attempt_fault `+op+` UPDATE OF `+column+` ON push_jobs BEGIN `+body+` END`)
				// BLOB 突变在专用连接忽略 CHECK，用于验证 storage class 后置 guard。
				if fault == "storage" {
					exec(t, f.store, `PRAGMA ignore_check_constraints=ON`)
					defer exec(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
				}
				update := `UPDATE push_jobs SET state='degraded',lease_hash=NULL,lease_until_ns=NULL WHERE id=?`
				if path == "begin" {
					update = `UPDATE push_jobs SET attempt=1,attempt_lease_hash=lease_hash,attempt_lease_until_ns=lease_until_ns,attempt_started_at_ns=? WHERE id=?`
				}
				args := []any{h.Job.Snapshot.ID}
				if path == "begin" {
					args = []any{n.UnixNano(), h.Job.Snapshot.ID}
				}
				before := database(t, f.store)
				sentinel := errors.New("control rollback")
				reached := false
				if fault != "ctx" {
					e := f.store.Write(ctx, func(tx *sql.Tx) error {
						result, e := tx.Exec(update, args...)
						if fault == "abort" {
							reached = e != nil
							return sentinel
						}
						if e != nil {
							return e
						}
						count, e := result.RowsAffected()
						if e != nil {
							return e
						}
						if fault == "ignore" {
							reached = count == 0
							return sentinel
						}
						if e = tx.QueryRow(`SELECT (`+condition+`) FROM push_jobs WHERE id=?`, h.Job.Snapshot.ID).Scan(&reached); e != nil {
							return e
						}
						if fault == "commit" {
							return nil
						}
						return sentinel
					})
					if fault == "commit" {
						var code interface{ Code() int }
						if !reached || !errors.As(e, &code) || code.Code() != 787 {
							t.Fatal("real COMMIT not reached", reached, e)
						}
					} else if !reached || !errors.Is(e, sentinel) {
						t.Fatal("injection not reached", reached, e)
					}
					if database(t, f.store) != before {
						t.Fatal("control committed")
					}
				}
				c := ctx
				var cancel context.CancelFunc
				if fault == "ctx" {
					c, cancel = context.WithCancel(ctx)
					defer cancel()
					cancelState.Lock()
					cancelState.cancel = cancel
					cancelState.reached = false
					cancelState.Unlock()
					defer func() { cancelState.Lock(); cancelState.cancel = nil; cancelState.Unlock() }()
				}
				if path == "result" {
					if ok, e := r.ApplyAlertResult(c, *ticket, push.AlertAccepted, n); e == nil || ok {
						t.Fatal("result escaped", ok, e)
					}
				} else {
					if x, e := r.BeginAlert(c, h.Lease, n); e == nil || x != nil {
						t.Fatal("begin escaped", x, e)
					}
				}
				if fault == "ctx" {
					cancelState.Lock()
					reached = cancelState.reached
					cancelState.Unlock()
					if !reached {
						t.Fatal("ctx trigger not reached")
					}
				}
				if database(t, f.store) != before {
					t.Fatal("partial transaction")
				}
				exec(t, f.store, `DROP TRIGGER attempt_fault`)
				if path == "result" {
					if ok, e := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n); e != nil || !ok {
						t.Fatal("normal result", e)
					}
				} else {
					x, e := r.BeginAlert(ctx, h.Lease, n)
					if e != nil {
						t.Fatal("normal begin", e)
					}
					if (x != nil) != (path == "begin") {
						t.Fatal("normal ticket", x)
					}
					if path == "exhaust" && read(t, f, h.Job.Snapshot.ID).State != "degraded" {
						t.Fatal("normal exhaust")
					}
					if path == "dnd" && read(t, f, h.Job.Snapshot.ID).State != "suppressed" {
						t.Fatal("normal dnd")
					}
				}
			})
		}
	}
}

func TestExpiryRetryBoundaryAndNegatives(t *testing.T) {
	for _, mode := range []string{"retry-at-expiry", "fcm-alert", "ineligible", "sync", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			f, h, n := prepared(t)
			r := push.NewJobRepository(f.store)
			if mode == "retry-at-expiry" {
				// 原 expiry 冻结，安排足够长租约使结果时间在 expiry 前十秒。
				if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
					t.Fatal(e)
				}
				n = h.Job.Snapshot.ExpiresAt.Add(-10 * time.Second)
				held, e := r.ReserveOPushAlerts(ctx, n, time.Minute, 1)
				if e != nil || len(held) != 1 {
					t.Fatal(e)
				}
				h = held[0]
				x := begin(t, f, h, n)
				if ok, e := r.ApplyAlertResult(ctx, *x, push.AlertTransient, n); e != nil || !ok {
					t.Fatal(e)
				}
				if read(t, f, h.Job.Snapshot.ID).State != "degraded" {
					t.Fatal("retry extended expiry")
				}
				return
			}
			// 通过独立新完整 job/legacy 走通用预留，负控不得得到 alert 票据。
			if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
				t.Fatal(e)
			}
			exec(t, f.store, `DELETE FROM push_jobs`)
			exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, n.Format(stamp))
			j := h.Job.Snapshot
			j.ID = "negative"
			switch mode {
			case "fcm-alert":
				j.Provider = "fcm"
			case "ineligible":
				j.AlertEligible = false
			case "sync":
				j.Purpose = "sync"
				j.Provider = "fcm"
				j.AlertEligible = false
			}
			if mode == "legacy" {
				exec(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)VALUES('negative',?,?,1,'legacy')`, j.SessionID, j.DeviceID)
				if x, e := r.BeginAlert(ctx, push.JobLease{}, n); e == nil || x != nil {
					t.Fatal(x, e)
				}
				return
			}
			if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
				t.Fatal(e)
			}
			held, e := r.Reserve(ctx, n, time.Minute, 1)
			if e != nil || len(held) != 1 {
				t.Fatal(e)
			}
			before := database(t, f.store)
			if x, e := r.BeginAlert(ctx, held[0].Lease, n); e != nil || x != nil {
				t.Fatal(x, e)
			}
			if before != database(t, f.store) {
				t.Fatal("negative wrote")
			}
		})
	}
}

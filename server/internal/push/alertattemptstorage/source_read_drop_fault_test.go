package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"relaynest/server/internal/push"
)

// 错误和缺失状态都不是已读证据；每次拒绝都核全库零写。
func TestAlertSourceReadDropUnavailableEvidence(t *testing.T) {
	for _, mode := range []string{"missing-read", "bad-read", "bad-source", "sql"} {
		t.Run(mode, func(t *testing.T) {
			f, h, n := prepared(t)
			switch mode {
			case "missing-read":
				exec(t, f.store, `DELETE FROM bot_read_states`)
			case "bad-read":
				exec(t, f.store, `PRAGMA ignore_check_constraints=ON`)
				defer exec(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
				exec(t, f.store, `UPDATE bot_read_states SET read_seq=CAST('1' AS BLOB)`)
			case "bad-source":
				exec(t, f.store, `PRAGMA ignore_check_constraints=ON`)
				defer exec(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
				exec(t, f.store, `UPDATE messages SET bot_seq=CAST(bot_seq AS BLOB)`)
			case "sql":
				exec(t, f.store, `ALTER TABLE messages RENAME TO unavailable_messages`)
			}
			before := database(t, f.store)
			if ok, err := push.NewJobRepository(f.store).DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n); err == nil || ok {
				t.Fatal("error became reason", ok, err)
			}
			if database(t, f.store) != before {
				t.Fatal("unavailable evidence wrote")
			}
		})
	}
}

// 复用既有SQLite观测函数，在实际调用中证明触发器已命中；不只运行旁路控制。
func TestAlertSourceReadDropAtomicFaults(t *testing.T) {
	for _, fault := range []string{"abort", "ignore", "commit", "ctx", "identity", "registration", "row", "storage", "source", "read"} {
		t.Run(fault, func(t *testing.T) {
			f, h, n := prepared(t)
			sourceReadAdvance(t, f, h.Job.Snapshot.BotSeq)
			op, body, condition := "AFTER", "", "1"
			switch fault {
			case "abort":
				op, body = "BEFORE", `SELECT RAISE(ABORT,'source read fault');`
			case "ignore":
				op, body = "BEFORE", `SELECT RAISE(IGNORE);`
			case "commit":
				exec(t, f.store, `CREATE TABLE source_read_fault_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
				body = `INSERT INTO source_read_fault_fk VALUES('missing-bot');`
				condition = `(SELECT count(*) FROM source_read_fault_fk)=1`
			case "ctx":
				// 公共观测语句同时在当前上下文中触发取消。
			case "identity":
				body = `UPDATE sessions SET revoked=1;`
				condition = `(SELECT revoked FROM sessions LIMIT 1)=1`
			case "registration":
				body = `UPDATE device_push_registrations SET registration_version=2;`
				condition = `(SELECT registration_version FROM device_push_registrations LIMIT 1)=2`
			case "row":
				body = `UPDATE push_jobs SET next_attempt_at='2099-01-01T00:00:00.000000000Z' WHERE id=NEW.id;`
				condition = `next_attempt_at='2099-01-01T00:00:00.000000000Z'`
			case "storage":
				exec(t, f.store, `PRAGMA ignore_check_constraints=ON`)
				defer exec(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
				body = `UPDATE push_jobs SET next_attempt_at=CAST(next_attempt_at AS BLOB) WHERE id=NEW.id;`
				condition = `typeof(next_attempt_at)='blob'`
			case "source":
				body = `UPDATE messages SET bot_seq=bot_seq+1 WHERE id=NEW.message_id;`
				condition = `(SELECT bot_seq FROM messages WHERE id=message_id)=bot_seq+1`
			case "read":
				body = `UPDATE bot_read_states SET read_seq=0;`
				condition = `(SELECT read_seq FROM bot_read_states LIMIT 1)=0`
			}
			exec(t, f.store, `CREATE TRIGGER source_read_fault `+op+` UPDATE OF state ON push_jobs WHEN NEW.state='suppressed' BEGIN SELECT relaynest_alert_attempt_cancel(); `+body+` END`)
			before := database(t, f.store)
			sentinel := errors.New("control rollback")
			if fault != "ctx" {
				reached := false
				err := f.store.Write(ctx, func(tx *sql.Tx) error {
					result, err := tx.Exec(`UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL WHERE id=?`, h.Job.Snapshot.ID)
					if fault == "abort" {
						reached = err != nil
						return sentinel
					}
					if err != nil {
						return err
					}
					if fault == "ignore" {
						count, err := result.RowsAffected()
						reached = count == 0
						if err != nil {
							return err
						}
						return sentinel
					}
					if err := tx.QueryRow(`SELECT (`+condition+`) FROM push_jobs WHERE id=?`, h.Job.Snapshot.ID).Scan(&reached); err != nil {
						return err
					}
					if fault == "commit" {
						return nil
					}
					return sentinel
				})
				if fault == "commit" {
					var code interface{ Code() int }
					if !reached || !errors.As(err, &code) || code.Code() != 787 {
						t.Fatal("real commit not reached", reached, err)
					}
				} else if !reached || !errors.Is(err, sentinel) {
					t.Fatal("control not reached", reached, err)
				}
				if database(t, f.store) != before {
					t.Fatal("control wrote")
				}
			}
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			cancelState.Lock()
			cancelState.reached = false
			cancelState.cancel = nil
			if fault == "ctx" {
				cancelState.cancel = cancel
			}
			cancelState.Unlock()
			defer func() { cancelState.Lock(); cancelState.cancel = nil; cancelState.Unlock() }()
			ok, err := push.NewJobRepository(f.store).DropAlertSourceRead(c, h.Lease, h.Job.Snapshot, n)
			cancelState.Lock()
			reached := cancelState.reached
			cancelState.cancel = nil
			cancelState.Unlock()
			if !reached || ok || err == nil {
				t.Fatal("fault escaped or not reached", reached, ok, err)
			}
			if fault == "commit" {
				var code interface{ Code() int }
				if !errors.As(err, &code) || code.Code() != 787 {
					t.Fatal("callback did not reach real COMMIT", err)
				}
			}
			if fault == "ctx" && !errors.Is(err, context.Canceled) {
				t.Fatal("expected cancellation", err)
			}
			if database(t, f.store) != before {
				t.Fatal("partial transaction")
			}
			exec(t, f.store, `DROP TRIGGER source_read_fault`)
			if ok, err := push.NewJobRepository(f.store).DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n); err != nil || !ok {
				t.Fatal(fmt.Sprint("normal control after ", fault), ok, err)
			}
		})
	}
}

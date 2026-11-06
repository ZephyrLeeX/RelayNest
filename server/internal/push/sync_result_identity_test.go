package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

// 覆盖所有结果分支；同事务写后资格失效必须同时回滚结果和身份表副作用。
func TestSyncAttemptResultPostWriteIdentityRollback(t *testing.T) {
	for _, outcome := range []SyncOutcome{SyncAccepted, SyncTargetInvalid, SyncTransient, SyncRateLimited, SyncConfigurationError, SyncUnknownDelivery} {
		f, held := leaseFixture(t)
		r := NewJobRepository(f.store)
		ticket := mustBeginSync(t, r, held.Lease, f.now)
		baseline := reusableBaseline(t, f.other)
		row := reusableJobRow(t, f.other, held.Job.Snapshot.ID)
		for _, mutation := range []struct{ name, sql string }{
			{"registration", `UPDATE device_push_registrations SET registration_version=registration_version+1 WHERE session_id=NEW.session_id;`},
			{"registration-delete", `DELETE FROM device_push_registrations WHERE session_id=NEW.session_id;`},
			{"registration-tombstone", `UPDATE device_push_registrations SET token=NULL WHERE session_id=NEW.session_id;`},
			{"revoked", `UPDATE sessions SET revoked=1 WHERE id=NEW.session_id;`},
			{"session-generation", `UPDATE sessions SET auth_generation=auth_generation+1 WHERE id=NEW.session_id;`},
			{"client", `UPDATE sessions SET client_type='web' WHERE id=NEW.session_id;`},
			{"admin-generation", `UPDATE administrator SET auth_generation=auth_generation+1 WHERE id=1;`},
			{"settings", `INSERT INTO device_notification_settings VALUES(NEW.device_id,1,'2026-01-01T00:00:00Z','{}');`},
			{"deployment", `UPDATE instance_identity SET instance_id='33333333-3333-4333-8333-333333333333' WHERE id=1;`},
			{"idle-expiry", `UPDATE sessions SET idle_expires_at=NEW.result_at_ns WHERE id=NEW.session_id;`},
			{"absolute-expiry", `UPDATE sessions SET idle_expires_at=NEW.result_at_ns,absolute_expires_at=NEW.result_at_ns WHERE id=NEW.session_id;`},
		} {
			t.Run(string(outcome)+"/"+mutation.name, func(t *testing.T) {
				baseline.check(t, f.other)
				if got := reusableJobRow(t, f.other, held.Job.Snapshot.ID); !reflect.DeepEqual(got, row) {
					t.Fatal("reused job baseline differs in values or SQLite types")
				}
				checkTicket := func() {
					t.Helper()
					if current, err := r.CheckSyncAttempt(jobContext, held.Lease, *ticket, f.now); !current || err != nil {
						t.Fatalf("reused ticket is not current: %v %v", current, err)
					}
				}
				checkTicket()
				t.Cleanup(func() {
					// 只还原 Apply 实际更新的六列；attempt 和身份仍来自真实 Begin。
					jobSQL(t, f.store, `DROP TRIGGER IF EXISTS result_identity`)
					jobSQL(t, f.store, `UPDATE push_jobs SET state=?,next_attempt_at=?,lease_hash=?,lease_until_ns=?,result_kind=?,result_at_ns=? WHERE id=?`, row.values[4], row.values[6], row.values[19], row.values[20], row.values[24], row.values[25], held.Job.Snapshot.ID)
					if got := reusableJobRow(t, f.other, held.Job.Snapshot.ID); !reflect.DeepEqual(got, row) {
						t.Fatal("reused job did not restore all 26 values and SQLite types")
					}
					baseline.check(t, f.other)
					checkTicket()
				})
				jobSQL(t, f.store, `CREATE TRIGGER result_identity AFTER UPDATE OF result_kind ON push_jobs BEGIN `+mutation.sql+` END`)
				faultSchema := reusableSchema(t, f.other)
				before := leaseDigest(t, f.other)
				if changed, err := r.ApplySyncResult(jobContext, *ticket, outcome, f.now); changed || err == nil {
					t.Fatalf("post-write identity change published result: changed=%v error=%v", changed, err)
				}
				if after := leaseDigest(t, f.other); after != before {
					t.Fatal("post-write identity change left partial committed data")
				}
				if got := reusableSchema(t, f.other); got != faultSchema {
					t.Fatal("fault changed database schema")
				}
				jobSQL(t, f.store, `DROP TRIGGER result_identity`)
				// 回滚后原票据仍可正常完成；同时防止把所有非 leased 终态一概拒绝。
				if changed, err := r.ApplySyncResult(jobContext, *ticket, outcome, f.now); !changed || err != nil {
					t.Fatalf("normal result after rollback: changed=%v error=%v", changed, err)
				}
			})
			if t.Failed() {
				t.Fatal("shared fixture failed; refusing to run another mutation")
			}
		}
	}
}

// 用真实八次派发及跨 Store 接管抵达耗尽分支，避免手工改 attempt 绕过元数据约束。
func TestSyncAttemptExhaustedPostWriteIdentityRollback(t *testing.T) {
	for _, mutation := range []struct{ name, sql string }{
		{"registration", `UPDATE device_push_registrations SET registration_version=registration_version+1 WHERE session_id=NEW.session_id;`},
		{"revoked", `UPDATE sessions SET revoked=1 WHERE id=NEW.session_id;`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			f, held := leaseFixture(t)
			r := NewJobRepository(f.store)
			now := f.now
			for attempt := int64(1); attempt <= MaxSyncAttempts; attempt++ {
				ticket := mustBeginSync(t, r, held.Lease, now)
				if ticket.attempt != attempt {
					t.Fatal("unexpected attempt", ticket.attempt)
				}
				if attempt == MaxSyncAttempts {
					break
				}
				if changed, err := r.ApplySyncResult(jobContext, *ticket, SyncTransient, now); !changed || err != nil {
					t.Fatal("retry result", changed, err)
				}
				now = jobRead(t, f.store, held.Job.Snapshot.ID).NextAttemptAt
				acquired, err := r.Reserve(jobContext, now, time.Minute, 1)
				if err != nil || len(acquired) != 1 {
					t.Fatal("retry reserve", len(acquired), err)
				}
				held = acquired[0]
			}
			now = held.Lease.until
			acquired, err := NewJobRepository(f.other).Reserve(jobContext, now, time.Minute, 1)
			if err != nil || len(acquired) != 1 {
				t.Fatal("exhausted takeover", len(acquired), err)
			}
			fresh := acquired[0]
			if fresh.Job.Attempt != MaxSyncAttempts || fresh.Lease == held.Lease {
				t.Fatal("expected new exhausted lease")
			}
			jobSQL(t, f.store, `CREATE TRIGGER exhausted_identity AFTER UPDATE OF state ON push_jobs BEGIN `+mutation.sql+` END`)
			before := leaseDigest(t, f.other)
			if ticket, err := r.BeginSync(jobContext, fresh.Lease, now); ticket != nil || err == nil {
				t.Fatalf("post-write exhausted identity change: ticket=%v error=%v", ticket != nil, err)
			}
			if before != leaseDigest(t, f.other) {
				t.Fatal("exhausted identity change left partial committed data")
			}
			jobSQL(t, f.store, `DROP TRIGGER exhausted_identity`)
			if ticket, err := r.BeginSync(jobContext, fresh.Lease, now); ticket != nil || err != nil {
				t.Fatalf("normal exhausted result: ticket=%v error=%v", ticket != nil, err)
			}
			got := jobRead(t, f.other, fresh.Job.Snapshot.ID)
			if got.State != "degraded" || got.Attempt != MaxSyncAttempts || got.Snapshot != fresh.Job.Snapshot {
				t.Fatal("normal exhaustion changed snapshot or attempt", got.State, got.Attempt)
			}
		})
	}
}

// 复用夹具必须逐例证明全表和完整 schema 均回到原点，不缓存资格或复制数据库。
type reusableDatabaseBaseline struct {
	digest [32]byte
	schema string
}

func reusableBaseline(t *testing.T, store *storage.Store) reusableDatabaseBaseline {
	t.Helper()
	return reusableDatabaseBaseline{leaseDigest(t, store), reusableSchema(t, store)}
}
func (b reusableDatabaseBaseline) check(t *testing.T, store *storage.Store) {
	t.Helper()
	if got := leaseDigest(t, store); got != b.digest {
		t.Fatal("shared fixture database differs from baseline")
	}
	if got := reusableSchema(t, store); got != b.schema {
		t.Fatal("shared fixture schema differs from baseline")
	}
}
func reusableSchema(t *testing.T, store *storage.Store) string {
	t.Helper()
	var all [][]any
	if err := store.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT type,name,tbl_name,rootpage,sql FROM sqlite_master ORDER BY type,name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			v := make([]any, 5)
			if err := rows.Scan(&v[0], &v[1], &v[2], &v[3], &v[4]); err != nil {
				return err
			}
			all = append(all, v)
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type reusableJobSnapshot struct{ values, types []any }

func reusableJobRow(t *testing.T, store *storage.Store, id string) reusableJobSnapshot {
	t.Helper()
	var got reusableJobSnapshot
	if err := store.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT * FROM push_jobs WHERE id=?`, id)
		if err != nil {
			return err
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return err
		}
		if len(cols) != 26 {
			rows.Close()
			return fmt.Errorf("expected 26 columns, got %d", len(cols))
		}
		if !rows.Next() {
			rows.Close()
			return sql.ErrNoRows
		}
		got.values = make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range ptr {
			ptr[i] = &got.values[i]
		}
		if err := rows.Scan(ptr...); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		types := make([]string, len(cols))
		got.types = make([]any, len(cols))
		for i, c := range cols {
			types[i] = `typeof("` + strings.ReplaceAll(c, `"`, `""`) + `")`
			ptr[i] = &got.types[i]
		}
		return tx.QueryRow(`SELECT `+strings.Join(types, ",")+` FROM push_jobs WHERE id=?`, id).Scan(ptr...)
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

// 子例 cleanup 使用独立 Background 事务，且恢复失败后父测试必须停止复用。
func reusablePolicyCase(t *testing.T, f *jobFixture, baseline reusableDatabaseBaseline) {
	t.Helper()
	baseline.check(t, f.other)
	checkConstraints := func() {
		t.Helper()
		if err := f.store.Write(context.Background(), func(tx *sql.Tx) error {
			var ignored int
			if err := tx.QueryRow(`PRAGMA ignore_check_constraints`).Scan(&ignored); err != nil {
				return err
			}
			if ignored != 0 {
				return fmt.Errorf("writer CHECK constraints were not restored")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	checkConstraints()
	t.Cleanup(func() {
		jobSQL(t, f.store, `DELETE FROM device_notification_settings`)
		checkConstraints()
		baseline.check(t, f.other)
	})
}

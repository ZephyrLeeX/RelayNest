package push

import (
	"testing"
	"time"
)

// 覆盖所有结果分支；同事务写后资格失效必须同时回滚结果和身份表副作用。
func TestSyncAttemptResultPostWriteIdentityRollback(t *testing.T) {
	for _, outcome := range []SyncOutcome{SyncAccepted, SyncTargetInvalid, SyncTransient, SyncRateLimited, SyncConfigurationError, SyncUnknownDelivery} {
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
				f, held := leaseFixture(t)
				r := NewJobRepository(f.store)
				ticket := mustBeginSync(t, r, held.Lease, f.now)
				jobSQL(t, f.store, `CREATE TRIGGER result_identity AFTER UPDATE OF result_kind ON push_jobs BEGIN `+mutation.sql+` END`)
				before := leaseDigest(t, f.other)
				if changed, err := r.ApplySyncResult(jobContext, *ticket, outcome, f.now); changed || err == nil {
					t.Fatalf("post-write identity change published result: changed=%v error=%v", changed, err)
				}
				if after := leaseDigest(t, f.other); after != before {
					t.Fatal("post-write identity change left partial committed data")
				}
				jobSQL(t, f.store, `DROP TRIGGER result_identity`)
				// 回滚后原票据仍可正常完成；同时防止把所有非 leased 终态一概拒绝。
				if changed, err := r.ApplySyncResult(jobContext, *ticket, outcome, f.now); !changed || err != nil {
					t.Fatalf("normal result after rollback: changed=%v error=%v", changed, err)
				}
			})
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

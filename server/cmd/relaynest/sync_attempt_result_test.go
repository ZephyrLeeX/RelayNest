package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

// 覆盖真实入口的业务表、receipt、会话与注册；允许的写入仅限 push_jobs。
func syncAttemptBusinessState(t *testing.T, f *botHTTPFixture) [32]byte {
	t.Helper()
	var all []any
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, table := range []string{"messages", "deliveries", "changes", "idempotency", "bot_read_states", "sessions", "administrator", "device_push_registrations", "device_notification_settings", "bots", "system_state", "instance_identity"} {
			rows, err := tx.Query(`SELECT * FROM ` + table + ` ORDER BY 1`)
			if err != nil {
				return err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			for rows.Next() {
				values, ptrs := make([]any, len(columns)), make([]any, len(columns))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					rows.Close()
					return err
				}
				all = append(all, []any{table, values})
			}
			err = rows.Err()
			closeErr := rows.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

func TestAssembledSyncAttemptResultProduction(t *testing.T) {
	for _, admin := range []bool{false, true} {
		for _, outcome := range []push.SyncOutcome{push.SyncAccepted, push.SyncTargetInvalid, push.SyncTransient, push.SyncRateLimited, push.SyncConfigurationError, push.SyncUnknownDelivery} {
			t.Run(fmt.Sprintf("admin=%v/%s", admin, outcome), func(t *testing.T) {
				f := botHTTP(t, nil) // 生产 handler 组装，受信任 httptest TLS 客户端。
				bot := clientMessageBot(t, f)
				syncCreateRegister(t, f, "synthetic-attempt-registration-secret", 7, 0)
				status, raw, headers := syncCreateRequest(f, bot, admin, clientMessageKey)
				receipt := syncCreateReceipt(t, status, raw, headers, admin)
				jobs := syncCreateJobs(t, f)
				if len(jobs) != 1 || jobs[0].Snapshot.MessageID != receipt.ID {
					t.Fatal("missing real sync job")
				}
				original := jobs[0]
				// 旧五列 writer 与 alert 同设备混存，不能被本叶提升为 sync。
				f.sql(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)
 SELECT 'attempt-legacy',session_id,device_id,target_change_seq,'legacy-original' FROM push_jobs WHERE id=?`, original.Snapshot.ID)
				f.sql(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,purpose,provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,alert_eligible)
 SELECT 'attempt-alert',session_id,device_id,target_change_seq,'queued',0,?,'alert',provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,1
 FROM push_jobs WHERE id=?`, original.Snapshot.CreatedAt.Add(time.Hour).Format("2006-01-02T15:04:05.000000000Z"), original.Snapshot.ID)
				business := syncAttemptBusinessState(t, f)
				now := time.Now().UTC()
				r := push.NewJobRepository(f.store)
				held, err := r.Reserve(context.Background(), now, time.Minute, 1)
				if err != nil || len(held) != 1 || held[0].Job.Snapshot != original.Snapshot {
					t.Fatal("real Reserve", err)
				}
				checked := assembledLeaseCheck(t, f, held[0].Lease, now, true)
				if checked.Job.Attempt != 0 {
					t.Fatal("Check consumed attempt")
				}
				ticket, err := r.BeginSync(context.Background(), held[0].Lease, now)
				if err != nil || ticket == nil {
					t.Fatal("Begin", err)
				}
				if again, e := r.BeginSync(context.Background(), held[0].Lease, now); e != nil || again != nil {
					t.Fatal("duplicate Begin", e)
				}
				if ok, e := r.ApplySyncResult(context.Background(), *ticket, outcome, now); e != nil || !ok {
					t.Fatal("typed result", e)
				}
				before := readSnapshot(t, f)
				if ok, e := r.ApplySyncResult(context.Background(), *ticket, outcome, now); e != nil || ok {
					t.Fatal("duplicate result", e)
				}
				if before != readSnapshot(t, f) || business != syncAttemptBusinessState(t, f) {
					t.Fatal("changed business/receipt or duplicate outcome")
				}
				got := syncCreateJobs(t, f)
				if len(got) != 2 {
					t.Fatal("mixed count")
				}
				for _, j := range got {
					if j.Snapshot.ID == original.Snapshot.ID {
						if j.Attempt != 1 || j.Snapshot != original.Snapshot || j.NextAttemptAt.After(j.Snapshot.ExpiresAt) {
							t.Fatal("snapshot reset")
						}
					} else if j.State != "queued" || j.Attempt != 0 || j.Snapshot.Purpose != "alert" {
						t.Fatal("alert modified")
					}
				}
				if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
					var n int
					if e := tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE id='attempt-legacy' AND state='pending' AND attempt=0 AND next_attempt_at='legacy-original'
 AND purpose IS NULL AND created_at_ns IS NULL AND attempt_lease_hash IS NULL AND result_kind IS NULL`).Scan(&n); e != nil {
						return e
					}
					if n != 1 {
						t.Fatal("legacy promoted")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				// 幂等 HTTP receipt 仍逐字一致，不因 push 结果补造任务或改变 delivery。
				status, replayed, h := syncCreateRequest(f, bot, admin, clientMessageKey)
				syncCreateReceipt(t, status, replayed, h, admin)
				if replayed != raw || status != 201 {
					t.Fatal("receipt changed")
				}
			})
		}
	}
}

package main

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

type assembledWorkerDispatch func(context.Context, push.SyncEnvelope) (push.SyncOutcome, error)

func (f assembledWorkerDispatch) Dispatch(ctx context.Context, e push.SyncEnvelope) (push.SyncOutcome, error) {
	return f(ctx, e)
}

func TestAssembledSyncWorkerProduction(t *testing.T) {
	for _, admin := range []bool{false, true} {
		for _, outcome := range []push.SyncOutcome{push.SyncAccepted, push.SyncTargetInvalid, push.SyncTransient, push.SyncRateLimited, push.SyncConfigurationError, push.SyncUnknownDelivery} {
			t.Run(fmt.Sprintf("admin=%v/%s", admin, outcome), func(t *testing.T) {
				f := botHTTP(t, nil)
				bot := clientMessageBot(t, f)
				syncCreateRegister(t, f, "synthetic-worker-registration-secret", 7, 0)
				status, raw, headers := syncCreateRequest(f, bot, admin, clientMessageKey)
				receipt := syncCreateReceipt(t, status, raw, headers, admin)
				jobs := syncCreateJobs(t, f)
				if len(jobs) != 1 || jobs[0].Snapshot.MessageID != receipt.ID {
					t.Fatal("missing real TLS sync producer")
				}
				original := jobs[0]
				f.sql(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)
 SELECT 'worker-legacy',session_id,device_id,target_change_seq,'legacy-original' FROM push_jobs WHERE id=?`, original.Snapshot.ID)
				// 更早且同设备的 alert 不能占用 limit=1 的 sync 候选。
				f.sql(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,purpose,provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,alert_eligible)
 SELECT 'worker-alert',session_id,device_id,target_change_seq,'queued',0,?,'alert',provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,1
 FROM push_jobs WHERE id=?`, original.Snapshot.CreatedAt.Add(-time.Second).Format("2006-01-02T15:04:05.000000000Z"), original.Snapshot.ID)
				business := syncAttemptBusinessState(t, f)
				calls := 0
				reads := 0
				now := time.Now().UTC()
				worker := push.NewSyncWorker(push.NewJobRepository(f.store), func() time.Time { reads++; return now.Add(time.Duration(reads) * time.Nanosecond) }, time.Minute, assembledWorkerDispatch(func(ctx context.Context, e push.SyncEnvelope) (push.SyncOutcome, error) {
					calls++
					if e.JobKey != original.Snapshot.ID || e.Purpose != "sync" || e.Provider != "fcm" || e.TargetChangeSeq != original.Snapshot.TargetChangeSeq || !e.ExpiresAt.Equal(original.Snapshot.ExpiresAt) {
						t.Fatal("wrong envelope")
					}
					return outcome, nil
				}))
				got, err := worker.Step(context.Background())
				if err != nil || got != (push.SyncStepResult{Dispatched: true, Applied: true}) || calls != 1 || reads != 4 {
					t.Fatal(got, err, calls, reads)
				}
				if business != syncAttemptBusinessState(t, f) {
					t.Fatal("business/receipt/read changed")
				}
				jobs = syncCreateJobs(t, f)
				if len(jobs) != 2 {
					t.Fatal("mixed jobs count")
				}
				for _, j := range jobs {
					if j.Snapshot.ID == original.Snapshot.ID {
						if j.Attempt != 1 || j.Snapshot != original.Snapshot {
							t.Fatal("snapshot reset")
						}
						state := "degraded"
						switch outcome {
						case push.SyncAccepted:
							state = "accepted"
						case push.SyncTransient, push.SyncRateLimited, push.SyncUnknownDelivery:
							state = "queued"
							if !j.NextAttemptAt.After(now) || !j.NextAttemptAt.Before(j.Snapshot.ExpiresAt) {
								t.Fatal("retry bound")
							}
						}
						if j.State != state {
							t.Fatal(j.State, state)
						}
					} else if j.State != "queued" || j.Attempt != 0 || j.Snapshot.Purpose != "alert" {
						t.Fatal("alert changed")
					}
				}
				if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
					var n int
					if e := tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE id='worker-legacy' AND state='pending' AND attempt=0 AND next_attempt_at='legacy-original' AND purpose IS NULL AND attempt_lease_hash IS NULL AND result_kind IS NULL`).Scan(&n); e != nil {
						return e
					}
					if n != 1 {
						t.Fatal("legacy changed")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				before := readSnapshot(t, f)
				if got, err = worker.Step(context.Background()); err != nil || got != (push.SyncStepResult{}) || calls != 1 || before != readSnapshot(t, f) {
					t.Fatal("immediate retransmission", got, err)
				}
				status, replayed, h := syncCreateRequest(f, bot, admin, clientMessageKey)
				syncCreateReceipt(t, status, replayed, h, admin)
				if status != 201 || raw != replayed {
					t.Fatal("HTTP receipt changed")
				}
			})
		}
	}
}

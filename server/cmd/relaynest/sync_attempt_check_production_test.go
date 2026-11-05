package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

func TestAssembledSyncAttemptCheckProduction(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint("admin=", admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f)
			syncCreateRegister(t, f, "synthetic-check-registration-secret", 7, 0)
			status, raw, headers := syncCreateRequest(f, bot, admin, clientMessageKey)
			receipt := syncCreateReceipt(t, status, raw, headers, admin)
			jobs := syncCreateJobs(t, f)
			if len(jobs) != 1 || jobs[0].Snapshot.MessageID != receipt.ID {
				t.Fatal("real job")
			}

			// 同设备旧五列与未来 alert 混存，检查不得提升或更改这些任务。
			f.sql(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)
 SELECT 'check-legacy',session_id,device_id,target_change_seq,'legacy-original' FROM push_jobs WHERE id=?`, jobs[0].Snapshot.ID)
			f.sql(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,purpose,provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,alert_eligible)
 SELECT 'check-alert',session_id,device_id,target_change_seq,'queued',0,?,'alert',provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,1
 FROM push_jobs WHERE id=?`, jobs[0].Snapshot.CreatedAt.Add(time.Hour).Format("2006-01-02T15:04:05.000000000Z"), jobs[0].Snapshot.ID)
			r := push.NewJobRepository(f.store)
			now := time.Now().UTC()
			held, e := r.Reserve(context.Background(), now, time.Minute, 1)
			if e != nil || len(held) != 1 {
				t.Fatal("Reserve", e)
			}
			ticket, e := r.BeginSync(context.Background(), held[0].Lease, now)
			if e != nil || ticket == nil {
				t.Fatal("Begin", e)
			}
			observer, e := storage.Open(context.Background(), storage.Config{Path: f.databasePath, BusyTimeout: 50 * time.Millisecond})
			if e != nil {
				t.Fatal(e)
			}
			defer observer.Close()
			check := func(repo *push.JobRepository, want bool) {
				t.Helper()
				before := readSnapshot(t, f)
				ok, e := repo.CheckSyncAttempt(context.Background(), held[0].Lease, *ticket, now.Add(time.Nanosecond))
				if e != nil || ok != want {
					t.Fatal("Check", ok, e)
				}
				if before != readSnapshot(t, f) {
					t.Fatal("check wrote")
				}
			}
			check(r, true)
			check(push.NewJobRepository(observer), true)
			syncCreateRegister(t, f, "synthetic-check-rotated-secret", 8, 7)
			check(r, false)
			check(push.NewJobRepository(observer), false)
		})
	}
}

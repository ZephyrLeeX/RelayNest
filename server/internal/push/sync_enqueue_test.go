package push

import (
	"database/sql"
	"fmt"
	"math"
	"testing"
	"time"

	"relaynest/server/internal/messages"
)

func TestSyncEnqueuePaginationSnapshotsAndIsolation(t *testing.T) {
	f := jobSetup(t)
	// 仓储压力夹具使用可信SQL批量造不同session，生产非零入口另由cmd真实HTTP证明。
	const total = 261
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		for i := 0; i < total; i++ {
			id := fmt.Sprintf("session-%03d", i)
			token := make([]byte, 32)
			token[0] = byte(i)
			token[1] = byte(i >> 8)
			if _, err := tx.Exec(`INSERT INTO sessions(id,token_hash,auth_generation,client_type,device_id,device_name,idle_expires_at,absolute_expires_at) VALUES(?,?,1,'android',?,'fixture',?,?)`, id, token, jobDeviceA, f.now.Add(time.Hour).UnixNano(), f.now.Add(2*time.Hour).UnixNano()); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO device_push_registrations VALUES(?,'fcm',7,'synthetic-paged-secret',?)`, id, f.now.Format(time.RFC3339Nano)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	created, err := messages.New(f.store).CreateApplication(jobContext, f.identity, messages.TextInput{BotID: f.bot.Bot.ID, Format: "plain", Text: "paged-private-text"})
	if err != nil {
		t.Fatal(err)
	}
	event := SyncEvent{MessageID: created.Message.ID, BotID: created.Message.BotID, BotSeq: created.Message.BotSeq, TargetChangeSeq: int64(created.LastChangeSeq), CreatedAt: created.Message.CreatedAt}
	started := time.Now()
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error { return EnqueueSyncTx(jobContext, tx, event) }); err != nil {
		t.Fatal(err)
	}
	t.Logf("261 sessions / 3 pages transaction: %s", time.Since(started))
	if got := jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE purpose='sync' AND provider='fcm' AND state='queued' AND registration_version=7 AND settings_version=0 AND alert_eligible=0`); got != total {
		t.Fatal("truncated or malformed", got)
	}
	if jobCount(t, f.store, `SELECT count(DISTINCT session_id) FROM push_jobs`) != total {
		t.Fatal("same-device sessions collapsed")
	}
	job := jobRead(t, f.store, "sync:"+event.MessageID+":session-260")
	if job.Snapshot.CreatedAt != event.CreatedAt || !job.Snapshot.ExpiresAt.Equal(event.CreatedAt.Add(24*time.Hour)) || job.Snapshot.TargetChangeSeq != event.TargetChangeSeq {
		t.Fatal("snapshot changed", job)
	}
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		pending, err := ReadPendingTx(jobContext, tx, time.Now().UTC(), 10)
		if err == nil && len(pending) != 0 {
			t.Fatal("complete sync polluted legacy")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// 同一真实schema混合三类待办：legacy只读不合并完整sync/alert的新水位。
	alert := job.Snapshot
	alert.ID = "synthetic-alert"
	alert.Purpose = "alert"
	alert.AlertEligible = true
	alert.TargetChangeSeq += 100
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		if err := CreateJobTx(jobContext, tx, alert, alert.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('synthetic-legacy',?,?,1,?)`, alert.SessionID, alert.DeviceID, event.CreatedAt.Format(time.RFC3339Nano))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
		pending, err := ReadPendingTx(jobContext, tx, time.Now().UTC(), 10)
		if err != nil {
			return err
		}
		if len(pending) != 1 || len(pending[0].Sessions) != 1 || pending[0].Sessions[0].SessionID != alert.SessionID || pending[0].Sessions[0].TargetChangeSeq != 1 {
			t.Fatal("mixed job watermark polluted legacy", pending)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := jobRead(t, f.store, alert.ID); got.Snapshot != alert || got.State != "queued" {
		t.Fatal("mixed alert changed", got)
	}

}

func TestSyncEnqueueRejectsInvalidEventAndNanosecondOverflow(t *testing.T) {
	f := jobSetup(t)
	f.session(t, jobDeviceA)
	created, err := messages.New(f.store).CreateApplication(jobContext, f.identity, messages.TextInput{BotID: f.bot.Bot.ID, Format: "plain", Text: "overflow"})
	if err != nil {
		t.Fatal(err)
	}
	base := SyncEvent{MessageID: created.Message.ID, BotID: created.Message.BotID, BotSeq: created.Message.BotSeq, TargetChangeSeq: int64(created.LastChangeSeq), CreatedAt: created.Message.CreatedAt}
	for _, change := range []func(*SyncEvent){func(e *SyncEvent) { e.CreatedAt = time.Unix(0, math.MaxInt64).UTC() }, func(e *SyncEvent) { e.CreatedAt = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC) }, func(e *SyncEvent) { e.MessageID = "absent" }, func(e *SyncEvent) { e.TargetChangeSeq = 0 }} {
		bad := base
		change(&bad)
		if err := f.store.Write(jobContext, func(tx *sql.Tx) error { return EnqueueSyncTx(jobContext, tx, bad) }); err == nil {
			t.Fatal("invalid event accepted", bad)
		}
		if jobCount(t, f.store, `SELECT count(*) FROM push_jobs`) != 0 {
			t.Fatal("invalid event persisted")
		}
	}
}

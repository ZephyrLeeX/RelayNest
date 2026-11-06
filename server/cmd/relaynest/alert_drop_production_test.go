package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"relaynest/server/internal/push"
)

// alert 仍由完整快照夹具直建；真实 TLS 消息生产者只生成 sync。
func TestAssembledAlertDropKeepsSyncWorker(t *testing.T) {
	f := botHTTP(t, nil)
	bot := clientMessageBot(t, f)
	syncCreateRegister(t, f, "synthetic-drop-registration", 7, 0)
	status, raw, headers := syncCreateRequest(f, bot, false, clientMessageKey)
	syncCreateReceipt(t, status, raw, headers, false)
	jobs := syncCreateJobs(t, f)
	if len(jobs) != 1 {
		t.Fatal("real sync producer")
	}
	original := jobs[0]
	now := time.Now().UTC()
	configuration := `{"enabled":true,"bot_overrides":[],"dnd":{"enabled":true,"timezone":"UTC","start":"` + now.Format("15:04") + `","end":"` + now.Add(time.Hour).Format("15:04") + `"},"preview":false,"notify_channel":"fcm","notify_provider":"fcm"}`
	// 故障/组合夹具保留原版本，不能冒充HTTP设置CAS行为。
	f.sql(`INSERT INTO device_notification_settings VALUES(?,1,?,?)`, original.Snapshot.DeviceID, now.Format(time.RFC3339Nano), configuration)
	// 原sync版本为0，另建匹配配置版本1的完整sync与alert，不提升旧事件。
	current := original.Snapshot
	current.ID = "drop-current-sync"
	current.SettingsVersion = 1
	alert := current
	alert.ID = "drop-current-alert"
	alert.Purpose = "alert"
	alert.AlertEligible = true
	if e := f.store.Write(context.Background(), func(tx *sql.Tx) error {
		if e := push.CreateJobTx(context.Background(), tx, current, current.CreatedAt); e != nil {
			return e
		}
		return push.CreateJobTx(context.Background(), tx, alert, alert.CreatedAt)
	}); e != nil {
		t.Fatal(e)
	}
	// 只使旧 sync 延后，通用预留仍真实选择按ID排序的 alert。
	f.sql(`UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, now.Add(time.Hour).Format("2006-01-02T15:04:05.000000000Z"), original.Snapshot.ID)
	r := push.NewJobRepository(f.store)
	held, e := r.Reserve(context.Background(), now, time.Minute, 1)
	if e != nil || len(held) != 1 || held[0].Job.Snapshot.ID != alert.ID {
		t.Fatal("real alert reserve", e)
	}
	// DND、共享已读和源清理不能被新终止入口提升为 silent sync 禁令。
	f.sql(`UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, current.BotSeq, current.BotID)
	f.sql(`DELETE FROM messages WHERE id=?`, current.MessageID)
	business := syncAttemptBusinessState(t, f)
	ok, e := r.DropAlertDND(context.Background(), held[0].Lease, alert, now)
	if e != nil || !ok {
		t.Fatal("drop", ok, e)
	}
	calls := 0
	worker := push.NewSyncWorker(r, func() time.Time { return now.Add(time.Second) }, time.Minute, assembledWorkerDispatch(func(_ context.Context, e push.SyncEnvelope) (push.SyncOutcome, error) {
		calls++
		if e.JobKey != current.ID {
			t.Fatal("wrong sync")
		}
		return push.SyncAccepted, nil
	}))
	got, e := worker.Step(context.Background())
	if e != nil || !got.Dispatched || !got.Applied || calls != 1 {
		t.Fatal("sync under DND", got, e, calls)
	}
	if business != syncAttemptBusinessState(t, f) {
		t.Fatal("business/read changed")
	}
	for _, j := range syncCreateJobs(t, f) {
		if j.Snapshot.ID == alert.ID && (j.State != "suppressed" || j.Attempt != 0) {
			t.Fatal("alert revived")
		}
	}
}

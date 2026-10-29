package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

func assembledLeaseCheck(t *testing.T, f *botHTTPFixture, lease push.JobLease, now time.Time, valid bool) *push.SyncLeaseSnapshot {
	t.Helper()
	before := readSnapshot(t, f)
	var got *push.SyncLeaseSnapshot
	err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		var err error
		got, err = push.CheckSyncLeaseTx(context.Background(), tx, lease, now)
		return err
	})
	if err != nil || (got != nil) != valid || before != readSnapshot(t, f) {
		t.Fatalf("check valid=%v got=%v err=%v", valid, got != nil, err)
	}
	return got
}

func TestAssembledSyncLeaseCheckProductionSnapshots(t *testing.T) {
	for _, admin := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			t.Run(fmt.Sprintf("admin=%v/configured=%v", admin, configured), func(t *testing.T) {
				f := botHTTP(t, nil)
				bot := clientMessageBot(t, f)
				syncCreateRegister(t, f, "synthetic-lease-registration-secret", 7, 0)
				if configured {
					// 真实偏好HTTP使配置为none+DND；静默sync不由提醒策略阻止。
					body := `{"expected_version":0,"enabled":false,"bot_overrides":[],"dnd":{"enabled":true,"timezone":"UTC","start":"22:00","end":"07:00"},"preview":false,"notify_channel":"none","notify_provider":null}`
					status, raw, h := f.admin("PUT", "/api/v1/devices/current/notifications", body)
					assertAssembledPush(t, status, 200, raw, h, "")
				}
				status, raw, headers := syncCreateRequest(f, bot, admin, clientMessageKey)
				m := syncCreateReceipt(t, status, raw, headers, admin)
				jobs := syncCreateJobs(t, f)
				if len(jobs) != 1 || jobs[0].Snapshot.MessageID != m.ID || jobs[0].Snapshot.Purpose != "sync" {
					t.Fatal("missing production HTTP job", jobs)
				}
				wantSettings := int64(0)
				if configured {
					wantSettings = 1
				}
				if jobs[0].Snapshot.SettingsVersion != wantSettings {
					t.Fatal("settings snapshot", jobs[0].Snapshot.SettingsVersion)
				}
				now := time.Now().UTC()
				var reserved []push.JobReservation
				if err := f.store.Write(context.Background(), func(tx *sql.Tx) error {
					var err error
					reserved, err = push.ReserveJobsTx(context.Background(), tx, now, time.Minute, 1)
					return err
				}); err != nil || len(reserved) != 1 {
					t.Fatal("real reservation", err, len(reserved))
				}
				checked := assembledLeaseCheck(t, f, reserved[0].Lease, now, true)
				if !reflect.DeepEqual(checked.Job, reserved[0].Job) {
					t.Fatal("snapshot replaced")
				}
				encoded, err := json.Marshal(checked)
				if err != nil {
					t.Fatal(err)
				}
				for _, private := range []string{"synthetic-lease-registration-secret", "lease_hash", "lease_until", "lease_token", "<html", "hello"} {
					if strings.Contains(string(encoded), private) {
						t.Fatal("sensitive projection")
					}
				}
				// 已读/源清理不把本层sync复核提升为提醒策略。
				f.sql(`UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, jobs[0].Snapshot.BotSeq, bot.Bot.ID)
				f.sql(`DELETE FROM messages WHERE id=?`, m.ID)
				assembledLeaseCheck(t, f, reserved[0].Lease, now, true)
				observer, err := storage.Open(context.Background(), storage.Config{Path: f.databasePath, BusyTimeout: 50 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				defer observer.Close()
				if err := observer.Read(context.Background(), func(tx *sql.Tx) error {
					got, err := push.CheckSyncLeaseTx(context.Background(), tx, reserved[0].Lease, now)
					if err == nil && got == nil {
						t.Fatal("reopen lost snapshot")
					}
					return err
				}); err != nil {
					t.Fatal(err)
				}
				// 真实注册HTTP轮换不能为旧job补新版本；旧秘密/快照均保持。
				syncCreateRegister(t, f, "synthetic-lease-new-registration-secret", 8, 7)
				assembledLeaseCheck(t, f, reserved[0].Lease, now, false)
				after := syncCreateJobs(t, f)
				if len(after) != 1 || after[0].Snapshot != jobs[0].Snapshot || after[0].Attempt != 0 {
					t.Fatal("rotation promoted old job")
				}
			})
		}
	}
}

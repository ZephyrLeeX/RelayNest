package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
	sharedsync "relaynest/server/internal/sync"
)

func sourceReadAdvance(t *testing.T, f *fixture, seq int64) {
	t.Helper()
	if err := f.store.Write(ctx, func(tx *sql.Tx) error {
		_, _, err := sharedsync.AdvanceReadTx(ctx, tx, f.bot.Bot.ID, seq)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// 同时检查全库和原始26列；只允许目标state和两列lease改变。
func sourceReadProjection(t *testing.T, f *fixture, id string, original map[string][]any) {
	t.Helper()
	want := append([]any(nil), original[id]...)
	want[4], want[19], want[20] = "suppressed", nil, nil
	got := rows(t, f.store)
	original[id] = want
	if !reflect.DeepEqual(original, got) {
		t.Fatal("unexpected job projection")
	}
}

func TestAlertSourceReadDropPersistentAndFenced(t *testing.T) {
	for _, reason := range []string{"missing", "equal", "above"} {
		for _, started := range []string{"none", "pending", "result"} {
			t.Run(reason+"/"+started, func(t *testing.T) {
				f, h, n := prepared(t)
				r := push.NewJobRepository(f.store)
				var ticket *push.AlertAttempt
				if started != "none" {
					ticket = begin(t, f, h, n)
				}
				if started == "result" {
					if ok, err := r.ApplyAlertResult(ctx, *ticket, push.AlertTransient, n); err != nil || !ok {
						t.Fatal("retry result", ok, err)
					}
					n = read(t, f, h.Job.Snapshot.ID).NextAttemptAt
					held, err := r.ReserveOPushAlerts(ctx, n, time.Minute, 1)
					if err != nil || len(held) != 1 {
						t.Fatal("retry lease", err, len(held))
					}
					h = held[0]
				}
				switch reason {
				case "missing":
					exec(t, f.store, `DELETE FROM messages WHERE id=?`, h.Job.Snapshot.MessageID)
				case "equal":
					sourceReadAdvance(t, f, h.Job.Snapshot.BotSeq)
				case "above":
					f.source(t)
					sourceReadAdvance(t, f, h.Job.Snapshot.BotSeq+1)
				}
				before, original := database(t, f.store), rows(t, f.store)
				oldRow := fmt.Sprintf("%#v", original[h.Job.Snapshot.ID])
				if ok, err := r.DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n); err != nil || !ok {
					t.Fatal("drop", ok, err)
				}
				sourceReadProjection(t, f, h.Job.Snapshot.ID, original)
				wantDB := strings.Replace(before, oldRow, fmt.Sprintf("%#v", original[h.Job.Snapshot.ID]), 1)
				if wantDB == before || database(t, f.store) != wantDB {
					t.Fatal("unexpected database write")
				}
				other, err := storage.Open(ctx, f.cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer other.Close()
				if database(t, other) != wantDB {
					t.Fatal("second store missed commit")
				}
				f.store.Close()
				f.store, err = storage.Open(ctx, f.cfg)
				if err != nil {
					t.Fatal(err)
				}
				r = push.NewJobRepository(f.store)
				if database(t, f.store) != wantDB {
					t.Fatal("reopen missed commit")
				}
				if ok, err := r.DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n); err != nil || ok {
					t.Fatal("duplicate", ok, err)
				}
				if ok, err := r.Release(ctx, h.Lease, n); err != nil || ok {
					t.Fatal("old release", ok, err)
				}
				if x, err := r.BeginAlert(ctx, h.Lease, n); err != nil || x != nil {
					t.Fatal("old begin", err)
				}
				if ticket != nil {
					if ok, err := r.CheckAlertAttempt(ctx, h.Lease, *ticket, n); err != nil || ok {
						t.Fatal("old check", ok, err)
					}
					if ok, err := r.ApplyAlertResult(ctx, *ticket, push.AlertAccepted, n); err != nil || ok {
						t.Fatal("late result", ok, err)
					}
				}
				if held, err := r.ReserveOPushAlerts(ctx, n.Add(2*time.Minute), time.Minute, 1); err != nil || len(held) != 0 {
					t.Fatal("revived", err, len(held))
				}
				if database(t, f.store) != wantDB {
					t.Fatal("late operation wrote")
				}
			})
		}
	}
}

func TestAlertSourceReadDropUsesPositiveEvidence(t *testing.T) {
	for _, policy := range []string{"disabled", "dnd", "corrupt"} {
		t.Run(policy, func(t *testing.T) {
			f, h, n := prepared(t)
			switch policy {
			case "disabled":
				f.configure(t, strings.Replace(configuration(), `"enabled":true`, `"enabled":false`, 1))
			case "dnd":
				f.configure(t, strings.ReplaceAll(strings.Replace(configuration(), `"enabled":false`, `"enabled":true`, 1), `"22:00","end":"07:00"`, fmt.Sprintf(`"%s","end":"%s"`, n.Format("15:04"), n.Add(time.Hour).Format("15:04"))))
			case "corrupt":
				f.configure(t, `{"unexpected":true}`)
			}
			before := database(t, f.store)
			if ok, err := push.NewJobRepository(f.store).DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n); err != nil || ok {
				t.Fatal("policy denial is not source evidence", ok, err)
			}
			if database(t, f.store) != before {
				t.Fatal("policy denial wrote")
			}
			// 同版本策略状态不充当正证据，也不能掩盖独立确认的源已清理。
			exec(t, f.store, `DELETE FROM messages`)
			if ok, err := push.NewJobRepository(f.store).DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n); err != nil || !ok {
				t.Fatal("missing source", ok, err)
			}
		})
	}
}

func TestAlertSourceReadDropNoAuthority(t *testing.T) {
	for _, mode := range []string{"unread", "below", "wrong-bot", "wrong-seq", "expected", "registration", "settings", "session", "generation", "instance", "client", "release", "takeover", "lease-expiry", "expiry", "fcm", "sync", "ineligible", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			f, h, n := prepared(t)
			r := push.NewJobRepository(f.store)
			expected := h.Job.Snapshot
			if mode == "below" {
				event := f.source(t)
				// 保留一个更高源，让read低于该原提醒而不是单测helper。
				if ok, err := r.Release(ctx, h.Lease, n); err != nil || !ok {
					t.Fatal(err)
				}
				exec(t, f.store, `DELETE FROM push_jobs`)
				j := h.Job.Snapshot
				j.MessageID, j.BotSeq, j.TargetChangeSeq = event.MessageID, event.BotSeq, event.TargetChangeSeq
				j.CreatedAt, j.ExpiresAt = event.CreatedAt, event.CreatedAt.Add(24*time.Hour)
				n = time.Now().UTC()
				if err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); err != nil {
					t.Fatal(err)
				}
				held, err := r.ReserveOPushAlerts(ctx, n, time.Minute, 1)
				if err != nil || len(held) != 1 {
					t.Fatal(err, len(held))
				}
				h = held[0]
				expected = h.Job.Snapshot
				sourceReadAdvance(t, f, expected.BotSeq-1)
			}
			if mode != "unread" && mode != "below" && mode != "wrong-bot" && mode != "wrong-seq" && mode != "fcm" && mode != "sync" && mode != "ineligible" {
				exec(t, f.store, `DELETE FROM messages`)
			}
			switch mode {
			case "wrong-bot":
				// 通过可信新bot行制造源与原快照错配，不给其永久终止资格。
				b, _, err := bots.New(f.store).Create(ctx, bots.CreateRequest{Name: "other", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
				if err != nil {
					t.Fatal(err)
				}
				exec(t, f.store, `UPDATE messages SET bot_id=?`, b.Bot.ID)
			case "wrong-seq":
				exec(t, f.store, `UPDATE messages SET bot_seq=bot_seq+1`)
			case "expected":
				expected.ID = "different"
			case "registration":
				exec(t, f.store, `UPDATE device_push_registrations SET registration_version=2`)
			case "settings":
				exec(t, f.store, `UPDATE device_notification_settings SET settings_version=2`)
			case "session":
				exec(t, f.store, `UPDATE sessions SET revoked=1`)
			case "generation":
				exec(t, f.store, `UPDATE administrator SET auth_generation=2`)
			case "instance":
				exec(t, f.store, `UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'`)
				f.store.Close()
				var err error
				f.store, err = storage.Open(ctx, f.cfg)
				if err != nil {
					t.Fatal(err)
				}
				r = push.NewJobRepository(f.store)
			case "client":
				exec(t, f.store, `UPDATE sessions SET client_type='web'`)
			case "release":
				if ok, err := r.Release(ctx, h.Lease, n); err != nil || !ok {
					t.Fatal(err)
				}
			case "takeover":
				n = n.Add(2 * time.Minute)
				if held, err := r.ReserveOPushAlerts(ctx, n, time.Minute, 1); err != nil || len(held) != 1 {
					t.Fatal(err, len(held))
				}
			case "lease-expiry":
				n = n.Add(time.Minute)
			case "expiry":
				n = expected.ExpiresAt
			case "fcm", "sync", "ineligible", "legacy":
				exec(t, f.store, `DELETE FROM push_jobs`)
				exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, n.Format(stamp))
				expected.ID = "negative"
				if mode == "legacy" {
					exec(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('negative',?,?,1,'legacy')`, expected.SessionID, expected.DeviceID)
				} else {
					if mode == "fcm" || mode == "sync" {
						expected.Provider = "fcm"
					}
					if mode == "sync" {
						expected.Purpose = "sync"
					}
					if mode != "fcm" {
						expected.AlertEligible = false
					}
					if err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, expected, n) }); err != nil {
						t.Fatal(err)
					}
					held, err := r.Reserve(ctx, n, time.Minute, 1)
					if err != nil || len(held) != 1 {
						t.Fatal(err, len(held))
					}
					h = held[0]
				}
				exec(t, f.store, `DELETE FROM messages`)
			}
			before := database(t, f.store)
			if ok, err := r.DropAlertSourceRead(ctx, h.Lease, expected, n); err != nil || ok {
				t.Fatal("unqualified", ok, err)
			}
			if database(t, f.store) != before {
				t.Fatal("unqualified wrote")
			}
		})
	}
}

func TestAlertSourceReadDropConcurrentHolder(t *testing.T) {
	f, h, n := prepared(t)
	r := push.NewJobRepository(f.store)
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, n.Format(stamp))
	j := h.Job.Snapshot
	j.ID, j.Purpose, j.Provider, j.AlertEligible = "sync-after-drop", "sync", "fcm", false
	if err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); err != nil {
		t.Fatal(err)
	}
	other, err := storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	r2 := push.NewJobRepository(other)
	if held, err := r2.ReserveSync(ctx, n, time.Minute, 1); err != nil || len(held) != 0 {
		t.Fatal("holder missing", err, len(held))
	}
	exec(t, f.store, `DELETE FROM messages`)
	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, repo := range []*push.JobRepository{r, r2} {
		wg.Add(1)
		go func(repo *push.JobRepository) {
			defer wg.Done()
			<-start
			ok, err := repo.DropAlertSourceRead(ctx, h.Lease, h.Job.Snapshot, n)
			results <- ok
			errs <- err
		}(repo)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	count := 0
	for ok := range results {
		if ok {
			count++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatal("expected one commit", count)
	}
	if held, err := r2.ReserveSync(ctx, n, time.Minute, 1); err != nil || len(held) != 1 || held[0].Job.Snapshot.ID != j.ID {
		t.Fatal("holder not released", err, len(held))
	}
}

func TestAlertSourceReadDropInputAnd24HourLimit(t *testing.T) {
	f, h, n := prepared(t)
	r := push.NewJobRepository(f.store)
	before := database(t, f.store)
	c, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		name string
		r    *push.JobRepository
		ctx  context.Context
		l    push.JobLease
		j    push.JobSnapshot
		n    time.Time
	}{
		{"nil-context", r, nil, h.Lease, h.Job.Snapshot, n},
		{"canceled", r, c, h.Lease, h.Job.Snapshot, n},
		{"nil-repo", nil, ctx, h.Lease, h.Job.Snapshot, n},
		{"zero-repo", &push.JobRepository{}, ctx, h.Lease, h.Job.Snapshot, n},
		{"lease", r, ctx, push.JobLease{}, h.Job.Snapshot, n},
		{"snapshot", r, ctx, h.Lease, push.JobSnapshot{}, n},
		{"time", r, ctx, h.Lease, h.Job.Snapshot, time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ok, err := tc.r.DropAlertSourceRead(tc.ctx, tc.l, tc.j, tc.n); err == nil || ok {
				t.Fatal(ok, err)
			}
			if database(t, f.store) != before {
				t.Fatal("input wrote")
			}
		})
	}
	if ok, err := r.Release(ctx, h.Lease, n); err != nil || !ok {
		t.Fatal(err)
	}
	exec(t, f.store, `DELETE FROM push_jobs`)
	j := h.Job.Snapshot
	j.ID, j.ExpiresAt = "long-ttl", j.CreatedAt.Add(48*time.Hour)
	exec(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=?`, j.CreatedAt.Add(72*time.Hour).UnixNano(), j.CreatedAt.Add(72*time.Hour).UnixNano())
	if err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); err != nil {
		t.Fatal(err)
	}
	held, err := r.ReserveOPushAlerts(ctx, n, 49*time.Hour, 1)
	if err != nil || len(held) != 1 {
		t.Fatal(err, len(held))
	}
	h = held[0]
	exec(t, f.store, `DELETE FROM messages`)
	limit := j.CreatedAt.Add(24 * time.Hour)
	if err := f.store.Read(ctx, func(tx *sql.Tx) error {
		checked, err := push.CheckAlertLeaseTx(ctx, tx, h.Lease, limit)
		if err == nil && checked == nil {
			return fmt.Errorf("expected current lease beyond 24h")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before = database(t, f.store)
	if ok, err := r.DropAlertSourceRead(ctx, h.Lease, j, limit); err != nil || ok {
		t.Fatal("24h boundary", ok, err)
	}
	if database(t, f.store) != before {
		t.Fatal("24h wrote")
	}
	if ok, err := r.DropAlertSourceRead(ctx, h.Lease, j, limit.Add(-time.Nanosecond)); err != nil || !ok {
		t.Fatal("before 24h", ok, err)
	}
}

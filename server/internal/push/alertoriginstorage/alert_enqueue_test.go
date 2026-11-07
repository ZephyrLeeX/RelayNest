// Package alertoriginstorage_test 用真实 SQLite 独立覆盖原始提醒资格，不改变原生 push 测试族。
package alertoriginstorage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

var ctx = context.Background()

const device = "11111111-1111-4111-8111-111111111111"

type fixture struct {
	store    *storage.Store
	cfg      storage.Config
	identity bots.Identity
	bot      bots.Created
	session  auth.Session
	serial   int
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "alert.sqlite")}}
	var err error
	f.store, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.store.Close() })
	exec(t, f.store, `INSERT INTO administrator VALUES(1,'admin','synthetic-hash',1)`)
	f.bot, _, err = bots.New(f.store).Create(ctx, bots.CreateRequest{Name: "alert", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.identity, err = bots.New(f.store).Authenticate(ctx, f.bot.Bot.ID, bots.APIToken, f.bot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	f.session = f.addSession(t, device)
	f.configure(t, configuration())
	return f
}
func exec(t *testing.T, s *storage.Store, q string, args ...any) {
	t.Helper()
	if err := s.Write(ctx, func(tx *sql.Tx) error { _, e := tx.ExecContext(ctx, q, args...); return e }); err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) addSession(t *testing.T, d string) auth.Session {
	t.Helper()
	s, _, err := auth.NewSessionStore(f.store).Create(ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: d, DeviceName: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'opush',1,'synthetic-secret',?)`, s.ID, time.Now().UTC().Format(time.RFC3339Nano))
	return s
}
func event(m messages.Created) push.SyncEvent {
	return push.SyncEvent{MessageID: m.Message.ID, BotID: m.Message.BotID, BotSeq: m.Message.BotSeq, TargetChangeSeq: int64(m.LastChangeSeq), CreatedAt: m.Message.CreatedAt}
}
func (f *fixture) source(t *testing.T) push.SyncEvent {
	t.Helper()
	m, err := messages.New(f.store).CreateApplication(ctx, f.identity, messages.TextInput{BotID: f.bot.Bot.ID, Format: "plain", Text: "synthetic-message"})
	if err != nil {
		t.Fatal(err)
	}
	return event(m)
}
func enqueue(t *testing.T, s *storage.Store, e push.SyncEvent) {
	t.Helper()
	if err := s.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueAlertTx(ctx, tx, e) }); err != nil {
		t.Fatal(err)
	}
}
func rows(t *testing.T, s *storage.Store) map[string][]any {
	t.Helper()
	out := map[string][]any{}
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		r, e := tx.QueryContext(ctx, `SELECT * FROM push_jobs ORDER BY id`)
		if e != nil {
			return e
		}
		defer r.Close()
		cols, e := r.Columns()
		if e != nil {
			return e
		}
		if len(cols) != 26 {
			t.Fatalf("expected all 26 columns, got %d", len(cols))
		}
		for r.Next() {
			v := make([]any, len(cols))
			p := make([]any, len(cols))
			for i := range v {
				p[i] = &v[i]
			}
			if e = r.Scan(p...); e != nil {
				return e
			}
			out[v[0].(string)] = v
		}
		return r.Err()
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// vendor 配置仅为内部 fixture，不代表真实平台注册或 readiness 已通过。
func configuration() string {
	return `{"enabled":true,"bot_overrides":[],"dnd":{"enabled":false,"timezone":"UTC","start":"22:00","end":"07:00"},"preview":false,"notify_channel":"vendor","notify_provider":"opush"}`
}
func (f *fixture) configure(t *testing.T, c string) {
	t.Helper()
	exec(t, f.store, `INSERT INTO device_notification_settings VALUES(?,1,?,?) ON CONFLICT(device_id) DO UPDATE SET configuration=excluded.configuration`, device, time.Now().UTC().Format(time.RFC3339Nano), c)
}
func (f *fixture) repository(hook func(context.Context, *sql.Tx, messages.Created) error) *messages.Repository {
	if hook == nil {
		hook = func(c context.Context, tx *sql.Tx, m messages.Created) error {
			return push.EnqueueAlertTx(c, tx, event(m))
		}
	}
	return messages.NewWithFirstCreate(f.store, hook)
}
func (f *fixture) create(c context.Context) (messages.Created, error) {
	f.serial++
	return f.repository(nil).CreateIdempotentApplication(c, f.identity, fmt.Sprintf("00000000-0000-4000-8000-%012d", f.serial), messages.CreateRequest{Format: "plain", Text: "atomic"}, nil)
}
func state(t *testing.T, s *storage.Store) string {
	t.Helper()
	var v string
	if e := s.Read(ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT json_array((SELECT count(*) FROM messages),(SELECT count(*) FROM changes),(SELECT count(*) FROM deliveries),(SELECT count(*) FROM idempotency),(SELECT max(bot_seq) FROM messages))`).Scan(&v)
	}); e != nil {
		t.Fatal(e)
	}
	return v
}
func unchanged(t *testing.T, f *fixture, before map[string][]any, prior string) {
	t.Helper()
	if !reflect.DeepEqual(before, rows(t, f.store)) || prior != state(t, f.store) {
		t.Fatal("事务发生部分提交")
	}
}
func TestOriginalSnapshotCrossBotSessionsAndOldRows(t *testing.T) {
	f := setup(t)
	other := f.addSession(t, device)
	// 构造旧 sync、legacy、leased、suppressed；新 alert 不得改动任一旧列。
	old := f.source(t)
	if e := f.store.Write(ctx, func(tx *sql.Tx) error {
		return push.CreateJobTx(ctx, tx, push.JobSnapshot{ID: "old-sync", SessionID: f.session.ID, DeviceID: device, Purpose: "sync", Provider: "fcm", TargetChangeSeq: old.TargetChangeSeq, RegistrationVersion: 1, SettingsVersion: 1, DeploymentID: instance(t, tx), MessageID: old.MessageID, BotID: old.BotID, BotSeq: old.BotSeq, SessionGeneration: 1, CreatedAt: old.CreatedAt, ExpiresAt: old.CreatedAt.Add(24 * time.Hour)}, old.CreatedAt)
	}); e == nil {
		t.Fatal("fixture must require fcm registration")
	}
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, time.Now().UTC().Format(time.RFC3339Nano))
	if e := f.store.Write(ctx, func(tx *sql.Tx) error {
		return push.CreateJobTx(ctx, tx, push.JobSnapshot{ID: "old-sync", SessionID: f.session.ID, DeviceID: device, Purpose: "sync", Provider: "fcm", TargetChangeSeq: old.TargetChangeSeq, RegistrationVersion: 1, SettingsVersion: 1, DeploymentID: instance(t, tx), MessageID: old.MessageID, BotID: old.BotID, BotSeq: old.BotSeq, SessionGeneration: 1, CreatedAt: old.CreatedAt, ExpiresAt: old.CreatedAt.Add(24 * time.Hour)}, old.CreatedAt)
	}); e != nil {
		t.Fatal(e)
	}
	exec(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)VALUES('legacy',?,?,1,'legacy')`, f.session.ID, device)
	enqueue(t, f.store, old)
	exec(t, f.store, `UPDATE push_jobs SET state='suppressed' WHERE purpose='alert' AND session_id=?`, f.session.ID)
	exec(t, f.store, `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=? WHERE purpose='alert' AND session_id=?`, time.Now().Add(time.Hour).UnixNano(), other.ID)
	before := rows(t, f.store)
	first, e := f.create(ctx)
	if e != nil {
		t.Fatal(e)
	}
	secondBot, _, e := bots.New(f.store).Create(ctx, bots.CreateRequest{Name: "other", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if e != nil {
		t.Fatal(e)
	}
	id, e := bots.New(f.store).Authenticate(ctx, secondBot.Bot.ID, bots.APIToken, secondBot.APIToken.Token)
	if e != nil {
		t.Fatal(e)
	}
	second, e := f.repository(nil).CreateIdempotentApplication(ctx, id, "00000000-0000-4000-8000-000000000900", messages.CreateRequest{Format: "plain", Text: "other"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	after := rows(t, f.store)
	if len(after) != len(before)+4 {
		t.Fatal("cross bot/session merged", len(after))
	}
	for id, v := range before {
		if !reflect.DeepEqual(v, after[id]) {
			t.Fatal("old row changed", id)
		}
	}
	for id, v := range after {
		if _, ok := before[id]; ok {
			continue
		}
		var m messages.Created
		if v[12] == first.Message.ID {
			m = first
		} else if v[12] == second.Message.ID {
			m = second
		} else {
			t.Fatal("wrong anchor", v)
		}
		if v[3] != int64(m.LastChangeSeq) || v[4] != "queued" || v[5] != int64(0) || v[7] != "alert" || v[8] != "opush" || v[13] != m.Message.BotID || v[14] != m.Message.BotSeq || v[15] != int64(1) || v[16] != m.Message.CreatedAt.UnixNano() || v[17] != m.Message.CreatedAt.Add(24*time.Hour).UnixNano() || v[18] != int64(1) || v[6] != m.Message.CreatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z") {
			t.Fatal("snapshot mismatch", v)
		}
		for _, v := range v[19:] {
			if v != nil {
				t.Fatal("nonempty metadata")
			}
		}
	}
}
func instance(t *testing.T, tx *sql.Tx) string {
	t.Helper()
	v, e := storage.ReadInstanceIDTx(ctx, tx)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestSourceValidationEvenWithoutTargets(t *testing.T) {
	for _, target := range []string{"vendor", "none", "fcm", "no-session"} {
		for _, fault := range []string{"valid", "bad-message", "bad-bot", "bad-seq", "bad-time", "bad-target", "bad-change", "bad-created-change", "bad-sender", "blob-sender", "admin", "admin-bad-change"} {
			t.Run(target+"/"+fault, func(t *testing.T) {
				f := setup(t)
				e := f.source(t)
				switch target {
				case "none":
					f.configure(t, strings.ReplaceAll(strings.ReplaceAll(configuration(), `"vendor"`, `"none"`), `"opush"`, `null`))
				case "fcm":
					f.configure(t, strings.ReplaceAll(strings.ReplaceAll(configuration(), `"vendor"`, `"fcm"`), `"opush"`, `"fcm"`))
				case "no-session":
					exec(t, f.store, `DELETE FROM sessions`)
				}
				switch fault {
				case "bad-message":
					e.MessageID = "absent"
				case "bad-bot":
					e.BotID = "absent"
				case "bad-seq":
					e.BotSeq++
				case "bad-time":
					e.CreatedAt = e.CreatedAt.Add(time.Nanosecond)
				case "bad-target":
					e.TargetChangeSeq--
				case "bad-change":
					exec(t, f.store, `UPDATE changes SET revision=2 WHERE entity_id=?`, e.MessageID)
				case "bad-created-change":
					exec(t, f.store, `UPDATE changes SET created_at='2000-01-01T00:00:00Z' WHERE entity_id=?`, e.MessageID)
				case "bad-sender":
					exec(t, f.store, `PRAGMA ignore_check_constraints=ON;UPDATE messages SET sender='unknown' WHERE id=?`, e.MessageID)
				case "blob-sender":
					exec(t, f.store, `PRAGMA ignore_check_constraints=ON;UPDATE messages SET sender=CAST(sender AS BLOB) WHERE id=?`, e.MessageID)
				case "admin", "admin-bad-change":
					exec(t, f.store, `UPDATE messages SET sender='administrator' WHERE id=?`, e.MessageID)
					if fault == "admin-bad-change" {
						exec(t, f.store, `UPDATE changes SET revision=2 WHERE entity_id=?`, e.MessageID)
					}
				}
				before := rows(t, f.store)
				err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueAlertTx(ctx, tx, e) })
				good := fault == "valid" || fault == "admin"
				if (err == nil) != good {
					t.Fatal("source outcome", err)
				}
				want := 0
				if good && target == "vendor" && fault == "valid" {
					want = 1
				}
				if len(rows(t, f.store)) != want {
					t.Fatal("unexpected writes")
				}
				if !good && !reflect.DeepEqual(before, rows(t, f.store)) {
					t.Fatal("failed source wrote")
				}
			})
		}
	}
}

func TestPolicyDeniedAndCorruptConfiguration(t *testing.T) {
	for _, name := range []string{"disabled", "bot", "read", "dnd", "dnd-end", "dnd-start", "bad-json", "bad-fields", "bad-clock", "bad-type", "no-reg", "empty-token", "bad-version"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			e := f.source(t)
			c := configuration()
			want := false
			bad := false
			switch name {
			case "disabled":
				c = strings.Replace(c, `"enabled":true`, `"enabled":false`, 1)
			case "bot":
				c = strings.Replace(c, `"bot_overrides":[]`, fmt.Sprintf(`"bot_overrides":[{"bot_id":%q,"enabled":false}]`, e.BotID), 1)
			case "read":
				exec(t, f.store, `INSERT INTO bot_read_states(bot_id,read_seq,updated_at)VALUES(?,?,?) ON CONFLICT(bot_id) DO UPDATE SET read_seq=excluded.read_seq`, e.BotID, e.BotSeq, e.CreatedAt.Format(time.RFC3339Nano))
			case "dnd", "dnd-start", "dnd-end":
				clock := e.CreatedAt.UTC().Format("15:04")
				start, end := clock, e.CreatedAt.Add(time.Minute).UTC().Format("15:04")
				if name == "dnd-end" {
					start = e.CreatedAt.Add(-time.Minute).UTC().Format("15:04")
					end = clock
					want = true
				}
				c = strings.Replace(c, `"enabled":false,"timezone":"UTC","start":"22:00","end":"07:00"`, fmt.Sprintf(`"enabled":true,"timezone":"UTC","start":%q,"end":%q`, start, end), 1)
			case "bad-json":
				c = "{"
				bad = true
			case "bad-fields":
				c = `{}`
				bad = true
			case "bad-clock":
				c = strings.Replace(c, `22:00`, `99:99`, 1)
				bad = true
			case "bad-type":
				exec(t, f.store, `UPDATE device_notification_settings SET configuration=CAST(configuration AS BLOB)`)
				bad = true
			case "no-reg":
				exec(t, f.store, `DELETE FROM device_push_registrations`)
			case "empty-token":
				exec(t, f.store, `PRAGMA ignore_check_constraints=ON;UPDATE device_push_registrations SET token=''`)
				bad = true
			case "bad-version":
				exec(t, f.store, `PRAGMA ignore_check_constraints=ON;UPDATE device_push_registrations SET registration_version=0`)
				bad = true
			}
			if name != "bad-type" {
				f.configure(t, c)
			}
			err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueAlertTx(ctx, tx, e) })
			if (err != nil) != bad {
				t.Fatal("denial vs corruption", err)
			}
			n := 0
			if want {
				n = 1
			}
			if len(rows(t, f.store)) != n {
				t.Fatal("policy wrote")
			}
			// DND 结束只允许未来新事件，生产入口不提供时钟覆盖或历史重放。
			if name == "dnd" {
				f.configure(t, configuration())
				if len(rows(t, f.store)) != 0 {
					t.Fatal("DND revived")
				}
				if _, err = f.create(ctx); err != nil {
					t.Fatal(err)
				}
				if len(rows(t, f.store)) != 1 {
					t.Fatal("future event missing")
				}
			}
		})
	}
}

func TestDuplicateAndRotationNeverRefreshOldRows(t *testing.T) {
	for _, kind := range []string{"same", "registration", "settings", "generation", "leased", "suppressed"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			e := f.source(t)
			enqueue(t, f.store, e)
			switch kind {
			case "registration":
				exec(t, f.store, `UPDATE device_push_registrations SET registration_version=2`)
			case "settings":
				exec(t, f.store, `UPDATE device_notification_settings SET settings_version=2`)
			case "generation":
				exec(t, f.store, `UPDATE administrator SET auth_generation=2;UPDATE sessions SET auth_generation=2;INSERT INTO device_push_registrations SELECT id,'opush',2,'new-synthetic','2026-11-07T00:00:00Z' FROM sessions`)
			case "leased":
				exec(t, f.store, `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=?`, time.Now().Add(time.Hour).UnixNano())
			case "suppressed":
				exec(t, f.store, `UPDATE push_jobs SET state='suppressed'`)
			}
			before, prior := rows(t, f.store), state(t, f.store)
			if err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueAlertTx(ctx, tx, e) }); err == nil {
				t.Fatal("duplicate succeeded")
			}
			unchanged(t, f, before, prior)
		})
	}
}

// BE10-ALERT-ORIGIN-A01-01：策略拒绝不得把真实目标的重复调用变成成功。
func TestDuplicateDeniedPolicyStillErrors(t *testing.T) {
	for _, policy := range []string{"disabled", "read", "dnd"} {
		t.Run(policy, func(t *testing.T) {
			for _, oldState := range []string{"queued", "leased", "suppressed", "absent"} {
				t.Run(oldState, func(t *testing.T) {
					f := setup(t)
					e := f.source(t)
					if oldState != "absent" {
						enqueue(t, f.store, e)
					}
					switch oldState {
					case "leased":
						exec(t, f.store, `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=?`, e.CreatedAt.Add(time.Hour).UnixNano())
					case "suppressed":
						exec(t, f.store, `UPDATE push_jobs SET state='suppressed'`)
					}
					c := configuration()
					switch policy {
					case "disabled":
						c = strings.Replace(c, `"enabled":true`, `"enabled":false`, 1)
					case "read":
						exec(t, f.store, `INSERT INTO bot_read_states(bot_id,read_seq,updated_at)VALUES(?,?,?) ON CONFLICT(bot_id) DO UPDATE SET read_seq=excluded.read_seq`, e.BotID, e.BotSeq, e.CreatedAt.Format(time.RFC3339Nano))
					case "dnd":
						c = strings.Replace(c, `"enabled":false,"timezone":"UTC","start":"22:00","end":"07:00"`, fmt.Sprintf(`"enabled":true,"timezone":"UTC","start":%q,"end":%q`, e.CreatedAt.UTC().Format("15:04"), e.CreatedAt.Add(time.Minute).UTC().Format("15:04")), 1)
					}
					f.configure(t, c)
					before, prior := rows(t, f.store), state(t, f.store)
					err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueAlertTx(ctx, tx, e) })
					if oldState == "absent" {
						if err != nil {
							t.Fatal("new denied event failed", err)
						}
					} else if !errors.Is(err, push.ErrJobData) {
						t.Fatal("duplicate denied event must fail", err)
					}
					unchanged(t, f, before, prior)
				})
			}
		})
	}
}

func TestWriteFaultsRollbackFirstCreate(t *testing.T) {
	faults := map[string]string{
		"ignore":       `CREATE TRIGGER fault BEFORE INSERT ON push_jobs BEGIN SELECT RAISE(IGNORE);END`,
		"sql":          `CREATE TRIGGER fault BEFORE INSERT ON push_jobs BEGIN SELECT RAISE(ABORT,'synthetic');END`,
		"source":       `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE changes SET revision=2 WHERE entity_id=NEW.message_id;END`,
		"sender":       `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE messages SET sender='administrator' WHERE id=NEW.message_id;END`,
		"registration": `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE device_push_registrations SET registration_version=registration_version+1;END`,
		"settings":     `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE device_notification_settings SET settings_version=settings_version+1;END`,
		"session":      `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE sessions SET revoked=1;END`,
		"read":         `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN INSERT INTO bot_read_states(bot_id,read_seq,updated_at) VALUES(NEW.bot_id,NEW.bot_seq,'2026-11-07T00:00:00Z') ON CONFLICT(bot_id) DO UPDATE SET read_seq=NEW.bot_seq;END`,
		"policy":       `CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE device_notification_settings SET configuration=replace(configuration,'"enabled":true','"enabled":false');END`,
		"commit":       `CREATE TABLE deferred_fault(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN INSERT INTO deferred_fault VALUES('absent');END`,
		"receipt":      `CREATE TRIGGER fault BEFORE INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'synthetic');END`,
	}
	for name, q := range faults {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.addSession(t, device)
			before, prior := rows(t, f.store), state(t, f.store)
			exec(t, f.store, q)
			out, err := f.create(ctx)
			if err == nil || out.Message.ID != "" {
				t.Fatal("published failed transaction", err)
			}
			unchanged(t, f, before, prior)
		})
	}
	// 真实 deferred FK 的有效参照阴性对照，必须成功提交。
	t.Run("valid-deferred-commit", func(t *testing.T) {
		f := setup(t)
		exec(t, f.store, `CREATE TABLE deferred_fault(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN INSERT INTO deferred_fault VALUES(NEW.bot_id);END`)
		if _, err := f.create(ctx); err != nil {
			t.Fatal(err)
		}
		if len(rows(t, f.store)) != 1 {
			t.Fatal("valid commit missing")
		}
	})
}

func TestAll26ColumnsPostWriteTamperRollback(t *testing.T) {
	mutations := map[string]string{
		"id": `id=id||'changed'`, "session_id": `session_id=(SELECT id FROM sessions WHERE id<>NEW.session_id LIMIT 1)`,
		"device_id": `session_id=(SELECT id FROM sessions WHERE device_id<>NEW.device_id LIMIT 1),device_id=(SELECT device_id FROM sessions WHERE device_id<>NEW.device_id LIMIT 1)`, "target_change_seq": `target_change_seq=target_change_seq+1`,
		"state": `state='suppressed'`, "attempt": `attempt=1`, "next_attempt_at": `next_attempt_at='2026-11-07T00:00:00.000000000Z'`,
		"purpose": `purpose='sync'`, "provider": `provider='fcm'`, "registration_version": `registration_version=2`,
		"settings_version": `settings_version=2`, "deployment_id": `deployment_id=deployment_id||'x'`, "message_id": `message_id=message_id||'x'`,
		"bot_id": `bot_id=bot_id||'x'`, "bot_seq": `bot_seq=bot_seq+1`, "session_generation": `session_generation=2`,
		"created_at_ns": `created_at_ns=created_at_ns+1`, "expires_at_ns": `expires_at_ns=expires_at_ns+1`, "alert_eligible": `alert_eligible=0`,
		"lease_hash": `lease_hash=zeroblob(32)`, "lease_until_ns": `lease_until_ns=1`, "attempt_lease_hash": `attempt_lease_hash=zeroblob(32)`,
		"attempt_lease_until_ns": `attempt_lease_until_ns=1`, "attempt_started_at_ns": `attempt_started_at_ns=1`, "result_kind": `result_kind='accepted'`, "result_at_ns": `result_at_ns=1`,
	}
	if len(mutations) != 26 {
		t.Fatal("fixture must cover 26 columns")
	}
	for name, mutation := range mutations {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.addSession(t, device)
			f.addSession(t, "22222222-2222-4222-8222-222222222222")
			// 放开 SQLite CHECK/immutable，要求生产写后核验本身拒绝篡改。
			exec(t, f.store, `PRAGMA ignore_check_constraints=ON;DROP TRIGGER push_jobs_snapshot_immutable;CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE push_jobs SET `+mutation+` WHERE id=NEW.id;END`)
			before, prior := rows(t, f.store), state(t, f.store)
			var hookErr error
			repo := f.repository(func(c context.Context, tx *sql.Tx, m messages.Created) error {
				hookErr = push.EnqueueAlertTx(c, tx, event(m))
				return hookErr
			})
			out, err := repo.CreateIdempotentApplication(ctx, f.identity, "00000000-0000-4000-8000-000000000030", messages.CreateRequest{Format: "plain", Text: "mutation"}, nil)
			if err == nil || hookErr == nil || out.Message.ID != "" {
				t.Fatal("write check failed", hookErr, err)
			}
			unchanged(t, f, before, prior)
		})
	}
	// 同值 BLOB 不能通过 Go typed Scan 转换伪装成原始 TEXT。
	t.Run("blob-text", func(t *testing.T) {
		f := setup(t)
		exec(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable;CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN UPDATE push_jobs SET bot_id=CAST(bot_id AS BLOB) WHERE id=NEW.id;END`)
		if _, e := f.create(ctx); e == nil {
			t.Fatal("accepted BLOB projection")
		}
		if len(rows(t, f.store)) != 0 {
			t.Fatal("partial BLOB commit")
		}
	})
}

func (f *fixture) paged(t *testing.T, n int) {
	t.Helper()
	exec(t, f.store, `DELETE FROM sessions`)
	if err := f.store.Write(ctx, func(tx *sql.Tx) error {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("paged-%03d", i)
			hash := make([]byte, 32)
			hash[0] = byte(i)
			hash[1] = byte(i >> 8)
			if _, e := tx.Exec(`INSERT INTO sessions(id,token_hash,auth_generation,client_type,device_id,device_name,idle_expires_at,absolute_expires_at)VALUES(?,?,1,'android',?,'paged',?,?)`, id, hash, device, time.Now().Add(time.Hour).UnixNano(), time.Now().Add(2*time.Hour).UnixNano()); e != nil {
				return e
			}
			if _, e := tx.Exec(`INSERT INTO device_push_registrations VALUES(?,'opush',1,'synthetic',?)`, id, time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestPagingCompleteAndLaterPageRollback(t *testing.T) {
	for _, name := range []string{"valid", "later-sql", "earlier-row", "earlier-identity", "later-config"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.paged(t, 261)
			switch name {
			case "later-sql":
				exec(t, f.store, `CREATE TRIGGER fault BEFORE INSERT ON push_jobs WHEN NEW.session_id='paged-130' BEGIN SELECT RAISE(ABORT,'later page');END`)
			case "earlier-row":
				exec(t, f.store, `CREATE TRIGGER fault AFTER INSERT ON push_jobs WHEN NEW.session_id='paged-130' BEGIN UPDATE push_jobs SET attempt=1 WHERE session_id='paged-000';END`)
			case "earlier-identity":
				exec(t, f.store, `CREATE TRIGGER fault AFTER INSERT ON push_jobs WHEN NEW.session_id='paged-130' BEGIN UPDATE device_push_registrations SET registration_version=2 WHERE session_id='paged-000';END`)
			case "later-config":
				exec(t, f.store, `CREATE TRIGGER fault AFTER INSERT ON push_jobs WHEN NEW.session_id='paged-130' BEGIN UPDATE device_notification_settings SET configuration='{}';END`)
			}
			before, prior := rows(t, f.store), state(t, f.store)
			out, err := f.create(ctx)
			if name != "valid" {
				if err == nil || out.Message.ID != "" {
					t.Fatal("later-page failure published", err)
				}
				unchanged(t, f, before, prior)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(rows(t, f.store)) != 261 {
				t.Fatal("paging truncated")
			}
			// 关闭后重开，原始快照仍完整持久。
			before = rows(t, f.store)
			if err = f.store.Close(); err != nil {
				t.Fatal(err)
			}
			f.store, err = storage.Open(ctx, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, rows(t, f.store)) {
				t.Fatal("reopen changed snapshot")
			}
		})
	}
}

func TestInputCancellationAndFirstCreateIdempotency(t *testing.T) {
	f := setup(t)
	e := f.source(t)
	for _, name := range []string{"nil-context", "cancel", "zero-time", "overflow", "zero-seq", "zero-target"} {
		t.Run(name, func(t *testing.T) {
			c := ctx
			input := e
			switch name {
			case "nil-context":
				c = nil
			case "cancel":
				var cancel context.CancelFunc
				c, cancel = context.WithCancel(ctx)
				cancel()
			case "zero-time":
				input.CreatedAt = time.Time{}
			case "overflow":
				input.CreatedAt = time.Unix(0, int64(^uint64(0)>>1))
			case "zero-seq":
				input.BotSeq = 0
			case "zero-target":
				input.TargetChangeSeq = 0
			}
			err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueAlertTx(c, tx, input) })
			if err == nil {
				t.Fatal("invalid input succeeded")
			}
		})
	}
	if err := push.EnqueueAlertTx(ctx, nil, e); !errors.Is(err, push.ErrJobInput) {
		t.Fatal("nil tx", err)
	}
	// 测试中手动装配真实 repo first-create hook；当前 main 并未接线。
	called := 0
	repo := f.repository(func(c context.Context, tx *sql.Tx, m messages.Created) error {
		called++
		return push.EnqueueAlertTx(c, tx, event(m))
	})
	key := "00000000-0000-4000-8000-000000000950"
	first, err := repo.CreateIdempotentApplication(ctx, f.identity, key, messages.CreateRequest{Format: "plain", Text: "idem"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, prior := rows(t, f.store), state(t, f.store)
	// 版本轮换后 HTTP 所用仓储幂等入口必须在 hook 前返回，保留旧 26 列。
	exec(t, f.store, `UPDATE device_push_registrations SET registration_version=2;UPDATE device_notification_settings SET settings_version=2`)
	second, err := repo.CreateIdempotentApplication(ctx, f.identity, key, messages.CreateRequest{Format: "plain", Text: "idem"}, nil)
	if err != nil || called != 1 || first.Message.ID != second.Message.ID {
		t.Fatal("idem reentered first create", called, err)
	}
	unchanged(t, f, before, prior)
	c, cancel := context.WithCancel(ctx)
	repo = f.repository(func(c context.Context, tx *sql.Tx, m messages.Created) error {
		if e := push.EnqueueAlertTx(c, tx, event(m)); e != nil {
			return e
		}
		cancel()
		return c.Err()
	})
	out, err := repo.CreateIdempotentApplication(c, f.identity, "00000000-0000-4000-8000-000000000951", messages.CreateRequest{Format: "plain", Text: "cancel"}, nil)
	if !errors.Is(err, context.Canceled) || out.Message.ID != "" {
		t.Fatal("cancel published", err)
	}
	unchanged(t, f, before, prior)
}

func TestCurrentVersionsAndProviderIsolation(t *testing.T) {
	f := setup(t)
	// 真实代次变更会清除注册，再显式注册；只有新事件允许携带这些新版本。
	exec(t, f.store, `UPDATE administrator SET auth_generation=2;UPDATE sessions SET auth_generation=2;UPDATE device_notification_settings SET settings_version=7`)
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'opush',5,'new-synthetic',?)`, f.session.ID, time.Now().UTC().Format(time.RFC3339Nano))
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',3,'other-channel',?)`, f.session.ID, time.Now().UTC().Format(time.RFC3339Nano))
	e := f.source(t)
	// 旧 FCM alert 不得被 opush 原始事件生产者改写或提升通道。
	if err := f.store.Write(ctx, func(tx *sql.Tx) error {
		return push.CreateJobTx(ctx, tx, push.JobSnapshot{ID: "old-fcm-alert", SessionID: f.session.ID, DeviceID: device, Purpose: "alert", Provider: "fcm", TargetChangeSeq: e.TargetChangeSeq, RegistrationVersion: 3, SettingsVersion: 7, DeploymentID: instance(t, tx), MessageID: e.MessageID, BotID: e.BotID, BotSeq: e.BotSeq, SessionGeneration: 2, CreatedAt: e.CreatedAt, ExpiresAt: e.CreatedAt.Add(24 * time.Hour), AlertEligible: true}, e.CreatedAt)
	}); err != nil {
		t.Fatal(err)
	}
	old := rows(t, f.store)["old-fcm-alert"]
	enqueue(t, f.store, e)
	after := rows(t, f.store)
	if len(after) != 2 || !reflect.DeepEqual(old, after["old-fcm-alert"]) {
		t.Fatal("provider isolation lost")
	}
	for id, v := range after {
		if id == "old-fcm-alert" {
			continue
		}
		if v[9] != int64(5) || v[10] != int64(7) || v[15] != int64(2) {
			t.Fatal("wrong original versions", v)
		}
	}
}

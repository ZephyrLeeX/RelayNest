// Package alertoriginstorage_test 用真实 SQLite 独立覆盖原始提醒资格，不改变原生 push 测试族。
package alertattemptstorage_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
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
func database(t *testing.T, s *storage.Store) string {
	t.Helper()
	var out strings.Builder
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		names, e := tx.Query(`SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name`)
		if e != nil {
			return e
		}
		var tables []string
		for names.Next() {
			var n string
			if e := names.Scan(&n); e != nil {
				return e
			}
			tables = append(tables, n)
		}
		if e := names.Close(); e != nil {
			return e
		}
		for _, table := range tables {
			rows, e := tx.Query(`SELECT * FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
			if e != nil {
				return e
			}
			cols, e := rows.Columns()
			if e != nil {
				return e
			}
			fmt.Fprint(&out, table)
			var records []string
			for rows.Next() {
				v := make([]any, len(cols))
				d := make([]any, len(cols))
				for i := range v {
					d[i] = &v[i]
				}
				if e := rows.Scan(d...); e != nil {
					return e
				}
				records = append(records, fmt.Sprintf("%#v", v))
			}
			if e := rows.Err(); e != nil {
				return e
			}
			rows.Close()
			sort.Strings(records)
			for _, record := range records {
				out.WriteString(record)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

const stamp = "2006-01-02T15:04:05.000000000Z"

func prepared(t *testing.T) (*fixture, push.JobReservation, time.Time) {
	t.Helper()
	f := setup(t)
	if _, e := f.create(ctx); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	held, e := push.NewJobRepository(f.store).ReserveOPushAlerts(ctx, now, time.Minute, 1)
	if e != nil || len(held) != 1 {
		t.Fatal("reserve", e, len(held))
	}
	return f, held[0], now
}
func read(t *testing.T, f *fixture, id string) push.Job {
	t.Helper()
	var j push.Job
	if e := f.store.Read(ctx, func(tx *sql.Tx) error { var e error; j, e = push.ReadJobTx(ctx, tx, id); return e }); e != nil {
		t.Fatal(e)
	}
	return j
}
func begin(t *testing.T, f *fixture, h push.JobReservation, n time.Time) *push.AlertAttempt {
	t.Helper()
	ticket, e := push.NewJobRepository(f.store).BeginAlert(ctx, h.Lease, n)
	if e != nil || ticket == nil {
		t.Fatal("begin", e, ticket)
	}
	return ticket
}

// Package mergestorage_test 用真实 SQLite 独立覆盖水位归并，不改变原生 push 测试族。
package mergestorage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
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
	f := &fixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "merge.sqlite")}}
	var err error
	f.store, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.store.Close() })
	exec(t, f.store, `INSERT INTO administrator VALUES(1,'admin','synthetic-hash',1)`)
	f.bot, _, err = bots.New(f.store).Create(ctx, bots.CreateRequest{Name: "merge", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.identity, err = bots.New(f.store).Authenticate(ctx, f.bot.Bot.ID, bots.APIToken, f.bot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	f.session = f.addSession(t, device)
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
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic-secret',?)`, s.ID, time.Now().UTC().Format(time.RFC3339Nano))
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
	if err := s.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueSyncTx(ctx, tx, e) }); err != nil {
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
func wantTargetOnly(t *testing.T, before, after map[string][]any, target int64) {
	t.Helper()
	want := map[string][]any{}
	for id, v := range before {
		cp := append([]any(nil), v...)
		cp[3] = target
		want[id] = cp
	}
	if !reflect.DeepEqual(want, after) {
		t.Fatalf("changed outside target or unexpected rows\nwant=%v\ngot=%v", want, after)
	}
}
func (f *fixture) repository() *messages.Repository {
	return messages.NewWithFirstCreate(f.store, func(c context.Context, tx *sql.Tx, m messages.Created) error {
		return push.EnqueueSyncTx(c, tx, event(m))
	})
}
func (f *fixture) create(c context.Context) (messages.Created, error) {
	f.serial++
	return f.repository().CreateIdempotentApplication(c, f.identity, fmt.Sprintf("00000000-0000-4000-8000-%012d", f.serial), messages.CreateRequest{Format: "plain", Text: "atomic"}, nil)
}

func TestMergeTargetOnlyDeterministicAndCovered(t *testing.T) {
	f := setup(t)
	first := f.source(t)
	enqueue(t, f.store, first)
	id := "sync:" + first.MessageID + ":" + f.session.ID
	// 留下退避与预算；选中后不能提前重试或续期。
	exec(t, f.store, `UPDATE push_jobs SET attempt=5,next_attempt_at=? WHERE id=?`, first.CreatedAt.Add(time.Hour).Format("2006-01-02T15:04:05.000000000Z"), id)
	// 同时存在两条合法 queued 时只处理 created/id 排序的第一条。
	if err := f.store.Write(ctx, func(tx *sql.Tx) error {
		j, e := push.ReadJobTx(ctx, tx, id)
		if e != nil {
			return e
		}
		j.Snapshot.ID = "z-other"
		return push.CreateJobTx(ctx, tx, j.Snapshot, j.NextAttemptAt)
	}); err != nil {
		t.Fatal(err)
	}
	before := rows(t, f.store)
	next := f.source(t)
	enqueue(t, f.store, next)
	after := rows(t, f.store)
	want := map[string][]any{}
	for k, v := range before {
		want[k] = append([]any(nil), v...)
	}
	want[id][3] = next.TargetChangeSeq
	if !reflect.DeepEqual(want, after) {
		t.Fatal("deterministic selection or 26 columns", want, after)
	}
	exec(t, f.store, `CREATE TRIGGER no_update BEFORE UPDATE ON push_jobs BEGIN SELECT RAISE(ABORT,'covered must not write');END`)
	enqueue(t, f.store, next)
	enqueue(t, f.store, first)
	if !reflect.DeepEqual(after, rows(t, f.store)) {
		t.Fatal("covered target changed rows")
	}
}

func TestMergeNewSourceMustBeValidated(t *testing.T) {
	mutations := []struct {
		name   string
		change func(*push.SyncEvent)
	}{
		{"missing", func(e *push.SyncEvent) { e.MessageID = "absent" }},
		{"bot", func(e *push.SyncEvent) { e.BotID = "absent" }},
		{"sequence", func(e *push.SyncEvent) { e.BotSeq++ }},
		{"time", func(e *push.SyncEvent) { e.CreatedAt = e.CreatedAt.Add(time.Nanosecond) }},
		{"creation-watermark", func(e *push.SyncEvent) { e.TargetChangeSeq = 1 }},
	}
	f := setup(t)
	enqueue(t, f.store, f.source(t))
	fresh := f.source(t)
	before := rows(t, f.store)
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			bad := fresh
			m.change(&bad)
			err := f.store.Write(ctx, func(tx *sql.Tx) error { return push.EnqueueSyncTx(ctx, tx, bad) })
			if !errors.Is(err, push.ErrJobData) {
				t.Fatal("bad new source accepted", err)
			}
			if !reflect.DeepEqual(before, rows(t, f.store)) {
				t.Fatal("bad source mutated old job")
			}
		})
	}
	// 创建 changes 关系本身也必须准确；变更日志是新事件来源，不以旧anchor替代。
	for _, q := range []string{`UPDATE changes SET entity_id='absent' WHERE change_seq=(SELECT created_change_seq FROM messages WHERE id=?)`, `UPDATE changes SET bot_id='absent' WHERE change_seq=(SELECT created_change_seq FROM messages WHERE id=?)`, `UPDATE changes SET created_at='bad-time' WHERE change_seq=(SELECT created_change_seq FROM messages WHERE id=?)`, `UPDATE changes SET type='bot.upsert' WHERE change_seq=(SELECT created_change_seq FROM messages WHERE id=?)`} {
		err := f.store.Write(ctx, func(tx *sql.Tx) error {
			if _, e := tx.Exec(q, fresh.MessageID); e != nil {
				return e
			}
			return push.EnqueueSyncTx(ctx, tx, fresh)
		})
		if !errors.Is(err, push.ErrJobData) {
			t.Fatal("bad creation relation accepted", q, err)
		}
	}
	exec(t, f.store, `DELETE FROM messages WHERE id IN(SELECT message_id FROM push_jobs)`)
	enqueue(t, f.store, fresh)
	wantTargetOnly(t, before, rows(t, f.store), fresh.TargetChangeSeq)
}

func TestMergeNamespacesAndExcludedCandidates(t *testing.T) {
	cases := []struct {
		name, q string
		args    func(push.SyncEvent) []any
	}{
		{"registration", `UPDATE device_push_registrations SET registration_version=2`, nil},
		{"configuration", `INSERT INTO device_notification_settings VALUES(?,1,?,'{}')`, func(e push.SyncEvent) []any { return []any{device, e.CreatedAt.Format(time.RFC3339Nano)} }},
		{"generation", `UPDATE administrator SET auth_generation=2;UPDATE sessions SET auth_generation=2;INSERT INTO device_push_registrations SELECT id,'fcm',1,'synthetic','now' FROM sessions`, nil},
		{"leased", `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=9223372036854775807`, nil},
		{"expired", `UPDATE push_jobs SET created_at_ns=1,expires_at_ns=2`, nil},
		{"future", `UPDATE push_jobs SET created_at_ns=9223372036854775805,expires_at_ns=9223372036854775806`, nil},
		{"deployment", `UPDATE push_jobs SET deployment_id='different-deployment'`, nil},
		{"alert", `UPDATE push_jobs SET purpose='alert',alert_eligible=1`, nil},
		{"suppressed", `UPDATE push_jobs SET purpose='alert',alert_eligible=1,state='suppressed'`, nil},
		{"accepted", `UPDATE push_jobs SET state='accepted',attempt=1,attempt_lease_hash=zeroblob(32),attempt_lease_until_ns=expires_at_ns,attempt_started_at_ns=created_at_ns,result_kind='accepted',result_at_ns=created_at_ns`, nil},
		{"degraded", `UPDATE push_jobs SET state='degraded',attempt=1,attempt_lease_hash=zeroblob(32),attempt_lease_until_ns=expires_at_ns,attempt_started_at_ns=created_at_ns,result_kind='transient',result_at_ns=created_at_ns`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t)
			first := f.source(t)
			enqueue(t, f.store, first)
			// 仅测试夹具改冻结快照；DDL产品文件完全不变，生产不会绕开不可变触发器。
			exec(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable`)
			var args []any
			if c.args != nil {
				args = c.args(first)
			}
			exec(t, f.store, c.q, args...)
			before := rows(t, f.store)
			next := f.source(t)
			enqueue(t, f.store, next)
			after := rows(t, f.store)
			if len(after) != 2 {
				t.Fatal("excluded namespace merged", after)
			}
			for id, v := range before {
				if !reflect.DeepEqual(v, after[id]) {
					t.Fatal("excluded old row changed")
				}
			}
		})
	}
}

func TestMergeTransactionFaultsRollbackEverything(t *testing.T) {
	faults := []struct{ name, q string }{
		{"zero-row", `CREATE TRIGGER fault BEFORE UPDATE OF target_change_seq ON push_jobs BEGIN SELECT RAISE(IGNORE);END`},
		{"sql", `CREATE TRIGGER fault BEFORE UPDATE OF target_change_seq ON push_jobs BEGIN SELECT RAISE(ABORT,'synthetic');END`},
		{"projection", `CREATE TRIGGER fault AFTER UPDATE OF target_change_seq ON push_jobs BEGIN UPDATE push_jobs SET attempt=attempt+1 WHERE id=NEW.id;END`},
		{"target-projection", `CREATE TRIGGER fault AFTER UPDATE OF target_change_seq ON push_jobs BEGIN UPDATE push_jobs SET target_change_seq=target_change_seq+1 WHERE id=NEW.id;END`},
		{"commit", `CREATE TABLE deferred_fault(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fault AFTER UPDATE OF target_change_seq ON push_jobs BEGIN INSERT INTO deferred_fault VALUES('absent');END`},
		{"receipt", `CREATE TRIGGER fault BEFORE INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'synthetic');END`},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			f := setup(t)
			f.addSession(t, device)
			if _, err := f.create(ctx); err != nil {
				t.Fatal(err)
			}
			before := rows(t, f.store)
			exec(t, f.store, fault.q)
			var stateBefore, stateAfter string
			state := func() string {
				var v string
				if err := f.store.Read(ctx, func(tx *sql.Tx) error {
					return tx.QueryRow(`SELECT json_array((SELECT count(*) FROM messages),(SELECT count(*) FROM changes),(SELECT count(*) FROM deliveries),(SELECT count(*) FROM idempotency),(SELECT max(bot_seq) FROM messages))`).Scan(&v)
				}); err != nil {
					t.Fatal(err)
				}
				return v
			}
			stateBefore = state()
			out, err := f.create(ctx)
			stateAfter = state()
			if err == nil || out.Message.ID != "" {
				t.Fatal("failed transaction published", out, err)
			}
			if stateBefore != stateAfter || !reflect.DeepEqual(before, rows(t, f.store)) {
				t.Fatal("partial transaction committed")
			}
		})
	}
}

func TestMergePagingAcrossStoresReopenAndCancellation(t *testing.T) {
	f := setup(t)
	exec(t, f.store, `DELETE FROM device_push_registrations`)
	// 同设备 261 session；真实源事件经同一消息事务逐页归并。
	if err := f.store.Write(ctx, func(tx *sql.Tx) error {
		for i := 0; i < 261; i++ {
			id := fmt.Sprintf("paged-%03d", i)
			hash := make([]byte, 32)
			hash[0] = byte(i)
			hash[1] = byte(i >> 8)
			if _, e := tx.Exec(`INSERT INTO sessions(id,token_hash,auth_generation,client_type,device_id,device_name,idle_expires_at,absolute_expires_at)VALUES(?,?,1,'android',?,'paged',?,?)`, id, hash, device, time.Now().Add(time.Hour).UnixNano(), time.Now().Add(2*time.Hour).UnixNano()); e != nil {
				return e
			}
			if _, e := tx.Exec(`INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, id, time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.create(ctx); err != nil {
		t.Fatal(err)
	}
	before := rows(t, f.store)
	if len(before) != 261 {
		t.Fatal("initial page truncated", len(before))
	}
	other, err := storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	// 两个 Store 共享文件写准入，两个消息事务不得覆盖彼此的 max。
	for i, s := range []*storage.Store{f.store, other} {
		wg.Add(1)
		go func(i int, s *storage.Store) {
			defer wg.Done()
			_, e := messages.NewWithFirstCreate(s, func(c context.Context, tx *sql.Tx, m messages.Created) error {
				return push.EnqueueSyncTx(c, tx, event(m))
			}).CreateIdempotentApplication(ctx, f.identity, fmt.Sprintf("00000000-0000-4000-8000-%012d", 100+i), messages.CreateRequest{Format: "plain", Text: "concurrent"}, nil)
			errs <- e
		}(i, s)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var highest int64
	if err = f.store.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRow(`SELECT max(change_seq) FROM changes`).Scan(&highest) }); err != nil {
		t.Fatal(err)
	}
	wantTargetOnly(t, before, rows(t, other), highest)
	if err = f.store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = other.Close(); err != nil {
		t.Fatal(err)
	}
	f.store, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	wantTargetOnly(t, before, rows(t, f.store), highest)
	c, cancel := context.WithCancel(ctx)
	repo := messages.NewWithFirstCreate(f.store, func(c context.Context, tx *sql.Tx, m messages.Created) error {
		if e := push.EnqueueSyncTx(c, tx, event(m)); e != nil {
			return e
		}
		cancel()
		return c.Err()
	})
	out, err := repo.CreateIdempotentApplication(c, f.identity, "00000000-0000-4000-8000-000000000999", messages.CreateRequest{Format: "plain", Text: "cancel"}, nil)
	if !errors.Is(err, context.Canceled) || out.Message.ID != "" {
		t.Fatal("cancellation published", err)
	}
	wantTargetOnly(t, before, rows(t, f.store), highest)
}

func TestMergeRetriedSnapshotAndLateTicketIsolation(t *testing.T) {
	f := setup(t)
	if _, err := f.create(ctx); err != nil {
		t.Fatal(err)
	}
	r := push.NewJobRepository(f.store)
	now := time.Now().UTC()
	held, err := r.ReserveSync(ctx, now, time.Minute, 1)
	if err != nil || len(held) != 1 {
		t.Fatal("reserve", held, err)
	}
	ticket, err := r.BeginSync(ctx, held[0].Lease, now.Add(time.Millisecond))
	if err != nil || ticket == nil {
		t.Fatal("begin", err)
	}
	if ok, err := r.ApplySyncResult(ctx, *ticket, push.SyncTransient, now.Add(2*time.Millisecond)); err != nil || !ok {
		t.Fatal("retry result", ok, err)
	}
	before := rows(t, f.store)
	// 真实尝试结果中的 attempt lease、时间与退避须随旧job保持，不能重置成新事件。
	if _, err := f.create(ctx); err != nil {
		t.Fatal(err)
	}
	after := rows(t, f.store)
	target := int64(0)
	for _, v := range after {
		target = v[3].(int64)
	}
	wantTargetOnly(t, before, after, target)
	if ok, err := r.ApplySyncResult(ctx, *ticket, push.SyncAccepted, now.Add(3*time.Millisecond)); err != nil || ok {
		t.Fatal("late old ticket applied", ok, err)
	}
	if !reflect.DeepEqual(after, rows(t, f.store)) {
		t.Fatal("old result polluted merged target")
	}
	// legacy仍是独立pending行；同设备不同session和不同设备均保持独立sync资格。
	exec(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at)VALUES('legacy',?,?,1,'legacy')`, f.session.ID, device)
	f.addSession(t, device)
	f.addSession(t, "22222222-2222-4222-8222-222222222222")
	legacy := rows(t, f.store)["legacy"]
	if _, err := f.create(ctx); err != nil {
		t.Fatal(err)
	}
	got := rows(t, f.store)
	if len(got) != 4 || !reflect.DeepEqual(legacy, got["legacy"]) {
		t.Fatal("legacy/device/session namespace", got)
	}
}

func TestMergeWebhookSkipsFirstCreate(t *testing.T) {
	f := setup(t)
	if _, err := f.create(ctx); err != nil {
		t.Fatal(err)
	}
	before := rows(t, f.store)
	identity, err := bots.New(f.store).Authenticate(ctx, f.bot.Bot.ID, bots.WebhookKey, f.bot.WebhookKey.Key)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	repo := messages.NewWithFirstCreate(f.store, func(context.Context, *sql.Tx, messages.Created) error {
		called = true
		return errors.New("webhook must skip hook")
	})
	out, err := repo.CreateIdempotentApplication(ctx, identity, "00000000-0000-4000-8000-000000000777", messages.CreateRequest{Format: "plain", Text: "webhook"}, nil)
	if err != nil || out.Message.ID == "" || called {
		t.Fatal("webhook behavior changed", err, called)
	}
	if !reflect.DeepEqual(before, rows(t, f.store)) {
		t.Fatal("webhook merged sync")
	}
}

func TestMergeSilentSyncAndExpiryBoundary(t *testing.T) {
	f := setup(t)
	configuration := `{"enabled":false,"bot_overrides":[],"dnd":{"enabled":true,"timezone":"UTC","start":"00:00","end":"23:59"},"preview":false,"notify_channel":"none","notify_provider":null}`
	exec(t, f.store, `INSERT INTO device_notification_settings VALUES(?,1,?,?)`, device, time.Now().UTC().Format(time.RFC3339Nano), configuration)
	first := f.source(t)
	enqueue(t, f.store, first)
	before := rows(t, f.store)
	next := f.source(t)
	exec(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, next.BotSeq, next.BotID)
	enqueue(t, f.store, next)
	wantTargetOnly(t, before, rows(t, f.store), next.TargetChangeSeq)
	// expires==event不是未过期；不续期/复活该旧job，原创建路径生成新job。
	fresh := f.source(t)
	exec(t, f.store, `DROP TRIGGER push_jobs_snapshot_immutable`)
	exec(t, f.store, `UPDATE push_jobs SET expires_at_ns=?`, fresh.CreatedAt.UnixNano())
	before = rows(t, f.store)
	enqueue(t, f.store, fresh)
	after := rows(t, f.store)
	if len(after) != 2 {
		t.Fatal("expiry boundary merged", after)
	}
	for id, v := range before {
		if !reflect.DeepEqual(v, after[id]) {
			t.Fatal("expired row revived")
		}
	}
}

package messages

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
)

var messageContext = context.Background()

type fixture struct {
	s, other *storage.Store
	r        *Repository
	b        *bots.Repository
	a, bbot  bots.Created
	identity bots.Identity
	cfg      storage.Config
}

func setupMessages(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "atomic.sqlite"), BusyTimeout: 50 * time.Millisecond}}
	var err error
	f.s, err = storage.Open(messageContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.s.Close() })
	f.other, err = storage.Open(messageContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.other.Close() })
	f.r = New(f.s)
	f.r.now = func() time.Time { return time.Date(2026, 10, 25, 1, 2, 3, 123, time.UTC) }
	f.b = bots.New(f.s)
	for _, dest := range []*bots.Created{&f.a, &f.bbot} {
		*dest, _, err = f.b.Create(messageContext, bots.CreateRequest{Name: "test", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
		if err != nil {
			t.Fatal(err)
		}
	}
	f.identity, err = f.b.Authenticate(messageContext, f.a.Bot.ID, bots.APIToken, f.a.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	execMessages(t, f.s, `INSERT INTO administrator VALUES(1,'admin','test-hash',1)`)
	execMessages(t, f.s, `INSERT INTO sessions VALUES('session',zeroblob(32),1,'android','device','phone',2000000000,2100000000,0)`)
	if v, e := f.s.EngineVersion(messageContext); e != nil {
		t.Fatal(e)
	} else {
		t.Log("SQLite", v)
	}
	return f
}

func execMessages(t *testing.T, s *storage.Store, q string, args ...any) {
	t.Helper()
	if err := s.Write(messageContext, func(tx *sql.Tx) error { _, e := tx.Exec(q, args...); return e }); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) input() TextInput {
	return TextInput{BotID: f.a.Bot.ID, Format: "markdown", Text: "正文 α", PushTargets: []PushTarget{{"session", "device"}}}
}

func adminWrite(f *fixture, in TextInput) (Created, error) {
	var out Created
	err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		var e error
		out, e = f.r.CreateAdministratorTx(messageContext, tx, in)
		return e
	})
	if err != nil {
		return Created{}, err
	}
	return out, nil
}

// 同时核对实体/待办/日志与 SQLite 自增状态；不同连接与重开均须一致。
func persisted(t *testing.T, s *storage.Store) []int64 {
	t.Helper()
	var values []int64
	err := s.Read(messageContext, func(tx *sql.Tx) error {
		for _, q := range []string{"SELECT count(*) FROM messages", "SELECT count(*) FROM messages WHERE created_change_seq IS NULL", "SELECT count(*) FROM deliveries", "SELECT count(*) FROM push_jobs", "SELECT count(*) FROM changes", "SELECT coalesce(max(change_seq),0) FROM changes", "SELECT coalesce(sum(bot_seq),0) FROM bots", "SELECT coalesce(max(seq),0) FROM sqlite_sequence WHERE name='changes'"} {
			var v int64
			if e := tx.QueryRow(q).Scan(&v); e != nil {
				return e
			}
			values = append(values, v)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return values
}

func assertPersisted(t *testing.T, f *fixture, want []int64) {
	t.Helper()
	if got := persisted(t, f.other); !reflect.DeepEqual(got, want) {
		t.Fatalf("other connection: got %v want %v", got, want)
	}
	reopened, e := storage.Open(messageContext, f.cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if got := persisted(t, reopened); !reflect.DeepEqual(got, want) {
		t.Fatalf("reopen: got %v want %v", got, want)
	}
}

func TestAtomicMessagesPendingAndSnapshot(t *testing.T) {
	f := setupMessages(t)
	before := persisted(t, f.other)
	var candidate Created
	err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		var e error
		candidate, e = f.r.CreateAdministratorTx(messageContext, tx, f.input())
		if e != nil {
			return e
		}
		if got := persisted(t, f.other); !reflect.DeepEqual(got, before) {
			t.Fatalf("uncommitted visible: %v", got)
		}
		var nulls int
		if e = tx.QueryRow("SELECT count(*) FROM messages WHERE created_change_seq IS NULL").Scan(&nulls); e != nil {
			return e
		}
		if nulls != 0 {
			t.Fatal("incomplete creation")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	m := candidate.Message
	d := candidate.Delivery
	if m.Revision != 1 || m.BotSeq != 1 || m.Sender != Administrator || m.CreatedChangeSeq == nil || d == nil || d.EnqueueSeq <= *m.CreatedChangeSeq || candidate.LastChangeSeq != d.EnqueueSeq || len(candidate.Pushes) != 1 || candidate.Pushes[0].TargetChangeSeq != d.EnqueueSeq {
		t.Fatalf("bad result: %+v", candidate)
	}
	var event struct {
		Type    string         `json:"type"`
		Message map[string]any `json:"message"`
	}
	if e := json.Unmarshal(d.PayloadJSON, &event); e != nil {
		t.Fatal(e)
	}
	if event.Type != "message.created" || len(event.Message) != 12 || event.Message["id"] != m.ID || event.Message["bot_id"] != m.BotID || event.Message["text"] != m.Text || event.Message["bot_seq"] != "1" || event.Message["sender"] != "administrator" || event.Message["reply_to_message_id"] != nil {
		t.Fatalf("bad frozen event: %s", d.PayloadJSON)
	}
	for _, key := range []string{"attachments", "buttons"} {
		if a, ok := event.Message[key].([]any); !ok || len(a) != 0 {
			t.Fatalf("bad %s", key)
		}
	}
	if event.Message["format"] != "markdown" || event.Message["revision"] != float64(1) || event.Message["created_at"] != m.CreatedAt.Format(time.RFC3339Nano) || event.Message["updated_at"] != m.UpdatedAt.Format(time.RFC3339Nano) {
		t.Fatal("snapshot fields")
	}
	err = f.other.Read(messageContext, func(tx *sql.Tx) error {
		var seq changes.Sequence
		var kind, entity, bot string
		var revision int64
		if e := tx.QueryRow("SELECT change_seq,type,entity_id,bot_id,revision FROM changes WHERE change_seq=?", *m.CreatedChangeSeq).Scan(&seq, &kind, &entity, &bot, &revision); e != nil {
			return e
		}
		if kind != string(changes.MessageUpsert) || entity != m.ID || bot != m.BotID || revision != 1 {
			t.Fatal("creation change mismatch")
		}
		if e := tx.QueryRow("SELECT type,entity_id,revision FROM changes WHERE change_seq=?", d.EnqueueSeq).Scan(&kind, &entity, &revision); e != nil {
			return e
		}
		if kind != string(changes.DeliveryUpsert) || entity != d.DeliveryID || revision != 1 {
			t.Fatal("delivery change mismatch")
		}
		var state, eventID, payload string
		var version, attempt, generation int
		if e := tx.QueryRow("SELECT state,state_version,attempt,retry_generation,event_id,payload_json FROM deliveries WHERE delivery_id=?", d.DeliveryID).Scan(&state, &version, &attempt, &generation, &eventID, &payload); e != nil {
			return e
		}
		if state != "pending" || version != 1 || attempt != 0 || generation != 0 || eventID != d.EventID || payload != string(d.PayloadJSON) {
			t.Fatal("delivery initial metadata")
		}
		var target changes.Sequence
		var session, device string
		if e := tx.QueryRow("SELECT session_id,device_id,target_change_seq FROM push_jobs").Scan(&session, &device, &target); e != nil {
			return e
		}
		if session != "session" || device != "device" || target != candidate.LastChangeSeq {
			t.Fatal("push target mismatch")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	execMessages(t, f.s, "UPDATE messages SET text='later',revision=2 WHERE id=?", m.ID)
	var snapshot string
	if e := f.other.Read(messageContext, func(tx *sql.Tx) error { return tx.QueryRow("SELECT payload_json FROM deliveries").Scan(&snapshot) }); e != nil {
		t.Fatal(e)
	}
	if snapshot != string(d.PayloadJSON) {
		t.Fatal("snapshot followed current message")
	}
	app, e := f.r.CreateApplication(messageContext, f.identity, f.input())
	if e != nil {
		t.Fatal(e)
	}
	if app.Message.Sender != Application || app.Delivery != nil || len(app.Pushes) != 1 || app.Pushes[0].TargetChangeSeq != *app.Message.CreatedChangeSeq {
		t.Fatal("application routing")
	}
	assertPersisted(t, f, persisted(t, f.s))
	// Bot 删除不保护正文；全局提示保留。会话删除则级联清理提示。
	execMessages(t, f.s, "DELETE FROM bots WHERE id=?", f.a.Bot.ID)
	state := persisted(t, f.other)
	if state[0] != 0 || state[2] != 0 || state[3] != 2 {
		t.Fatalf("Bot cleanup: %v", state)
	}
	execMessages(t, f.s, "DELETE FROM sessions WHERE id='session'")
	if persisted(t, f.other)[3] != 0 {
		t.Fatal("session push cleanup")
	}
}

func TestMessageSQLAndCommitFailuresRollback(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"counter", `CREATE TRIGGER fail AFTER UPDATE OF bot_seq ON bots BEGIN SELECT RAISE(ABORT,'counter failure'); END`},
		{"message", `CREATE TRIGGER fail AFTER INSERT ON messages BEGIN SELECT RAISE(ABORT,'message failure'); END`},
		{"append", `CREATE TRIGGER fail AFTER INSERT ON changes WHEN NEW.type='message.upsert' BEGIN SELECT RAISE(ABORT,'append failure'); END`},
		{"backfill", `CREATE TRIGGER fail AFTER UPDATE OF created_change_seq ON messages BEGIN SELECT RAISE(ABORT,'backfill failure'); END`},
		{"delivery_append", `CREATE TRIGGER fail AFTER INSERT ON changes WHEN NEW.type='delivery.upsert' BEGIN SELECT RAISE(ABORT,'delivery append failure'); END`},
		{"delivery", `CREATE TRIGGER fail AFTER INSERT ON deliveries BEGIN SELECT RAISE(ABORT,'delivery failure'); END`},
		{"push", `CREATE TRIGGER fail AFTER INSERT ON push_jobs BEGIN SELECT RAISE(ABORT,'push failure'); END`},
		{"commit", `CREATE TABLE fault_parent(id INTEGER PRIMARY KEY);CREATE TABLE fault_child(id INTEGER REFERENCES fault_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail AFTER INSERT ON push_jobs BEGIN INSERT INTO fault_child VALUES(42);END`},
		{"affected_counter", `CREATE TRIGGER fail BEFORE UPDATE OF bot_seq ON bots BEGIN SELECT RAISE(IGNORE);END`},
		{"affected_message", `CREATE TRIGGER fail BEFORE INSERT ON messages BEGIN SELECT RAISE(IGNORE);END`},
		{"affected_backfill", `CREATE TRIGGER fail BEFORE UPDATE OF created_change_seq ON messages BEGIN SELECT RAISE(IGNORE);END`},
		{"affected_delivery", `CREATE TRIGGER fail BEFORE INSERT ON deliveries BEGIN SELECT RAISE(IGNORE);END`},
		{"affected_push", `CREATE TRIGGER fail BEFORE INSERT ON push_jobs BEGIN SELECT RAISE(IGNORE);END`},
		{"constraint", `CREATE TRIGGER fail AFTER INSERT ON messages BEGIN INSERT INTO messages SELECT * FROM messages WHERE id=NEW.id;END`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupMessages(t)
			execMessages(t, f.s, tc.sql)
			before := persisted(t, f.other)
			callbackOK := false
			var candidate Created
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var e error
				candidate, e = f.r.CreateAdministratorTx(messageContext, tx, f.input())
				callbackOK = e == nil
				return e
			})
			if err == nil {
				t.Fatal("fault accepted")
			}
			if tc.name == "commit" {
				if !callbackOK || candidate.Message.ID == "" {
					t.Fatal("not a real COMMIT failure")
				}
				t.Log("callback succeeded; COMMIT rejected deferred FK:", err)
			}
			assertPersisted(t, f, before)
			// 正式应用包装器还必须在真实提交失败时清空所有候选返回值。
			if tc.name == "commit" {
				out, e := f.r.CreateApplication(messageContext, f.identity, f.input())
				if e == nil || !reflect.DeepEqual(out, Created{}) {
					t.Fatalf("candidate leaked: %+v %v", out, e)
				}
				assertPersisted(t, f, before)
			}
		})
	}
}

func TestMessageRandomInputAndSequenceFailures(t *testing.T) {
	for _, n := range []int{0, 16, 32, 48} {
		t.Run(fmt.Sprintf("random_%d", n), func(t *testing.T) {
			f := setupMessages(t)
			f.r.random = bytes.NewReader(make([]byte, n))
			before := persisted(t, f.other)
			if _, e := adminWrite(f, f.input()); e == nil {
				t.Fatal("random failure accepted")
			}
			assertPersisted(t, f, before)
		})
	}
	f := setupMessages(t)
	for _, mutate := range []func(*TextInput){func(i *TextInput) { i.Attachments = []string{"attachment"} }, func(i *TextInput) { v := "ref"; i.ReplyToMessageID = &v }, func(i *TextInput) { i.Text = "" }, func(i *TextInput) { i.Text = strings.Repeat("x", 65537) }, func(i *TextInput) { i.Text = string([]byte{255}) }, func(i *TextInput) { i.Format = "html" }, func(i *TextInput) { i.PushTargets = make([]PushTarget, 101) }} {
		in := f.input()
		mutate(&in)
		before := persisted(t, f.other)
		out, e := f.r.CreateApplication(messageContext, f.identity, in)
		if e == nil || !reflect.DeepEqual(out, Created{}) {
			t.Fatal("invalid input accepted")
		}
		assertPersisted(t, f, before)
	}
	if _, e := f.r.CreateApplicationTx(messageContext, nil, f.identity, f.input()); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := f.r.CreateAdministratorTx(messageContext, nil, f.input()); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	execMessages(t, f.s, "UPDATE bots SET bot_seq=? WHERE id=?", int64(math.MaxInt64), f.a.Bot.ID)
	before := persisted(t, f.other)
	out, e := f.r.CreateApplication(messageContext, f.identity, f.input())
	if !errors.Is(e, ErrSequenceExhausted) || !reflect.DeepEqual(out, Created{}) {
		t.Fatal("sequence overflow", e)
	}
	assertPersisted(t, f, before)
}

func TestMessagePushTargetValidity(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		targets   []PushTarget
	}{
		{"revoked", "UPDATE sessions SET revoked=1", nil},
		{"idle", "UPDATE sessions SET idle_expires_at=1", nil},
		{"absolute", "UPDATE sessions SET idle_expires_at=1,absolute_expires_at=2", nil},
		{"generation", "UPDATE administrator SET auth_generation=2", nil},
		{"web", "UPDATE sessions SET client_type='web'", nil},
		{"missing", "DELETE FROM sessions", nil},
		{"binding", "", []PushTarget{{"session", "other"}}},
		{"empty", "", []PushTarget{{"", "device"}}},
		{"duplicate", "", []PushTarget{{"session", "device"}, {"session", "device"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupMessages(t)
			if tc.sql != "" {
				execMessages(t, f.s, tc.sql)
			}
			in := f.input()
			if tc.targets != nil {
				in.PushTargets = tc.targets
			}
			before := persisted(t, f.other)
			out, e := f.r.CreateApplication(messageContext, f.identity, in)
			if !errors.Is(e, ErrPushTarget) || !reflect.DeepEqual(out, Created{}) {
				t.Fatalf("target accepted: %v", e)
			}
			assertPersisted(t, f, before)
		})
	}
}

func TestMessageBotAuthorizationRecheck(t *testing.T) {
	for _, kind := range []bots.Kind{bots.APIToken, bots.WebhookKey} {
		for _, mode := range []string{"cross_bot", "receive_only", "disabled", "reset", "deleted", "zero_identity"} {
			t.Run(string(kind)+"_"+mode, func(t *testing.T) {
				f := setupMessages(t)
				secret := f.a.APIToken.Token
				if kind == bots.WebhookKey {
					secret = f.a.WebhookKey.Key
				}
				identity, e := f.b.Authenticate(messageContext, f.a.Bot.ID, kind, secret)
				if e != nil {
					t.Fatal(e)
				}
				in := f.input()
				switch mode {
				case "cross_bot":
					in.BotID = f.bbot.Bot.ID
				case "receive_only":
					execMessages(t, f.s, "UPDATE bots SET can_send=0 WHERE id=?", f.a.Bot.ID)
				case "disabled":
					execMessages(t, f.s, "UPDATE bots SET enabled=0 WHERE id=?", f.a.Bot.ID)
				case "reset":
					if kind == bots.APIToken {
						_, _, e = f.b.RotateAPIToken(messageContext, f.a.Bot.ID)
					} else {
						_, _, e = f.b.RotateWebhookKey(messageContext, f.a.Bot.ID)
					}
					if e != nil {
						t.Fatal(e)
					}
				case "deleted":
					execMessages(t, f.s, "DELETE FROM bots WHERE id=?", f.a.Bot.ID)
				case "zero_identity":
					identity = bots.Identity{}
				}
				before := persisted(t, f.other)
				out, e := f.r.CreateApplication(messageContext, identity, in)
				if e == nil || !reflect.DeepEqual(out, Created{}) {
					t.Fatal("authorization bypass")
				}
				assertPersisted(t, f, before)
			})
		}
	}
	f := setupMessages(t)
	wh, e := f.b.Authenticate(messageContext, f.a.Bot.ID, bots.WebhookKey, f.a.WebhookKey.Key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.r.CreateApplication(messageContext, wh, f.input()); e != nil {
		t.Fatal("webhook Send rejected", e)
	}
	// capability 快照不能在认证后增权；当前数据库能力也不能被旧快照绕过。
	execMessages(t, f.s, "UPDATE bots SET can_send=0 WHERE id=?", f.a.Bot.ID)
	receiveOnly, e := f.b.Authenticate(messageContext, f.a.Bot.ID, bots.APIToken, f.a.APIToken.Token)
	if e != nil {
		t.Fatal(e)
	}
	execMessages(t, f.s, "UPDATE bots SET can_send=1 WHERE id=?", f.a.Bot.ID)
	if _, e = f.r.CreateApplication(messageContext, receiveOnly, f.input()); !errors.Is(e, bots.ErrForbidden) {
		t.Fatal("identity gained capability", e)
	}
	for _, sql := range []string{"UPDATE bots SET can_receive=0", "UPDATE bots SET enabled=0"} {
		execMessages(t, f.s, sql)
		before := persisted(t, f.other)
		if _, e = adminWrite(f, f.input()); !errors.Is(e, bots.ErrForbidden) {
			t.Fatal("administrator Bot check", e)
		}
		assertPersisted(t, f, before)
	}
}

func TestMessageQueuedInvalidationAndContext(t *testing.T) {
	for _, mutation := range []string{"UPDATE bots SET enabled=0", "UPDATE bots SET can_send=0", "UPDATE bot_credentials SET generation=generation+1", "DELETE FROM bots"} {
		t.Run(mutation, func(t *testing.T) {
			f := setupMessages(t)
			held := make(chan struct{})
			release := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- f.s.Write(messageContext, func(tx *sql.Tx) error { _, e := tx.Exec(mutation); close(held); <-release; return e })
			}()
			<-held
			started := make(chan struct{})
			result := make(chan error, 1)
			go func() {
				close(started)
				out, e := f.r.CreateApplication(messageContext, f.identity, f.input())
				if !reflect.DeepEqual(out, Created{}) {
					result <- fmt.Errorf("leaked result")
					return
				}
				result <- e
			}()
			<-started
			close(release)
			if e := <-done; e != nil {
				t.Fatal(e)
			}
			before := persisted(t, f.other)
			if e := <-result; e == nil {
				t.Fatal("queued invalid identity accepted")
			}
			assertPersisted(t, f, before)
		})
	}
	f := setupMessages(t)
	before := persisted(t, f.other)
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- f.s.Write(messageContext, func(tx *sql.Tx) error { _, e := tx.Exec("UPDATE bots SET name=name"); close(held); <-release; return e })
	}()
	<-held
	ctx, cancel := context.WithTimeout(messageContext, 20*time.Millisecond)
	defer cancel()
	out, e := f.r.CreateApplication(ctx, f.identity, f.input())
	if e == nil || !reflect.DeepEqual(out, Created{}) {
		t.Fatal("blocked context succeeded")
	}
	// 独立 Store 也不能穿过真实文件写锁，错误不释放候选结果。
	out, e = New(f.other).CreateApplication(messageContext, f.identity, f.input())
	if e == nil || !reflect.DeepEqual(out, Created{}) {
		t.Fatal("file lock bypass")
	}
	close(release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	assertPersisted(t, f, before)
}

func TestMessageIndependentStoreConcurrentMonotonic(t *testing.T) {
	f := setupMessages(t)
	identityB, e := f.b.Authenticate(messageContext, f.bbot.Bot.ID, bots.APIToken, f.bbot.APIToken.Token)
	if e != nil {
		t.Fatal(e)
	}
	const count = 32
	results := make(chan Created, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := f.r
			if i%2 != 0 {
				r = New(f.other)
			}
			identity := f.identity
			in := f.input()
			if i%4 >= 2 {
				identity = identityB
				in.BotID = f.bbot.Bot.ID
			}
			// 不同 Store 的短事务竞争可明确返回 busy；调用方在无提交结果时重试。
			for attempt := 0; attempt < 100; attempt++ {
				out, e := r.CreateApplication(messageContext, identity, in)
				if e == nil {
					results <- out
					return
				}
				if !strings.Contains(e.Error(), "SQLITE_BUSY") {
					errs <- e
					return
				}
				time.Sleep(time.Millisecond)
			}
			errs <- errors.New("busy retry exhausted")
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	byBot := map[string][]int64{}
	var globals []int
	for out := range results {
		byBot[out.Message.BotID] = append(byBot[out.Message.BotID], out.Message.BotSeq)
		globals = append(globals, int(*out.Message.CreatedChangeSeq))
		if len(out.Pushes) != 1 || out.Delivery != nil {
			t.Fatal("concurrent pending mismatch")
		}
	}
	if len(globals) != count || len(byBot) != 2 {
		t.Fatal("missing commits")
	}
	sort.Ints(globals)
	for i := 1; i < len(globals); i++ {
		if globals[i] <= globals[i-1] {
			t.Fatal("global order")
		}
	}
	for _, seqs := range byBot {
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		if len(seqs) != count/2 {
			t.Fatal("Bot isolation")
		}
		for i, n := range seqs {
			if n != int64(i+1) {
				t.Fatal("Bot sequence", seqs)
			}
		}
	}
	state := persisted(t, f.other)
	if state[0] != count || state[1] != 0 || state[3] != count || state[4] != count+2 || state[6] != count {
		t.Fatal("atomic concurrency", state)
	}
	assertPersisted(t, f, state)
	// 删除内容和日志不回绕持久计数，重开后下一条仍严格增长。
	execMessages(t, f.s, "DELETE FROM messages; DELETE FROM changes")
	reopened, e := storage.Open(messageContext, f.cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	out, e := New(reopened).CreateApplication(messageContext, f.identity, f.input())
	if e != nil {
		t.Fatal(e)
	}
	if out.Message.BotSeq != count/2+1 || int(*out.Message.CreatedChangeSeq) <= globals[len(globals)-1] {
		t.Fatal("reopen/delete regressed sequence")
	}
}

func TestMessageSequenceUpperBoundsAndCallerRollback(t *testing.T) {
	f := setupMessages(t)
	execMessages(t, f.s, "UPDATE bots SET bot_seq=? WHERE id=?", int64(math.MaxInt64-1), f.a.Bot.ID)
	out, err := f.r.CreateApplication(messageContext, f.identity, f.input())
	if err != nil || out.Message.BotSeq != math.MaxInt64 {
		t.Fatalf("last valid Bot sequence: %+v %v", out, err)
	}
	before := persisted(t, f.other)
	if out, err = f.r.CreateApplication(messageContext, f.identity, f.input()); !errors.Is(err, ErrSequenceExhausted) || !reflect.DeepEqual(out, Created{}) {
		t.Fatal("overflow accepted", err)
	}
	assertPersisted(t, f, before)

	g := setupMessages(t)
	// SQLite AUTOINCREMENT 自身到达 int64 上界；真实 Append 失败必须回滚先前计数/消息。
	execMessages(t, g.s, "UPDATE sqlite_sequence SET seq=? WHERE name='changes'", int64(math.MaxInt64))
	before = persisted(t, g.other)
	if out, err = g.r.CreateApplication(messageContext, g.identity, g.input()); err == nil || !reflect.DeepEqual(out, Created{}) {
		t.Fatal("global sequence overflow accepted", err)
	}
	assertPersisted(t, g, before)

	h := setupMessages(t)
	before = persisted(t, h.other)
	failure := errors.New("caller idempotency write failed")
	err = h.s.Write(messageContext, func(tx *sql.Tx) error {
		if _, e := h.r.CreateAdministratorTx(messageContext, tx, h.input()); e != nil {
			return e
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	assertPersisted(t, h, before)
	// 候选回滚后可继续创建；Bot 元数据 version 不随内容序号改变。
	out, err = adminWrite(h, h.input())
	if err != nil || out.Message.BotSeq != 1 {
		t.Fatal("creation after rollback", err)
	}
	bot, err := h.b.Get(messageContext, h.a.Bot.ID)
	if err != nil || bot.Version != 1 {
		t.Fatal("message changed Bot metadata version", err)
	}
}

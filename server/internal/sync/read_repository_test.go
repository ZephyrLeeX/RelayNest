package sync_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	stdsync "sync"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/changes"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
	read "relaynest/server/internal/sync"
)

var ctx = context.Background()

type fixture struct {
	s, other *storage.Store
	cfg      storage.Config
	bot      bots.Created
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "read.sqlite"), BusyTimeout: 50 * time.Millisecond}}
	var err error
	f.s, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.s.Close() })
	f.other, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.other.Close() })
	f.bot, _, err = bots.New(f.s).Create(ctx, bots.CreateRequest{Name: "read", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func execSQL(t *testing.T, s *storage.Store, q string, args ...any) {
	t.Helper()
	if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func scalar(t *testing.T, s *storage.Store, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := s.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRow(q, args...).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}
func get(t *testing.T, s *storage.Store, id string) read.ReadState {
	t.Helper()
	var out read.ReadState
	if err := s.Read(ctx, func(tx *sql.Tx) error { var err error; out, err = read.GetReadStateTx(ctx, tx, id); return err }); err != nil {
		t.Fatal(err)
	}
	return out
}

// 测试调用方遵循提交后发布规则，生产仓储不额外包授权或自行开启事务。
func advance(c context.Context, s *storage.Store, id string, n int64) (read.ReadState, changes.Sequence, error) {
	var out read.ReadState
	var seq changes.Sequence
	err := s.Write(c, func(tx *sql.Tx) error { var err error; out, seq, err = read.AdvanceReadTx(c, tx, id, n); return err })
	if err != nil {
		return read.ReadState{}, 0, err
	}
	return out, seq, nil
}
func seed(t *testing.T, f *fixture, n int) {
	t.Helper()
	b := bots.New(f.s)
	identity, err := b.Authenticate(ctx, f.bot.Bot.ID, bots.APIToken, f.bot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	for range n {
		if _, err := messages.New(f.s).CreateApplication(ctx, identity, messages.TextInput{BotID: f.bot.Bot.ID, Format: "plain", Text: "body"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadInitializationUnreadAndDeletedUpper(t *testing.T) {
	f := setup(t)
	id := f.bot.Bot.ID
	initial := get(t, f.other, id)
	if initial.ReadSeq != 0 || initial.StateVersion != 1 || initial.UpdatedAt.IsZero() || initial.UpdatedAt.Location() != time.UTC || initial.UnreadCount != 0 {
		t.Fatal(initial)
	}
	if scalar(t, f.s, `SELECT count(*) FROM changes WHERE type='read.updated'`) != 0 {
		t.Fatal("initialization emitted read event")
	}
	reopenedInitial, err := storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if get(t, reopenedInitial, id) != initial {
		t.Fatal("new Bot initialization changed on reopen")
	}
	reopenedInitial.Close()
	// callback 等事件只占变更日志，不形成 application 消息，不增加未读。
	if err := f.s.Write(ctx, func(tx *sql.Tx) error {
		_, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.DeliveryUpsert, EntityID: "callback-event", Revision: 1, BotID: &id, CreatedAt: time.Now()})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	seed(t, f, 4)
	execSQL(t, f.s, `UPDATE messages SET sender='administrator' WHERE bot_id=? AND bot_seq=2`, id)
	execSQL(t, f.s, `UPDATE messages SET revision=2,text='edited' WHERE bot_id=? AND bot_seq=3`, id)
	execSQL(t, f.s, `DELETE FROM messages WHERE bot_id=? AND bot_seq=4`, id)
	// callback 不属于消息行；现存 application 计数不受 Bot、其他事件和编辑影响。
	if got := get(t, f.s, id); got.UnreadCount != 2 || got.ReadSeq != 0 || got.StateVersion != 1 {
		t.Fatal(got)
	}
	execSQL(t, f.s, `UPDATE bots SET enabled=0 WHERE id=?`, id)
	one, seq, err := advance(ctx, f.s, id, 1)
	if err != nil || seq == 0 || one.UnreadCount != 1 || one.StateVersion != 2 {
		t.Fatal(one, seq, err)
	}
	for _, n := range []int64{1, 0} {
		out, seq, err := advance(ctx, f.other, id, n)
		if err != nil || seq != 0 || out != one {
			t.Fatal("no-op changed state", out, seq, err)
		}
	}
	before := get(t, f.s, id)
	for _, n := range []int64{-1, 5, math.MaxInt64} {
		out, seq, err := advance(ctx, f.s, id, n)
		if !errors.Is(err, read.ErrInvalidReadSequence) || out != (read.ReadState{}) || seq != 0 {
			t.Fatal("invalid upper", out, seq, err)
		}
	}
	if get(t, f.s, id) != before {
		t.Fatal("invalid input mutated state")
	}
	out, seq, err := advance(ctx, f.s, id, 4)
	if err != nil || out.ReadSeq != 4 || out.UnreadCount != 0 || out.StateVersion != 3 || seq == 0 {
		t.Fatal("deleted highest rejected", out, seq, err)
	}
	reopened, err := storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if get(t, reopened, id) != out {
		t.Fatal("reopen changed state")
	}
	if scalar(t, f.s, `SELECT count(*) FROM changes WHERE type='read.updated' AND entity_id=? AND bot_id=?`, id, id) != 2 {
		t.Fatal("event count")
	}
	execSQL(t, f.s, `DELETE FROM bots WHERE id=?`, id)
	if scalar(t, f.s, `SELECT count(*) FROM bot_read_states WHERE bot_id=?`, id) != 0 {
		t.Fatal("cascade")
	}
	if err := f.s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO bot_read_states VALUES('absent',0,1,'2026-01-01T00:00:00Z')`)
		return err
	}); err == nil {
		t.Fatal("missing FK")
	}
}

func TestReadConcurrentMaxTwoStores(t *testing.T) {
	f := setup(t)
	seed(t, f, 12)
	id := f.bot.Bot.ID
	start := make(chan struct{})
	errs := make(chan error, 12)
	advanced := make(chan int64, 12)
	var wg stdsync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			s := f.s
			if i%2 == 1 {
				s = f.other
			}
			out, seq, err := advance(ctx, s, id, int64(i+1))
			if err == nil && seq > 0 {
				advanced <- out.StateVersion
			}
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	close(advanced)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	versions := map[int64]bool{}
	for v := range advanced {
		if versions[v] {
			t.Fatal("duplicate version")
		}
		versions[v] = true
	}
	final := get(t, f.other, id)
	n := int64(len(versions))
	if final.ReadSeq != 12 || final.UnreadCount != 0 || final.StateVersion != n+1 || scalar(t, f.s, `SELECT count(*) FROM changes WHERE type='read.updated'`) != n {
		t.Fatal(final, n)
	}
	for v := int64(2); v <= n+1; v++ {
		if !versions[v] {
			t.Fatal("version gap", v)
		}
	}
	if scalar(t, f.s, `SELECT count(*) FROM changes c JOIN bot_read_states r ON r.bot_id=c.bot_id WHERE c.type='read.updated' AND (c.revision<2 OR c.revision>r.state_version OR c.entity_id!=r.bot_id)`) != 0 {
		t.Fatal("event metadata")
	}
}

func TestReadFailClosedAndOverflow(t *testing.T) {
	for _, mode := range []string{"missing-bot", "missing-row", "bad-time", "non-utc", "zero-time", "negative", "ahead", "zero-version", "real-version", "real-read", "negative-bot", "real-bot", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			seed(t, f, 2)
			id := f.bot.Bot.ID
			switch mode {
			case "missing-bot":
				id = "absent"
			case "missing-row":
				execSQL(t, f.s, `DELETE FROM bot_read_states`)
			case "bad-time":
				execSQL(t, f.s, `UPDATE bot_read_states SET updated_at='bad'`)
			case "non-utc":
				execSQL(t, f.s, `UPDATE bot_read_states SET updated_at='2026-01-01T01:00:00+01:00'`)
			case "zero-time":
				execSQL(t, f.s, `UPDATE bot_read_states SET updated_at='0001-01-01T00:00:00Z'`)
			case "ahead":
				execSQL(t, f.s, `UPDATE bot_read_states SET read_seq=3`)
			case "negative", "zero-version", "real-version", "real-read", "negative-bot", "real-bot":
				err := f.s.Write(ctx, func(tx *sql.Tx) error {
					if _, err := tx.Exec(`PRAGMA ignore_check_constraints=ON`); err != nil {
						return err
					}
					defer tx.Exec(`PRAGMA ignore_check_constraints=OFF`)
					q := map[string]string{"negative": `UPDATE bot_read_states SET read_seq=-1`, "zero-version": `UPDATE bot_read_states SET state_version=0`, "real-version": `UPDATE bot_read_states SET state_version=1.5`, "real-read": `UPDATE bot_read_states SET read_seq=0.5`, "negative-bot": `UPDATE bots SET bot_seq=-1`, "real-bot": `UPDATE bots SET bot_seq=2.5`}[mode]
					_, err := tx.Exec(q)
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			case "overflow":
				execSQL(t, f.s, `UPDATE bot_read_states SET state_version=?`, int64(math.MaxInt64))
				stable := get(t, f.s, id)
				out, seq, err := advance(ctx, f.s, id, 0)
				if err != nil || out != stable || seq != 0 {
					t.Fatal("max-version no-op", err)
				}
			}
			out, seq, err := advance(ctx, f.s, id, 1)
			if err == nil || out != (read.ReadState{}) || seq != 0 {
				t.Fatal("fail closed", out, seq, err)
			}
			if mode == "missing-bot" {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatal("missing Bot error classification", err)
				}
			} else if !errors.Is(err, read.ErrReadStateUnavailable) {
				t.Fatal("unavailable state error classification", err)
			}
			if scalar(t, f.s, `SELECT count(*) FROM changes WHERE type='read.updated'`) != 0 {
				t.Fatal("failed read emitted event")
			}
			if mode == "missing-row" && scalar(t, f.s, `SELECT count(*) FROM bot_read_states`) != 0 {
				t.Fatal("lazy repaired")
			}
			if mode != "overflow" {
				if err := f.s.Read(ctx, func(tx *sql.Tx) error {
					out, err := read.GetReadStateTx(ctx, tx, id)
					if err == nil || out != (read.ReadState{}) {
						t.Fatal("GET fail closed", out, err)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if out, err := read.GetReadStateTx(ctx, nil, "x"); err == nil || out != (read.ReadState{}) {
		t.Fatal("nil read")
	}
	if out, seq, err := read.AdvanceReadTx(ctx, nil, "x", 1); err == nil || out != (read.ReadState{}) || seq != 0 {
		t.Fatal("nil advance")
	}
}

func TestReadAtomicStorageFailures(t *testing.T) {
	for _, mode := range []string{"state", "changes", "late-read", "late-version", "commit", "cancel", "sql"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			seed(t, f, 2)
			id := f.bot.Bot.ID
			before := get(t, f.s, id)
			events := scalar(t, f.s, `SELECT count(*) FROM changes`)
			switch mode {
			case "state":
				execSQL(t, f.s, `CREATE TRIGGER read_fail BEFORE UPDATE ON bot_read_states BEGIN SELECT RAISE(ABORT,'state fault'); END`)
			case "changes":
				execSQL(t, f.s, `CREATE TRIGGER read_fail BEFORE INSERT ON changes WHEN NEW.type='read.updated' BEGIN SELECT RAISE(ABORT,'changes fault'); END`)
			case "late-read":
				execSQL(t, f.s, `CREATE TRIGGER read_fail AFTER UPDATE ON bot_read_states BEGIN UPDATE bot_read_states SET updated_at='bad'; END`)
			case "late-version":
				execSQL(t, f.s, `CREATE TRIGGER read_fail AFTER UPDATE ON bot_read_states BEGIN UPDATE bot_read_states SET state_version=state_version+1; END`)
			case "commit":
				execSQL(t, f.s, `CREATE TABLE commit_fault(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
			}
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			var candidate read.ReadState
			var seq changes.Sequence
			entered := false
			callbackOK := false
			err := f.s.Write(c, func(tx *sql.Tx) error {
				entered = true
				var err error
				candidate, seq, err = read.AdvanceReadTx(c, tx, id, 1)
				if err != nil {
					return err
				}
				switch mode {
				case "commit":
					_, err = tx.Exec(`INSERT INTO commit_fault VALUES('absent')`)
				case "cancel":
					cancel()
				case "sql":
					_, err = tx.Exec(`INSERT INTO absent_table VALUES(1)`)
				}
				callbackOK = err == nil
				return err
			})
			if err == nil || !entered {
				t.Fatal("fault accepted", err)
			}
			if mode == "commit" && !callbackOK {
				t.Fatal("not actual COMMIT failure", err)
			}
			// 仓储自身失败必须返回零值；末次提交失败可能已有事务内候选，
			// 实际发布边界另由真实授权调用方回归验证，不能把候选当作提交结果。
			if mode == "state" || mode == "changes" || mode == "late-read" || mode == "late-version" {
				if candidate != (read.ReadState{}) || seq != 0 {
					t.Fatal("repository failure released partial state", candidate, seq)
				}
			} else if candidate.ReadSeq != 1 || seq == 0 {
				t.Fatal("fault did not reach post-update boundary", candidate, seq)
			}
			if got := get(t, f.other, id); got != before {
				t.Fatal("state did not roll back", got, before)
			}
			if scalar(t, f.other, `SELECT count(*) FROM changes`) != events {
				t.Fatal("changes did not roll back")
			}
		})
	}
}

func TestReadCancelledAdmissionDoesNotEnter(t *testing.T) {
	f := setup(t)
	c, cancel := context.WithCancel(ctx)
	cancel()
	entered := false
	err := f.s.Write(c, func(tx *sql.Tx) error {
		entered = true
		_, _, err := read.AdvanceReadTx(c, tx, f.bot.Bot.ID, 1)
		return err
	})
	if !errors.Is(err, context.Canceled) || entered {
		t.Fatal("cancel entered", err)
	}
	// 持有共享写门时，另一 Store 的取消不能进入回调。
	err = f.s.Write(ctx, func(*sql.Tx) error {
		c, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()
		err := f.other.Write(c, func(*sql.Tx) error { entered = true; return nil })
		if !errors.Is(err, context.DeadlineExceeded) || entered {
			return fmt.Errorf("blocked cancellation: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

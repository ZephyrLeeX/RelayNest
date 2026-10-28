package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
)

var pendingContext = context.Background()

type pendingFixture struct {
	store, other *storage.Store
	cfg          storage.Config
	sessions     *auth.SessionStore
	repo         *messages.Repository
	identity     bots.Identity
	botID        string
}

func setupPending(t *testing.T) *pendingFixture {
	t.Helper()
	f := &pendingFixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "pending.sqlite"), BusyTimeout: 50 * time.Millisecond}}
	var err error
	f.store, err = storage.Open(pendingContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.store.Close() })
	f.other, err = storage.Open(pendingContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.other.Close() })
	if err := auth.InitAdministrator(pendingContext, f.store, "admin", []byte("pending-test-password")); err != nil {
		t.Fatal(err)
	}
	f.sessions = auth.NewSessionStore(f.store)
	f.repo = messages.New(f.store)
	b := bots.New(f.store)
	created, _, err := b.Create(pendingContext, bots.CreateRequest{Name: "pending", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.botID = created.Bot.ID
	f.identity, err = b.Authenticate(pendingContext, f.botID, bots.APIToken, created.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *pendingFixture) session(t *testing.T, device string) (auth.Session, auth.SessionSecrets) {
	t.Helper()
	s, secret, err := f.sessions.Create(pendingContext, 1, auth.SessionInput{ClientType: "android", DeviceID: device, DeviceName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return s, secret
}

// 非零待办全部来自真实消息仓储事务；当前生产消息 HTTP 的 targets 仍为空。
func (f *pendingFixture) message(t *testing.T, store *storage.Store, targets ...auth.Session) messages.Created {
	t.Helper()
	in := messages.TextInput{BotID: f.botID, Format: "plain", Text: "private-test-body"}
	for _, s := range targets {
		in.PushTargets = append(in.PushTargets, messages.PushTarget{SessionID: s.ID, DeviceID: s.DeviceID})
	}
	var out messages.Created
	err := store.Write(pendingContext, func(tx *sql.Tx) error {
		var err error
		out, err = f.repo.CreateApplicationTx(pendingContext, tx, f.identity, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Pushes) != len(targets) || len(out.Pushes) == 0 {
		t.Fatal("nonzero repository jobs required")
	}
	return out
}

func execPending(t *testing.T, store *storage.Store, query string, args ...any) {
	t.Helper()
	if err := store.Write(pendingContext, func(tx *sql.Tx) error { _, err := tx.ExecContext(pendingContext, query, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func readPending(t *testing.T, store *storage.Store, now time.Time, limit int) []PendingDevice {
	t.Helper()
	var out []PendingDevice
	err := store.Read(pendingContext, func(tx *sql.Tx) error {
		var err error
		out, err = ReadPendingTx(pendingContext, tx, now, limit)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// 对所有持久表（含业务、changes、自增、注册与秘密摘要）做确定 hash；不打印原值。
func pendingState(t *testing.T, store *storage.Store) [32]byte {
	t.Helper()
	h := sha256.New()
	err := store.Read(pendingContext, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name`)
		if err != nil {
			return err
		}
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, table := range tables {
			fmt.Fprintln(h, table)
			r, err := tx.Query(`SELECT * FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
			if err != nil {
				return err
			}
			columns, err := r.Columns()
			if err != nil {
				r.Close()
				return err
			}
			var encodedRows []string
			for r.Next() {
				values := make([]any, len(columns))
				ptrs := make([]any, len(columns))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if err := r.Scan(ptrs...); err != nil {
					r.Close()
					return err
				}
				data, err := json.Marshal(values)
				if err != nil {
					r.Close()
					return err
				}
				encodedRows = append(encodedRows, string(data))
			}
			if err := r.Err(); err != nil {
				r.Close()
				return err
			}
			if err := r.Close(); err != nil {
				return err
			}
			sort.Strings(encodedRows)
			for _, encoded := range encodedRows {
				fmt.Fprintln(h, encoded)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

const pendingDeviceA = "11111111-1111-4111-8111-111111111111"
const pendingDeviceB = "22222222-2222-4222-8222-222222222222"

func TestPendingRepositoryMergeIsolationAndReadOnly(t *testing.T) {
	f := setupPending(t)
	a, _ := f.session(t, pendingDeviceA)
	b, _ := f.session(t, pendingDeviceB)
	same, _ := f.session(t, pendingDeviceA)
	f.message(t, f.store, a, b, same)
	latest := f.message(t, f.store, a)
	now := time.Now().Add(time.Hour)
	before := pendingState(t, f.store)
	got := readPending(t, f.other, now, MaxPendingSessions)
	if len(got) != 2 || got[0].DeviceID != pendingDeviceA || got[1].DeviceID != pendingDeviceB || len(got[0].Sessions) != 2 || len(got[1].Sessions) != 1 {
		t.Fatal("device/session isolation")
	}
	for i, s := range got[0].Sessions {
		if i > 0 && got[0].Sessions[i-1].SessionID >= s.SessionID {
			t.Fatal("session order")
		}
		if s.SessionID == a.ID && s.TargetChangeSeq != latest.LastChangeSeq {
			t.Fatal("maximum watermark lost")
		}
		if s.SessionID == same.ID && s.TargetChangeSeq >= latest.LastChangeSeq {
			t.Fatal("cross-session watermark")
		}
	}
	limited := readPending(t, f.store, now, 1)
	if len(limited) != 1 || len(limited[0].Sessions) != 1 || limited[0].Sessions[0] != got[0].Sessions[0] {
		t.Fatal("bounded deterministic prefix")
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-test-body") || strings.Contains(string(encoded), "Token") || strings.Contains(string(encoded), "URL") {
		t.Fatal("secret/content exposed")
	}
	if !reflect.DeepEqual(got, readPending(t, f.store, now, MaxPendingSessions)) || before != pendingState(t, f.other) {
		t.Fatal("repeat read changed state")
	}
	reopened, err := storage.Open(pendingContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if !reflect.DeepEqual(got, readPending(t, reopened, now, MaxPendingSessions)) || before != pendingState(t, reopened) {
		t.Fatal("reopen instability")
	}
}

func TestPendingDueTimeNanosecondAndEmpty(t *testing.T) {
	f := setupPending(t)
	s, _ := f.session(t, pendingDeviceA)
	if len(readPending(t, f.store, time.Now(), 1)) != 0 {
		t.Fatal("not empty")
	}
	one := f.message(t, f.store, s)
	two := f.message(t, f.store, s)
	now := time.Now().UTC().Truncate(time.Second)
	// 整秒和有小数的 RFC3339Nano 字符串字典序并非时间顺序。
	execPending(t, f.store, "UPDATE push_jobs SET next_attempt_at=? WHERE id=?", now.Format(time.RFC3339Nano), one.Pushes[0].ID)
	execPending(t, f.store, "UPDATE push_jobs SET next_attempt_at=? WHERE id=?", now.Add(time.Nanosecond).Format(time.RFC3339Nano), two.Pushes[0].ID)
	got := readPending(t, f.store, now, 1)
	if len(got) != 1 || got[0].Sessions[0].TargetChangeSeq != one.LastChangeSeq {
		t.Fatal("future job included or equality excluded")
	}
	got = readPending(t, f.store, now.Add(time.Nanosecond), 1)
	if len(got) != 1 || got[0].Sessions[0].TargetChangeSeq != two.LastChangeSeq {
		t.Fatal("nanosecond due boundary")
	}
	if len(readPending(t, f.store, now.Add(-time.Nanosecond), 1)) != 0 {
		t.Fatal("future-only projection")
	}
	// 非 UTC 合法偏移也按实际时间判断，不假设写入字符串的时区格式。
	execPending(t, f.store, "UPDATE push_jobs SET next_attempt_at=?", now.In(time.FixedZone("offset", 8*3600)).Format(time.RFC3339Nano))
	if readPending(t, f.store, now, 1)[0].Sessions[0].TargetChangeSeq != two.LastChangeSeq {
		t.Fatal("offset timestamp")
	}
}

func TestPendingRealSessionValidity(t *testing.T) {
	for _, mode := range []string{"revoke", "idle", "absolute", "generation", "web"} {
		t.Run(mode, func(t *testing.T) {
			f := setupPending(t)
			s, secret := f.session(t, pendingDeviceA)
			f.message(t, f.store, s)
			now := time.Now().Add(time.Hour)
			switch mode {
			case "revoke":
				if err := f.sessions.Revoke(pendingContext, secret.Token, "android"); err != nil {
					t.Fatal(err)
				}
			case "idle":
				now = s.IdleExpiresAt
				if len(readPending(t, f.store, now.Add(-time.Nanosecond), 1)) != 1 {
					t.Fatal("idle early expiry")
				}
			case "absolute":
				now = s.AbsoluteExpiresAt
				execPending(t, f.store, "UPDATE sessions SET idle_expires_at=? WHERE id=?", now.UnixNano(), s.ID)
				if len(readPending(t, f.store, now.Add(-time.Nanosecond), 1)) != 1 {
					t.Fatal("absolute early expiry")
				}
			case "generation":
				if err := auth.ResetPassword(pendingContext, f.store, []byte("new-pending-test-password")); err != nil {
					t.Fatal(err)
				}
			case "web":
				execPending(t, f.store, "UPDATE sessions SET client_type='web' WHERE id=?", s.ID)
			}
			before := pendingState(t, f.store)
			if len(readPending(t, f.store, now, 1)) != 0 {
				t.Fatal("invalid session projected")
			}
			if before != pendingState(t, f.other) {
				t.Fatal("invalid projection wrote state")
			}
		})
	}
}

func TestPendingBindingMismatchSameTransaction(t *testing.T) {
	f := setupPending(t)
	s, _ := f.session(t, pendingDeviceA)
	f.message(t, f.store, s)
	before := pendingState(t, f.store)
	rollback := errors.New("rollback fault injection")
	err := f.store.Write(pendingContext, func(tx *sql.Tx) error {
		if _, err := tx.Exec("PRAGMA defer_foreign_keys=ON"); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE push_jobs SET device_id=?", pendingDeviceB); err != nil {
			return err
		}
		got, err := ReadPendingTx(pendingContext, tx, time.Now().Add(time.Hour), 1)
		if err != nil {
			return err
		}
		if len(got) != 0 {
			t.Fatal("binding mismatch included")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if before != pendingState(t, f.other) {
		t.Fatal("fault not rolled back")
	}
}

func TestPendingTwoStoreSnapshotKeepsUnseenJobs(t *testing.T) {
	f := setupPending(t)
	s, _ := f.session(t, pendingDeviceA)
	one := f.message(t, f.store, s)
	now := time.Now().Add(time.Hour)
	err := f.store.Read(pendingContext, func(tx *sql.Tx) error {
		first, err := ReadPendingTx(pendingContext, tx, now, 1)
		if err != nil {
			return err
		}
		// 同一快照存活时，另一 Store 完成真实消息提交；不依赖睡眠或放宽超时。
		type writeResult struct {
			created messages.Created
			err     error
		}
		done := make(chan writeResult, 1)
		go func() {
			var out messages.Created
			err := f.other.Write(pendingContext, func(tx *sql.Tx) error {
				var err error
				out, err = f.repo.CreateApplicationTx(pendingContext, tx, f.identity, messages.TextInput{BotID: f.botID, Format: "plain", Text: "concurrent-private-body", PushTargets: []messages.PushTarget{{SessionID: s.ID, DeviceID: s.DeviceID}}})
				return err
			})
			done <- writeResult{out, err}
		}()
		written := <-done
		if written.err != nil {
			return written.err
		}
		second := written.created
		if len(second.Pushes) != 1 {
			t.Fatal("nonzero concurrent repository job required")
		}
		if second.LastChangeSeq <= one.LastChangeSeq {
			t.Fatal("new job missing")
		}
		again, err := ReadPendingTx(pendingContext, tx, now, 1)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(first, again) || first[0].Sessions[0].TargetChangeSeq != one.LastChangeSeq {
			t.Fatal("snapshot changed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := pendingState(t, f.store)
	got := readPending(t, f.other, now, 1)
	if got[0].Sessions[0].TargetChangeSeq <= one.LastChangeSeq {
		t.Fatal("unseen new job lost")
	}
	var n int
	if err := f.store.Read(pendingContext, func(tx *sql.Tx) error { return tx.QueryRow("SELECT count(*) FROM push_jobs").Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	if n != 2 || before != pendingState(t, f.other) {
		t.Fatal("read consumed jobs")
	}
}

func TestPendingFailuresNeverPublishPartial(t *testing.T) {
	f := setupPending(t)
	a, _ := f.session(t, pendingDeviceA)
	b, _ := f.session(t, pendingDeviceB)
	f.message(t, f.store, a, b)
	now := time.Now().Add(time.Hour)
	if got, err := ReadPendingTx(pendingContext, nil, now, 1); got != nil || !errors.Is(err, ErrPendingInput) {
		t.Fatal("nil tx")
	}
	err := f.store.Read(pendingContext, func(tx *sql.Tx) error {
		for _, limit := range []int{0, -1, MaxPendingSessions + 1} {
			if got, err := ReadPendingTx(pendingContext, tx, now, limit); got != nil || !errors.Is(err, ErrPendingInput) {
				t.Fatal("limit input")
			}
		}
		for _, bad := range []time.Time{{}, time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)} {
			if got, err := ReadPendingTx(pendingContext, tx, bad, 1); got != nil || !errors.Is(err, ErrPendingInput) {
				t.Fatal("time input")
			}
		}
		if got, err := ReadPendingTx(nil, tx, now, 1); got != nil || !errors.Is(err, ErrPendingInput) {
			t.Fatal("nil context")
		}
		ctx, cancel := context.WithCancel(pendingContext)
		cancel()
		if got, err := ReadPendingTx(ctx, tx, now, 1); got != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := pendingState(t, f.store)
	rollback := errors.New("rollback fault injection")
	for _, query := range []string{"DROP TABLE push_jobs", "PRAGMA ignore_check_constraints=ON; UPDATE push_jobs SET target_change_seq='invalid' WHERE device_id='" + pendingDeviceB + "'", "UPDATE push_jobs SET next_attempt_at='broken' WHERE device_id='" + pendingDeviceB + "'"} {
		err = f.store.Write(pendingContext, func(tx *sql.Tx) error {
			if _, err := tx.Exec(query); err != nil {
				return err
			}
			got, err := ReadPendingTx(pendingContext, tx, now, MaxPendingSessions)
			if err == nil || got != nil {
				t.Fatal("failure leaked partial candidates")
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
	}
	if before != pendingState(t, f.other) {
		t.Fatal("error path changed state")
	}
}

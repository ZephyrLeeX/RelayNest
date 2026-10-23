package changes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

var ctx = context.Background()

func openStore(t *testing.T, path string, timeout time.Duration) *storage.Store {
	t.Helper()
	s, err := storage.Open(ctx, storage.Config{Path: path, BusyTimeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func metadata(typ Type, id string) Metadata {
	return Metadata{Type: typ, EntityID: id, Revision: 1, CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 123, time.FixedZone("UTC+8", 8*3600))}
}
func exec(t *testing.T, tx *sql.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}
func write(t *testing.T, s *storage.Store, fn func(*sql.Tx) error) {
	t.Helper()
	if err := s.Write(ctx, fn); err != nil {
		t.Fatal(err)
	}
}
func count(t *testing.T, s *storage.Store, query string) int64 {
	t.Helper()
	var n int64
	if err := s.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRowContext(ctx, query).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}
func epoch(t *testing.T, s *storage.Store) string {
	t.Helper()
	var e string
	if err := s.Read(ctx, func(tx *sql.Tx) (err error) { e, err = Epoch(ctx, tx); return }); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestSequenceJSON(t *testing.T) {
	for _, value := range []string{"0", "1", "9007199254740991", "9007199254740992", "9007199254740993", "9223372036854775807"} {
		seq, err := ParseSequence(value)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(seq)
		if err != nil || string(b) != `"`+value+`"` {
			t.Fatalf("%s: %s %v", value, b, err)
		}
		var got Sequence
		if err := json.Unmarshal(b, &got); err != nil || got != seq {
			t.Fatalf("roundtrip %s: %v", b, err)
		}
	}
	for _, bad := range []string{"-1", "9223372036854775808", "18446744073709551616", "01", "+1", "1e3", "", " 1", "1.0"} {
		if _, err := ParseSequence(bad); !errors.Is(err, ErrInvalidSequence) {
			t.Fatalf("accepted %q", bad)
		}
		b, _ := json.Marshal(bad)
		n := Sequence(7)
		if err := json.Unmarshal(b, &n); !errors.Is(err, ErrInvalidSequence) || n != 7 {
			t.Fatalf("accepted or mutated %s", b)
		}
	}
	for _, bad := range []string{`0`, `9007199254740993`, `null`, `true`, `{}`, `[]`} {
		var n Sequence
		if json.Unmarshal([]byte(bad), &n) == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	if _, err := json.Marshal(Sequence(-1)); !errors.Is(err, ErrInvalidSequence) {
		t.Fatal(err)
	}
}

func TestFrozenTypes(t *testing.T) {
	cases := []struct {
		typ     Type
		literal string
		keys    []string
	}{
		{MessageUpsert, "message.upsert", []string{"message"}},
		{MessageDeleted, "message.deleted", []string{"id", "bot_id", "bot_seq", "revision", "deleted_at"}},
		{BotUpsert, "bot.upsert", []string{"bot"}},
		{BotDeleted, "bot.deleted", []string{"id", "version", "deleted_at"}},
		{ReadUpdated, "read.updated", []string{"read"}},
		{DeliveryUpsert, "delivery.upsert", []string{"delivery"}},
		{DeviceNotificationsUpdated, "device.notifications.updated", []string{"settings"}},
		{SettingsUpdated, "settings.updated", []string{"settings_version", "recovery_path"}},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.typ)
		if err != nil || string(b) != `"`+c.literal+`"` || !reflect.DeepEqual(c.typ.ProjectionKeys(), c.keys) {
			t.Fatalf("type %q", c.typ)
		}
		c.typ.ProjectionKeys()[0] = "mutated"
		if !reflect.DeepEqual(c.typ.ProjectionKeys(), c.keys) {
			t.Fatal("shared mutable keys")
		}
	}
	if Type("unknown").ProjectionKeys() != nil {
		t.Fatal("unknown type")
	}
	b, err := json.Marshal(struct {
		Revision int64 `json:"revision"`
	}{9007199254740993})
	if err != nil || string(b) != `{"revision":9007199254740993}` {
		t.Fatalf("version: %s %v", b, err)
	}
}

func TestCallerTransaction(t *testing.T) {
	s := openStore(t, filepath.Join(t.TempDir(), "db.sqlite"), 0)
	if count(t, s, "SELECT count(*) FROM changes") != 0 {
		t.Fatal("fake initial event")
	}
	write(t, s, func(tx *sql.Tx) error {
		exec(t, tx, "CREATE TABLE test_entities(id TEXT PRIMARY KEY, revision INTEGER NOT NULL)")
		exec(t, tx, "INSERT INTO test_entities VALUES ('existing',1)")
		return nil
	})
	original := epoch(t, s)
	var seq Sequence
	write(t, s, func(tx *sql.Tx) error {
		exec(t, tx, "INSERT INTO test_entities VALUES ('committed',1)")
		exec(t, tx, "UPDATE test_entities SET revision=2 WHERE id='existing'")
		m := metadata(BotUpsert, "committed")
		m.Revision = 9007199254740993
		var err error
		seq, err = Append(ctx, tx, m)
		if err != nil {
			return err
		}
		if count(t, s, "SELECT count(*) FROM changes") != 0 || count(t, s, "SELECT count(*) FROM test_entities") != 1 || count(t, s, "SELECT revision FROM test_entities WHERE id='existing'") != 1 {
			t.Fatal("early commit")
		}
		return nil
	})
	if seq <= 0 || count(t, s, "SELECT change_seq FROM changes") != int64(seq) || count(t, s, "SELECT revision FROM changes") != 9007199254740993 {
		t.Fatal("stored values differ")
	}
	// 独立失败分支避免与成功快照混淆。
	for _, invalid := range []bool{false, true} {
		err := s.Write(ctx, func(tx *sql.Tx) error {
			exec(t, tx, "INSERT INTO test_entities VALUES ('rollback',1)")
			exec(t, tx, "UPDATE test_entities SET revision=3 WHERE id='existing'")
			m := metadata(MessageDeleted, "rollback")
			if invalid {
				m.EntityID = "\x00bad"
			}
			_, err := Append(ctx, tx, m)
			if err != nil {
				return err
			}
			return errors.New("after append")
		})
		if err == nil {
			t.Fatal("expected failure")
		}
		if count(t, s, "SELECT count(*) FROM test_entities") != 2 || count(t, s, "SELECT revision FROM test_entities WHERE id='existing'") != 2 || count(t, s, "SELECT count(*) FROM changes") != 1 || epoch(t, s) != original {
			t.Fatal("partial write")
		}
	}
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		var bot sql.NullString
		var timestamp string
		err := tx.QueryRowContext(ctx, "SELECT bot_id,created_at FROM changes").Scan(&bot, &timestamp)
		if bot.Valid || timestamp != "2026-10-23T04:00:00.000000123Z" {
			t.Fatalf("metadata: %v %s", bot, timestamp)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReopenAndEpoch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	s := openStore(t, path, 0)
	initial := epoch(t, s)
	if !epochPattern.MatchString(initial) {
		t.Fatal(initial)
	}
	for range 2 {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s = openStore(t, path, 0)
		if epoch(t, s) != initial || count(t, s, "SELECT count(*) FROM changes") != 0 {
			t.Fatal("empty restart changed state")
		}
	}
	var previous Sequence
	for _, typ := range []Type{MessageUpsert, ReadUpdated, BotDeleted, SettingsUpdated} {
		write(t, s, func(tx *sql.Tx) error {
			seq, err := Append(ctx, tx, metadata(typ, string(typ)))
			if seq <= previous {
				t.Fatalf("not monotonic %d <= %d", seq, previous)
			}
			previous = seq
			return err
		})
		write(t, s, func(tx *sql.Tx) error { _, err := tx.ExecContext(ctx, "DELETE FROM changes"); return err })
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s = openStore(t, path, 0)
		if epoch(t, s) != initial {
			t.Fatal("restart changed epoch")
		}
	}
	other := openStore(t, filepath.Join(t.TempDir(), "other.sqlite"), 0)
	if epoch(t, other) == initial {
		t.Fatal("new database reused epoch")
	}
	write(t, s, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE system_state SET sync_epoch=?", strings.Repeat("x", 36))
		return err
	})
	if err := s.Read(ctx, func(tx *sql.Tx) error { _, err := Epoch(ctx, tx); return err }); !errors.Is(err, ErrInvalidEpoch) {
		t.Fatal(err)
	}
	write(t, s, func(tx *sql.Tx) error { _, err := tx.ExecContext(ctx, "DELETE FROM system_state"); return err })
	for range 2 {
		if err := s.Read(ctx, func(tx *sql.Tx) error { _, err := Epoch(ctx, tx); return err }); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
	}
	if count(t, s, "SELECT count(*) FROM system_state") != 0 {
		t.Fatal("implicitly repaired epoch")
	}
}

func TestIndependentStoresConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	stores := []*storage.Store{openStore(t, path, 5*time.Second), openStore(t, path, 5*time.Second), openStore(t, path, 5*time.Second)}
	var wg sync.WaitGroup
	results := make(chan error, 60)
	for i, s := range stores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 20 {
				results <- s.Write(ctx, func(tx *sql.Tx) error {
					_, err := Append(ctx, tx, metadata(MessageUpsert, fmt.Sprintf("%d-%d", i, j)))
					return err
				})
			}
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count(t, stores[0], "SELECT count(DISTINCT change_seq) FROM changes") != 60 || count(t, stores[0], "SELECT count(DISTINCT entity_id) FROM changes") != 60 {
		t.Fatal("lost or duplicate event")
	}
}

func TestLockFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	first := openStore(t, path, 0)
	second := openStore(t, path, 10*time.Millisecond)
	write(t, first, func(tx *sql.Tx) error {
		exec(t, tx, "CREATE TABLE test_entities(id INTEGER PRIMARY KEY)")
		return nil
	})
	write(t, first, func(tx *sql.Tx) error {
		_, err := Append(ctx, tx, metadata(BotUpsert, "first"))
		if err != nil {
			return err
		}
		entered := false
		err = second.Write(ctx, func(other *sql.Tx) error {
			entered = true
			exec(t, other, "INSERT INTO test_entities VALUES (1)")
			_, err := Append(ctx, other, metadata(BotUpsert, "second"))
			return err
		})
		if err == nil || entered {
			t.Fatalf("lock did not propagate: %v entered=%v", err, entered)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if err := second.Write(cancelled, func(*sql.Tx) error { t.Fatal("cancelled callback"); return nil }); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		return nil
	})
	if count(t, first, "SELECT count(*) FROM changes") != 1 || count(t, first, "SELECT count(*) FROM test_entities") != 0 {
		t.Fatal("half write")
	}
}

func TestSequenceExhaustion(t *testing.T) {
	s := openStore(t, filepath.Join(t.TempDir(), "db.sqlite"), 0)
	initial := epoch(t, s)
	write(t, s, func(tx *sql.Tx) error {
		exec(t, tx, "CREATE TABLE test_entities(id INTEGER PRIMARY KEY)")
		// 仅测试库构造 SQLite 序列边界，不改生产序列/表或 epoch。
		exec(t, tx, "INSERT INTO changes(change_seq,type,entity_id,revision,created_at) VALUES (?, 'bot.upsert','old',1,'2026-10-23T00:00:00Z')", int64(math.MaxInt64-1))
		return nil
	})
	write(t, s, func(tx *sql.Tx) error {
		seq, err := Append(ctx, tx, metadata(BotDeleted, "last"))
		if seq != Sequence(math.MaxInt64) {
			t.Fatalf("last=%d", seq)
		}
		return err
	})
	err := s.Write(ctx, func(tx *sql.Tx) error {
		exec(t, tx, "INSERT INTO test_entities VALUES (1)")
		_, err := Append(ctx, tx, metadata(BotUpsert, "overflow"))
		return err
	})
	if err == nil {
		t.Fatal("exhaustion succeeded")
	}
	if count(t, s, "SELECT count(*) FROM test_entities") != 0 || count(t, s, "SELECT count(*) FROM changes") != 2 || count(t, s, "SELECT change_seq FROM changes WHERE entity_id='old'") != math.MaxInt64-1 || count(t, s, "SELECT change_seq FROM changes WHERE entity_id='last'") != math.MaxInt64 || epoch(t, s) != initial {
		t.Fatal("exhaustion corrupted state")
	}
}

func TestMetadataValidationAndErrors(t *testing.T) {
	s := openStore(t, filepath.Join(t.TempDir(), "db.sqlite"), 0)
	for _, mutate := range []func(*Metadata){
		func(m *Metadata) { m.Type = "unknown" }, func(m *Metadata) { m.EntityID = "" },
		func(m *Metadata) { m.Revision = 0 }, func(m *Metadata) { m.Revision = -1 },
		func(m *Metadata) { m.BotID = new(string) }, func(m *Metadata) { m.CreatedAt = time.Time{} },
		func(m *Metadata) { m.CreatedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) },
	} {
		err := s.Write(ctx, func(tx *sql.Tx) error {
			m := metadata(BotUpsert, "invalid")
			mutate(&m)
			_, err := Append(ctx, tx, m)
			return err
		})
		if !errors.Is(err, ErrInvalidMetadata) {
			t.Fatal(err)
		}
	}
	if count(t, s, "SELECT count(*) FROM changes") != 0 {
		t.Fatal("invalid row")
	}
	if _, err := Append(ctx, nil, metadata(BotUpsert, "nil")); !errors.Is(err, ErrInvalidTransaction) {
		t.Fatal(err)
	}
	if _, err := Epoch(ctx, nil); !errors.Is(err, ErrInvalidTransaction) {
		t.Fatal(err)
	}
	var closed *sql.Tx
	write(t, s, func(tx *sql.Tx) error { closed = tx; return nil })
	if _, err := Append(ctx, closed, metadata(BotUpsert, "closed")); !errors.Is(err, sql.ErrTxDone) {
		t.Fatal(err)
	}
	write(t, s, func(tx *sql.Tx) error {
		m := metadata(BotUpsert, "max-version")
		m.Revision = math.MaxInt64
		bot := "bot-id"
		m.BotID = &bot
		_, err := Append(ctx, tx, m)
		return err
	})
	if count(t, s, "SELECT revision FROM changes") != math.MaxInt64 {
		t.Fatal("revision truncated")
	}
}

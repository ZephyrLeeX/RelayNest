package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"relaynest/server/migrations"
)

func openTest(t *testing.T) (*Store, Config) {
	t.Helper()
	cfg := Config{Path: filepath.Join(t.TempDir(), "db ?#.sqlite"), ReadConnections: 4}
	s, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s, cfg
}

func TestConnectionsAndTransactions(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	if s.writer.Stats().MaxOpenConnections != 1 || s.reader.Stats().MaxOpenConnections != 4 {
		t.Fatal("unexpected pool limits")
	}
	// 同时占用全部物理连接，避免只重复检查池中的同一连接。
	for _, pool := range []*sql.DB{s.writer, s.reader} {
		var conns []*sql.Conn
		for i := 0; i < pool.Stats().MaxOpenConnections; i++ {
			c, err := pool.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			conns = append(conns, c)
		}
		var wg sync.WaitGroup
		for _, c := range conns {
			wg.Add(1)
			go func(c *sql.Conn) {
				defer wg.Done()
				defer c.Close()
				for pragma, want := range map[string]string{"journal_mode": "wal", "synchronous": "2", "foreign_keys": "1", "busy_timeout": "5000"} {
					var got string
					if err := c.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil {
						t.Error(err)
						continue
					}
					if got != want {
						t.Errorf("%s=%s want %s", pragma, got, want)
					}
				}
			}(c)
		}
		wg.Wait()
	}
	err := s.Read(ctx, func(tx *sql.Tx) error { _, err := tx.Exec("UPDATE system_state SET config_version=2"); return err })
	if err == nil {
		t.Fatal("read transaction allowed writes")
	}
	sentinel := errors.New("abort")
	err = s.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE system_state SET config_version=2"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("rollback error: %v", err)
	}
	var version int
	if err = s.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRow("SELECT config_version FROM system_state").Scan(&version) }); err != nil || version != 1 {
		t.Fatalf("rollback version=%d err=%v", version, err)
	}
	if err = s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE system_state SET config_version=3")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = s.Write(canceled, func(*sql.Tx) error { t.Error("called canceled transaction"); return nil }); err == nil {
		t.Fatal("expected cancellation")
	}
	// 使用测试临时表实际验证外键执行，而不只依赖 PRAGMA 读数。
	if err = s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE parent(id INTEGER PRIMARY KEY); CREATE TABLE child(id INTEGER REFERENCES parent(id));")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec("INSERT INTO child VALUES(1)"); return err }); err == nil {
		t.Fatal("foreign key not enforced")
	}
}

func TestPersistenceAndSequence(t *testing.T) {
	s, cfg := openTest(t)
	ctx := context.Background()
	engine, err := s.EngineVersion(ctx)
	if err != nil || engine == "" {
		t.Fatalf("engine=%q err=%v", engine, err)
	}
	t.Logf("SQLite engine %s", engine)
	var epoch string
	if err := s.reader.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	if len(epoch) != 36 || epoch[14] != '4' || !strings.ContainsRune("89ab", rune(epoch[19])) {
		t.Fatalf("invalid epoch %s", epoch)
	}
	for i := 0; i < 2; i++ {
		if err := s.Write(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO changes(type,entity_id,revision,created_at) VALUES('test','x',1,'2026-10-22T00:00:00Z')")
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec("DELETE FROM changes"); return err }); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var gotEpoch string
	var seq, version int
	if err := reopened.reader.QueryRow("SELECT sync_epoch FROM system_state").Scan(&gotEpoch); err != nil {
		t.Fatal(err)
	}
	if gotEpoch != epoch {
		t.Fatal("epoch changed on reopen")
	}
	if err := reopened.reader.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 1 {
		t.Fatalf("persisted schema version=%d err=%v", version, err)
	}
	if err := reopened.reader.QueryRow("SELECT change_seq FROM changes").Scan(&seq); err != nil || seq != 2 {
		t.Fatalf("sequence=%d err=%v", seq, err)
	}
}

func TestConcurrentOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			s, err := Open(context.Background(), Config{Path: path})
			if err != nil {
				t.Error(err)
				return
			}
			defer s.Close()
			var count int
			if err := s.reader.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 1 {
				t.Errorf("ledger count=%d err=%v", count, err)
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestInvalidConfig(t *testing.T) {
	for _, cfg := range []Config{{}, {Path: ":memory:"}, {Path: "x", ReadConnections: -1}, {Path: "x", ReadConnections: 17}, {Path: "x", BusyTimeout: time.Nanosecond}, {Path: "x", BusyTimeout: 2 * time.Minute}} {
		if s, err := Open(context.Background(), cfg); err == nil {
			s.Close()
			t.Fatalf("accepted %+v", cfg)
		}
	}
}

func TestRejectLedgerWithoutRebuild(t *testing.T) {
	for name, statement := range map[string]string{
		"checksum": "UPDATE schema_migrations SET checksum='wrong' WHERE version=1",
		"name":     "UPDATE schema_migrations SET name='0001_changed.sql' WHERE version=1",
		"future":   "INSERT INTO schema_migrations VALUES(2,'0002_future.sql','unknown','now')",
		"gap":      "UPDATE schema_migrations SET version=3 WHERE version=1",
	} {
		t.Run(name, func(t *testing.T) {
			s, cfg := openTest(t)
			var epoch string
			if err := s.reader.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
				t.Fatal(err)
			}
			if _, err := s.writer.Exec(statement); err != nil {
				t.Fatal(err)
			}
			s.Close()
			if opened, err := Open(context.Background(), cfg); err == nil {
				opened.Close()
				t.Fatal("incompatible ledger accepted")
			}
			db, err := openPool(cfg.Path, Config{BusyTimeout: time.Second}, false)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var after string
			if err := db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&after); err != nil || after != epoch {
				t.Fatalf("database changed: %s %v", after, err)
			}
		})
	}
}

func TestMigrationUpgradeFailureAndLock(t *testing.T) {
	s, cfg := openTest(t)
	ctx := context.Background()
	first, err := migrations.Files.ReadFile("0001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := fstest.MapFS{
		"0001_initial.sql": &fstest.MapFile{Data: first},
		"0002_upgrade.sql": &fstest.MapFile{Data: []byte("CREATE TABLE upgraded(id INTEGER PRIMARY KEY); INSERT INTO upgraded VALUES(9);")},
		"0003_failure.sql": &fstest.MapFile{Data: []byte("CREATE TABLE partial(id); INSERT INTO missing VALUES(1);")},
	}
	if err := migrate(ctx, s.writer, source); err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := s.reader.QueryRow("SELECT count(*) FROM sqlite_master WHERE name IN ('partial','upgraded')").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial upgrade remained: %d %v", count, err)
	}
	if err := s.reader.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed upgrade changed ledger: %d %v", count, err)
	}
	delete(source, "0003_failure.sql")
	// 第二写池模拟独立启动者；写锁未释放前不能迁移，也不能跳过台账校验。
	conn, err := s.writer.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	competing, err := openPool(cfg.Path, Config{BusyTimeout: 20 * time.Millisecond}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer competing.Close()
	if err := migrate(ctx, competing, source); err == nil {
		t.Fatal("migration bypassed held write lock")
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if err := migrate(ctx, s.writer, source); err != nil {
		t.Fatal(err)
	}
	if err := s.reader.QueryRow("SELECT id FROM upgraded").Scan(&count); err != nil || count != 9 {
		t.Fatalf("upgrade failed: %d %v", count, err)
	}
	if err := migrate(ctx, s.writer, source); err != nil {
		t.Fatal(err)
	}
	// 使用新连接重新校验升级后的持久台账，不能依赖进程内版本缓存。
	upgraded, err := openPool(cfg.Path, Config{BusyTimeout: time.Second}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if err := migrate(ctx, upgraded, source); err != nil {
		t.Fatal(err)
	}
	if err := upgraded.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&count); err != nil || count != 2 {
		t.Fatalf("persisted upgrade version=%d err=%v", count, err)
	}
	// 旧二进制拒绝已经升级的新数据库。
	if err := migrate(ctx, s.writer, migrations.Files); err == nil {
		t.Fatal("old binary accepted newer schema")
	}
}

func TestMigrationFilesValidation(t *testing.T) {
	for i, source := range []fstest.MapFS{
		{}, {"bad.sql": &fstest.MapFile{Data: []byte("SELECT 1;")}},
		{"0002_gap.sql": &fstest.MapFile{Data: []byte("SELECT 1;")}},
		{"0001_a.sql": &fstest.MapFile{}, "0001_b.sql": &fstest.MapFile{}},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := loadMigrations(source); err == nil {
				t.Fatal("invalid migrations accepted")
			}
		})
	}
}

func TestCommitFailureAndConcurrentWrites(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE commit_parent(id INTEGER PRIMARY KEY); CREATE TABLE commit_child(id INTEGER REFERENCES commit_parent(id) DEFERRABLE INITIALLY DEFERRED);")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO commit_child VALUES(1)")
		return err
	}); err == nil {
		t.Fatal("commit failure not returned")
	}
	var count int
	if err := s.reader.QueryRow("SELECT count(*) FROM commit_child").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed commit left data: %d %v", count, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Write(ctx, func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE system_state SET config_version=config_version+1")
				return err
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := s.reader.QueryRow("SELECT config_version FROM system_state").Scan(&count); err != nil || count != 21 {
		t.Fatalf("lost write: %d %v", count, err)
	}
}

func TestFreshMigrationFailureIsAtomic(t *testing.T) {
	db, err := openPool(filepath.Join(t.TempDir(), "fresh.sqlite"), Config{BusyTimeout: time.Second}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := fstest.MapFS{"0001_broken.sql": &fstest.MapFile{Data: []byte("CREATE TABLE partial(id); INVALID SQL;")}}
	if err := migrate(context.Background(), db, source); err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name IN ('partial','schema_migrations')").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed first migration left schema: %d %v", count, err)
	}
	if err := migrate(context.Background(), db, migrations.Files); err != nil {
		t.Fatal(err)
	}
}

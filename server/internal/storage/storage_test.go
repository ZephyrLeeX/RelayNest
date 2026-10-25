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
	if err := reopened.reader.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != len(migrationFixture(t)) {
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
			if err := s.reader.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != len(migrationFixture(t)) {
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
		"future":   fmt.Sprintf("INSERT INTO schema_migrations VALUES(%d,'%04d_future.sql','unknown','now')", len(migrationFixture(t))+1, len(migrationFixture(t))+1),
		"gap":      fmt.Sprintf("UPDATE schema_migrations SET version=%d WHERE version=1", len(migrationFixture(t))+1),
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
	t.Run("real administrator upgrade", func(t *testing.T) {
		ctx := context.Background()
		db, err := openPool(filepath.Join(t.TempDir(), "old.sqlite"), Config{BusyTimeout: time.Second}, false)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		first, err := migrations.Files.ReadFile("0001_initial.sql")
		if err != nil {
			t.Fatal(err)
		}
		old := fstest.MapFS{"0001_initial.sql": &fstest.MapFile{Data: first}}
		if err := migrate(ctx, db, old); err != nil {
			t.Fatal(err)
		}
		var epoch, after string
		if err := db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
			t.Fatal(err)
		}
		if err := migrate(ctx, db, migrations.Files); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&after); err != nil || epoch != after {
			t.Fatal("upgrade lost system state", err)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM administrator").Scan(&count); err != nil || count != 0 {
			t.Fatal("administrator upgrade failed", err)
		}
		if err := migrate(ctx, db, old); err == nil {
			t.Fatal("old binary accepted administrator schema")
		}
	})
	t.Run("real session upgrade", func(t *testing.T) {
		ctx := context.Background()
		db, err := openPool(filepath.Join(t.TempDir(), "admin.sqlite"), Config{BusyTimeout: time.Second}, false)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		old := fstest.MapFS{}
		for _, name := range []string{"0001_initial.sql", "0002_admin.sql"} {
			data, err := migrations.Files.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			old[name] = &fstest.MapFile{Data: data}
		}
		if err := migrate(ctx, db, old); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO administrator VALUES(1,'original','fixture',7)"); err != nil {
			t.Fatal(err)
		}
		if err := migrate(ctx, db, migrations.Files); err != nil {
			t.Fatal(err)
		}
		var username string
		var generation, count int
		if err := db.QueryRow("SELECT username,auth_generation FROM administrator WHERE id=1").Scan(&username, &generation); err != nil || username != "original" || generation != 7 {
			t.Fatal("session upgrade changed administrator", err)
		}
		if err := db.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil || count != 0 {
			t.Fatal("session upgrade failed", err)
		}
		if err := migrate(ctx, db, old); err == nil {
			t.Fatal("old binary accepted session schema")
		}
	})
	t.Run("real bot upgrade", func(t *testing.T) {
		db, err := openPool(filepath.Join(t.TempDir(), "bots-upgrade.sqlite"), Config{BusyTimeout: time.Second}, false)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		old := migrationFixtureThrough(t, 3)
		if err := migrate(context.Background(), db, old); err != nil {
			t.Fatal(err)
		}
		var epoch, after string
		if err := db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO administrator VALUES(1,'original','fixture',7)"); err != nil {
			t.Fatal(err)
		}
		if err := migrate(context.Background(), db, migrations.Files); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&after); err != nil || after != epoch {
			t.Fatal("bot upgrade changed epoch", err)
		}
		var generation, count int
		if err := db.QueryRow("SELECT auth_generation FROM administrator WHERE id=1").Scan(&generation); err != nil || generation != 7 {
			t.Fatal("bot upgrade changed administrator", err)
		}
		for _, table := range []string{"bots", "bot_credentials"} {
			if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
				t.Fatal("bot upgrade failed", table, err)
			}
		}
		if err := migrate(context.Background(), db, migrations.Files); err != nil {
			t.Fatal(err)
		}
		if err := migrate(context.Background(), db, old); err == nil {
			t.Fatal("old binary accepted bot schema")
		}
	})
	s, cfg := openTest(t)
	ctx := context.Background()
	source := migrationFixture(t)
	baseVersion := len(source)
	upgradeName := fmt.Sprintf("%04d_upgrade.sql", baseVersion+1)
	failureName := fmt.Sprintf("%04d_failure.sql", baseVersion+2)
	source[upgradeName] = &fstest.MapFile{Data: []byte("CREATE TABLE upgraded(id INTEGER PRIMARY KEY); INSERT INTO upgraded VALUES(9);")}
	source[failureName] = &fstest.MapFile{Data: []byte("CREATE TABLE partial(id); INSERT INTO missing VALUES(1);")}
	if err := migrate(ctx, s.writer, source); err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := s.reader.QueryRow("SELECT count(*) FROM sqlite_master WHERE name IN ('partial','upgraded')").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial upgrade remained: %d %v", count, err)
	}
	if err := s.reader.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&count); err != nil || count != len(migrationFixture(t)) {
		t.Fatalf("failed upgrade changed ledger: %d %v", count, err)
	}
	delete(source, failureName)
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
	if err := upgraded.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&count); err != nil || count != baseVersion+1 {
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

// migrationFixture 使用全部真实迁移，人工升级始终取下一编号，避免新增正式迁移撞号。
func migrationFixture(t *testing.T) fstest.MapFS {
	t.Helper()
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	source := fstest.MapFS{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		source[entry.Name()] = &fstest.MapFile{Data: data}
	}
	return source
}

// 历史起点只保留目标版本前缀；全量 fixture 始终包含所有真实迁移。
func migrationFixtureThrough(t *testing.T, version int) fstest.MapFS {
	t.Helper()
	source := migrationFixture(t)
	for name := range source {
		if name >= fmt.Sprintf("%04d_", version+1) {
			delete(source, name)
		}
	}
	return source
}

// 消息升级只新增结构；已有身份、epoch 与认证/Bot 数据必须逐值保留。
func TestMessagesUpgradeAndFailureBoundaries(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"upgrade", "ddl failure", "missing identity", "corrupt identity"} {
		t.Run(mode, func(t *testing.T) {
			cfg := Config{Path: filepath.Join(t.TempDir(), "instance-v5.sqlite"), BusyTimeout: time.Second}
			db, err := openPool(cfg.Path, cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			old := migrationFixtureThrough(t, 5)
			if err := migrate(ctx, db, old); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO administrator VALUES(1,'original','fixture',7);
                INSERT INTO sessions VALUES('session',zeroblob(32),7,'android','device','name',100,200,0);
                INSERT INTO bots VALUES('bot','original bot',1,1,1,1,31);`); err != nil {
				t.Fatal(err)
			}
			// 保存旧业务表与台账的全部列，不只比较行数。
			snapshot := func(db *sql.DB) string {
				t.Helper()
				var result string
				for _, table := range []string{"system_state", "instance_identity", "administrator", "sessions", "bots", "schema_migrations"} {
					rows, err := db.Query("SELECT * FROM " + table + " ORDER BY 1")
					if err != nil {
						t.Fatal(err)
					}
					columns, err := rows.Columns()
					if err != nil {
						t.Fatal(err)
					}
					for rows.Next() {
						values := make([]any, len(columns))
						targets := make([]any, len(columns))
						for i := range values {
							targets[i] = &values[i]
						}
						if err := rows.Scan(targets...); err != nil {
							t.Fatal(err)
						}
						if table == "schema_migrations" && values[0].(int64) > 5 {
							continue
						}
						result += fmt.Sprintf("%s:%#v\n", table, values)
					}
					if err := rows.Err(); err != nil {
						t.Fatal(err)
					}
					rows.Close()
				}
				return result
			}
			before := snapshot(db)
			switch mode {
			case "ddl failure":
				_, err = db.Exec("CREATE TABLE push_jobs(original INTEGER)")
			case "missing identity":
				_, err = db.Exec("DELETE FROM instance_identity")
			case "corrupt identity":
				_, err = db.Exec("PRAGMA ignore_check_constraints=ON; UPDATE instance_identity SET instance_id='corrupt'; PRAGMA ignore_check_constraints=OFF")
			}
			if err != nil {
				t.Fatal(err)
			}
			damaged := snapshot(db)
			err = migrate(ctx, db, migrations.Files)
			if mode != "upgrade" {
				if err == nil {
					t.Fatal("invalid upgrade accepted")
				}
				var count int
				if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name IN ('messages','deliveries','messages_created_at','sessions_id_device')").Scan(&count); err != nil || count != 0 {
					t.Fatal("partial message schema", count, err)
				}
				if err := db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&count); err != nil || count != 5 {
					t.Fatal("failed upgrade changed ledger", count, err)
				}
				if after := snapshot(db); after != damaged {
					t.Fatal("failed upgrade changed existing data/identity/ledger")
				}
				db.Close()
				if opened, err := Open(ctx, cfg); err == nil {
					opened.Close()
					t.Fatal("Open accepted invalid upgrade")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// 快照仅比较原五版台账；完整升级目标另逐项校验，不能隐藏后续真实迁移。
			db.Close()
			for i := 0; i < 2; i++ {
				s, err := Open(ctx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				after := snapshot(s.reader)
				if after != before {
					t.Fatal("upgrade/reopen changed existing data")
				}
				var count int
				if err := s.reader.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=6 AND name='0006_messages.sql'").Scan(&count); err != nil || count != 1 {
					t.Fatal("missing message migration", err)
				}
				all, err := loadMigrations(migrations.Files)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.reader.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != len(all) {
					t.Fatal("incomplete latest migration ledger", count, err)
				}
				for index, m := range all {
					if err := s.reader.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=? AND name=? AND checksum=?", index+1, m.name, m.checksum).Scan(&count); err != nil || count != 1 {
						t.Fatal("latest migration name/checksum mismatch", m.name, err)
					}
				}
				if err := s.reader.QueryRow("SELECT count(*) FROM idempotency").Scan(&count); err != nil || count != 0 {
					t.Fatal("missing or nonempty idempotency table", count, err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			checkDB, err := openPool(cfg.Path, cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			defer checkDB.Close()
			if err := migrate(ctx, checkDB, old); err == nil || !strings.Contains(err.Error(), "incompatible migration version 6") {
				t.Fatal("v5 binary did not reject message schema version", err)
			}
		})
	}
}

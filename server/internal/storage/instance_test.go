package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"relaynest/server/migrations"
)

func readIdentity(t *testing.T, s *Store) string {
	t.Helper()
	var id string
	if err := s.Read(context.Background(), func(tx *sql.Tx) error {
		var err error
		id, err = ReadInstanceIDTx(context.Background(), tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestInstancePersistenceUpgradeAndMove(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Path: filepath.Join(t.TempDir(), "old.sqlite"), BusyTimeout: time.Second}
	db, err := openPool(cfg.Path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	source := migrationFixture(t)
	delete(source, instanceMigration)
	if err := migrate(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	var epoch, ledger string
	if err := db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	query := "SELECT group_concat(version || ':' || name || ':' || checksum || ':' || applied_at, '|') FROM (SELECT * FROM schema_migrations WHERE version <= 4 ORDER BY version)"
	if err := db.QueryRow(query).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	id := readIdentity(t, s)
	if !instanceIDPattern.MatchString(id) || id == epoch {
		t.Fatal("invalid or reused identity", id)
	}
	var after string
	if err := s.reader.QueryRow(query).Scan(&after); err != nil || after != ledger {
		t.Fatal("old ledger changed", err)
	}
	if err := s.reader.QueryRow("SELECT sync_epoch FROM system_state").Scan(&after); err != nil || after != epoch {
		t.Fatal("epoch changed", err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		same, err := InitInstanceIDTx(ctx, tx)
		if same != id {
			t.Fatal("init replaced committed identity")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved.sqlite")
	// 所有连接关闭后移动原库，模拟无回滚停机迁移。
	if err := os.Rename(cfg.Path, moved); err != nil {
		t.Fatal(err)
	}
	cfg.Path = moved
	s, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if readIdentity(t, s) != id {
		t.Fatal("restart/move changed identity")
	}
}

func TestInstanceDamageFailsClosed(t *testing.T) {
	cases := map[string]string{
		"missing":   "DELETE FROM instance_identity",
		"empty":     "PRAGMA ignore_check_constraints=ON; UPDATE instance_identity SET instance_id=''",
		"corrupt":   "PRAGMA ignore_check_constraints=ON; UPDATE instance_identity SET instance_id='00000000-0000-0000-0000-000000000000'",
		"table":     "DROP TABLE instance_identity",
		"duplicate": "ALTER TABLE instance_identity RENAME TO original_identity; CREATE TABLE instance_identity(id,instance_id); INSERT INTO instance_identity SELECT * FROM original_identity; INSERT INTO instance_identity SELECT * FROM original_identity",
		"null":      "ALTER TABLE instance_identity RENAME TO original_identity; CREATE TABLE instance_identity(id,instance_id); INSERT INTO instance_identity VALUES(1,NULL)",
	}
	for name, mutation := range cases {
		t.Run(name, func(t *testing.T) {
			s, cfg := openTest(t)
			if _, err := s.writer.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			called := false
			fn := func(*sql.Tx) error { called = true; return nil }
			if err := s.Read(context.Background(), fn); !errors.Is(err, ErrInstanceUnavailable) {
				t.Fatal("read accepted damage", err)
			}
			if err := s.Write(context.Background(), fn); !errors.Is(err, ErrInstanceUnavailable) {
				t.Fatal("write accepted damage", err)
			}
			if called {
				t.Fatal("callback ran after damage")
			}
			tx, err := s.writer.Begin()
			if err != nil {
				t.Fatal(err)
			}
			id, err := InitInstanceIDTx(context.Background(), tx)
			tx.Rollback()
			if id != "" || !errors.Is(err, ErrInstanceUnavailable) {
				t.Fatal("init repaired damage", id, err)
			}
			s.Close()
			reopened, err := Open(context.Background(), cfg)
			if reopened != nil {
				reopened.Close()
				t.Fatal("damaged store reopened")
			}
			if !errors.Is(err, ErrInstanceUnavailable) {
				t.Fatal(err)
			}
		})
	}
}

type failingIdentityRandom struct{}

func (failingIdentityRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestInstanceRandomFailureAtomic(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Path: filepath.Join(t.TempDir(), "random.sqlite"), BusyTimeout: time.Second}
	original := rand.Reader
	defer func() { rand.Reader = original }()
	rand.Reader = failingIdentityRandom{}
	s, err := Open(ctx, cfg)
	rand.Reader = original
	if s != nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("random failure accepted", err)
	}
	db, err := openPool(cfg.Path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed init left DDL/ledger", count, err)
	}
	s, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old := readIdentity(t, s)
	rand.Reader = failingIdentityRandom{}
	err = s.Write(ctx, func(tx *sql.Tx) error {
		id, err := RotateInstanceIDTx(ctx, tx, old)
		if id != "" {
			t.Fatal("random failure returned candidate")
		}
		return err
	})
	rand.Reader = original
	if !errors.Is(err, io.ErrUnexpectedEOF) || readIdentity(t, s) != old {
		t.Fatal("rotate random failure mutated identity", err)
	}
}

func TestInstanceRotateTransaction(t *testing.T) {
	ctx := context.Background()
	s, cfg := openTest(t)
	old := readIdentity(t, s)
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		id, err := RotateInstanceIDTx(ctx, tx, "wrong")
		if id != "" || !errors.Is(err, ErrInstanceMismatch) {
			t.Fatal("expected mismatch", id, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	abort := errors.New("abort maintenance")
	var candidate string
	err := s.Write(ctx, func(tx *sql.Tx) error {
		var err error
		candidate, err = RotateInstanceIDTx(ctx, tx, old)
		if err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) || candidate == old || !instanceIDPattern.MatchString(candidate) || readIdentity(t, s) != old {
		t.Fatal("rollback failed", err)
	}
	if _, err := s.writer.Exec("CREATE TRIGGER reject_identity BEFORE UPDATE ON instance_identity BEGIN SELECT RAISE(ABORT, 'injected failure'); END"); err != nil {
		t.Fatal(err)
	}
	err = s.Write(ctx, func(tx *sql.Tx) error {
		id, err := RotateInstanceIDTx(ctx, tx, old)
		if id != "" {
			t.Fatal("update failure returned candidate")
		}
		return err
	})
	if !errors.Is(err, ErrInstanceUnavailable) || readIdentity(t, s) != old {
		t.Fatal("update failure mutated identity", err)
	}
	s.writer.Exec("DROP TRIGGER reject_identity")
	err = s.Write(ctx, func(tx *sql.Tx) error {
		var err error
		candidate, err = RotateInstanceIDTx(ctx, tx, old)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if err := s.Read(ctx, func(*sql.Tx) error { called = true; return nil }); !errors.Is(err, ErrInstanceUnavailable) || called {
		t.Fatal("cached read accepted change", err)
	}
	if err := s.Write(ctx, func(*sql.Tx) error { called = true; return nil }); !errors.Is(err, ErrInstanceUnavailable) || called {
		t.Fatal("cached write accepted change", err)
	}
	s.Close()
	reopened, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if readIdentity(t, reopened) != candidate {
		t.Fatal("committed rotation not persisted")
	}
}

func TestInstanceMigrationCommitAndLockFailures(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Path: filepath.Join(t.TempDir(), "failure.sqlite"), BusyTimeout: 20 * time.Millisecond}
	db, err := openPool(cfg.Path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	held, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	competing, err := openPool(cfg.Path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer competing.Close()
	if err := migrate(ctx, competing, migrations.Files); err == nil || !strings.Contains(err.Error(), "migration lock") {
		t.Fatal("lock failure ignored", err)
	}
	held.Rollback()
	source := migrationFixture(t)
	source["0006_commit_failure.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE parent(id PRIMARY KEY); CREATE TABLE child(id REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED); INSERT INTO child VALUES(1);")}
	if err := migrate(ctx, db, source); err == nil || !strings.Contains(err.Error(), "migration commit") {
		t.Fatal("expected real deferred FK commit failure", err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&count); err != nil || count != 0 {
		t.Fatal("commit failure left DDL/identity/ledger", count, err)
	}
	if err := migrate(ctx, db, migrations.Files); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceSubprocess(t *testing.T) {
	mode := os.Getenv("RELAYNEST_INSTANCE_TEST_MODE")
	if mode == "" {
		return
	}
	path := os.Getenv("RELAYNEST_INSTANCE_TEST_DB")
	ctx := context.Background()
	cfg := Config{Path: path, BusyTimeout: 5 * time.Second}
	if mode == "open" {
		s, err := Open(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		id := readIdentity(t, s)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("RELAYNEST_INSTANCE_TEST_RESULT"), []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	// 子进程在同一真实写事务中完成 DDL/身份/台账写入后直接退出，不执行清理。
	db, err := openPool(path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := enableWAL(ctx, db, cfg.BusyTimeout); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	all, err := loadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT,checksum TEXT,applied_at TEXT)"); err != nil {
		t.Fatal(err)
	}
	for i, m := range all {
		if _, err := tx.Exec(m.sql); err != nil {
			t.Fatal(err)
		}
		if m.name == instanceMigration {
			if _, err := InitInstanceIDTx(ctx, tx); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations VALUES(?,?,?,'test')", i+1, m.name, m.checksum); err != nil {
			t.Fatal(err)
		}
	}
	os.Exit(0)
}

func instanceProcess(t *testing.T, mode, path, result string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestInstanceSubprocess$", "-test.timeout=20s")
	cmd.Env = append(os.Environ(), "RELAYNEST_INSTANCE_TEST_MODE="+mode, "RELAYNEST_INSTANCE_TEST_DB="+path, "RELAYNEST_INSTANCE_TEST_RESULT="+result)
	return cmd
}

func TestInstanceConcurrentProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.sqlite")
	var commands []*exec.Cmd
	for i := 0; i < 2; i++ {
		cmd := instanceProcess(t, "open", path, filepath.Join(dir, fmt.Sprint(i)))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	a, err := os.ReadFile(filepath.Join(dir, "0"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "1"))
	if err != nil || string(a) != string(b) || !instanceIDPattern.Match(a) {
		t.Fatal("processes published different identity", string(a), string(b), err)
	}
}

func TestInstanceCrashRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash.sqlite")
	if out, err := instanceProcess(t, "crash", path, "").CombinedOutput(); err != nil {
		t.Fatalf("crash helper: %v %s", err, out)
	}
	cfg := Config{Path: path, BusyTimeout: time.Second}
	db, err := openPool(path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&count); err != nil || count != 0 {
		t.Fatal("process exit left partial transaction", count, err)
	}
	db.Close()
	s, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	readIdentity(t, s)
}

func TestInstanceConstraintsAndReadError(t *testing.T) {
	s, _ := openTest(t)
	id := readIdentity(t, s)
	for _, statement := range []string{
		"INSERT INTO instance_identity VALUES(2,'" + id + "')",
		"INSERT INTO instance_identity VALUES(1,'" + id + "')",
		"UPDATE instance_identity SET instance_id='00000000-0000-0000-0000-000000000000'",
		"UPDATE instance_identity SET instance_id='" + strings.ToUpper("aaaaaaaa"+id[8:]) + "'",
	} {
		if _, err := s.writer.Exec(statement); err == nil {
			t.Fatal("schema accepted invalid identity", statement)
		}
	}
	tx, err := s.writer.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := ReadInstanceIDTx(canceled, tx); got != "" || !errors.Is(err, ErrInstanceUnavailable) || !errors.Is(err, context.Canceled) {
		t.Fatal("read failure swallowed", got, err)
	}
}

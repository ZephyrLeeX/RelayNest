package sync_test

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

func TestReadUpgradeNineMigrations(t *testing.T) {
	cfg := storage.Config{Path: filepath.Join(t.TempDir(), "old.sqlite")}
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") || name >= "0010" {
			continue
		}
		data, err := fs.ReadFile(migrations.Files, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(string(data)); err != nil {
			t.Fatal(name, err)
		}
		if strings.HasPrefix(name, "0005_") {
			if _, err = storage.InitInstanceIDTx(ctx, tx); err != nil {
				t.Fatal(err)
			}
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		checks[name] = sum
		if _, err = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?,'old')`, len(checks), name, sum); err != nil {
			t.Fatal(err)
		}
	}
	if len(checks) != 9 {
		t.Fatal("not nine migration fixture", len(checks))
	}
	for _, id := range []string{"old-a", "old-b"} {
		if _, err = tx.Exec(`INSERT INTO bots(id,name,enabled,can_send,can_receive,version) VALUES(?,?,0,1,1,1)`, id, id); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if scalar(t, s, `SELECT count(*) FROM schema_migrations`) != 12 {
		t.Fatal("migration count")
	}
	for name, sum := range checks {
		var got string
		if err := s.Read(ctx, func(tx *sql.Tx) error {
			return tx.QueryRow(`SELECT checksum FROM schema_migrations WHERE name=?`, name).Scan(&got)
		}); err != nil || got != sum {
			t.Fatal("checksum changed", name, err)
		}
	}
	before := get(t, s, "old-a")
	other := get(t, s, "old-b")
	if before.ReadSeq != 0 || before.StateVersion != 1 || before.UnreadCount != 0 || before.UpdatedAt.IsZero() || before.UpdatedAt.Location() != time.UTC || other.UpdatedAt != before.UpdatedAt {
		t.Fatal("old initialization", before, other)
	}
	if scalar(t, s, `SELECT count(*) FROM changes`) != 0 {
		t.Fatal("migration emitted changes")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if get(t, s, "old-a") != before {
		t.Fatal("reopen reset old read")
	}
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		rows, err := tx.Query(`PRAGMA foreign_key_check`)
		if err != nil {
			return err
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatal("foreign key violation")
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	execSQL(t, s, `DELETE FROM bots WHERE id='old-a'`)
	if scalar(t, s, `SELECT count(*) FROM bot_read_states`) != 1 {
		t.Fatal("cascade")
	}
}

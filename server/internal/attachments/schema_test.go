package attachments

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

func TestUpgradeSevenMigrationsPreservesChecksums(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.sqlite")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	_, e = tx.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at TEXT NOT NULL)`)
	if e != nil {
		t.Fatal(e)
	}
	entries, e := fs.ReadDir(migrations.Files, ".")
	if e != nil {
		t.Fatal(e)
	}
	expected := map[int]string{}
	n := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".sql" || entry.Name() >= "0008" {
			continue
		}
		n++
		b, e := fs.ReadFile(migrations.Files, entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(string(b)); e != nil {
			t.Fatal(e)
		}
		if n == 5 {
			if _, e = storage.InitInstanceIDTx(ctx, tx); e != nil {
				t.Fatal(e)
			}
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(b))
		expected[n] = sum
		if _, e = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?,'old')`, n, entry.Name(), sum); e != nil {
			t.Fatal(e)
		}
	}
	if n != 7 {
		t.Fatal(n)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	db.Close()
	s, e := storage.Open(ctx, storage.Config{Path: path})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	e = s.Read(ctx, func(tx *sql.Tx) error {
		rows, e := tx.Query(`SELECT version,checksum FROM schema_migrations ORDER BY version`)
		if e != nil {
			return e
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var v int
			var checksum string
			if e = rows.Scan(&v, &checksum); e != nil {
				return e
			}
			if v <= 7 && checksum != expected[v] {
				t.Fatalf("changed checksum %d", v)
			}
			count++
		}
		if count != 9 {
			t.Fatal(count)
		}
		return rows.Err()
	})
	if e != nil {
		t.Fatal(e)
	}
}

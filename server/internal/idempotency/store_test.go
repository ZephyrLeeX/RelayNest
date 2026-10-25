package idempotency

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

func TestUUIDv4Key(t *testing.T) {
	valid := "abcdefff-1234-4567-89ab-123456789abc"
	for _, key := range []string{valid, strings.ToUpper(valid)} {
		if !ValidKey(key) {
			t.Fatal("valid key rejected")
		}
	}
	for _, key := range []string{"", valid + "x", strings.Replace(valid, "4567", "3567", 1), strings.Replace(valid, "89ab", "79ab", 1), strings.Replace(valid, "abcdefff", "xbcdefff", 1)} {
		if ValidKey(key) {
			t.Fatal("invalid key accepted")
		}
	}
}

// 以旧 SQL 的真实字节构造版本6，再通过正式 Open 路径升级；不修改旧 SQL/checksum。
func TestRealVersionSixUpgrade(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v6.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`PRAGMA foreign_keys=ON;CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for index, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() > "0006_messages.sql" {
			continue
		}
		data, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(string(data)); err != nil {
			t.Fatal(err)
		}
		if entry.Name() == "0005_instance_identity.sql" {
			if _, err = storage.InitInstanceIDTx(ctx, tx); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = tx.Exec("INSERT INTO schema_migrations VALUES(?,?,?,'original')", index+1, entry.Name(), fmt.Sprintf("%x", sha256.Sum256(data))); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var result string
		if err := db.QueryRow("SELECT group_concat(version||name||checksum||applied_at,'|') FROM (SELECT * FROM schema_migrations WHERE version<=6 ORDER BY version)").Scan(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot()
	s, err := storage.Open(ctx, storage.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if before != snapshot() {
		t.Fatal("upgrade changed old checksums/ledger")
	}
	var n int
	if err = db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=7 AND name='0007_idempotency.sql'").Scan(&n); err != nil || n != 1 {
		t.Fatal("missing real version7", err)
	}
	if err = db.QueryRow("SELECT count(*) FROM idempotency").Scan(&n); err != nil || n != 0 {
		t.Fatal("upgrade created records", err)
	}
	if _, err = db.Exec("UPDATE schema_migrations SET checksum='bad' WHERE version=6"); err != nil {
		t.Fatal(err)
	}
	if opened, err := storage.Open(ctx, storage.Config{Path: path}); err == nil {
		opened.Close()
		t.Fatal("corrupt old checksum accepted")
	}
}

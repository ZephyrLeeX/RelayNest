package dropstorage_test

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

func oldDatabase(t *testing.T, version int) (string, *sql.DB, map[int]string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.sqlite")
	db, e := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL,checksum TEXT NOT NULL,applied_at TEXT NOT NULL)`); e != nil {
		t.Fatal(e)
	}
	entries, e := fs.ReadDir(migrations.Files, ".")
	if e != nil {
		t.Fatal(e)
	}
	checks := map[int]string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var n int
		if _, e := fmt.Sscanf(entry.Name(), "%d_", &n); e != nil {
			t.Fatal(e)
		}
		if n > version {
			continue
		}
		data, e := migrations.Files.ReadFile(entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(string(data)); e != nil {
			t.Fatal(e)
		}
		if n == 5 {
			if _, e = storage.InitInstanceIDTx(ctx, tx); e != nil {
				t.Fatal(e)
			}
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(data))
		checks[n] = checksum
		if _, e = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?,'old-applied-time')`, n, entry.Name(), checksum); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	return path, db, checks
}
func foreignKeys(t *testing.T, s *storage.Store) {
	t.Helper()
	if e := s.Read(ctx, func(tx *sql.Tx) error {
		var enabled int
		if e := tx.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); e != nil {
			return e
		}
		if enabled != 1 {
			return fmt.Errorf("FK disabled")
		}
		rows, e := tx.Query(`PRAGMA foreign_key_check`)
		if e != nil {
			return e
		}
		defer rows.Close()
		if rows.Next() {
			return fmt.Errorf("foreign key violation")
		}
		return rows.Err()
	}); e != nil {
		t.Fatal(e)
	}
}
func TestEveryHistoricalLedgerUpgrade(t *testing.T) {
	for version := 1; version <= 13; version++ {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			path, _, checks := oldDatabase(t, version)
			s, e := storage.Open(ctx, storage.Config{Path: path})
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			foreignKeys(t, s)
			if e := s.Read(ctx, func(tx *sql.Tx) error {
				rows, e := tx.Query(`SELECT version,checksum,applied_at FROM schema_migrations ORDER BY version`)
				if e != nil {
					return e
				}
				defer rows.Close()
				n := 0
				for rows.Next() {
					n++
					var v int
					var checksum, applied string
					if e := rows.Scan(&v, &checksum, &applied); e != nil {
						return e
					}
					if v <= version && (checksum != checks[v] || applied != "old-applied-time") {
						return fmt.Errorf("old ledger changed: %d", v)
					}
				}
				if n != 14 {
					return fmt.Errorf("ledger count=%d", n)
				}
				return rows.Err()
			}); e != nil {
				t.Fatal(e)
			}
		})
	}
}

// 使用真实公开 API 产生 accepted/degraded、退避、在途 attempt 和原 alert，再逐列转存到真实 0013 库。
func mixedRows(t *testing.T) (*fixture, map[string][]any) {
	t.Helper()
	f := newFixture(t, "fcm", true)
	r := push.NewJobRepository(f.s)
	if ok, e := r.Release(ctx, f.held.Lease, f.now); e != nil || !ok {
		t.Fatal(e)
	}
	rows := map[string][]any{f.j.ID: row(t, f.s, f.j.ID)}
	for _, outcome := range []push.SyncOutcome{push.SyncAccepted, push.SyncConfigurationError, push.SyncTransient, ""} {
		j := f.j
		j.ID = "sync-" + string(outcome)
		j.Purpose = "sync"
		j.AlertEligible = false
		if e := f.s.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, f.now) }); e != nil {
			t.Fatal(e)
		}
		held, e := r.ReserveSync(ctx, f.now, time.Minute, 1)
		if e != nil || len(held) != 1 || held[0].Job.Snapshot.ID != j.ID {
			t.Fatal("sync reservation", e)
		}
		ticket, e := r.BeginSync(ctx, held[0].Lease, f.now)
		if e != nil || ticket == nil {
			t.Fatal("sync begin", e)
		}
		if outcome != "" {
			if ok, e := r.ApplySyncResult(ctx, *ticket, outcome, f.now); e != nil || !ok {
				t.Fatal("sync result", e)
			}
		}
		rows[j.ID] = row(t, f.s, j.ID)
	}
	write(t, f.s, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy',?,?,99,'legacy-time')`, f.j.SessionID, f.j.DeviceID)
	rows["legacy"] = row(t, f.s, "legacy")
	return f, rows
}
func seedOldRows(t *testing.T, db *sql.DB, f *fixture, rows map[string][]any) {
	t.Helper()
	_, e := db.Exec(`INSERT INTO sessions VALUES(?,zeroblob(32),1,'android',?,'fixture',?,?,0)`, f.j.SessionID, f.j.DeviceID, f.now.Add(24*time.Hour).UnixNano(), f.now.Add(48*time.Hour).UnixNano())
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range rows {
		q := `INSERT INTO push_jobs VALUES(` + strings.TrimSuffix(strings.Repeat("?,", len(v)), ",") + `)`
		if _, e := db.Exec(q, v...); e != nil {
			t.Fatal(e)
		}
	}
}
func TestMigrationPreservesAll26ColumnsAndConstraints(t *testing.T) {
	f, rows := mixedRows(t)
	path, db, _ := oldDatabase(t, 13)
	seedOldRows(t, db, f, rows)
	s, e := storage.Open(ctx, storage.Config{Path: path})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for id, v := range rows {
		if !reflect.DeepEqual(v, row(t, s, id)) {
			t.Fatalf("26 columns changed for %s", id)
		}
	}
	foreignKeys(t, s)
	if e := s.Read(ctx, func(tx *sql.Tx) error {
		expected := map[string]string{"push_jobs_pending": "index", "push_jobs_session": "index", "push_jobs_device_lease": "index", "push_jobs_one_lease": "index", "push_jobs_snapshot_immutable": "trigger", "push_jobs_suppressed_terminal": "trigger"}
		for name, kind := range expected {
			var n int
			if e := tx.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name=? AND type=?`, name, kind).Scan(&n); e != nil {
				return e
			}
			if n != 1 {
				return fmt.Errorf("missing %s", name)
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	// 新增状态仅 alert，原 leased 唯一约束与复合 FK、快照冻结都继续实际拒绝。
	for _, q := range []string{
		`UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL WHERE id='sync-'`,
		`UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=1 WHERE id='alert-original'`,
		`UPDATE push_jobs SET message_id='other' WHERE id='alert-original'`,
		`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('bad-fk','missing','other',1,'legacy')`,
	} {
		if e := s.Write(ctx, func(tx *sql.Tx) error { _, e := tx.Exec(q); return e }); e == nil {
			t.Fatal("constraint accepted", q)
		}
	}
	// terminal约束由新API持久测试覆盖；这里核 legacy五列writer与级联。
	write(t, s, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy-new',?,?,1,'old-time')`, f.j.SessionID, f.j.DeviceID)
	write(t, s, `DELETE FROM sessions WHERE id=?`, f.j.SessionID)
	if e := s.Read(ctx, func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRow(`SELECT count(*) FROM push_jobs`).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			return fmt.Errorf("cascade lost")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	foreignKeys(t, s)
}
func TestMigrationFailureIsAtomic(t *testing.T) {
	f, rows := mixedRows(t)
	for _, fault := range []string{"collision", "ledger-abort", "ledger-rollback"} {
		t.Run(fault, func(t *testing.T) {
			path, db, _ := oldDatabase(t, 13)
			seedOldRows(t, db, f, rows)
			q := `CREATE TABLE push_jobs_alert_suppression(id TEXT)`
			if fault == "ledger-abort" {
				q = `CREATE TRIGGER migration_fault BEFORE INSERT ON schema_migrations WHEN NEW.version=14 BEGIN SELECT RAISE(ABORT,'ledger-fault');END`
			}
			if fault == "ledger-rollback" {
				q = `CREATE TRIGGER migration_fault AFTER INSERT ON schema_migrations WHEN NEW.version=14 BEGIN SELECT RAISE(ROLLBACK,'ledger-fault');END`
			}
			if _, e := db.Exec(q); e != nil {
				t.Fatal(e)
			}
			if s, e := storage.Open(ctx, storage.Config{Path: path}); e == nil {
				s.Close()
				t.Fatal("bad migration succeeded")
			}
			var n int
			if e := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); e != nil || n != 13 {
				t.Fatal("ledger not atomic", n, e)
			}
			// 原表和所有26列必须恢复；不能只检查台账条数。
			for id, want := range rows {
				r := db.QueryRow(`SELECT * FROM push_jobs WHERE id=?`, id)
				got := make([]any, 26)
				dest := make([]any, 26)
				for i := range got {
					dest[i] = &got[i]
				}
				if e := r.Scan(dest...); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal("partial row migration", id)
				}
			}
			if fault == "collision" {
				_, _ = db.Exec(`DROP TABLE push_jobs_alert_suppression`)
			} else {
				_, _ = db.Exec(`DROP TRIGGER migration_fault`)
			}
			s, e := storage.Open(ctx, storage.Config{Path: path})
			if e != nil {
				t.Fatal("recovery", e)
			}
			defer s.Close()
			foreignKeys(t, s)
		})
	}
}

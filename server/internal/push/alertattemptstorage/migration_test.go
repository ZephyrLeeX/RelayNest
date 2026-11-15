package alertattemptstorage_test

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"relaynest/server/internal/push"
	"strings"
	"testing"
	"time"

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
	for v := 1; v <= 14; v++ {
		t.Run(fmt.Sprint(v), func(t *testing.T) {
			path, _, checks := oldDatabase(t, v)
			s, e := storage.Open(ctx, storage.Config{Path: path})
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			foreignKeys(t, s)
			if e = s.Read(ctx, func(tx *sql.Tx) error {
				var n int
				if e := tx.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); e != nil {
					return e
				}
				if n != 15 {
					return fmt.Errorf("ledger count %d", n)
				}
				for version, checksum := range checks {
					var c, a string
					if e := tx.QueryRow(`SELECT checksum,applied_at FROM schema_migrations WHERE version=?`, version).Scan(&c, &a); e != nil {
						return e
					}
					if c != checksum || a != "old-applied-time" {
						return fmt.Errorf("old ledger changed %d", version)
					}
				}
				return nil
			}); e != nil {
				t.Fatal(e)
			}
		})
	}
}

// 将真实 producer/租约/sync 结果与 legacy 混合逐值转存到 0014 库，再升级 0015。
func TestMixedMigrationAtomicityAndReopen(t *testing.T) {
	f, h, n := prepared(t)
	r := push.NewJobRepository(f.store)
	if ok, e := r.Release(ctx, h.Lease, n); e != nil || !ok {
		t.Fatal(e)
	}
	exec(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic',?)`, f.session.ID, n.Format(stamp))
	for i, out := range []push.SyncOutcome{push.SyncAccepted, push.SyncConfigurationError, push.SyncTransient, ""} {
		j := h.Job.Snapshot
		j.ID = fmt.Sprint("sync-", i)
		j.Purpose = "sync"
		j.Provider = "fcm"
		j.AlertEligible = false
		if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
			t.Fatal(e)
		}
		held, e := r.ReserveSync(ctx, n, time.Minute, 1)
		if e != nil || len(held) != 1 {
			t.Fatal(e)
		}
		ticket, e := r.BeginSync(ctx, held[0].Lease, n)
		if e != nil || ticket == nil {
			t.Fatal(e)
		}
		if out == "" {
			if ok, e := r.Release(ctx, held[0].Lease, n); e != nil || !ok {
				t.Fatal(e)
			}
		}
		if out != "" {
			if ok, e := r.ApplySyncResult(ctx, *ticket, out, n); e != nil || !ok {
				t.Fatal(e)
			}
		}
	}
	exec(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy',?,?,1,'legacy')`, f.session.ID, device)
	j := h.Job.Snapshot
	j.ID = "suppressed"
	if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, n) }); e != nil {
		t.Fatal(e)
	}
	exec(t, f.store, `UPDATE push_jobs SET state='suppressed' WHERE id='suppressed'`)
	// 保留 queued 与 leased alert，以及已释放但带未结果 attempt 的 sync。
	queued := h.Job.Snapshot
	queued.ID = "alert-queued"
	if e := f.store.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, queued, n) }); e != nil {
		t.Fatal(e)
	}
	exec(t, f.store, `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=? WHERE id=?`, n.Add(time.Minute).UnixNano(), h.Job.Snapshot.ID)
	wants := rows(t, f.store)
	for _, fault := range []string{"none", "middle", "end", "ledger"} {
		t.Run(fault, func(t *testing.T) {
			path, db, _ := oldDatabase(t, 14)
			if _, e := db.Exec(`INSERT INTO sessions VALUES(?,zeroblob(32),1,'android',?,'fixture',?,?,0)`, f.session.ID, device, n.Add(24*time.Hour).UnixNano(), n.Add(48*time.Hour).UnixNano()); e != nil {
				t.Fatal(e)
			}
			for _, v := range wants {
				if _, e := db.Exec(`INSERT INTO push_jobs VALUES(`+strings.TrimSuffix(strings.Repeat("?,", 26), ",")+`)`, v...); e != nil {
					t.Fatal(e)
				}
			}
			switch fault {
			case "middle":
				// 此 view 在复制前有效；DROP 旧表后，真正替换表的 ALTER 才因失效引用失败。
				if _, e := db.Exec(`CREATE VIEW migration_middle_fault AS SELECT id FROM push_jobs`); e != nil {
					t.Fatal(e)
				}
			case "end":
				if _, e := db.Exec(`DROP INDEX push_jobs_pending; CREATE TABLE push_jobs_pending(x TEXT)`); e != nil {
					t.Fatal(e)
				}
			case "ledger":
				if _, e := db.Exec(`CREATE TRIGGER migration_fault BEFORE INSERT ON schema_migrations WHEN NEW.version=15 BEGIN SELECT RAISE(ABORT,'ledger fault');END`); e != nil {
					t.Fatal(e)
				}
			}
			if fault == "middle" {
				var count int
				if e := db.QueryRow(`SELECT count(*) FROM migration_middle_fault`).Scan(&count); e != nil || count != len(wants) {
					t.Fatal("middle view must be valid before replacement", e, count)
				}
			}
			s, e := storage.Open(ctx, storage.Config{Path: path})
			if fault != "none" {
				if e == nil {
					s.Close()
					t.Fatal("fault accepted")
				}
				if fault == "middle" {
					// 唯有复制并删除旧表后，ALTER 的 schema 校验才会报此 view 引用失效。
					if !strings.Contains(e.Error(), "error in view migration_middle_fault") || !strings.Contains(e.Error(), "no such table: main.push_jobs") {
						t.Fatal("middle fault did not reach table replacement", e)
					}
					t.Logf("reached table replacement: %v", e)
					var temporaryTables int
					if e := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='push_jobs_opush_alert_attempt'`).Scan(&temporaryTables); e != nil || temporaryTables != 0 {
						t.Fatal("middle replacement table not rolled back", e, temporaryTables)
					}
				}
				var count int
				if e = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); e != nil || count != 14 {
					t.Fatal("ledger atomic", e, count)
				}
				for id, want := range wants {
					got := make([]any, 26)
					args := make([]any, 26)
					for i := range got {
						args[i] = &got[i]
					}
					if e = db.QueryRow(`SELECT * FROM push_jobs WHERE id=?`, id).Scan(args...); e != nil || !reflect.DeepEqual(got, want) {
						t.Fatal("row atomic", e, id)
					}
				}
				switch fault {
				case "middle":
					_, e = db.Exec(`DROP VIEW migration_middle_fault`)
				case "end":
					_, e = db.Exec(`DROP TABLE push_jobs_pending`)
				case "ledger":
					_, e = db.Exec(`DROP TRIGGER migration_fault`)
				}
				if e != nil {
					t.Fatal(e)
				}
				s, e = storage.Open(ctx, storage.Config{Path: path})
			}
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			foreignKeys(t, s)
			if !reflect.DeepEqual(wants, rows(t, s)) {
				t.Fatal("26 columns/storage class changed")
			}
			for _, q := range []string{`UPDATE push_jobs SET state='queued' WHERE id='suppressed'`, `UPDATE push_jobs SET message_id='changed' WHERE purpose='alert'`, `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=1 WHERE purpose='alert' AND state='queued'`, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('bad','missing','missing',1,'old')`} {
				if e = s.Write(ctx, func(tx *sql.Tx) error { _, e := tx.Exec(q); return e }); e == nil {
					t.Fatal("constraint accepted", q)
				}
			}
			// 放宽只允许 opush 原资格；FCM alert 和空 purpose 仍被 CHECK 拒绝。
			id := h.Job.Snapshot.ID
			for _, mutation := range []string{"provider='fcm'", "alert_eligible=0", "purpose=NULL"} {
				e = s.Write(ctx, func(tx *sql.Tx) error {
					if _, e := tx.Exec(`DROP TRIGGER push_jobs_snapshot_immutable`); e != nil {
						return e
					}
					_, e := tx.Exec(`UPDATE push_jobs SET `+mutation+`,state='accepted',lease_hash=NULL,lease_until_ns=NULL,attempt=1,attempt_lease_hash=zeroblob(32),attempt_lease_until_ns=?,attempt_started_at_ns=?,result_kind='accepted',result_at_ns=? WHERE id=?`, n.Add(time.Minute).UnixNano(), n.UnixNano(), n.UnixNano(), id)
					return e
				})
				if e == nil {
					t.Fatal("scope accepted", mutation)
				}
			}
			exec(t, s, `UPDATE push_jobs SET state='accepted',lease_hash=NULL,lease_until_ns=NULL,attempt=1,attempt_lease_hash=zeroblob(32),attempt_lease_until_ns=?,attempt_started_at_ns=?,result_kind='accepted',result_at_ns=? WHERE id=?`, n.Add(time.Minute).UnixNano(), n.UnixNano(), n.UnixNano(), id)
			exec(t, s, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy-new',?,?,1,'old')`, f.session.ID, device)
			before := database(t, s)
			s.Close()
			s, e = storage.Open(ctx, storage.Config{Path: path})
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			if before != database(t, s) {
				t.Fatal("reopen changed")
			}
			exec(t, s, `DELETE FROM sessions`)
			if len(rows(t, s)) != 0 {
				t.Fatal("cascade lost")
			}
			foreignKeys(t, s)
		})
	}
}

package push

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

func attemptRawRows(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	all := [][]any{}
	for rows.Next() {
		values, ptrs := make([]any, len(columns)), make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		all = append(all, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSyncAttemptMigrationTwelveMixedAndAtomic(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "twelve.sqlite")
			jobOldDatabase(t, path, false)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"0011_push_schedule.sql", "0012_message_attachments.sql"} {
				data, e := fs.ReadFile(migrations.Files, name)
				if e != nil {
					t.Fatal(e)
				}
				tx, e := db.Begin()
				if e != nil {
					t.Fatal(e)
				}
				if _, e = tx.Exec(string(data)); e != nil {
					tx.Rollback()
					t.Fatal(e)
				}
				if _, e = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?,'original-twelve')`, map[string]int{"0011_push_schedule.sql": 11, "0012_message_attachments.sql": 12}[name], name, fmt.Sprintf("%x", sha256.Sum256(data))); e != nil {
					tx.Rollback()
					t.Fatal(e)
				}
				if e = tx.Commit(); e != nil {
					t.Fatal(e)
				}
			}
			if _, err = db.Exec(`INSERT INTO sessions VALUES('alert-session',randomblob(32),1,'android','alert-device','alert',100,200,0)`); err != nil {
				t.Fatal(err)
			}
			for _, j := range []struct{ id, purpose, session, device, state string }{
				{"queued-sync", "sync", "old-session", "old-device", "queued"}, {"leased-sync", "sync", "old-session", "old-device", "leased"},
				{"queued-alert", "alert", "alert-session", "alert-device", "queued"}, {"leased-alert", "alert", "alert-session", "alert-device", "leased"},
			} {
				var hash any
				var until any
				if j.state == "leased" {
					hash = make([]byte, 32)
					until = int64(100)
				}
				_, err = db.Exec(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,purpose,provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,alert_eligible,lease_hash,lease_until_ns)
 VALUES(?,?,?,99,?,3,'1970-01-01T00:00:00.000000001Z',?,'fcm',1,0,'old-deployment','old-message','old-bot',7,1,1,1000,?,?,?)`, j.id, j.session, j.device, j.state, j.purpose, j.purpose == "alert", hash, until)
				if err != nil {
					t.Fatal(err)
				}
			}
			const oldProjection = `SELECT ` + jobColumns + `,lease_hash,lease_until_ns FROM push_jobs ORDER BY id`
			before := attemptRawRows(t, db, oldProjection)
			ledger := attemptRawRows(t, db, `SELECT * FROM schema_migrations ORDER BY version`)
			sessions := attemptRawRows(t, db, `SELECT * FROM sessions ORDER BY id`)
			identity := attemptRawRows(t, db, `SELECT * FROM instance_identity ORDER BY id`)
			if collision {
				// 在最后一个索引重建处失败，覆盖复制、DROP、RENAME 后整次迁移回滚。
				if _, err = db.Exec(`DROP INDEX push_jobs_one_lease;CREATE INDEX push_jobs_one_lease ON sessions(device_name)`); err != nil {
					t.Fatal(err)
				}
				if s, e := storage.Open(jobContext, storage.Config{Path: path}); e == nil {
					s.Close()
					t.Fatal("migration collision accepted")
				}
				if before != attemptRawRows(t, db, oldProjection) || ledger != attemptRawRows(t, db, `SELECT * FROM schema_migrations ORDER BY version`) {
					t.Fatal("partial migration data/ledger")
				}
				if got := attemptRawRows(t, db, `SELECT count(*) FROM sqlite_master WHERE name='push_jobs_attempt_results'`); got != "[[0]]" {
					t.Fatal("temporary table survived", got)
				}
				if _, err = db.Exec(`DROP INDEX push_jobs_one_lease;CREATE UNIQUE INDEX push_jobs_one_lease ON push_jobs(device_id) WHERE state='leased'`); err != nil {
					t.Fatal(err)
				}
			}
			s, err := storage.Open(jobContext, storage.Config{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if before != attemptRawRows(t, db, oldProjection) || ledger != attemptRawRows(t, db, `SELECT * FROM schema_migrations WHERE version<=12 ORDER BY version`) || sessions != attemptRawRows(t, db, `SELECT * FROM sessions ORDER BY id`) || identity != attemptRawRows(t, db, `SELECT * FROM instance_identity ORDER BY id`) {
				t.Fatal("migration changed original snapshot/lease/identity/ledger")
			}
			if got := attemptRawRows(t, db, `SELECT count(*) FROM push_jobs WHERE attempt_lease_hash IS NOT NULL OR attempt_lease_until_ns IS NOT NULL OR attempt_started_at_ns IS NOT NULL OR result_kind IS NOT NULL OR result_at_ns IS NOT NULL`); got != "[[0]]" {
				t.Fatal("migration manufactured attempts")
			}
			if got := attemptRawRows(t, db, `SELECT count(*) FROM sqlite_master WHERE name IN ('push_jobs_pending','push_jobs_session','push_jobs_device_lease','push_jobs_one_lease','push_jobs_snapshot_immutable')`); got != "[[5]]" {
				t.Fatal("missing constraints", got)
			}
			if got := attemptRawRows(t, db, `PRAGMA foreign_key_check`); got != "[]" {
				t.Fatal("foreign key failure", got)
			}
			if _, err = db.Exec(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('writer-after13','old-session','old-device',20,'original-time')`); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=100 WHERE id='queued-sync'`); err == nil {
				t.Fatal("same-device lease not unique")
			}
			if _, err = db.Exec(`UPDATE push_jobs SET registration_version=2 WHERE id='queued-sync'`); err == nil {
				t.Fatal("snapshot no longer immutable")
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, e := storage.Open(jobContext, storage.Config{Path: path})
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			if sessions != attemptRawRows(t, db, `SELECT * FROM sessions ORDER BY id`) {
				t.Fatal("reopen changed sessions")
			}
			if _, err = db.Exec(`DELETE FROM sessions WHERE id='old-session'`); err != nil {
				t.Fatal(err)
			}
			if got := attemptRawRows(t, db, `SELECT count(*) FROM push_jobs WHERE session_id='old-session'`); got != "[[0]]" {
				t.Fatal("session cascade broken", got)
			}
			if got := attemptRawRows(t, db, `SELECT count(*) FROM sessions WHERE id='alert-session'`); got != "[[1]]" {
				t.Fatal("cascade touched other session")
			}
		})
	}
}

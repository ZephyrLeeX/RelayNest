package messages

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

// 从真实原SQL逐份建立旧11库；不用新库删除台账冒充升级。
func attachmentOldEleven(t *testing.T, path string, collision bool) map[string]string {
	t.Helper()
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`PRAGMA foreign_keys=ON`); e != nil {
		t.Fatal(e)
	}
	tx, e := db.BeginTx(messageContext, nil)
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
	checks := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") || name >= "0012" {
			continue
		}
		data, e := fs.ReadFile(migrations.Files, name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(string(data)); e != nil {
			t.Fatal(name, e)
		}
		if strings.HasPrefix(name, "0005_") {
			if _, e = storage.InitInstanceIDTx(messageContext, tx); e != nil {
				t.Fatal(e)
			}
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		checks[name] = sum
		if _, e = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?,'old-ledger')`, len(checks), name, sum); e != nil {
			t.Fatal(e)
		}
	}
	if len(checks) != 11 {
		t.Fatal("not old eleven", len(checks))
	}
	const mid = "11111111-1111-4111-8111-111111111111"
	const aid = "22222222-2222-4222-8222-222222222222"
	statements := []string{
		`INSERT INTO bots(id,name,enabled,can_send,can_receive,version,bot_seq) VALUES('old-bot','old',1,1,1,1,7)`,
		`INSERT INTO messages(id,bot_id,bot_seq,created_change_seq,sender,revision,format,text,buttons_json,created_at,updated_at) VALUES('` + mid + `','old-bot',7,5,'application',2,'plain','old-message','[]','2026-01-01T00:00:00Z','2026-01-02T00:00:00Z')`,
		`INSERT INTO bot_read_states VALUES('old-bot',3,4,'2026-01-02T00:00:00Z')`,
		`INSERT INTO administrator VALUES(1,'old-admin','synthetic',1)`,
		`INSERT INTO sessions VALUES('old-session',zeroblob(32),1,'android','old-device','old',2000000000000000000,2100000000000000000,0)`,
		`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('old-legacy','old-session','old-device',5,'2026-01-02T00:00:00Z')`,
		`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,next_attempt_at,purpose,provider,registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,created_at_ns,expires_at_ns,alert_eligible) VALUES('old-complete','old-session','old-device',99,'queued','2026-01-02T00:00:00.000000000Z','sync','fcm',1,0,'old-deployment','` + mid + `','old-bot',7,1,1,2100000000000000000,0)`,
	}
	for _, q := range statements {
		if _, e = tx.Exec(q); e != nil {
			t.Fatal(e)
		}
	}
	sum := sha256.Sum256([]byte("old-upload-bytes"))
	if _, e = tx.Exec(`INSERT INTO attachments(id,bot_id,kind,filename,media_type,storage_key,state,size,sha256,created_at) VALUES(?,'old-bot','file','private.txt','text/plain','old-random.ready','ready',16,?,'2026-01-01T00:00:00Z')`, aid, sum[:]); e != nil {
		t.Fatal(e)
	}
	if collision {
		if _, e = tx.Exec(`CREATE INDEX message_attachments_attachment ON attachments(id)`); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	return checks
}
func attachmentOldDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var all []any
	for _, table := range []string{"bots", "messages", "attachments", "bot_read_states", "push_jobs", "sessions", "administrator"} {
		columns := "*"
		if table == "push_jobs" {
			// 0013 增加尝试列；逐列保留全部旧21列比较，不能把新增NULL误作历史数据改变。
			columns = `id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at,purpose,provider,
 registration_version,settings_version,deployment_id,message_id,bot_id,bot_seq,session_generation,
 created_at_ns,expires_at_ns,alert_eligible,lease_hash,lease_until_ns`
		}
		rows, e := db.Query(`SELECT ` + columns + ` FROM ` + table + ` ORDER BY 1`)
		if e != nil {
			t.Fatal(e)
		}
		cols, e := rows.Columns()
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
			v := make([]any, len(cols))
			ptr := make([]any, len(cols))
			for i := range v {
				ptr[i] = &v[i]
			}
			if e = rows.Scan(ptr...); e != nil {
				t.Fatal(e)
			}
			all = append(all, v)
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
	}
	raw, e := json.Marshal(all)
	if e != nil {
		t.Fatal(e)
	}
	return sha256.Sum256(raw)
}
func TestMessageAttachmentMigrationElevenToTwelve(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "old.sqlite")
			checks := attachmentOldEleven(t, path, collision)
			file := filepath.Join(dir, "old-random.ready")
			if e := os.WriteFile(file, []byte("old-upload-bytes"), 0600); e != nil {
				t.Fatal(e)
			}
			before := attachmentOldDigest(t, path)
			if collision {
				s, e := storage.Open(messageContext, storage.Config{Path: path})
				if e == nil {
					s.Close()
					t.Fatal("index collision accepted")
				}
				if attachmentOldDigest(t, path) != before {
					t.Fatal("rollback changed old data")
				}
				db, e := sql.Open("sqlite", path)
				if e != nil {
					t.Fatal(e)
				}
				var n int
				if e = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); e != nil || n != 11 {
					t.Fatal("partial ledger", n, e)
				}
				if e = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='message_attachments'`).Scan(&n); e != nil || n != 0 {
					t.Fatal("partial DDL", n, e)
				}
				if _, e = db.Exec(`DROP INDEX message_attachments_attachment`); e != nil {
					t.Fatal(e)
				}
				db.Close()
			}
			s, e := storage.Open(messageContext, storage.Config{Path: path})
			if e != nil {
				t.Fatal(e)
			}
			if attachmentOldDigest(t, path) != before {
				t.Fatal("migration changed old upload/message/read/jobs")
			}
			if e = s.Read(messageContext, func(tx *sql.Tx) error {
				var n int
				if e := tx.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); e != nil {
					return e
				}
				if n != 13 {
					t.Fatal(n)
				}
				// 新列只能保持空值，迁移不得替原legacy或完整job补造派发/结果。
				if e := tx.QueryRow(`SELECT count(*) FROM push_jobs WHERE attempt_lease_hash IS NOT NULL
 OR attempt_lease_until_ns IS NOT NULL OR attempt_started_at_ns IS NOT NULL
 OR result_kind IS NOT NULL OR result_at_ns IS NOT NULL`).Scan(&n); e != nil {
					return e
				}
				if n != 0 {
					t.Fatal("migration manufactured attempt data", n)
				}
				for name, want := range checks {
					var got string
					var stamp string
					if e := tx.QueryRow(`SELECT checksum,applied_at FROM schema_migrations WHERE name=?`, name).Scan(&got, &stamp); e != nil {
						return e
					}
					if got != want || stamp != "old-ledger" {
						t.Fatal("old ledger changed", name)
					}
				}
				rows, e := tx.Query(`PRAGMA foreign_key_check`)
				if e != nil {
					return e
				}
				defer rows.Close()
				if rows.Next() {
					t.Fatal("foreign keys")
				}
				return rows.Err()
			}); e != nil {
				t.Fatal(e)
			}
			if e = s.Write(messageContext, func(tx *sql.Tx) error {
				_, e := tx.Exec(`INSERT INTO message_attachments VALUES('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222','file',0)`)
				return e
			}); e != nil {
				t.Fatal(e)
			}
			if e = s.Close(); e != nil {
				t.Fatal(e)
			}
			s, e = storage.Open(messageContext, storage.Config{Path: path})
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			if e = s.Read(messageContext, func(tx *sql.Tx) error {
				m, e := New(s).GetHistoryMessageByIDTx(messageContext, tx, "11111111-1111-4111-8111-111111111111")
				if e == nil && (len(m.AttachmentRefs) != 1 || m.Text != "old-message") {
					t.Fatal(m)
				}
				return e
			}); e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(file)
			if e != nil || string(data) != "old-upload-bytes" {
				t.Fatal("old physical bytes changed", e)
			}
		})
	}
}

package push

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

var jobContext = context.Background()

const jobDeviceA = "11111111-1111-4111-8111-111111111111"
const jobDeviceB = "22222222-2222-4222-8222-222222222222"

type jobFixture struct {
	store, other *storage.Store
	cfg          storage.Config
	now          time.Time
	sessions     *auth.SessionStore
	bot          bots.Created
	identity     bots.Identity
	deployment   string
	serial       int
}

func jobSetup(t *testing.T) *jobFixture {
	t.Helper()
	f := &jobFixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "jobs.sqlite"), BusyTimeout: 50 * time.Millisecond}, now: time.Now().UTC()}
	var err error
	f.store, err = storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.store.Close() })
	f.other, err = storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.other.Close() })
	jobSQL(t, f.store, `INSERT INTO administrator VALUES(1,'admin','synthetic-hash',1)`)
	f.sessions = auth.NewSessionStore(f.store)
	f.bot, _, err = bots.New(f.store).Create(jobContext, bots.CreateRequest{Name: "job", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.identity, err = bots.New(f.store).Authenticate(jobContext, f.bot.Bot.ID, bots.APIToken, f.bot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.Read(jobContext, func(tx *sql.Tx) error {
		var e error
		f.deployment, e = storage.ReadInstanceIDTx(jobContext, tx)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	return f
}
func jobSQL(t *testing.T, s *storage.Store, q string, args ...any) {
	t.Helper()
	if err := s.Write(jobContext, func(tx *sql.Tx) error { _, err := tx.ExecContext(jobContext, q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func jobCount(t *testing.T, s *storage.Store, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := s.Read(jobContext, func(tx *sql.Tx) error { return tx.QueryRowContext(jobContext, q, args...).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}
func jobRead(t *testing.T, s *storage.Store, id string) Job {
	t.Helper()
	var j Job
	if err := s.Read(jobContext, func(tx *sql.Tx) error { var e error; j, e = ReadJobTx(jobContext, tx, id); return e }); err != nil {
		t.Fatal(err)
	}
	return j
}
func (f *jobFixture) session(t *testing.T, device string) auth.Session {
	t.Helper()
	s, _, err := f.sessions.Create(jobContext, 1, auth.SessionInput{ClientType: "android", DeviceID: device, DeviceName: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	// 完整快照仓储夹具；不声称此直接注册行来自生产 HTTP。
	jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic-registration-secret',?)`, s.ID, f.now.Format(time.RFC3339Nano))
	return s
}
func (f *jobFixture) snapshot(t *testing.T, s auth.Session, purpose string) JobSnapshot {
	t.Helper()
	m, err := messages.New(f.store).CreateApplication(jobContext, f.identity, messages.TextInput{BotID: f.bot.Bot.ID, Format: "plain", Text: "private-job-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	f.serial++
	return JobSnapshot{ID: fmt.Sprintf("job-%03d", f.serial), SessionID: s.ID, DeviceID: s.DeviceID, Purpose: purpose, TargetChangeSeq: int64(m.LastChangeSeq), Provider: "fcm", RegistrationVersion: 1, SettingsVersion: 0, DeploymentID: f.deployment, MessageID: m.Message.ID, BotID: f.bot.Bot.ID, BotSeq: m.Message.BotSeq, SessionGeneration: 1, CreatedAt: f.now, ExpiresAt: f.now.Add(time.Hour), AlertEligible: purpose == "alert"}
}
func jobCreate(t *testing.T, f *jobFixture, j JobSnapshot) {
	t.Helper()
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error { return CreateJobTx(jobContext, tx, j, j.CreatedAt) }); err != nil {
		t.Fatal(err)
	}
}

func TestJobSnapshotPersistenceAndImmutableLegacy(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	jobCreate(t, f, j)
	before := jobRead(t, f.other, j.ID)
	if before.Snapshot != j || before.State != "queued" || before.Attempt != 0 {
		t.Fatal("snapshot persistence")
	}
	// 最终水位不要求等于关联消息创建水位；归并不重置原资格或时间。
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		changed, e := MergeJobWatermarkTx(jobContext, tx, j.ID, j.TargetChangeSeq+10)
		if e == nil && !changed {
			t.Fatal("watermark not merged")
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
	merged := jobRead(t, f.store, j.ID)
	want := before
	want.Snapshot.TargetChangeSeq += 10
	if merged != want {
		t.Fatal("merge changed immutable snapshot")
	}
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		changed, e := MergeJobWatermarkTx(jobContext, tx, j.ID, 1)
		if changed {
			t.Fatal("watermark regressed")
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
	alert := f.snapshot(t, s, "alert")
	jobCreate(t, f, alert)
	for _, q := range []string{`UPDATE push_jobs SET expires_at_ns=expires_at_ns+1 WHERE id=?`, `UPDATE push_jobs SET created_at_ns=created_at_ns+1 WHERE id=?`, `UPDATE push_jobs SET message_id='other' WHERE id=?`, `UPDATE push_jobs SET settings_version=1 WHERE id=?`, `UPDATE push_jobs SET alert_eligible=1 WHERE id=?`, `UPDATE push_jobs SET state='pending' WHERE id=?`, `UPDATE push_jobs SET target_change_seq=1 WHERE id=?`} {
		if err := f.store.Write(jobContext, func(tx *sql.Tx) error { _, e := tx.Exec(q, j.ID); return e }); err == nil {
			t.Fatal("immutable field changed")
		}
	}
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE push_jobs SET target_change_seq=target_change_seq+1 WHERE id=?`, alert.ID)
		return e
	}); err == nil {
		t.Fatal("alert eligibility revived by merge")
	}
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy',?,?,99,'old-unknown-time')`, s.ID, s.DeviceID)
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE push_jobs SET state='queued',purpose='sync' WHERE id='legacy'`)
		return e
	}); err == nil {
		t.Fatal("legacy upgraded")
	}
	if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
		_, e := ReadJobTx(jobContext, tx, "legacy")
		if !errors.Is(e, sql.ErrNoRows) {
			t.Fatal("legacy read as full job", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if jobRead(t, reopened, j.ID) != merged {
		t.Fatal("reopen changed snapshot")
	}
	jobSQL(t, f.store, `DELETE FROM sessions WHERE id=?`, s.ID)
	if jobCount(t, f.other, `SELECT count(*) FROM push_jobs`) != 0 {
		t.Fatal("session cascade lost")
	}
}

func TestJobCreateSnapshotValidation(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	for _, mutate := range []func(*JobSnapshot){func(j *JobSnapshot) { j.SettingsVersion = 1 }, func(j *JobSnapshot) { j.RegistrationVersion = 2 }, func(j *JobSnapshot) { j.DeviceID = jobDeviceB }, func(j *JobSnapshot) { j.DeploymentID = "other" }, func(j *JobSnapshot) { j.SessionGeneration = 2 }, func(j *JobSnapshot) { j.BotSeq++ }, func(j *JobSnapshot) { j.MessageID = "missing" }} {
		bad := j
		mutate(&bad)
		err := f.store.Write(jobContext, func(tx *sql.Tx) error { return CreateJobTx(jobContext, tx, bad, bad.CreatedAt) })
		if err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs`) != 0 {
		t.Fatal("invalid snapshot persisted")
	}
	jobSQL(t, f.store, `UPDATE sessions SET revoked=1 WHERE id=?`, s.ID)
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error { return CreateJobTx(jobContext, tx, j, j.CreatedAt) }); err == nil {
		t.Fatal("revoked snapshot accepted")
	}
}

// 用真实前十次迁移构造旧库；台账校验与原行逐列保留，不写升级时间到旧 hint。
func jobOldDatabase(t *testing.T, path string, collision bool) map[string]string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(jobContext, nil)
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
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") || e.Name() >= "0011" {
			continue
		}
		data, err := fs.ReadFile(migrations.Files, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(string(data)); err != nil {
			t.Fatal(e.Name(), err)
		}
		if strings.HasPrefix(e.Name(), "0005_") {
			if _, err = storage.InitInstanceIDTx(jobContext, tx); err != nil {
				t.Fatal(err)
			}
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		checks[e.Name()] = sum
		if _, err = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?,'old')`, len(checks), e.Name(), sum); err != nil {
			t.Fatal(err)
		}
	}
	if len(checks) != 10 {
		t.Fatal("not ten migration fixture")
	}
	if _, err = tx.Exec(`INSERT INTO administrator VALUES(1,'old-admin','synthetic',1); INSERT INTO sessions VALUES('old-session',zeroblob(32),1,'android','old-device','old',100,200,0); INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,attempt,next_attempt_at) VALUES('old-hint','old-session','old-device',17,4,'unknown-original-time')`); err != nil {
		t.Fatal(err)
	}
	if collision {
		if _, err = tx.Exec(`CREATE INDEX push_jobs_one_lease ON sessions(device_name)`); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return checks
}
func TestJobMigrationTenToElevenAndAtomicFailure(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "old.sqlite")
			checks := jobOldDatabase(t, path, collision)
			if collision {
				s, err := storage.Open(jobContext, storage.Config{Path: path})
				if err == nil {
					s.Close()
					t.Fatal("DDL collision accepted")
				}
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				var n int
				if err = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 10 {
					t.Fatal("partial migration ledger", n, err)
				}
				var target, attempt int
				var stamp string
				if err = db.QueryRow(`SELECT target_change_seq,attempt,next_attempt_at FROM push_jobs WHERE id='old-hint'`).Scan(&target, &attempt, &stamp); err != nil || target != 17 || attempt != 4 || stamp != "unknown-original-time" {
					t.Fatal("legacy damaged on rollback", err)
				}
				if _, err = db.Exec(`DROP INDEX push_jobs_one_lease`); err != nil {
					t.Fatal(err)
				}
				db.Close()
			}
			s, err := storage.Open(jobContext, storage.Config{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if jobCount(t, s, `SELECT count(*) FROM schema_migrations`) != 11 {
				t.Fatal("migration count")
			}
			for name, sum := range checks {
				if err = s.Read(jobContext, func(tx *sql.Tx) error {
					var got string
					if e := tx.QueryRow(`SELECT checksum FROM schema_migrations WHERE name=?`, name).Scan(&got); e != nil {
						return e
					}
					if got != sum {
						t.Fatal("old checksum changed")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			if jobCount(t, s, `SELECT count(*) FROM push_jobs WHERE id='old-hint' AND state='pending' AND attempt=4 AND target_change_seq=17 AND next_attempt_at='unknown-original-time' AND created_at_ns IS NULL AND expires_at_ns IS NULL AND purpose IS NULL`) != 1 {
				t.Fatal("legacy not preserved")
			}
			if err = s.Read(jobContext, func(tx *sql.Tx) error {
				rows, e := tx.Query(`PRAGMA foreign_key_check`)
				if e != nil {
					return e
				}
				defer rows.Close()
				if rows.Next() {
					t.Fatal("FK failure")
				}
				return rows.Err()
			}); err != nil {
				t.Fatal(err)
			}
			before := jobCount(t, s, `SELECT count(*) FROM push_jobs`)
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = storage.Open(jobContext, storage.Config{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if jobCount(t, s, `SELECT count(*) FROM push_jobs`) != before {
				t.Fatal("reopen altered hints")
			}
		})
	}
}

func TestJobTransactionInputFailures(t *testing.T) {
	f := jobSetup(t)
	now := f.now
	if _, err := ReserveJobsTx(jobContext, nil, now, time.Minute, 1); !errors.Is(err, ErrJobInput) {
		t.Fatal(err)
	}
	if err := CreateJobTx(jobContext, nil, JobSnapshot{}, now); !errors.Is(err, ErrJobInput) {
		t.Fatal(err)
	}
	if _, err := ReadJobTx(jobContext, nil, "id"); !errors.Is(err, ErrJobInput) {
		t.Fatal(err)
	}
	if _, err := MergeJobWatermarkTx(jobContext, nil, "id", 1); !errors.Is(err, ErrJobInput) {
		t.Fatal(err)
	}
	if _, err := ReleaseJobTx(jobContext, nil, JobLease{}, now); !errors.Is(err, ErrJobInput) {
		t.Fatal(err)
	}
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		canceled, cancel := context.WithCancel(jobContext)
		cancel()
		for _, ctx := range []context.Context{nil, canceled} {
			if got, e := ReserveJobsTx(ctx, tx, now, time.Minute, 1); e == nil || got != nil {
				t.Fatal("context failure published")
			}
			if e := CreateJobTx(ctx, tx, JobSnapshot{}, now); e == nil {
				t.Fatal("create context")
			}
			if got, e := ReadJobTx(ctx, tx, "id"); e == nil || !reflect.DeepEqual(got, Job{}) {
				t.Fatal("read context")
			}
			if changed, e := MergeJobWatermarkTx(ctx, tx, "id", 1); e == nil || changed {
				t.Fatal("merge context")
			}
			if changed, e := ReleaseJobTx(ctx, tx, JobLease{}, now); e == nil || changed {
				t.Fatal("release context")
			}
		}
		for _, limit := range []int{0, MaxJobReservations + 1} {
			if got, e := ReserveJobsTx(jobContext, tx, now, time.Minute, limit); e == nil || got != nil {
				t.Fatal("limit")
			}
		}
		if got, e := ReserveJobsTx(jobContext, tx, now, 0, 1); e == nil || got != nil {
			t.Fatal("ttl")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestJobLegacyWriterAndDatabaseSnapshotChecks(t *testing.T) {
	f := jobSetup(t)
	session := f.session(t, jobDeviceA)
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy-only',?,?,7,?)`, session.ID, session.DeviceID, f.now.Format(time.RFC3339Nano))
	got, err := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(got) != 0 {
		t.Fatal("legacy hint leased", err)
	}
	if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE id='legacy-only' AND state='pending' AND purpose IS NULL AND lease_hash IS NULL AND created_at_ns IS NULL`) != 1 {
		t.Fatal("legacy qualification synthesized")
	}
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,state,next_attempt_at) VALUES('incomplete',?,?,7,'queued',?)`, session.ID, session.DeviceID, f.now.Format(jobTimestamp))
		return e
	}); err == nil {
		t.Fatal("incomplete queued snapshot accepted")
	}
	j := f.snapshot(t, session, "sync")
	jobCreate(t, f, j)
	for _, assignment := range []string{"registration_version=0", "settings_version=-1", "provider='opush'", "expires_at_ns=created_at_ns", "state='leased'", "alert_eligible=1"} {
		// 用 INSERT SELECT 独立检验 CHECK，不能让不可变 UPDATE trigger 掩盖缺失约束。
		cols := strings.Split(strings.ReplaceAll(jobColumns, "\n", ""), ",")
		values := append([]string(nil), cols...)
		values[0] = "'invalid-copy'"
		parts := strings.SplitN(assignment, "=", 2)
		for i, c := range cols {
			if strings.TrimSpace(c) == parts[0] {
				values[i] = parts[1]
			}
		}
		query := `INSERT INTO push_jobs (` + jobColumns + `) SELECT ` + strings.Join(values, ",") + ` FROM push_jobs WHERE id=?`
		if err := f.store.Write(jobContext, func(tx *sql.Tx) error { _, e := tx.Exec(query, j.ID); return e }); err == nil {
			t.Fatal("database snapshot CHECK accepted", assignment)
		}
	}
}

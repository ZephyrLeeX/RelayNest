// Package dropstorage_test 单独承载真实仓储组合，避免增加旧 push 包的累计时间预算。
package dropstorage_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/devices"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

var ctx = context.Background()

const stamp = "2006-01-02T15:04:05.000000000Z"

type fixture struct {
	s, other      *storage.Store
	cfg           storage.Config
	now           time.Time
	j             push.JobSnapshot
	held          push.JobReservation
	configuration devices.NotificationConfiguration
}

func write(t *testing.T, s *storage.Store, q string, args ...any) {
	t.Helper()
	if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.ExecContext(ctx, q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func newFixture(t *testing.T, provider string, eligible bool) *fixture {
	t.Helper()
	f := &fixture{cfg: storage.Config{Path: filepath.Join(t.TempDir(), "drop.sqlite"), BusyTimeout: 50 * time.Millisecond}, now: time.Now().UTC()}
	var err error
	f.s, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.s.Close() })
	f.other, err = storage.Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.other.Close() })
	write(t, f.s, `INSERT INTO administrator VALUES(1,'admin','synthetic-hash',1)`)
	session, _, err := auth.NewSessionStore(f.s).Create(ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: "11111111-1111-4111-8111-111111111111", DeviceName: "drop-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	bot, _, err := bots.New(f.s).Create(ctx, bots.CreateRequest{Name: "drop", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := bots.New(f.s).Authenticate(ctx, bot.Bot.ID, bots.APIToken, bot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := messages.New(f.s).CreateApplication(ctx, identity, messages.TextInput{BotID: bot.Bot.ID, Format: "plain", Text: "synthetic-drop-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	write(t, f.s, `INSERT INTO device_push_registrations VALUES(?,?,1,'synthetic-registration',?)`, session.ID, provider, f.now.Format(stamp))
	p := devices.PushProvider(provider)
	channel := "fcm"
	if provider == "opush" {
		channel = "vendor"
	}
	f.configuration = devices.NotificationConfiguration{Enabled: true, BotOverrides: []devices.NotificationBotOverride{}, DND: devices.NotificationDND{Enabled: true, Timezone: "UTC", Start: f.now.Format("15:04"), End: f.now.Add(time.Hour).Format("15:04")}, NotifyChannel: channel, NotifyProvider: &p}
	f.settings(t)
	var deployment string
	if err := f.s.Read(ctx, func(tx *sql.Tx) error { var e error; deployment, e = storage.ReadInstanceIDTx(ctx, tx); return e }); err != nil {
		t.Fatal(err)
	}
	f.j = push.JobSnapshot{ID: "alert-original", SessionID: session.ID, DeviceID: session.DeviceID, Purpose: "alert", TargetChangeSeq: int64(msg.LastChangeSeq), Provider: provider, RegistrationVersion: 1, SettingsVersion: 1, DeploymentID: deployment, MessageID: msg.Message.ID, BotID: bot.Bot.ID, BotSeq: msg.Message.BotSeq, SessionGeneration: 1, CreatedAt: f.now, ExpiresAt: f.now.Add(4 * time.Hour), AlertEligible: eligible}
	// 尚无真实 alert 生产者：显式构造完整事件，再走真实仓储创建和通用预留。
	if err := f.s.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, f.j, f.now) }); err != nil {
		t.Fatal(err)
	}
	held, err := push.NewJobRepository(f.s).Reserve(ctx, f.now, time.Minute, 1)
	if err != nil || len(held) != 1 {
		t.Fatal("reserve", err, len(held))
	}
	f.held = held[0]
	return f
}
func (f *fixture) settings(t *testing.T) {
	t.Helper()
	raw, err := json.Marshal(f.configuration)
	if err != nil {
		t.Fatal(err)
	}
	write(t, f.s, `INSERT INTO device_notification_settings VALUES('11111111-1111-4111-8111-111111111111',1,?,?) ON CONFLICT(device_id) DO UPDATE SET configuration=excluded.configuration`, f.now.Format(stamp), string(raw))
}
func row(t *testing.T, s *storage.Store, id string) []any {
	t.Helper()
	var values []any
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT * FROM push_jobs WHERE id=?`, id)
		if e != nil {
			return e
		}
		defer rows.Close()
		cols, e := rows.Columns()
		if e != nil {
			return e
		}
		if len(cols) != 26 {
			return fmt.Errorf("columns=%d", len(cols))
		}
		if !rows.Next() {
			return sql.ErrNoRows
		}
		values = make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		return rows.Scan(dest...)
	}); err != nil {
		t.Fatal(err)
	}
	return values
}
func database(t *testing.T, s *storage.Store) string {
	t.Helper()
	var out strings.Builder
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		names, e := tx.Query(`SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name`)
		if e != nil {
			return e
		}
		var tables []string
		for names.Next() {
			var n string
			if e := names.Scan(&n); e != nil {
				return e
			}
			tables = append(tables, n)
		}
		if e := names.Close(); e != nil {
			return e
		}
		for _, table := range tables {
			rows, e := tx.Query(`SELECT * FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
			if e != nil {
				return e
			}
			cols, e := rows.Columns()
			if e != nil {
				return e
			}
			fmt.Fprint(&out, table)
			var records []string
			for rows.Next() {
				v := make([]any, len(cols))
				d := make([]any, len(cols))
				for i := range v {
					d[i] = &v[i]
				}
				if e := rows.Scan(d...); e != nil {
					return e
				}
				records = append(records, fmt.Sprintf("%#v", v))
			}
			if e := rows.Err(); e != nil {
				return e
			}
			rows.Close()
			sort.Strings(records)
			for _, record := range records {
				out.WriteString(record)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
func drop(t *testing.T, f *fixture, s *storage.Store, want, wantErr bool) {
	t.Helper()
	ok, e := push.NewJobRepository(s).DropAlertDND(ctx, f.held.Lease, f.j, f.now)
	if ok != want || (e != nil) != wantErr {
		t.Fatalf("changed=%v err=%v want=%v error=%v", ok, e, want, wantErr)
	}
}
func TestDropPersistsAndFreesDevice(t *testing.T) {
	for _, provider := range []string{"fcm", "opush"} {
		t.Run(provider, func(t *testing.T) {
			f := newFixture(t, provider, true)
			before := row(t, f.s, f.j.ID)
			drop(t, f, f.other, true, false)
			want := append([]any(nil), before...)
			want[4], want[19], want[20] = "suppressed", nil, nil
			if !reflect.DeepEqual(want, row(t, f.s, f.j.ID)) {
				t.Fatal("changed more than state/lease")
			}
			drop(t, f, f.s, false, false)
			f.configuration.DND.Enabled = false
			f.settings(t)
			reopened, e := storage.Open(ctx, f.cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			for _, s := range []*storage.Store{f.s, f.other, reopened} {
				held, e := push.NewJobRepository(s).Reserve(ctx, f.now.Add(2*time.Hour), time.Minute, 10)
				if e != nil || len(held) != 0 {
					t.Fatal("revived after DND", e, len(held))
				}
				ok, e := push.NewJobRepository(s).Release(ctx, f.held.Lease, f.now)
				if e != nil || ok {
					t.Fatal("terminal released", e)
				}
			}
			for _, q := range []string{`UPDATE push_jobs SET state='queued' WHERE id=?`, `UPDATE push_jobs SET state='leased',lease_hash=zeroblob(32),lease_until_ns=1 WHERE id=?`, `UPDATE push_jobs SET state='accepted',result_kind='accepted' WHERE id=?`, `UPDATE push_jobs SET state='degraded' WHERE id=?`} {
				if e := f.s.Write(ctx, func(tx *sql.Tx) error { _, e := tx.Exec(q, f.j.ID); return e }); e == nil {
					t.Fatal("terminal transition permitted")
				}
			}
			next := f.j
			next.ID = "another-valid-alert"
			if e := f.s.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, next, f.now) }); e != nil {
				t.Fatal(e)
			}
			held, e := push.NewJobRepository(f.other).Reserve(ctx, f.now, time.Minute, 1)
			if e != nil || len(held) != 1 || held[0].Job.Snapshot.ID != next.ID {
				t.Fatal("device not freed", e)
			}
		})
	}
}
func TestDropNoopsAndErrors(t *testing.T) {
	cases := []string{"wrong-expected", "old-takeover", "released", "lease-expired", "job-expired", "rotated", "token-removed", "revoked", "settings-version", "instance", "generation", "ineligible", "disabled", "bot-disabled", "provider", "no-dnd", "bad-json", "bad-timezone", "sql", "sync"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, "fcm", name != "ineligible")
			wantErr := false
			target, observer := f.other, f.s
			switch name {
			case "wrong-expected":
				f.j.MessageID = "other"
			case "old-takeover":
				f.now = f.now.Add(time.Minute)
				held, e := push.NewJobRepository(f.other).Reserve(ctx, f.now, time.Minute, 1)
				if e != nil || len(held) != 1 {
					t.Fatal(e)
				}
			case "released":
				ok, e := push.NewJobRepository(f.other).Release(ctx, f.held.Lease, f.now)
				if e != nil || !ok {
					t.Fatal(e)
				}
			case "lease-expired":
				f.now = f.now.Add(time.Minute)
			case "job-expired":
				f.now = f.j.ExpiresAt
			case "rotated":
				write(t, f.s, `UPDATE device_push_registrations SET registration_version=2`)
			case "token-removed":
				write(t, f.s, `UPDATE device_push_registrations SET token=NULL`)
			case "revoked":
				write(t, f.s, `UPDATE sessions SET revoked=1`)
			case "settings-version":
				write(t, f.s, `UPDATE device_notification_settings SET settings_version=2`)
			case "instance":
				write(t, f.s, `UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'`)
				reopened, e := storage.Open(ctx, f.cfg)
				if e != nil {
					t.Fatal(e)
				}
				defer reopened.Close()
				target, observer = reopened, reopened
			case "generation":
				write(t, f.s, `UPDATE administrator SET auth_generation=2`)
			case "disabled":
				f.configuration.Enabled = false
				f.settings(t)
			case "bot-disabled":
				f.configuration.BotOverrides = []devices.NotificationBotOverride{{BotID: f.j.BotID, Enabled: false}}
				f.settings(t)
			case "provider":
				p := devices.PushOPush
				f.configuration.NotifyProvider = &p
				f.configuration.NotifyChannel = "vendor"
				f.settings(t)
			case "no-dnd":
				f.configuration.DND.Enabled = false
				f.settings(t)
			case "sql":
				write(t, f.s, `ALTER TABLE device_notification_settings RENAME TO missing_settings`)
				wantErr = true
			case "bad-json":
				write(t, f.s, `UPDATE device_notification_settings SET configuration='{}'`)
				wantErr = true
			case "bad-timezone":
				f.configuration.DND.Timezone = "Invalid/Zone"
				f.settings(t)
				wantErr = true
			case "sync":
				f.j.Purpose = "sync"
				f.j.AlertEligible = false
			}
			before := database(t, observer)
			drop(t, f, target, false, wantErr)
			if before != database(t, observer) {
				t.Fatal("denial wrote database")
			}
		})
	}
}
func TestDropFaultsRollback(t *testing.T) {
	faults := map[string]string{
		"abort":        `CREATE TRIGGER drop_fault BEFORE UPDATE OF state ON push_jobs BEGIN SELECT RAISE(ABORT,'drop fault');END`,
		"ignore":       `CREATE TRIGGER drop_fault BEFORE UPDATE OF state ON push_jobs BEGIN SELECT RAISE(IGNORE);END`,
		"commit":       `CREATE TABLE drop_fault_fk(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN INSERT INTO drop_fault_fk VALUES('missing-bot');END`,
		"identity":     `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE sessions SET revoked=1;END`,
		"registration": `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE device_push_registrations SET registration_version=2;END`,
		"settings":     `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE device_notification_settings SET settings_version=2;END`,
		"reason":       `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE device_notification_settings SET configuration=json_set(configuration,'$.dnd.enabled',json('false'));END`,
		"instance":     `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222';END`,
		"attempt":      `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE push_jobs SET attempt=attempt+1 WHERE id=NEW.id;END`,
		"next":         `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE push_jobs SET next_attempt_at='2099-01-01T00:00:00.000000000Z' WHERE id=NEW.id;END`,
		"storage":      `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE push_jobs SET next_attempt_at=CAST(next_attempt_at AS BLOB) WHERE id=NEW.id;END`,
		"metadata":     `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE push_jobs SET result_kind='transient' WHERE id=NEW.id;END`,
		"lease":        `CREATE TRIGGER drop_fault AFTER UPDATE OF state ON push_jobs BEGIN UPDATE push_jobs SET lease_hash=zeroblob(32),lease_until_ns=1 WHERE id=NEW.id;END`,
	}
	for name, q := range faults {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, "fcm", true)
			write(t, f.s, q)
			// 损坏存储类/非法lease/metadata须先证明schema本来拒绝，再仅在专用连接模拟受损trigger。
			corrupt := name == "storage" || name == "lease" || name == "metadata"
			if corrupt {
				if e := f.s.Write(ctx, func(tx *sql.Tx) error {
					_, e := tx.Exec(`UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL WHERE id=?`, f.j.ID)
					return e
				}); e == nil {
					t.Fatal("schema accepted corrupt trigger")
				}
				for _, s := range []*storage.Store{f.s, f.other} {
					write(t, s, `PRAGMA ignore_check_constraints=ON`)
					defer write(t, s, `PRAGMA ignore_check_constraints=OFF`)
				}
			}
			before := database(t, f.s)
			if name != "commit" {
				sentinel := errors.New("control rollback")
				reached := false
				e := f.s.Write(ctx, func(tx *sql.Tx) error {
					result, e := tx.Exec(`UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL WHERE id=?`, f.j.ID)
					if name == "abort" {
						reached = e != nil
						return sentinel
					}
					if e != nil {
						return e
					}
					n, e := result.RowsAffected()
					if e != nil {
						return e
					}
					if name == "ignore" {
						reached = n == 0
						return sentinel
					}
					conditions := map[string]string{
						"identity":     `(SELECT revoked FROM sessions LIMIT 1)=1`,
						"registration": `(SELECT registration_version FROM device_push_registrations LIMIT 1)=2`,
						"settings":     `(SELECT settings_version FROM device_notification_settings LIMIT 1)=2`,
						"reason":       `(SELECT json_extract(configuration,'$.dnd.enabled') FROM device_notification_settings LIMIT 1)=0`,
						"instance":     `(SELECT instance_id FROM instance_identity)='22222222-2222-4222-8222-222222222222'`,
						"attempt":      `attempt=1`, "next": `next_attempt_at='2099-01-01T00:00:00.000000000Z'`,
						"storage": `typeof(next_attempt_at)='blob'`, "lease": `lease_hash IS NOT NULL AND lease_until_ns=1`,
						"metadata": `typeof(result_kind)='text' AND result_kind='transient'`,
					}
					var matched bool
					if e := tx.QueryRow(`SELECT state='suppressed' AND (`+conditions[name]+`) FROM push_jobs WHERE id=?`, f.j.ID).Scan(&matched); e != nil {
						return e
					}
					reached = n == 1 && matched
					return sentinel
				})
				if !errors.Is(e, sentinel) || !reached {
					t.Fatal("fault branch not reached", name, reached, e)
				}
				if before != database(t, f.s) {
					t.Fatal("control rollback changed database")
				}
			}
			if name == "commit" {
				// 先证明 callback 内的 UPDATE 与 FK 检查均可达，唯一失败发生于真实 COMMIT。
				reached := false
				e := f.s.Write(ctx, func(tx *sql.Tx) error {
					if _, e := tx.Exec(`UPDATE push_jobs SET state='suppressed',lease_hash=NULL,lease_until_ns=NULL WHERE id=?`, f.j.ID); e != nil {
						return e
					}
					var n int
					if e := tx.QueryRow(`SELECT count(*) FROM drop_fault_fk`).Scan(&n); e != nil {
						return e
					}
					reached = n == 1
					return nil
				})
				var foreignKeyError interface{ Code() int }
				if !reached || !errors.As(e, &foreignKeyError) || foreignKeyError.Code() != 787 {
					t.Fatal("deferred FK COMMIT fault unreachable", reached, e)
				}
			}
			drop(t, f, f.other, false, true)
			if before != database(t, f.s) {
				t.Fatal("partial write")
			}
			write(t, f.s, `DROP TRIGGER drop_fault`)
			drop(t, f, f.s, true, false)
		})
	}
}
func TestDropConcurrentAndCancelled(t *testing.T) {
	f := newFixture(t, "fcm", true)
	before := database(t, f.s)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	ok, e := push.NewJobRepository(f.s).DropAlertDND(canceled, f.held.Lease, f.j, f.now)
	if ok || !errors.Is(e, context.Canceled) {
		t.Fatal("cancel", ok, e)
	}
	if before != database(t, f.s) {
		t.Fatal("cancel persisted")
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	for _, s := range []*storage.Store{f.s, f.other} {
		wg.Add(1)
		go func(s *storage.Store) {
			defer wg.Done()
			ok, e := push.NewJobRepository(s).DropAlertDND(ctx, f.held.Lease, f.j, f.now)
			results <- ok
			errs <- e
		}(s)
	}
	wg.Wait()
	close(results)
	close(errs)
	n := 0
	for v := range results {
		if v {
			n++
		}
	}
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if n != 1 {
		t.Fatal("changed", n)
	}
}

func TestDropMatchesEveryExpectedField(t *testing.T) {
	f := newFixture(t, "fcm", true)
	original := f.j
	before := database(t, f.s)
	mutations := []func(*push.JobSnapshot){
		func(j *push.JobSnapshot) { j.ID += "other" }, func(j *push.JobSnapshot) { j.SessionID += "other" }, func(j *push.JobSnapshot) { j.DeviceID += "other" },
		func(j *push.JobSnapshot) { j.Purpose = "sync"; j.AlertEligible = false }, func(j *push.JobSnapshot) { j.TargetChangeSeq++ }, func(j *push.JobSnapshot) { j.Provider = "opush" },
		func(j *push.JobSnapshot) { j.RegistrationVersion++ }, func(j *push.JobSnapshot) { j.SettingsVersion++ }, func(j *push.JobSnapshot) { j.DeploymentID += "other" },
		func(j *push.JobSnapshot) { j.MessageID += "other" }, func(j *push.JobSnapshot) { j.BotID += "other" }, func(j *push.JobSnapshot) { j.BotSeq++ },
		func(j *push.JobSnapshot) { j.SessionGeneration++ }, func(j *push.JobSnapshot) { j.CreatedAt = j.CreatedAt.Add(-time.Nanosecond) },
		func(j *push.JobSnapshot) { j.ExpiresAt = j.ExpiresAt.Add(time.Nanosecond) }, func(j *push.JobSnapshot) { j.AlertEligible = false },
	}
	for i, mutate := range mutations {
		f.j = original
		mutate(&f.j)
		t.Run(fmt.Sprint(i), func(t *testing.T) { drop(t, f, f.other, false, false) })
	}
	if before != database(t, f.s) {
		t.Fatal("wrong expected wrote")
	}
	f.j = original
	drop(t, f, f.other, true, false)
}

var cancelSQL struct {
	sync.Mutex
	cancel context.CancelFunc
}
var cancellationReached atomic.Bool

func init() {
	// 函数仅用于专用测试进程；在SQLite执行AFTER trigger时取消真正的操作context。
	sqlite.MustRegisterScalarFunction("relaynest_drop_cancel", 0, func(_ *sqlite.FunctionContext, _ []driver.Value) (driver.Value, error) {
		cancelSQL.Lock()
		cancel := cancelSQL.cancel
		cancelSQL.Unlock()
		if cancel != nil {
			cancellationReached.Store(true)
			cancel()
		}
		return int64(0), nil
	})
}
func TestDropCancellationDuringSQL(t *testing.T) {
	f := newFixture(t, "fcm", true)
	write(t, f.s, `CREATE TRIGGER drop_cancel AFTER UPDATE OF state ON push_jobs BEGIN SELECT relaynest_drop_cancel();END`)
	before := database(t, f.s)
	operation, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelSQL.Lock()
	cancelSQL.cancel = cancel
	cancelSQL.Unlock()
	defer func() { cancelSQL.Lock(); cancelSQL.cancel = nil; cancelSQL.Unlock() }()
	cancellationReached.Store(false)
	ok, e := push.NewJobRepository(f.other).DropAlertDND(operation, f.held.Lease, f.j, f.now)
	if ok || e == nil || !cancellationReached.Load() {
		t.Fatal("SQL cancellation not reached", ok, e)
	}
	if before != database(t, f.s) {
		t.Fatal("SQL cancel committed")
	}
	write(t, f.s, `DROP TRIGGER drop_cancel`)
	drop(t, f, f.s, true, false)
}

func TestDropActualSyncAndInvalidInputs(t *testing.T) {
	f := newFixture(t, "fcm", true)
	r := push.NewJobRepository(f.s)
	if ok, e := r.Release(ctx, f.held.Lease, f.now); e != nil || !ok {
		t.Fatal(e)
	}
	j := f.j
	j.ID = "actual-sync"
	j.Purpose = "sync"
	j.AlertEligible = false
	if e := f.s.Write(ctx, func(tx *sql.Tx) error { return push.CreateJobTx(ctx, tx, j, f.now) }); e != nil {
		t.Fatal(e)
	}
	held, e := r.ReserveSync(ctx, f.now, time.Minute, 1)
	if e != nil || len(held) != 1 {
		t.Fatal(e)
	}
	before := database(t, f.s)
	if ok, e := r.DropAlertDND(ctx, held[0].Lease, j, f.now); e != nil || ok {
		t.Fatal("actual sync suppressed", ok, e)
	}
	var nilRepository *push.JobRepository
	for _, call := range []func() (bool, error){
		func() (bool, error) { return r.DropAlertDND(nil, f.held.Lease, f.j, f.now) },
		func() (bool, error) { return nilRepository.DropAlertDND(ctx, f.held.Lease, f.j, f.now) },
		func() (bool, error) { return push.NewJobRepository(nil).DropAlertDND(ctx, f.held.Lease, f.j, f.now) },
		func() (bool, error) { return r.DropAlertDND(ctx, push.JobLease{}, f.j, f.now) },
		func() (bool, error) { return r.DropAlertDND(ctx, f.held.Lease, push.JobSnapshot{}, f.now) },
		func() (bool, error) { return r.DropAlertDND(ctx, f.held.Lease, f.j, time.Time{}) },
	} {
		if ok, e := call(); ok || !errors.Is(e, push.ErrJobInput) {
			t.Fatal("bad input", ok, e)
		}
	}
	if before != database(t, f.s) {
		t.Fatal("invalid input changed database")
	}
	ticket, e := r.BeginSync(ctx, held[0].Lease, f.now)
	if e != nil || ticket == nil {
		t.Fatal("sync attempt changed", e)
	}
}

package opushreservestorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/push"
	"relaynest/server/internal/storage"
)

const opushTimestamp = "2006-01-02T15:04:05.000000000Z"

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
func jobRead(t *testing.T, s *storage.Store, id string) push.Job {
	t.Helper()
	var j push.Job
	if err := s.Read(jobContext, func(tx *sql.Tx) error { var e error; j, e = push.ReadJobTx(jobContext, tx, id); return e }); err != nil {
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
func (f *jobFixture) snapshot(t *testing.T, s auth.Session, purpose string) push.JobSnapshot {
	t.Helper()
	m, err := messages.New(f.store).CreateApplication(jobContext, f.identity, messages.TextInput{BotID: f.bot.Bot.ID, Format: "plain", Text: "private-job-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	f.serial++
	return push.JobSnapshot{ID: fmt.Sprintf("job-%03d", f.serial), SessionID: s.ID, DeviceID: s.DeviceID, Purpose: purpose, TargetChangeSeq: int64(m.LastChangeSeq), Provider: "fcm", RegistrationVersion: 1, SettingsVersion: 0, DeploymentID: f.deployment, MessageID: m.Message.ID, BotID: f.bot.Bot.ID, BotSeq: m.Message.BotSeq, SessionGeneration: 1, CreatedAt: f.now, ExpiresAt: f.now.Add(time.Hour), AlertEligible: purpose == "alert"}
}
func jobCreate(t *testing.T, f *jobFixture, j push.JobSnapshot) {
	t.Helper()
	if err := f.store.Write(jobContext, func(tx *sql.Tx) error { return push.CreateJobTx(jobContext, tx, j, j.CreatedAt) }); err != nil {
		t.Fatal(err)
	}
}

// 读取全部真实列，避免回收测试只比较 Job 投影而漏掉 attempt/result 等持久字段。
func scopeRows(t *testing.T, s *storage.Store) [][]any {
	t.Helper()
	var out [][]any
	err := s.Read(jobContext, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT * FROM push_jobs ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return err
		}
		for rows.Next() {
			values := make([]any, len(cols))
			targets := make([]any, len(cols))
			for i := range values {
				targets[i] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				return err
			}
			out = append(out, values)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func scopeIDs(got []push.JobReservation) []string {
	ids := make([]string, len(got))
	for i, j := range got {
		ids[i] = j.Job.Snapshot.ID
	}
	return ids
}

// 仓储夹具通过真实 push.CreateJobTx 和会话注册生成完整原快照，不冒称生产者或平台验证。
func (f *jobFixture) opushSnapshot(t *testing.T, s auth.Session) push.JobSnapshot {
	t.Helper()
	jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'opush',1,'synthetic-opush-registration',?) ON CONFLICT(session_id,provider) DO NOTHING`, s.ID, f.now.Format(time.RFC3339Nano))
	j := f.snapshot(t, s, "alert")
	j.Provider = "opush"
	return j
}

func TestReserveOPushAlertsFiltersBeforeRankAndLimit(t *testing.T) {
	f := jobSetup(t)
	a, b, c := f.session(t, jobDeviceA), f.session(t, jobDeviceB), f.session(t, "33333333-3333-4333-8333-333333333333")
	// 较早 sync、FCM alert、原 false 和 suppressed 均不可占设备 rank 或全局 limit。
	syncA, fcmB := f.snapshot(t, a, "sync"), f.snapshot(t, b, "alert")
	ineligible, suppressed := f.opushSnapshot(t, a), f.opushSnapshot(t, b)
	ineligible.AlertEligible = false
	laterA, firstA, firstB, firstC := f.opushSnapshot(t, a), f.opushSnapshot(t, a), f.opushSnapshot(t, b), f.opushSnapshot(t, c)
	for _, j := range []push.JobSnapshot{syncA, fcmB, ineligible, suppressed, laterA, firstA, firstB, firstC} {
		jobCreate(t, f, j)
	}
	jobSQL(t, f.store, `UPDATE push_jobs SET state='suppressed' WHERE id=?`, suppressed.ID)
	for _, item := range []struct {
		id    string
		delay time.Duration
	}{{laterA.ID, 3 * time.Second}, {firstA.ID, time.Second}, {firstB.ID, 2 * time.Second}, {firstC.ID, 2 * time.Second}} {
		jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.Add(item.delay).Format(opushTimestamp), item.id)
	}
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy',?,?,7,'unknown')`, c.ID, c.DeviceID)
	before := scopeRows(t, f.store)
	r := push.NewJobRepository(f.store)
	got, err := r.ReserveOPushAlerts(jobContext, f.now.Add(2*time.Second), time.Minute, 2)
	if err != nil || !reflect.DeepEqual(scopeIDs(got), []string{firstA.ID, firstB.ID}) {
		t.Fatal("scope/rank/order/limit", scopeIDs(got), err)
	}
	remaining, err := r.ReserveOPushAlerts(jobContext, f.now.Add(2*time.Second), time.Minute, push.MaxJobReservations)
	if err != nil || !reflect.DeepEqual(scopeIDs(remaining), []string{firstC.ID}) {
		t.Fatal("remaining device", scopeIDs(remaining), err)
	}
	after := scopeRows(t, f.store)
	for i := range before {
		id := before[i][0].(string)
		for col := range before[i] {
			// 原表固定 26 列；仅选中行 state、lease_hash、lease_until_ns 可以改变。
			if (id == firstA.ID || id == firstB.ID || id == firstC.ID) && (col == 4 || col == 19 || col == 20) {
				continue
			}
			if len(before[i]) != 26 || !reflect.DeepEqual(before[i][col], after[i][col]) {
				t.Fatalf("unexpected mutation %s column %d", id, col)
			}
		}
	}
	if empty, e := r.ReserveOPushAlerts(jobContext, f.now.Add(2*time.Second), time.Minute, 1); e != nil || len(empty) != 0 {
		t.Fatal("active holders bypassed", e)
	}
}

func TestReserveOPushAlertsAlertHolderRecoveryAndReopen(t *testing.T) {
	for _, holderPurpose := range []string{"sync", "alert"} {
		t.Run(holderPurpose, func(t *testing.T) {
			f := jobSetup(t)
			a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
			alert, otherAlert := f.snapshot(t, a, holderPurpose), f.snapshot(t, b, holderPurpose)
			jobCreate(t, f, alert)
			jobCreate(t, f, otherAlert)
			// 非零历史列验证最小回收不清除任何结果／尝试状态。
			jobSQL(t, f.store, `UPDATE push_jobs SET attempt=3 WHERE state='queued'`)
			r := push.NewJobRepository(f.store)
			old, err := r.Reserve(jobContext, f.now, time.Minute, 2)
			if err != nil || len(old) != 2 {
				t.Fatal(err)
			}
			sameDevice := f.session(t, jobDeviceA)
			target := f.opushSnapshot(t, sameDevice)
			jobCreate(t, f, target)
			before := scopeRows(t, f.store)
			reopened, err := storage.Open(jobContext, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			opushRepo := push.NewJobRepository(reopened)
			if got, e := opushRepo.ReserveOPushAlerts(jobContext, f.now.Add(time.Minute-time.Nanosecond), time.Minute, 1); e != nil || len(got) != 0 {
				t.Fatal("active alert bypassed", e)
			}
			got, err := opushRepo.ReserveOPushAlerts(jobContext, f.now.Add(time.Minute), time.Minute, 1)
			if err != nil || !reflect.DeepEqual(scopeIDs(got), []string{target.ID}) {
				t.Fatal("exact expired alert takeover", err)
			}
			after := scopeRows(t, f.store)
			// SELECT * 的真实列名确定唯一允许变化，其他设备即使到期也完全不动。
			if err := f.store.Read(jobContext, func(tx *sql.Tx) error {
				rows, e := tx.Query(`SELECT * FROM push_jobs LIMIT 0`)
				if e != nil {
					return e
				}
				defer rows.Close()
				cols, e := rows.Columns()
				if e != nil {
					return e
				}
				for row := range before {
					id := before[row][0].(string)
					for col, name := range cols {
						if id == alert.ID && (name == "state" || name == "lease_hash" || name == "lease_until_ns") {
							continue
						}
						if id == target.ID && (name == "state" || name == "lease_hash" || name == "lease_until_ns") {
							continue
						}
						if !reflect.DeepEqual(before[row][col], after[row][col]) {
							t.Fatalf("unexpected mutation %s.%s", id, name)
						}
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE id=? AND state='queued' AND lease_hash IS NULL AND lease_until_ns IS NULL`, alert.ID) != 1 {
				t.Fatal("expired alert not minimally reclaimed")
			}
			if jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE device_id=? AND state='leased'`, jobDeviceA) != 1 {
				t.Fatal("cross purpose device uniqueness")
			}
			if changed, e := r.Release(jobContext, old[0].Lease, f.now.Add(time.Minute)); e != nil || changed {
				t.Fatal("stale alert release", e)
			}
			// 自身到期 leased 候选可重新领取，原快照、attempt 与原 expiry 均保持。
			again, e := opushRepo.ReserveOPushAlerts(jobContext, f.now.Add(2*time.Minute), time.Minute, 1)
			if e != nil || len(again) != 1 || again[0].Job.Snapshot != target || again[0].Job.Attempt != 0 {
				t.Fatal("expired opush takeover", e)
			}
			if changed, e := r.Release(jobContext, got[0].Lease, f.now.Add(2*time.Minute)); e != nil || changed {
				t.Fatal("old opush release", e)
			}
			if changed, e := r.Release(jobContext, again[0].Lease, f.now.Add(2*time.Minute)); e != nil || !changed {
				t.Fatal("new opush release", e)
			}
		})
	}
}

func TestReserveOPushAlertsDueExpiryAndSessionBoundaries(t *testing.T) {
	for _, scenario := range []string{"future", "expiry", "idle", "absolute", "revoked", "generation", "client"} {
		t.Run(scenario, func(t *testing.T) {
			f := jobSetup(t)
			s := f.session(t, jobDeviceA)
			j := f.opushSnapshot(t, s)
			jobCreate(t, f, j)
			now := f.now
			switch scenario {
			case "future":
				jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, now.Add(time.Nanosecond).Format(opushTimestamp), j.ID)
			case "expiry":
				now = j.ExpiresAt
			case "idle":
				jobSQL(t, f.store, `UPDATE sessions SET idle_expires_at=? WHERE id=?`, now.UnixNano(), s.ID)
			case "absolute":
				jobSQL(t, f.store, `UPDATE sessions SET idle_expires_at=?,absolute_expires_at=? WHERE id=?`, now.UnixNano(), now.UnixNano(), s.ID)
			case "revoked":
				jobSQL(t, f.store, `UPDATE sessions SET revoked=1 WHERE id=?`, s.ID)
			case "generation":
				jobSQL(t, f.store, `UPDATE administrator SET auth_generation=2 WHERE id=1`)
			case "client":
				jobSQL(t, f.store, `UPDATE sessions SET client_type='web' WHERE id=?`, s.ID)
			}
			before := scopeRows(t, f.store)
			r := push.NewJobRepository(f.store)
			if got, e := r.ReserveOPushAlerts(jobContext, now, time.Minute, 1); e != nil || len(got) != 0 {
				t.Fatal("boundary accepted", e)
			}
			if !reflect.DeepEqual(before, scopeRows(t, f.store)) {
				t.Fatal("rejected candidate mutated")
			}
			if scenario == "future" {
				got, e := r.ReserveOPushAlerts(jobContext, now.Add(time.Nanosecond), time.Minute, 1)
				if e != nil || len(got) != 1 {
					t.Fatal("due equality", e)
				}
			}
		})
	}
}

func TestReserveOPushAlertsCompetesWithSyncAcrossStores(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	jobCreate(t, f, f.snapshot(t, s, "sync"))
	jobCreate(t, f, f.opushSnapshot(t, s))
	start := make(chan struct{})
	out := make(chan []push.JobReservation, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var got []push.JobReservation
			var err error
			if i%2 == 0 {
				got, err = push.NewJobRepository(f.store).ReserveSync(jobContext, f.now, time.Minute, 1)
			} else {
				got, err = push.NewJobRepository(f.other).ReserveOPushAlerts(jobContext, f.now, time.Minute, 1)
			}
			out <- got
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(out)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	count := 0
	for got := range out {
		count += len(got)
	}
	if count != 1 || jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE state='leased'`) != 1 {
		t.Fatal("duplicate device lease", count)
	}
}

// 两个设备各有到期 FCM alert，第二个 opush alert 故障时逐列证明整批回滚。
func TestReserveOPushAlertsBatchFaultsAndCommit(t *testing.T) {
	for _, fault := range []string{"sql", "scan", "zero", "commit"} {

		t.Run(fault, func(t *testing.T) {
			f := jobSetup(t)
			a, b := f.session(t, jobDeviceA), f.session(t, jobDeviceB)
			jobCreate(t, f, f.snapshot(t, a, "alert"))
			jobCreate(t, f, f.snapshot(t, b, "alert"))
			r := push.NewJobRepository(f.store)
			if got, e := r.Reserve(jobContext, f.now, time.Minute, 2); e != nil || len(got) != 2 {
				t.Fatal(e)
			}
			one, two := f.opushSnapshot(t, a), f.opushSnapshot(t, b)
			jobCreate(t, f, one)
			jobCreate(t, f, two)
			jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.AddDate(-2, 0, 0).Format(opushTimestamp), one.ID)
			// 第二项更新前实查第一项已领取、两个旧 FCM alert 已回收，防止排序错误使故障提前。
			// 守卫错误刻意不同于各预期故障，下面必须核对具体错误，不能任意错误都算通过。
			orderGuard := `SELECT CASE WHEN (SELECT state FROM push_jobs WHERE id='job-003') IS NOT 'leased'
 OR (SELECT state FROM push_jobs WHERE id='job-001') IS NOT 'queued'
 OR (SELECT state FROM push_jobs WHERE id='job-002') IS NOT 'queued'
 THEN RAISE(ABORT,'scope order guard failed') END;`
			if fault == "scan" || fault == "commit" {
				jobSQL(t, f.store, `CREATE TRIGGER scope_order BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` END`)
			}
			expectFault := func(err error) {
				t.Helper()
				valid := err != nil
				switch fault {
				case "sql":
					valid = valid && strings.Contains(err.Error(), "scope batch fault")
				case "scan", "zero":
					valid = errors.Is(err, push.ErrJobData)
				case "commit":
					valid = valid && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
				}
				if !valid {
					t.Fatalf("wrong %s fault: %v", fault, err)
				}
			}
			switch fault {
			case "sql":
				jobSQL(t, f.store, `CREATE TRIGGER scope_fault BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` SELECT RAISE(ABORT,'scope batch fault'); END`)
			case "scan":
				// SQLite 会归一化二月三十日，因此通过原 CHECK；Go 日历解析拒绝。
				// 第一个 opush alert 的时间更早，守卫确保此错误发生在第二项读取阶段。
				jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, fmt.Sprintf("%04d-02-30T00:00:00.000000000Z", f.now.Year()-1), two.ID)
			case "zero":
				jobSQL(t, f.store, `CREATE TRIGGER scope_fault BEFORE UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN `+orderGuard+` SELECT RAISE(IGNORE); END`)
			case "commit":
				jobSQL(t, f.store, `CREATE TABLE scope_commit(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER scope_fault AFTER UPDATE ON push_jobs WHEN NEW.state='leased' AND NEW.id='job-004' BEGIN INSERT INTO scope_commit VALUES('absent'); END`)
			}
			before := scopeRows(t, f.store)
			now := f.now.Add(time.Minute)

			got, e := r.ReserveOPushAlerts(jobContext, now, time.Minute, 2)
			if got != nil {
				t.Fatal("fault published candidates")
			}
			expectFault(e)

			if !reflect.DeepEqual(before, scopeRows(t, f.other)) {
				t.Fatal("partial reclaim/reservation survived")
			}
			if fault == "commit" && jobCount(t, f.store, `SELECT count(*) FROM scope_commit`) != 0 {
				t.Fatal("failed commit persisted side effect")
			}
		})

	}
}

func TestReserveOPushAlertsCancellationAndInputs(t *testing.T) {
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	jobCreate(t, f, f.opushSnapshot(t, s))
	r := push.NewJobRepository(f.other)
	before := scopeRows(t, f.store)
	canceled, cancel := context.WithCancel(jobContext)
	cancel()
	if got, e := r.ReserveOPushAlerts(canceled, f.now, time.Minute, 1); !errors.Is(e, context.Canceled) || got != nil {
		t.Fatal("canceled reserve", e)
	}
	if err := f.store.Write(jobContext, func(*sql.Tx) error {
		ctx, cancel := context.WithTimeout(jobContext, 10*time.Millisecond)
		defer cancel()
		got, e := r.ReserveOPushAlerts(ctx, f.now, time.Minute, 1)
		if !errors.Is(e, context.DeadlineExceeded) || got != nil {
			return fmt.Errorf("waiting cancellation: %v", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, repo := range []*push.JobRepository{nil, push.NewJobRepository(nil), r} {
		if got, e := repo.ReserveOPushAlerts(nil, f.now, time.Minute, 1); !errors.Is(e, push.ErrJobInput) || got != nil {
			t.Fatal("nil input", e)
		}
	}
	for _, repo := range []*push.JobRepository{nil, push.NewJobRepository(nil)} {
		if got, e := repo.ReserveOPushAlerts(jobContext, f.now, time.Minute, 1); !errors.Is(e, push.ErrJobInput) || got != nil {
			t.Fatal("nil repository", e)
		}
	}
	for _, tc := range []struct {
		now   time.Time
		ttl   time.Duration
		limit int
	}{
		{f.now, 0, 1}, {f.now, -1, 1}, {f.now, time.Minute, 0}, {f.now, time.Minute, push.MaxJobReservations + 1},
		{time.Time{}, time.Minute, 1}, {time.Unix(0, 1), -time.Second, 1},
		{time.Unix(0, 1<<63-1), time.Nanosecond, 1}, {time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC), time.Minute, 1},
	} {
		if got, e := r.ReserveOPushAlerts(jobContext, tc.now, tc.ttl, tc.limit); !errors.Is(e, push.ErrJobInput) || got != nil {
			t.Fatal("parameter boundary", e)
		}
	}

	if !reflect.DeepEqual(before, scopeRows(t, f.store)) {
		t.Fatal("invalid input mutated")
	}
}

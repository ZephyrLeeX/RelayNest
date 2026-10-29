package push

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

func leaseFixture(t *testing.T) (*jobFixture, JobReservation) {
	t.Helper()
	f := jobSetup(t)
	s := f.session(t, jobDeviceA)
	j := f.snapshot(t, s, "sync")
	jobCreate(t, f, j)
	got, err := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(got) != 1 {
		t.Fatal("reserve", err, len(got))
	}
	return f, got[0]
}

// 独立枚举所有真实表；检查不能通过遗漏job/auth表掩盖任何状态写入。
func leaseDigest(t *testing.T, store *storage.Store) [32]byte {
	t.Helper()
	var all []any
	err := store.Read(jobContext, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
		if err != nil {
			return err
		}
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			names = append(names, name)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, name := range names {
			rows, err := tx.Query(`SELECT * FROM "` + strings.ReplaceAll(name, `"`, `""`) + `" ORDER BY 1`)
			if err != nil {
				return err
			}
			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			for rows.Next() {
				values, pointers := make([]any, len(cols)), make([]any, len(cols))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err := rows.Scan(pointers...); err != nil {
					rows.Close()
					return err
				}
				all = append(all, []any{name, values})
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// 复合键的第一列可重复；按完整行的有类型编码排序，保留重复行和所有表。
	encoded := make([]string, 0, len(all))
	for _, row := range all {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, string(raw))
	}
	sort.Strings(encoded)
	raw, err := json.Marshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

func leaseExpect(t *testing.T, store *storage.Store, lease JobLease, now time.Time, valid bool) *SyncLeaseSnapshot {
	t.Helper()
	before := leaseDigest(t, store)
	var got *SyncLeaseSnapshot
	err := store.Read(jobContext, func(tx *sql.Tx) error {
		var err error
		got, err = CheckSyncLeaseTx(jobContext, tx, lease, now)
		return err
	})
	if err != nil || (got != nil) != valid {
		t.Fatalf("check valid=%v got=%v err=%v", valid, got != nil, err)
	}
	if before != leaseDigest(t, store) {
		t.Fatal("read-only check changed database")
	}
	return got
}

func TestSyncLeaseCheckFencingReleaseTakeoverAndReopen(t *testing.T) {
	f, held := leaseFixture(t)
	checked := leaseExpect(t, f.other, held.Lease, f.now, true)
	if !reflect.DeepEqual(checked.Job, held.Job) {
		t.Fatal("authoritative snapshot changed")
	}
	raw, err := json.Marshal(checked)
	if err != nil {
		t.Fatal(err)
	}
	printed := fmt.Sprintf("%v %+v %#v", checked, checked, checked)
	for _, secret := range []string{hex.EncodeToString(held.Lease.secret[:]), "synthetic-registration-secret", "private-job-fixture"} {
		if strings.Contains(string(raw), secret) || strings.Contains(printed, secret) {
			t.Fatal("sensitive check result")
		}
	}
	wrong := held.Lease
	wrong.secret[0] ^= 1
	leaseExpect(t, f.store, wrong, f.now, false)
	wrong = held.Lease
	wrong.until = wrong.until.Add(time.Nanosecond)
	leaseExpect(t, f.store, wrong, f.now, false)
	wrong = held.Lease
	wrong.jobID = "missing-job"
	leaseExpect(t, f.store, wrong, f.now, false)
	leaseExpect(t, f.store, held.Lease, held.Lease.until, false)
	leaseExpect(t, f.store, held.Lease, f.now.Add(-time.Nanosecond), false)

	taken, err := NewJobRepository(f.other).Reserve(jobContext, held.Lease.until, time.Minute, 1)
	if err != nil || len(taken) != 1 {
		t.Fatal("takeover", err)
	}
	leaseExpect(t, f.store, held.Lease, held.Lease.until, false)
	leaseExpect(t, f.store, taken[0].Lease, held.Lease.until, true)
	reopened, err := storage.Open(jobContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	leaseExpect(t, reopened, taken[0].Lease, held.Lease.until, true)
	if ok, err := NewJobRepository(f.other).Release(jobContext, taken[0].Lease, held.Lease.until); err != nil || !ok {
		t.Fatal("release", err)
	}
	leaseExpect(t, reopened, taken[0].Lease, held.Lease.until, false)
	if jobRead(t, f.store, held.Job.Snapshot.ID).Attempt != 0 {
		t.Fatal("check advanced attempt")
	}
}

func TestSyncLeaseCheckMixedJobsAndNoPolicyPromotion(t *testing.T) {
	f, sync := leaseFixture(t)
	s := f.session(t, jobDeviceB)
	alert := f.snapshot(t, s, "alert")
	jobCreate(t, f, alert)
	jobSQL(t, f.store, `INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES('legacy',?,?,1,?)`, s.ID, s.DeviceID, f.now.Format(time.RFC3339Nano))
	alerts, err := NewJobRepository(f.store).Reserve(jobContext, f.now, time.Minute, 1)
	if err != nil || len(alerts) != 1 || alerts[0].Job.Snapshot.ID != alert.ID {
		t.Fatal("mixed reserve", err)
	}
	leaseExpect(t, f.store, alerts[0].Lease, f.now, false)
	leaseExpect(t, f.store, JobLease{jobID: "legacy", secret: [32]byte{1}, until: f.now.Add(time.Minute)}, f.now, false)
	// 原消息消失、所有内容已读和DND不在本层提升成静默sync的禁令。
	jobSQL(t, f.store, `DELETE FROM messages WHERE id=?`, sync.Job.Snapshot.MessageID)
	jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, sync.Job.Snapshot.BotSeq, sync.Job.Snapshot.BotID)
	leaseExpect(t, f.other, sync.Lease, f.now, true)
	if jobRead(t, f.store, alert.ID).Snapshot != alert || jobCount(t, f.store, `SELECT count(*) FROM push_jobs WHERE state='pending'`) != 1 {
		t.Fatal("mixed categories changed")
	}
}

func TestSyncLeaseCheckDueAndEventExpiry(t *testing.T) {
	f, held := leaseFixture(t)
	jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.Add(time.Nanosecond).Format(jobTimestamp), held.Job.Snapshot.ID)
	leaseExpect(t, f.store, held.Lease, f.now, false)
	leaseExpect(t, f.store, held.Lease, f.now.Add(time.Nanosecond), true)
	jobSQL(t, f.store, `UPDATE push_jobs SET next_attempt_at=? WHERE id=?`, f.now.Format(jobTimestamp), held.Job.Snapshot.ID)
	if ok, err := NewJobRepository(f.store).Release(jobContext, held.Lease, f.now); err != nil || !ok {
		t.Fatal(err)
	}
	long, err := NewJobRepository(f.store).Reserve(jobContext, f.now, 2*time.Hour, 1)
	if err != nil || len(long) != 1 {
		t.Fatal(err)
	}
	leaseExpect(t, f.store, long[0].Lease, held.Job.Snapshot.ExpiresAt.Add(-time.Nanosecond), true)
	leaseExpect(t, f.store, long[0].Lease, held.Job.Snapshot.ExpiresAt, false)
}

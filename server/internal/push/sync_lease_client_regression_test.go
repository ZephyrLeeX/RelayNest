package push

import (
	"database/sql"
	"testing"
)

// 保留真实触发器及外键：先证明原撤销触发器删映射，再构造坏会话仍带原注册的坏库。
func TestSyncLeaseCheckCurrentSessionClientTextStorageRegression(t *testing.T) {
	f, held := leaseFixture(t)
	leaseExpect(t, f.store, held.Lease, f.now, true)
	jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
	jobSQL(t, f.store, `UPDATE sessions SET client_type=CAST(client_type AS BLOB) WHERE id=?`, held.Job.Snapshot.SessionID)
	jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
	if jobCount(t, f.store, `SELECT count(*) FROM device_push_registrations`) != 0 {
		t.Fatal("original invalidation trigger did not run")
	}
	leaseExpect(t, f.other, held.Lease, f.now, false)
	// 可信坏库夹具恢复原映射；不声称生产HTTP可对畸形会话注册。
	jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic-registration-secret',?)`, held.Job.Snapshot.SessionID, f.now.Format(jobTimestamp))
	before := leaseDigest(t, f.store)
	if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
		var kind string
		var android, checksIgnored bool
		if err := tx.QueryRowContext(jobContext, `SELECT typeof(client_type),client_type='android' FROM sessions WHERE id=?`, held.Job.Snapshot.SessionID).Scan(&kind, &android); err != nil {
			return err
		}
		if err := tx.QueryRowContext(jobContext, `PRAGMA ignore_check_constraints`).Scan(&checksIgnored); err != nil {
			return err
		}
		if kind != "blob" || android || checksIgnored {
			t.Fatal("bad fixture", kind, android, checksIgnored)
		}
		got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now)
		t.Logf("sqlite_type=%s sql_android=%v candidate=%v error=%v", kind, android, got != nil, err)
		if got != nil || err != nil {
			t.Errorf("non-TEXT current client candidate=%v error=%v", got != nil, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if before != leaseDigest(t, f.other) {
		t.Fatal("checker wrote data")
	}
	jobSQL(t, f.store, `UPDATE sessions SET client_type='android' WHERE id=?`, held.Job.Snapshot.SessionID)
	jobSQL(t, f.store, `INSERT INTO device_push_registrations VALUES(?,'fcm',1,'synthetic-registration-secret',?)`, held.Job.Snapshot.SessionID, f.now.Format(jobTimestamp))
	leaseExpect(t, f.other, held.Lease, f.now, true)
}

// 合法非 Android 文本及缺会话仍为普通无候选，不升级成坏数据错误。
func TestSyncLeaseCheckCurrentSessionClientAbsentControls(t *testing.T) {
	for _, query := range []string{
		`UPDATE sessions SET client_type='web' WHERE id=?`,
		`DELETE FROM sessions WHERE id=?`,
	} {
		t.Run(query, func(t *testing.T) {
			f, held := leaseFixture(t)
			leaseExpect(t, f.store, held.Lease, f.now, true)
			jobSQL(t, f.store, query, held.Job.Snapshot.SessionID)
			before := leaseDigest(t, f.store)
			if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
				got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now)
				if got != nil || err != nil {
					t.Fatalf("ordinary missing eligibility candidate=%v error=%v", got != nil, err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if before != leaseDigest(t, f.other) {
				t.Fatal("control check wrote data")
			}
		})
	}
}

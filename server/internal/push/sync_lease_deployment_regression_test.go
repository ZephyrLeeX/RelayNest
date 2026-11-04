package push

import (
	"database/sql"
	"errors"
	"testing"

	"relaynest/server/internal/storage"
)

// 部署身份的同值 BLOB 必须失败关闭，恢复合法 TEXT 后原租约仍可用。
func TestSyncLeaseCheckDeploymentTextStorageRegression(t *testing.T) {
	f, held := leaseFixture(t)
	leaseExpect(t, f.store, held.Lease, f.now, true)
	original := held.Job.Snapshot.DeploymentID
	jobSQL(t, f.store, `PRAGMA ignore_check_constraints=ON`)
	jobSQL(t, f.store, `UPDATE instance_identity SET instance_id=CAST(instance_id AS BLOB) WHERE id=1`)
	jobSQL(t, f.store, `PRAGMA ignore_check_constraints=OFF`)
	before := leaseDigest(t, f.store)
	if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
		var kind string
		var checksIgnored bool
		if err := tx.QueryRowContext(jobContext, `SELECT typeof(instance_id) FROM instance_identity WHERE id=1`).Scan(&kind); err != nil {
			return err
		}
		if err := tx.QueryRowContext(jobContext, `PRAGMA ignore_check_constraints`).Scan(&checksIgnored); err != nil {
			return err
		}
		if kind != "blob" || checksIgnored {
			t.Fatal("invalid deployment fixture", kind, checksIgnored)
		}
		// 共享读取的现有字符串语义不变，本叶独立保证原存储类。
		current, err := storage.ReadInstanceIDTx(jobContext, tx)
		if err != nil || current != original {
			t.Fatal("shared instance identity semantics changed", err)
		}
		got, err := CheckSyncLeaseTx(jobContext, tx, held.Lease, f.now)
		if got != nil || !errors.Is(err, ErrJobData) {
			t.Fatalf("damaged deployment candidate=%v error=%v", got != nil, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if before != leaseDigest(t, f.other) {
		t.Fatal("deployment validation wrote database")
	}
	jobSQL(t, f.store, `UPDATE instance_identity SET instance_id=? WHERE id=1`, original)
	leaseExpect(t, f.other, held.Lease, f.now, true)
}

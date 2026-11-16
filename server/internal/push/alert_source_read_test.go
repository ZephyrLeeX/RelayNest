package push

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"relaynest/server/internal/devices"
	sharedsync "relaynest/server/internal/sync"
)

// 每次独立读取均核对所有真实表，原因不构成数据库修改或终止许可。
func sourceReadExpect(t *testing.T, f *jobFixture, j JobSnapshot, want alertSourceReadDecision, wantError error) {
	t.Helper()
	before := leaseDigest(t, f.store)
	if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
		got, err := evaluateAlertSourceReadTx(jobContext, tx, j)
		if got != want || !errors.Is(err, wantError) {
			t.Fatalf("source/read=%q error=%v want=%q error=%v", got, err, want, wantError)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if before != leaseDigest(t, f.store) {
		t.Fatal("source/read wrote database")
	}
}

func TestAlertSourceReadProjection(t *testing.T) {
	for _, name := range []string{"unread", "missing", "wrong-bot", "wrong-seq", "equal", "above", "below", "missing-read", "bad-read", "blob-read", "bad-version", "bad-time", "bad-upper", "blob-source"} {
		t.Run(name, func(t *testing.T) {
			f, j, _ := reasonFixture(t)
			want, wantErr := alertSourceUnread, error(nil)
			switch name {
			case "missing":
				jobSQL(t, f.store, `DELETE FROM messages WHERE id=?`, j.MessageID)
				// 源缺失优先于已读状态损坏，不能将错误当作“已读”。
				jobSQL(t, f.store, `DELETE FROM bot_read_states WHERE bot_id=?`, j.BotID)
				want = alertSourceMissing
			case "wrong-bot":
				j.BotID = jobDeviceB
				want = alertSourceMismatch
			case "wrong-seq":
				j.BotSeq++
				jobSQL(t, f.store, `DELETE FROM bot_read_states WHERE bot_id=?`, j.BotID)
				want = alertSourceMismatch
			case "equal":
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq, j.BotID)
				want = alertSourceRead
			case "above":
				_ = f.snapshot(t, f.session(t, jobDeviceB), "alert")
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq+1, j.BotID)
				want = alertSourceRead
			case "below":
				j = f.snapshot(t, f.session(t, jobDeviceB), "alert")
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq-1, j.BotID)
			case "missing-read":
				jobSQL(t, f.store, `DELETE FROM bot_read_states WHERE bot_id=?`, j.BotID)
				want, wantErr = "", sharedsync.ErrReadStateUnavailable
			case "bad-read":
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=999 WHERE bot_id=?`, j.BotID)
				want, wantErr = "", sharedsync.ErrReadStateUnavailable
			case "blob-read":
				reasonCorrupt(t, f, `UPDATE bot_read_states SET read_seq=CAST('0' AS BLOB) WHERE bot_id=?`, j.BotID)
				want, wantErr = "", sharedsync.ErrReadStateUnavailable
			case "bad-version":
				reasonCorrupt(t, f, `UPDATE bot_read_states SET state_version=CAST('1' AS BLOB) WHERE bot_id=?`, j.BotID)
				want, wantErr = "", sharedsync.ErrReadStateUnavailable
			case "bad-time":
				jobSQL(t, f.store, `UPDATE bot_read_states SET updated_at='bad' WHERE bot_id=?`, j.BotID)
				want, wantErr = "", sharedsync.ErrReadStateUnavailable
			case "bad-upper":
				reasonCorrupt(t, f, `UPDATE bots SET bot_seq=CAST('1' AS BLOB) WHERE id=?`, j.BotID)
				want, wantErr = "", sharedsync.ErrReadStateUnavailable
			case "blob-source":
				reasonCorrupt(t, f, `UPDATE messages SET bot_seq=CAST('1' AS BLOB) WHERE id=?`, j.MessageID)
				want, wantErr = "", ErrJobData
			}
			sourceReadExpect(t, f, j, want, wantErr)
		})
	}
}

func TestAlertSourceReadTransactionAndInput(t *testing.T) {
	f, j, _ := reasonFixture(t)
	before := leaseDigest(t, f.store)
	check := func(ctx context.Context, tx *sql.Tx, snapshot JobSnapshot, want error) {
		t.Helper()
		got, err := evaluateAlertSourceReadTx(ctx, tx, snapshot)
		if got != "" || !errors.Is(err, want) {
			t.Fatalf("%q %v want %v", got, err, want)
		}
	}
	check(jobContext, nil, j, ErrJobInput)
	var closed *sql.Tx
	if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
		check(nil, tx, j, ErrJobInput)
		ctx, cancel := context.WithCancel(jobContext)
		cancel()
		check(ctx, tx, j, context.Canceled)
		for _, mutate := range []func(*JobSnapshot){
			func(j *JobSnapshot) { j.DeviceID = "" },
			func(j *JobSnapshot) { j.MessageID = "" },
			func(j *JobSnapshot) { j.BotID = "" },
			func(j *JobSnapshot) { j.BotSeq = 0 },
			func(j *JobSnapshot) { j.CreatedAt = time.Time{} },
		} {
			bad := j
			mutate(&bad)
			check(jobContext, tx, bad, ErrJobInput)
		}
		closed = tx
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check(jobContext, closed, j, sql.ErrTxDone)
	if before != leaseDigest(t, f.store) {
		t.Fatal("invalid input wrote database")
	}
}

func TestAlertSourceReadSQLFailure(t *testing.T) {
	for _, table := range []string{"messages", "bot_read_states"} {
		t.Run(table, func(t *testing.T) {
			f, j, _ := reasonFixture(t)
			before := leaseDigest(t, f.store)
			rollback := errors.New("rollback injected schema failure")
			err := f.store.Write(jobContext, func(tx *sql.Tx) error {
				if _, err := tx.Exec(`DROP TABLE ` + table); err != nil {
					return err
				}
				got, err := evaluateAlertSourceReadTx(jobContext, tx, j)
				var sqlite interface{ Code() int }
				if got != "" || !errors.As(err, &sqlite) {
					t.Fatalf("SQL failure became %q / %v", got, err)
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
			if before != leaseDigest(t, f.store) {
				t.Fatal("schema failure did not roll back")
			}
			sourceReadExpect(t, f, j, alertSourceUnread, nil)
		})
	}
}

func TestAlertSourceReadSnapshot(t *testing.T) {
	for _, change := range []string{"read", "delete"} {
		t.Run(change, func(t *testing.T) {
			f, j, _ := reasonFixture(t)
			var afterWrite [32]byte
			if err := f.other.Read(jobContext, func(tx *sql.Tx) error {
				check := func() {
					t.Helper()
					got, err := evaluateAlertSourceReadTx(jobContext, tx, j)
					if got != alertSourceUnread || err != nil {
						t.Fatalf("lost read snapshot: %q / %v", got, err)
					}
				}
				before := leaseDigest(t, f.store)
				check()
				if before != leaseDigest(t, f.store) {
					t.Fatal("initial read wrote database")
				}
				if change == "delete" {
					jobSQL(t, f.store, `DELETE FROM messages WHERE id=?`, j.MessageID)
				} else if err := f.store.Write(jobContext, func(writer *sql.Tx) error {
					_, _, err := sharedsync.AdvanceReadTx(jobContext, writer, j.BotID, j.BotSeq)
					return err
				}); err != nil {
					t.Fatal(err)
				}
				afterWrite = leaseDigest(t, f.store)
				check()
				if afterWrite != leaseDigest(t, f.store) {
					t.Fatal("old snapshot read wrote database")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			want := alertSourceRead
			if change == "delete" {
				want = alertSourceMissing
			}
			sourceReadExpect(t, f, j, want, nil)
			if afterWrite != leaseDigest(t, f.store) {
				t.Fatal("fresh transaction wrote database")
			}
		})
	}
}

func TestAlertSourceReadPolicyOrder(t *testing.T) {
	for _, name := range []string{"deleted", "read", "damaged-read", "disabled"} {
		t.Run(name, func(t *testing.T) {
			f, j, c := reasonFixture(t)
			c.DND = devices.NotificationDND{Enabled: true, Timezone: "UTC", Start: f.now.Format("15:04"), End: f.now.Add(time.Hour).Format("15:04")}
			want := alertPolicyDND
			switch name {
			case "deleted":
				jobSQL(t, f.store, `DELETE FROM messages WHERE id=?`, j.MessageID)
			case "read":
				jobSQL(t, f.store, `UPDATE bot_read_states SET read_seq=? WHERE bot_id=?`, j.BotSeq, j.BotID)
			case "damaged-read":
				jobSQL(t, f.store, `UPDATE bot_read_states SET updated_at='bad' WHERE bot_id=?`, j.BotID)
			case "disabled":
				c.Enabled = false
				jobSQL(t, f.store, `DELETE FROM bot_read_states WHERE bot_id=?`, j.BotID)
				want = alertPolicyOtherDenied
			}
			reasonSettings(t, f, j, c)
			j.SettingsVersion = 1
			reasonExpectDecision(t, f, j, f.now, want, false)
		})
	}
}

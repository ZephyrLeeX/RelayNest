package sync_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
	read "relaynest/server/internal/sync"
)

type securityFixture struct {
	*fixture
	sessions *auth.SessionStore
	actors   [2]auth.AdminRequest
	instance string
	entered  bool
}

func securitySetup(t *testing.T) *securityFixture {
	t.Helper()
	f := &securityFixture{fixture: setup(t)}
	seed(t, f.fixture, 3)
	execSQL(t, f.s, `INSERT INTO administrator VALUES(1,'admin','synthetic-hash',1)`)
	f.sessions = auth.NewSessionStore(f.s)
	if err := f.s.Read(ctx, func(tx *sql.Tx) error { var err error; f.instance, err = storage.ReadInstanceIDTx(ctx, tx); return err }); err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.NewOriginPolicy("https://relaynest.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	security := auth.NewRequestSecurity(f.sessions, policy)
	for i, id := range []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"} {
		_, secrets, err := f.sessions.Create(ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: id, DeviceName: "phone"})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("POST", "https://relaynest.test/client/read", nil)
		request.Header.Set("Authorization", "Bearer "+secrets.Token)
		request.Header.Set("X-RelayNest-Instance-ID", f.instance)
		var ok bool
		f.actors[i], ok = security.Authorize(httptest.NewRecorder(), request, auth.RequestWrite)
		if !ok {
			t.Fatal("real request authorization failed")
		}
	}
	return f
}

// 此包装仅是测试里的受信调用方，提交/末次复核失败必须丢弃候选。
func (f *securityFixture) advance(c context.Context, actor auth.AdminRequest, n int64, work func(*sql.Tx) error) (read.ReadState, error) {
	var candidate read.ReadState
	f.entered = false
	err := f.sessions.WithAuthorizedWrite(c, actor, func(tx *sql.Tx) error {
		f.entered = true
		var err error
		candidate, _, err = read.AdvanceReadTx(c, tx, f.bot.Bot.ID, n)
		if err != nil {
			return err
		}
		if work != nil {
			return work(tx)
		}
		return nil
	})
	if err != nil {
		return read.ReadState{}, err
	}
	return candidate, nil
}
func idle(t *testing.T, f *securityFixture) int64 {
	return scalar(t, f.other, `SELECT idle_expires_at FROM sessions WHERE id=?`, f.actors[0].Session.ID)
}
func TestReadRealTwoDeviceAuthorization(t *testing.T) {
	f := securitySetup(t)
	first, err := f.advance(ctx, f.actors[0], 2, nil)
	if err != nil || first.ReadSeq != 2 || first.UnreadCount != 1 {
		t.Fatal(first, err)
	}
	old, err := f.advance(ctx, f.actors[1], 1, nil)
	if err != nil || old != first {
		t.Fatal("old device regressed", old, err)
	}
	last, err := f.advance(ctx, f.actors[1], 3, nil)
	if err != nil || last.ReadSeq != 3 || last.StateVersion != 3 {
		t.Fatal(last, err)
	}
	var got read.ReadState
	_, _, err = f.sessions.WithAuthorizedRead(ctx, f.actors[0], func(tx *sql.Tx) error {
		var err error
		got, err = read.GetReadStateTx(ctx, tx, f.bot.Bot.ID)
		return err
	})
	if err != nil || got != last {
		t.Fatal("authorized GET", got, err)
	}
}
func TestReadAuthorizationDenialsDoNotEnter(t *testing.T) {
	for _, mode := range []string{"forged", "tampered", "revoked", "generation", "instance", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := securitySetup(t)
			actor := f.actors[0]
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			switch mode {
			case "forged":
				actor = auth.AdminRequest{Session: actor.Session, Source: actor.Source}
			case "tampered":
				actor.Session.DeviceID = f.actors[1].Session.DeviceID
			case "revoked":
				execSQL(t, f.other, `UPDATE sessions SET revoked=1 WHERE id=?`, actor.Session.ID)
			case "generation":
				execSQL(t, f.other, `UPDATE administrator SET auth_generation=auth_generation+1`)
			case "instance":
				if err := f.other.Write(ctx, func(tx *sql.Tx) error { _, err := storage.RotateInstanceIDTx(ctx, tx, f.instance); return err }); err != nil {
					t.Fatal(err)
				}
				observer, err := storage.Open(ctx, f.cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer observer.Close()
				f.other.Close()
				f.other = observer
			case "cancel":
				cancel()
			}
			before := get(t, f.other, f.bot.Bot.ID)
			beforeIdle := idle(t, f)
			out, err := f.advance(c, actor, 1, nil)
			if err == nil || f.entered || out != (read.ReadState{}) {
				t.Fatal("unauthorized entered", out, err)
			}
			if get(t, f.other, f.bot.Bot.ID) != before || idle(t, f) != beforeIdle || scalar(t, f.other, `SELECT count(*) FROM changes WHERE type='read.updated'`) != 0 {
				t.Fatal("denial mutated state")
			}
		})
	}
}
func TestReadFinalReauthorizationRollback(t *testing.T) {
	for _, mode := range []string{"revoked", "generation", "instance", "renew-trigger", "commit", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := securitySetup(t)
			execSQL(t, f.s, `UPDATE sessions SET idle_expires_at=?`, time.Now().Add(time.Hour).UnixNano())
			before := get(t, f.s, f.bot.Bot.ID)
			beforeIdle := idle(t, f)
			if mode == "renew-trigger" {
				execSQL(t, f.s, `CREATE TRIGGER read_renew AFTER UPDATE OF idle_expires_at ON sessions BEGIN UPDATE administrator SET auth_generation=auth_generation+1; END`)
			}
			if mode == "commit" {
				execSQL(t, f.s, `CREATE TABLE read_commit_failure(id INTEGER REFERENCES administrator(id) DEFERRABLE INITIALLY DEFERRED)`)
			}
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			workOK := false
			out, err := f.advance(c, f.actors[0], 1, func(tx *sql.Tx) error {
				var err error
				switch mode {
				case "revoked":
					_, err = tx.Exec(`UPDATE sessions SET revoked=1`)
				case "generation":
					_, err = tx.Exec(`UPDATE administrator SET auth_generation=auth_generation+1`)
				case "instance":
					_, err = storage.RotateInstanceIDTx(ctx, tx, f.instance)
				case "commit":
					_, err = tx.Exec(`INSERT INTO read_commit_failure VALUES(2)`)
				case "cancel":
					cancel()
				}
				workOK = err == nil
				return err
			})
			if err == nil || !f.entered || !workOK || out != (read.ReadState{}) {
				t.Fatal("final failure released state", out, err)
			}
			if get(t, f.other, f.bot.Bot.ID) != before || idle(t, f) != beforeIdle || scalar(t, f.other, `SELECT count(*) FROM changes WHERE type='read.updated'`) != 0 {
				t.Fatal("final failure not atomic")
			}
			if mode == "renew-trigger" {
				execSQL(t, f.s, `DROP TRIGGER read_renew`)
			}
			if _, err := f.advance(ctx, f.actors[0], 1, nil); err != nil {
				t.Fatal("failed transaction changed auth", err)
			}
		})
	}
}

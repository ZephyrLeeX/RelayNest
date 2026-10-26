package devices

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/storage"
	"relaynest/server/migrations"
)

func TestPushDelayedResultsAndSecretBoundary(t *testing.T) {
	f := newPushFixture(t)
	putPush(t, f, 0, PushFCM, 1, 0, "old-secret")
	putPush(t, f, 0, PushOPush, 1, 0, "opush-secret")
	putPush(t, f, 1, PushFCM, 1, 0, "other-secret")
	if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], notificationInput()); err != nil {
		t.Fatal(err)
	}
	targets, err := f.repo.ReadPushTargets(f.ctx)
	if err != nil || len(targets) != 3 {
		t.Fatal("targets", err)
	}
	var target PushTarget
	for _, candidate := range targets {
		if candidate.SessionID() == f.actors[0].Session.ID && candidate.Provider() == PushFCM {
			target = candidate
		}
	}
	if target.DeviceID() != pushDevice || target.RegistrationVersion() != 1 || target.Token() != "old-secret" {
		t.Fatal("target snapshot")
	}
	raw, _ := json.Marshal(target)
	formatted := fmt.Sprintf("%v %+v %#v %s", target, target, target, target)
	if strings.Contains(string(raw), "secret") || strings.Contains(formatted, "secret") {
		t.Fatal("target secret serialized")
	}
	for _, result := range []PushDeliveryResult{PushAccepted, PushTransient, PushRateLimited, PushConfigurationError, PushUnknownDelivery, "unrecognized"} {
		changed, err := f.repo.ApplyPushResult(f.ctx, target, result)
		if err != nil || changed {
			t.Fatal("unknown result cleared", err)
		}
	}
	putPush(t, f, 0, PushFCM, 2, 1, "new-secret")
	if changed, err := f.repo.ApplyPushResult(f.ctx, target, PushTargetInvalid); err != nil || changed {
		t.Fatal("old result erased new token", err)
	}
	refreshed, err := f.repo.ReadPushTargets(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range refreshed {
		if candidate.SessionID() == target.SessionID() && candidate.Provider() == PushFCM {
			target = candidate
		}
	}
	// 回调必须同时匹配设备/Session/provider/版本，不能重定向其他身份。
	for _, mutate := range []func(*PushTarget){func(t *PushTarget) { t.device = "other" }, func(t *PushTarget) { t.session = f.actors[1].Session.ID }, func(t *PushTarget) { t.provider = PushOPush }, func(t *PushTarget) { t.version-- }} {
		fake := target
		mutate(&fake)
		if changed, err := f.repo.ApplyPushResult(f.ctx, fake, PushTargetInvalid); err != nil || changed {
			t.Fatal("mismatched result", err)
		}
	}
	changed, err := f.repo.ApplyPushResult(f.ctx, target, PushTargetInvalid)
	if err != nil || !changed {
		t.Fatal("current invalid target", err)
	}
	state, err := f.repo.GetCurrentPush(f.ctx, f.actors[0])
	if err != nil || state.Registrations[0].RegistrationVersion != 3 || state.Registrations[0].Registered || !state.Registrations[1].Registered {
		t.Fatal("conditional clear", err)
	}
	if changed, err := f.repo.ApplyPushResult(f.ctx, target, PushTargetInvalid); err != nil || changed {
		t.Fatal("duplicate clear", err)
	}
	if pushCount(t, f, "SELECT count(*) FROM sessions WHERE revoked=0") != 2 || pushCount(t, f, "SELECT count(*) FROM device_push_registrations WHERE token IS NOT NULL") != 2 || pushCount(t, f, "SELECT settings_version FROM device_notification_settings") != 1 {
		t.Fatal("cross entity mutation")
	}
}
func TestPushPersistentSecurityRevocation(t *testing.T) {
	for _, action := range []string{"revoke", "logout", "reset", "offline-delete", "generation-only", "administrator-delete"} {
		t.Run(action, func(t *testing.T) {
			f := newPushFixture(t)
			for i := range 2 {
				putPush(t, f, i, PushFCM, math.MaxInt64, 0, "fcm-secret")
				putPush(t, f, i, PushOPush, 1, 0, "opush-secret")
			}
			if _, err := f.repo.PutCurrentNotifications(f.ctx, f.actors[0], notificationInput()); err != nil {
				t.Fatal(err)
			}
			targets, err := f.repo.ReadPushTargets(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			switch action {
			case "revoke":
				err = f.sessions.RevokeDeviceSession(f.ctx, f.actors[1], f.actors[0].Session.ID)
			case "logout":
				err = f.sessions.Revoke(f.ctx, f.tokens[0].Token, "android")
			case "reset":
				err = auth.ResetPassword(f.ctx, f.store, []byte("synthetic-reset-password"))
			case "generation-only":
				mustPushSQL(t, f, "UPDATE administrator SET auth_generation=auth_generation+1 WHERE id=1")
			case "administrator-delete":
				mustPushSQL(t, f, "DELETE FROM administrator")
			case "offline-delete":
				if err = f.store.Close(); err != nil {
					t.Fatal(err)
				}
				f.store, err = storage.OpenOffline(f.ctx, storage.Config{Path: f.path})
				if err != nil {
					t.Fatal(err)
				}
				mustPushSQL(t, f, "DELETE FROM sessions WHERE id=?", f.actors[0].Session.ID)
				if err = f.store.Close(); err != nil {
					t.Fatal(err)
				}
				f.store, err = storage.Open(f.ctx, storage.Config{Path: f.path})
				if err != nil {
					t.Fatal(err)
				}
				f.repo = NewPushRepository(f.store, f.sessions, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			remaining := int64(2)
			if action == "reset" || action == "generation-only" || action == "administrator-delete" {
				remaining = 0
			}
			if pushCount(t, f, "SELECT count(*) FROM device_push_registrations") != remaining {
				t.Fatal("revocation left persisted secret")
			}
			if pushCount(t, f, "SELECT settings_version FROM device_notification_settings") != 1 {
				t.Fatal("revocation deleted shared preferences")
			}
			if action != "offline-delete" {
				if _, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "late", 2, 0}); !errors.Is(err, auth.ErrInvalidSession) {
					t.Fatal("late registration", err)
				}
			}
			for _, target := range targets {
				if target.SessionID() == f.actors[0].Session.ID {
					if changed, err := f.repo.ApplyPushResult(f.ctx, target, PushTargetInvalid); err != nil || changed {
						t.Fatal("revoked callback", err)
					}
				}
			}
			// 新登录拥有全新 Session.id；旧回调不能触碰新注册。
			if action == "administrator-delete" {
				return
			}
			generation := int64(1)
			if action == "reset" || action == "generation-only" {
				generation = 2
			}
			sessions := auth.NewSessionStore(f.store)
			_, secret, err := sessions.Create(f.ctx, generation, auth.SessionInput{ClientType: "android", DeviceID: pushDevice, DeviceName: "new-login"})
			if err != nil {
				t.Fatal(err)
			}
			f.sessions = sessions
			f.repo = NewPushRepository(f.store, sessions, nil)
			f.tokens = []auth.SessionSecrets{secret}
			f.authorize(t)
			putPush(t, f, 0, PushFCM, 1, 0, "new-login-secret")
			for _, target := range targets {
				if target.session != f.actors[0].Session.ID {
					_, err = f.repo.ApplyPushResult(f.ctx, target, PushTargetInvalid)
					if err != nil && !errors.Is(err, ErrPushExhausted) {
						t.Fatal(err)
					}
				}
			}
			current, err := f.repo.GetCurrentPush(f.ctx, f.actors[0])
			if err != nil || !current.Registrations[0].Registered || current.Registrations[0].RegistrationVersion != 1 {
				t.Fatal("new login affected", err)
			}
		})
	}
}
func TestPushResultExhaustionAndCommitRollback(t *testing.T) {
	f := newPushFixture(t)
	putPush(t, f, 0, PushFCM, math.MaxInt64, 0, "last")
	targets, err := f.repo.ReadPushTargets(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := f.repo.ApplyPushResult(f.ctx, targets[0], PushTargetInvalid)
	if changed {
		t.Fatal("exhausted mutation")
	}
	assertPushError(t, err, ErrPushExhausted, math.MaxInt64, "/devices/current/push")
	if pushCount(t, f, "SELECT count(*) FROM device_push_registrations WHERE token IS NOT NULL") != 1 {
		t.Fatal("partial exhausted clear")
	}
	mustPushSQL(t, f, `CREATE TABLE deferred_push_fault(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED)`)
	mustPushSQL(t, f, `CREATE TRIGGER push_commit_fault AFTER INSERT ON device_push_registrations BEGIN INSERT INTO deferred_push_fault VALUES('missing'); END`)
	state, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushOPush, "no-commit", 1, 0})
	if !errors.Is(err, auth.ErrStorage) || state.SessionID != "" {
		t.Fatal("published before commit", err)
	}
	if pushCount(t, f, "SELECT count(*) FROM device_push_registrations") != 1 || pushCount(t, f, "SELECT count(*) FROM deferred_push_fault") != 0 {
		t.Fatal("commit rollback")
	}
	// 目标条件解绑同样只有成功提交后才能报告 changed。
	mustPushSQL(t, f, `DROP TRIGGER push_commit_fault`)
	putPush(t, f, 0, PushOPush, 1, 0, "no-clear")
	targets, err = f.repo.ReadPushTargets(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var target PushTarget
	for _, candidate := range targets {
		if candidate.provider == PushOPush {
			target = candidate
		}
	}
	mustPushSQL(t, f, `CREATE TRIGGER result_commit_fault AFTER UPDATE ON device_push_registrations BEGIN INSERT INTO deferred_push_fault VALUES('missing'); END`)
	changed, err = f.repo.ApplyPushResult(f.ctx, target, PushTargetInvalid)
	if changed || !errors.Is(err, auth.ErrStorage) {
		t.Fatal("result published before commit", err)
	}
	if pushCount(t, f, "SELECT count(*) FROM device_push_registrations WHERE token IS NOT NULL") != 2 {
		t.Fatal("partial result clear")
	}
}
func TestPushAuthorizedReadRollback(t *testing.T) {
	f := newPushFixture(t)
	putPush(t, f, 0, PushFCM, 1, 0, "secret")
	mustPushSQL(t, f, `CREATE TRIGGER read_auth_fault AFTER UPDATE OF idle_expires_at ON sessions BEGIN UPDATE administrator SET auth_generation=auth_generation+1 WHERE id=1; END`)
	state, err := f.repo.GetCurrentPush(f.ctx, f.actors[0])
	if !errors.Is(err, auth.ErrInvalidSession) || state.SessionID != "" {
		t.Fatal("read published after failed authorization", err)
	}
	if pushCount(t, f, "SELECT count(*) FROM device_push_registrations") != 1 || pushCount(t, f, "SELECT auth_generation FROM administrator") != 1 {
		t.Fatal("read rollback")
	}
}
func TestPushMigrationEightUpgrade(t *testing.T) {
	// 构造真实八迁移老库与准确 checksum 台账，再由正式 Open 只追加 0009。
	f := &pushFixture{ctx: t.Context(), path: t.TempDir() + "/old.sqlite"}
	db, err := sql.Open("sqlite", f.path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(f.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT,checksum TEXT,applied_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	oldChecks := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") || name >= "0009" {
			continue
		}
		data, err := fs.ReadFile(migrations.Files, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(string(data)); err != nil {
			t.Fatal(name, err)
		}
		if strings.HasPrefix(name, "0005_") {
			if _, err = storage.InitInstanceIDTx(f.ctx, tx); err != nil {
				t.Fatal(err)
			}
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(data))
		oldChecks[name] = checksum
		if _, err = tx.Exec("INSERT INTO schema_migrations VALUES(?,?,?,?)", len(oldChecks), name, checksum, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	if len(oldChecks) != 8 {
		t.Fatal("not an eight-migration database")
	}
	if _, err = tx.Exec(`INSERT INTO administrator(id,username,password_hash) VALUES(1,'old-admin','synthetic-hash')`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	f.store, err = storage.Open(f.ctx, storage.Config{Path: f.path})
	if err != nil {
		t.Fatal(err)
	}
	defer f.store.Close()
	if pushCount(t, f, "SELECT count(*) FROM schema_migrations") != 9 || pushCount(t, f, "SELECT count(*) FROM administrator WHERE username='old-admin'") != 1 {
		t.Fatal("upgrade lost existing data")
	}
	if err = f.store.Read(f.ctx, func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT name,checksum FROM schema_migrations WHERE version<=8 ORDER BY version")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name, sum string
			if err = rows.Scan(&name, &sum); err != nil {
				return err
			}
			if oldChecks[name] != sum {
				t.Fatal("old checksum changed")
			}
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(f.store)
	_, secret, err := sessions.Create(f.ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: pushDevice, DeviceName: "upgraded"})
	if err != nil {
		t.Fatal(err)
	}
	f.sessions = sessions
	f.repo = NewPushRepository(f.store, sessions, nil)
	f.tokens = []auth.SessionSecrets{secret}
	f.authorize(t)
	putPush(t, f, 0, PushFCM, 1, 0, "upgraded-token")
}

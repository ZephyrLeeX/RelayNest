package devices

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

const pushDevice = "00000000-0000-4000-8000-000000000001"

type pushFixture struct {
	ctx      context.Context
	path     string
	store    *storage.Store
	sessions *auth.SessionStore
	repo     *PushRepository
	tokens   []auth.SessionSecrets
	actors   []auth.AdminRequest
	instance string
}

func newPushFixture(t *testing.T) *pushFixture {
	t.Helper()
	f := &pushFixture{ctx: context.Background(), path: filepath.Join(t.TempDir(), "push.sqlite")}
	var err error
	f.store, err = storage.Open(f.ctx, storage.Config{Path: f.path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.store.Close() })
	mustPushSQL(t, f, `INSERT INTO administrator(id,username,password_hash) VALUES(1,'admin','test-not-a-real-hash')`)
	f.sessions = auth.NewSessionStore(f.store)
	f.repo = NewPushRepository(f.store, f.sessions, nil)
	for i := 0; i < 2; i++ {
		_, token, err := f.sessions.Create(f.ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: pushDevice, DeviceName: "test"})
		if err != nil {
			t.Fatal(err)
		}
		f.tokens = append(f.tokens, token)
	}
	f.authorize(t)
	return f
}
func (f *pushFixture) authorize(t *testing.T) {
	t.Helper()
	if err := f.store.Read(f.ctx, func(tx *sql.Tx) error {
		var err error
		f.instance, err = storage.ReadInstanceIDTx(f.ctx, tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.NewOriginPolicy("https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	security := auth.NewRequestSecurity(f.sessions, policy)
	f.actors = nil
	for _, token := range f.tokens {
		req := httptest.NewRequest("PUT", "https://example.com/devices/current/push", nil)
		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("X-RelayNest-Instance-ID", f.instance)
		recorder := httptest.NewRecorder()
		actor, ok := security.AuthorizeRequired(recorder, req, auth.RequestWrite)
		if !ok {
			t.Fatalf("authorize status %d", recorder.Code)
		}
		f.actors = append(f.actors, actor)
	}
}
func mustPushSQL(t *testing.T, f *pushFixture, query string, args ...any) {
	t.Helper()
	if err := f.store.Write(f.ctx, func(tx *sql.Tx) error { _, err := tx.ExecContext(f.ctx, query, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func pushCount(t *testing.T, f *pushFixture, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.store.Read(f.ctx, func(tx *sql.Tx) error { return tx.QueryRowContext(f.ctx, query, args...).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}
func putPush(t *testing.T, f *pushFixture, actor int, provider PushProvider, version, expected int64, token string) PushRegistrationState {
	t.Helper()
	s, err := f.repo.PutCurrentPush(f.ctx, f.actors[actor], PushRegistrationUpdate{provider, token, version, expected})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func assertPushError(t *testing.T, err, cause error, version int64, path string) {
	t.Helper()
	var v *VersionError
	if !errors.Is(err, cause) || !errors.As(err, &v) || v.LatestVersion != version || v.RecoveryPath != path {
		t.Fatalf("wrong version error: %v", err)
	}
}
func TestPushRegistrationLifecycle(t *testing.T) {
	f := newPushFixture(t)
	initial, err := f.repo.GetCurrentPush(f.ctx, f.actors[0])
	if err != nil {
		t.Fatal(err)
	}
	if initial.SessionID != f.actors[0].Session.ID || len(initial.Registrations) != 2 {
		t.Fatal("initial state")
	}
	for _, p := range initial.Registrations {
		if p.RegistrationVersion != 0 || p.Registered || p.UpdatedAt != nil {
			t.Fatal("initial barrier")
		}
	}
	if pushCount(t, f, "SELECT count(*) FROM device_push_registrations") != 0 {
		t.Fatal("GET persisted")
	}
	a := putPush(t, f, 0, PushFCM, 5, 0, "sensitive-fcm")
	putPush(t, f, 0, PushOPush, 2, 0, "sensitive-opush")
	putPush(t, f, 1, PushFCM, 1, 0, "second-session")
	replay, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "sensitive-fcm", 5, 0})
	if err != nil {
		t.Fatal(err)
	}
	if !a.Registrations[0].UpdatedAt.Equal(*replay.Registrations[0].UpdatedAt) {
		t.Fatal("replay changed time")
	}
	for _, in := range []PushRegistrationUpdate{{PushFCM, "other", 5, 5}, {PushFCM, "older", 4, 5}, {PushFCM, "newer", 6, 0}} {
		_, err = f.repo.PutCurrentPush(f.ctx, f.actors[0], in)
		assertPushError(t, err, ErrPushConflict, 5, "/devices/current/push")
	}
	tomb, err := f.repo.DeleteCurrentPush(f.ctx, f.actors[0], PushFCM, 5)
	if err != nil {
		t.Fatal(err)
	}
	if tomb.Registrations[0].RegistrationVersion != 6 || tomb.Registrations[0].Registered || !tomb.Registrations[1].Registered {
		t.Fatal("tombstone/other provider")
	}
	noop, err := f.repo.DeleteCurrentPush(f.ctx, f.actors[0], PushFCM, 6)
	if err != nil {
		t.Fatal(err)
	}
	if !noop.Registrations[0].UpdatedAt.Equal(*tomb.Registrations[0].UpdatedAt) {
		t.Fatal("noop changed time")
	}
	_, err = f.repo.DeleteCurrentPush(f.ctx, f.actors[0], PushFCM, 5)
	assertPushError(t, err, ErrPushConflict, 6, "/devices/current/push")
	_, err = f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "sensitive-fcm", 5, 6})
	assertPushError(t, err, ErrPushConflict, 6, "/devices/current/push")
	putPush(t, f, 0, PushFCM, 7, 6, "new-token")
	raw, _ := json.Marshal(replay)
	if strings.Contains(string(raw), "sensitive") || strings.Contains(string(raw), "token") {
		t.Fatal("public secret")
	}
	if pushCount(t, f, "SELECT count(*) FROM changes") != 0 {
		t.Fatal("registration generated preference change")
	}
}
func TestPushReopenAndConcurrentCAS(t *testing.T) {
	f := newPushFixture(t)
	putPush(t, f, 0, PushFCM, 1, 0, "original")
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	f.store, err = storage.Open(f.ctx, storage.Config{Path: f.path})
	if err != nil {
		t.Fatal(err)
	}
	f.sessions = auth.NewSessionStore(f.store)
	f.repo = NewPushRepository(f.store, f.sessions, nil)
	f.authorize(t)
	s, err := f.repo.GetCurrentPush(f.ctx, f.actors[0])
	if err != nil || s.Registrations[0].RegistrationVersion != 1 {
		t.Fatal("reopen", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, token := range []string{"winner-a", "winner-b"} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			_, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, token, 2, 1})
			results <- err
		}(token)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrPushConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("CAS was not serialized")
	}
}
func TestPushExhaustionAndBoundary(t *testing.T) {
	f := newPushFixture(t)
	putPush(t, f, 0, PushFCM, math.MaxInt64, 0, "last")
	replay, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "last", math.MaxInt64, 0})
	if err != nil || !replay.Registrations[0].Registered {
		t.Fatal("max replay", err)
	}
	_, err = f.repo.DeleteCurrentPush(f.ctx, f.actors[0], PushFCM, math.MaxInt64)
	assertPushError(t, err, ErrPushExhausted, math.MaxInt64, "/devices/current/push")
	_, err = f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "different", math.MaxInt64, math.MaxInt64})
	assertPushError(t, err, ErrPushConflict, math.MaxInt64, "/devices/current/push")
	for _, in := range []PushRegistrationUpdate{{"other", "secret", 1, 0}, {PushOPush, "", 1, 0}, {PushOPush, "x", 0, 0}, {PushOPush, "x", 1, -1}, {PushOPush, strings.Repeat("字", 4097), 1, 0}} {
		if _, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], in); !errors.Is(err, ErrPushInput) {
			t.Fatal("invalid input", err)
		}
	}
	putPush(t, f, 0, PushOPush, 1, 0, strings.Repeat("字", 4096))
	putPush(t, f, 1, PushOPush, 1, 0, "\x00synthetic")
	var decoded PushRegistrationUpdate
	if err := json.Unmarshal([]byte(`{"provider":"fcm","token":"private-value","registration_version":1,"expected_version":0}`), &decoded); err != nil || decoded.Token != "private-value" {
		t.Fatal("write-only token decode", err)
	}
	encoded, _ := json.Marshal(decoded)
	if strings.Contains(string(encoded), "private-value") {
		t.Fatal("input token encoded")
	}
	input := PushRegistrationUpdate{Token: "private-value"}
	if strings.Contains(fmt.Sprintf("%+v %#v %s", input, input, input), "private-value") {
		t.Fatal("formatted token")
	}
}
func TestPushIdentityCannotBeSelected(t *testing.T) {
	f := newPushFixture(t)
	fake := f.actors[0]
	fake.Session = f.actors[1].Session
	for _, actor := range []auth.AdminRequest{fake, {}, {Session: f.actors[0].Session}} {
		if _, err := f.repo.PutCurrentPush(f.ctx, actor, PushRegistrationUpdate{PushFCM, "secret", 1, 0}); !errors.Is(err, auth.ErrInvalidSession) {
			t.Fatal("forged identity", err)
		}
		if _, err := f.repo.GetCurrentNotifications(f.ctx, actor); !errors.Is(err, auth.ErrInvalidSession) {
			t.Fatal("forged read", err)
		}
	}
	other := auth.NewSessionStore(f.store)
	if _, err := NewPushRepository(f.store, other, nil).GetCurrentPush(f.ctx, f.actors[0]); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatal("foreign issuer", err)
	}
	_, secrets, err := f.sessions.Create(f.ctx, 1, auth.SessionInput{ClientType: "web", DeviceName: "web"})
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := httpapi.NewOriginPolicy("https://example.com", nil)
	security := auth.NewRequestSecurity(f.sessions, policy)
	req := httptest.NewRequest("GET", "https://example.com/devices/current/push", nil)
	req.Header.Set("Cookie", "relaynest_session="+secrets.Token)
	actor, ok := security.AuthorizeRequired(httptest.NewRecorder(), req, auth.RequestRead)
	if !ok {
		t.Fatal("web auth")
	}
	if _, err := f.repo.GetCurrentPush(f.ctx, actor); !errors.Is(err, ErrAndroidRequired) {
		t.Fatal("web admitted", err)
	}
}
func TestPushSecretStatementErrorAndRollback(t *testing.T) {
	f := newPushFixture(t)
	mustPushSQL(t, f, `CREATE TRIGGER push_fault BEFORE INSERT ON device_push_registrations BEGIN SELECT RAISE(ABORT,'synthetic-private-token'); END`)
	_, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "synthetic-private-token", 1, 0})
	if !errors.Is(err, auth.ErrStorage) || strings.Contains(fmt.Sprint(err), "synthetic-private-token") {
		t.Fatal("raw error leaked", err)
	}
	if pushCount(t, f, "SELECT count(*) FROM device_push_registrations") != 0 {
		t.Fatal("failed registration persisted")
	}
}
func TestPushExpiredTargets(t *testing.T) {
	f := newPushFixture(t)
	putPush(t, f, 0, PushFCM, 1, 0, "old")
	targets, err := f.repo.ReadPushTargets(f.ctx)
	if err != nil || len(targets) != 1 {
		t.Fatal(err)
	}
	mustPushSQL(t, f, "UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(-time.Second).UnixNano(), f.actors[0].Session.ID)
	found, err := f.repo.ReadPushTargets(f.ctx)
	if err != nil || len(found) != 0 {
		t.Fatal("expired targets", err)
	}
	changed, err := f.repo.ApplyPushResult(f.ctx, targets[0], PushTargetInvalid)
	if err != nil || changed {
		t.Fatal("expired callback", err)
	}
	if _, err := f.repo.PutCurrentPush(f.ctx, f.actors[0], PushRegistrationUpdate{PushFCM, "late", 2, 1}); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatal("expired registration", err)
	}
}

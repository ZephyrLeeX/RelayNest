package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"relaynest/server/internal/httpapi"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

// 测试通过真实 Store 事务读取部署 ID，不使用固定桩值。
func testInstanceID(t *testing.T, store *storage.Store) string {
	t.Helper()
	var id string
	if err := store.Read(context.Background(), func(tx *sql.Tx) error {
		var err error
		id, err = storage.ReadInstanceIDTx(context.Background(), tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func boundRequest(t *testing.T, f *httpFixture, method, path, body string, headers http.Header) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	r.Header = headers.Clone()
	if r.Header == nil {
		r.Header = make(http.Header)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler := NewHandler(f.db, f.sessions, nil)
	policy, err := httpapi.NewOriginPolicy(f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler.origin = policy
	switch path {
	case "/login":
		handler.Login(w, r)
	case "/session":
		handler.GetSession(w, r)
	case "/logout":
		handler.Logout(w, r)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	var out map[string]any
	if w.Code != 204 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
	}
	return w.Code, out
}

func TestInstanceBindingHTTPOrder(t *testing.T) {
	f := newHTTPFixture(t, true)
	body := `{"username":"Admin","password":{},"client_type":"android","device_name":"phone","device_id":"11111111-1111-4111-8111-111111111111"}`
	before := f.snapshot()
	for _, tc := range []struct {
		name   string
		ids    []string
		status int
		code   string
	}{
		{"missing", nil, 400, "INSTANCE_REQUIRED"},
		{"empty", []string{""}, 400, "INVALID_REQUEST"},
		{"duplicate", []string{f.instanceID, f.instanceID}, 400, "INVALID_REQUEST"},
		{"malformed", []string{"invalid"}, 400, "INVALID_REQUEST"},
		{"uppercase", []string{"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"}, 400, "INVALID_REQUEST"},
		{"wrong", []string{"22222222-2222-4222-8222-222222222222"}, 409, "INSTANCE_MISMATCH"},
		{"valid", []string{f.instanceID}, 400, "INVALID_REQUEST"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{"/login", "/session", "/logout"} {
				h := make(http.Header)
				if tc.ids != nil {
					h[instanceHeader] = tc.ids
				}
				expectedStatus, expectedCode := tc.status, tc.code
				if path != "/login" {
					h.Set("Authorization", "Bearer invalid")
					if tc.name == "valid" {
						expectedStatus, expectedCode = 401, "AUTHENTICATION_FAILED"
					}
				}
				status, out := boundRequest(t, f, "POST", path, body, h)
				if status != expectedStatus || out["error"].(map[string]any)["code"] != expectedCode {
					t.Fatalf("%s: %d %v", path, status, out)
				}
				raw, _ := json.Marshal(out)
				if strings.Contains(string(raw), f.instanceID) {
					t.Fatal("instance leak")
				}
			}
			if f.snapshot() != before {
				t.Fatal("rejected binding changed sessions")
			}
		})
	}
}

func TestInstanceBindingHTTPProjection(t *testing.T) {
	f := newHTTPFixture(t, true)
	token, _ := f.login("android")
	status, out := boundRequest(t, f, "GET", "/session", "", http.Header{"Authorization": {"Bearer " + token}, instanceHeader: {f.instanceID}})
	session := out["session"].(map[string]any)
	if status != 200 || session["instance_id"] != f.instanceID || len(session) != 8 || len(out) != 1 {
		t.Fatalf("projection: %d %v", status, out)
	}
	status, out = boundRequest(t, f, "POST", "/logout", "", http.Header{"Authorization": {"Bearer " + token}, instanceHeader: {"22222222-2222-4222-8222-222222222222"}})
	if status != 409 || out["error"].(map[string]any)["code"] != "INSTANCE_MISMATCH" {
		t.Fatal("wrong logout accepted")
	}
	status, _ = boundRequest(t, f, "GET", "/session", "", http.Header{"Authorization": {"Bearer " + token}, instanceHeader: {f.instanceID}})
	if status != 200 {
		t.Fatal("wrong binding revoked normal session")
	}
}

func TestInstanceBindingStorageFailure(t *testing.T) {
	for _, failure := range []string{"closed", "missing", "corrupt"} {
		t.Run(failure, func(t *testing.T) {
			f := newHTTPFixture(t, true)
			if failure == "closed" {
				if err := f.db.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				err := f.db.Write(context.Background(), func(tx *sql.Tx) error {
					if failure == "missing" {
						_, err := tx.Exec("DELETE FROM instance_identity")
						return err
					}
					if _, err := tx.Exec("PRAGMA ignore_check_constraints=ON"); err != nil {
						return err
					}
					_, err := tx.Exec("UPDATE instance_identity SET instance_id='corrupt'")
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			status, out := boundRequest(t, f, "GET", "/session", "", http.Header{"Authorization": {"Bearer invalid"}, instanceHeader: {f.instanceID}})
			if status != 503 || out["error"].(map[string]any)["code"] != "INSTANCE_UNAVAILABLE" {
				t.Fatalf("failure mapped to %d %v", status, out)
			}
			if _, exists := out["session"]; exists {
				t.Fatal("published failed session")
			}
		})
	}
}

func bindingSQL(t *testing.T, s *SessionStore, statement string) {
	t.Helper()
	if err := s.store.Write(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec(statement); return err }); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceBindingTransactionRollback(t *testing.T) {
	for _, operation := range []string{"read", "write"} {
		for _, mutation := range []struct {
			name, sql string
			expected  error
		}{
			{"identity", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", storage.ErrInstanceMismatch},
			{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", ErrInvalidSession},
			{"revocation", "UPDATE sessions SET revoked=1", ErrInvalidSession},
			{"commit", "INSERT INTO binding_commit_failure VALUES(999)", ErrStorage},
		} {
			t.Run(operation+"/"+mutation.name, func(t *testing.T) {
				s, _ := sessionFixture(t)
				_, secret := createSession(t, s, "android")
				actor := deviceActor(t, s, secret, "android")
				originalID := testInstanceID(t, s.store)
				bindingSQL(t, s, "CREATE TABLE binding_commit_failure(id INTEGER REFERENCES administrator(id) DEFERRABLE INITIALLY DEFERRED)")
				work := func(tx *sql.Tx) error {
					if _, err := tx.Exec("INSERT INTO binding_commit_failure VALUES(1)"); err != nil {
						return err
					}
					_, err := tx.Exec(mutation.sql)
					return err
				}
				var err error
				if operation == "read" {
					var session Session
					var id string
					session, id, err = s.WithAuthorizedRead(context.Background(), actor, work)
					if session != (Session{}) || id != "" {
						t.Fatal("failed transaction published result")
					}
				} else {
					err = s.WithAuthorizedWrite(context.Background(), actor, work)
				}
				if !errors.Is(err, mutation.expected) {
					t.Fatalf("error: %v want %v", err, mutation.expected)
				}
				if testInstanceID(t, s.store) != originalID {
					t.Fatal("identity partially committed")
				}
				if err := s.store.Read(context.Background(), func(tx *sql.Tx) error {
					var count int
					if err := tx.QueryRow("SELECT COUNT(*) FROM binding_commit_failure").Scan(&count); err != nil {
						return err
					}
					if count != 0 {
						t.Fatal("business candidate partially committed")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				current, _, err := s.Inspect(context.Background(), secret.Token, "android")
				if err != nil || current.DeviceName != actor.Session.DeviceName {
					t.Fatalf("session partially committed: %v %v", current, err)
				}
			})
		}
	}
}

func TestInstanceBindingCreateAndRenewTriggers(t *testing.T) {
	for _, phase := range []string{"create", "renew"} {
		for _, mutation := range []struct {
			name, sql string
			expected  error
		}{
			{"identity", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", storage.ErrInstanceMismatch},
			{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", ErrInvalidSession},
		} {
			t.Run(phase+"/"+mutation.name, func(t *testing.T) {
				s, _ := sessionFixture(t)
				expected := testInstanceID(t, s.store)
				if phase == "create" {
					bindingSQL(t, s, "CREATE TRIGGER binding_mutation AFTER INSERT ON sessions BEGIN "+mutation.sql+"; END")
					session, secrets, err := s.create(context.Background(), 1, SessionInput{ClientType: "android", DeviceName: "phone", DeviceID: "11111111-1111-4111-8111-111111111111"}, expected)
					if !errors.Is(err, mutation.expected) || session != (Session{}) || secrets != (SessionSecrets{}) {
						t.Fatalf("create leaked: %v %v", session, err)
					}
					if err := s.store.Read(context.Background(), func(tx *sql.Tx) error {
						var count int
						if err := tx.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
							return err
						}
						if count != 0 {
							t.Fatal("created session committed")
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				} else {
					_, secret := createSession(t, s, "android")
					actor := deviceActor(t, s, secret, "android")
					bindingSQL(t, s, "CREATE TRIGGER binding_mutation AFTER UPDATE OF idle_expires_at ON sessions BEGIN "+mutation.sql+"; END")
					session, id, err := s.WithAuthorizedRead(context.Background(), actor, func(*sql.Tx) error { return nil })
					if !errors.Is(err, mutation.expected) || session != (Session{}) || id != "" {
						t.Fatalf("renew leaked: %v %v", session, err)
					}
				}
				if testInstanceID(t, s.store) != expected {
					t.Fatal("trigger mutation committed")
				}
			})
		}
	}
}

func TestInstanceBindingCreateAfterSwitch(t *testing.T) {
	a, _ := sessionFixture(t)
	b, _ := sessionFixture(t)
	expected := testInstanceID(t, a.store)
	session, secret, err := b.create(context.Background(), 1, SessionInput{ClientType: "android", DeviceName: "phone", DeviceID: "11111111-1111-4111-8111-111111111111"}, expected)
	if !errors.Is(err, storage.ErrInstanceMismatch) || session != (Session{}) || secret != (SessionSecrets{}) {
		t.Fatalf("database switch accepted: %v", err)
	}
}

func TestInstanceBindingReadSnapshotConcurrentChanges(t *testing.T) {
	for _, mutation := range []struct {
		name, sql string
		expected  error
	}{
		{"revocation", "UPDATE sessions SET revoked=1", ErrInvalidSession},
		{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", ErrInvalidSession},
		{"identity", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", storage.ErrInstanceUnavailable},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			s, _ := sessionFixture(t)
			_, secret := createSession(t, s, "android")
			actor := deviceActor(t, s, secret, "android")
			expectedID := testInstanceID(t, s.store)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			entered, release, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
			readDone, writeDone := make(chan error, 1), make(chan error, 1)
			var projectionID string
			var projectionGeneration int64
			go func() {
				session, id, err := s.WithAuthorizedRead(ctx, actor, func(tx *sql.Tx) error {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
					var err error
					projectionID, err = storage.ReadInstanceIDTx(ctx, tx)
					if err != nil {
						return err
					}
					return tx.QueryRowContext(ctx, "SELECT auth_generation FROM administrator WHERE id=1").Scan(&projectionGeneration)
				})
				if err == nil && (id != expectedID || session.ID != actor.Session.ID) {
					err = errors.New("inconsistent session projection")
				}
				readDone <- err
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			go func() {
				close(started)
				writeDone <- s.store.Write(ctx, func(tx *sql.Tx) error { _, err := tx.ExecContext(ctx, mutation.sql); return err })
			}()
			<-started
			close(release)
			if err := <-readDone; err != nil {
				t.Fatal(err)
			}
			if projectionID != expectedID || projectionGeneration != actor.binding.generation {
				t.Fatal("business projection stitched snapshots")
			}
			if err := <-writeDone; err != nil {
				t.Fatal(err)
			}
			called := false
			session, id, err := s.WithAuthorizedRead(ctx, actor, func(*sql.Tx) error { called = true; return nil })
			if !errors.Is(err, mutation.expected) || called || session != (Session{}) || id != "" {
				t.Fatalf("changed identity accepted: %v", err)
			}
		})
	}
}

func TestInstanceBindingReadRejectsForgedActor(t *testing.T) {
	s, _ := sessionFixture(t)
	_, secret := createSession(t, s, "android")
	actor := deviceActor(t, s, secret, "android")
	forged := actor
	forged.Session.DeviceID = "22222222-2222-4222-8222-222222222222"
	for _, invalid := range []AdminRequest{{Session: actor.Session}, forged} {
		called := false
		session, id, err := s.WithAuthorizedRead(context.Background(), invalid, func(*sql.Tx) error { called = true; return nil })
		if !errors.Is(err, ErrInvalidSession) || called || session != (Session{}) || id != "" {
			t.Fatalf("forged actor accepted: %v", err)
		}
	}
}

func TestInstanceBindingCredentialChannelPriority(t *testing.T) {
	f := newHTTPFixture(t, true)
	for _, h := range []http.Header{
		{"Authorization": {"Basic invalid"}},
		{"Authorization": {"Bearer "}},
		{"Authorization": {"Bearer a b"}},
		{"Authorization": {"Bearer invalid", "Bearer other"}},
		{"Authorization": {"Bearer invalid"}, "Cookie": {sessionCookie + "=web"}},
	} {
		status, out := boundRequest(t, f, "GET", "/session", "", h)
		if status != 401 || out["error"].(map[string]any)["code"] != "AUTHENTICATION_FAILED" {
			t.Fatalf("channel priority: %d %v", status, out)
		}
	}
	token, _ := f.login("android")
	bindingSQL(t, f.sessions, "DROP TABLE sessions")
	status, out := boundRequest(t, f, "GET", "/session", "", http.Header{"Authorization": {"Bearer " + token}, instanceHeader: {f.instanceID}})
	if status != 503 || out["error"].(map[string]any)["code"] != "SERVICE_UNAVAILABLE" {
		t.Fatalf("session storage classified as auth failure: %d %v", status, out)
	}
}

func TestInstanceBindingWebIgnoresHeader(t *testing.T) {
	f := newHTTPFixture(t, true)
	token, csrf := f.login("web")
	for _, ids := range [][]string{nil, {"malformed"}, {"", "other"}, {"22222222-2222-4222-8222-222222222222"}} {
		headers := http.Header{"Cookie": {sessionCookie + "=" + token}, "Origin": {f.server.URL}, "X-CSRF-Token": {csrf}}
		if ids != nil {
			headers[instanceHeader] = ids
		}
		status, out := boundRequest(t, f, "GET", "/session", "", headers)
		if status != 200 || len(out["session"].(map[string]any)) != 7 || len(out) != 2 {
			t.Fatalf("Web projection or ignored header changed: %d %v", status, out)
		}
		headers.Set("Origin", "https://evil.invalid")
		status, out = boundRequest(t, f, "POST", "/logout", "", headers)
		if status != 403 || out["error"].(map[string]any)["code"] != "FORBIDDEN" {
			t.Fatal("Web origin bypass")
		}
	}
}

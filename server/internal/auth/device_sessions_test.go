package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
)

// 必须走真实入口签发 actor，避免伪造公开 Session 跳过私有安全绑定。
func deviceActor(t *testing.T, s *SessionStore, secret SessionSecrets, client string) AdminRequest {
	t.Helper()
	guard := deviceGuard(t, s)
	r := httptest.NewRequest("DELETE", "https://relay.test/api/v1/devices/fixture/session", nil)
	if client == "web" {
		r.Header.Set("Cookie", "relaynest_session="+secret.Token)
		r.Header.Set("Origin", "https://relay.test")
		r.Header.Set("X-CSRF-Token", secret.CSRFToken)
	} else {
		r.Header.Set("Authorization", "Bearer "+secret.Token)
	}
	w := httptest.NewRecorder()
	actor, ok := guard.Authorize(w, r, RequestWrite)
	if !ok {
		t.Fatalf("real request authorization: %d %s", w.Code, w.Body.String())
	}
	return actor
}

func deviceGuard(t *testing.T, s *SessionStore) *RequestSecurity {
	t.Helper()
	p, err := httpapi.NewOriginPolicy("https://relay.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewRequestSecurity(s, p)
}

func deviceSQL(t *testing.T, s *SessionStore, q string, args ...any) {
	t.Helper()
	if err := s.store.Write(sessionContext, func(tx *sql.Tx) error { _, err := tx.Exec(q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func deviceInt(t *testing.T, s *SessionStore, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error { return tx.QueryRow(q, args...).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}
func deviceRevoked(t *testing.T, s *SessionStore, id string) int64 {
	t.Helper()
	return deviceInt(t, s, "SELECT revoked FROM sessions WHERE id=?", id)
}

func TestDeviceSessionsPersistenceAndExactRevocation(t *testing.T) {
	s, _ := sessionFixture(t)
	a, sa := createSession(t, s, "android")
	b, sb := createSession(t, s, "android")
	c, sc, err := s.Create(sessionContext, 1, SessionInput{"android", "22222222-2222-4222-8222-222222222222", "另一安装"})
	if err != nil {
		t.Fatal(err)
	}
	web, sw := createSession(t, s, "web")
	if a.ID == b.ID || b.ID == c.ID || sa.Token == sb.Token || sb.Token == sc.Token || a.DeviceID != b.DeviceID || a.DeviceID == c.DeviceID || web.DeviceID == a.DeviceID {
		t.Fatal("installation/session isolation")
	}
	// 真实关闭重开 SQLite，不沿用任何内存会话状态。
	var path string
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error {
		var seq int
		var name string
		return tx.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path)
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.Close(); err != nil {
		t.Fatal(err)
	}
	s.store = testStore(t, path)
	actor := deviceActor(t, s, sw, "web")
	page, err := s.ListDeviceSessions(sessionContext, actor, "", 0)
	if err != nil || len(page.Items) != 4 {
		t.Fatalf("reopen list %v %+v", err, page)
	}
	expected := map[string]Session{a.ID: a, b.ID: b, c.ID: c, web.ID: web}
	for _, v := range page.Items {
		if v != expected[v.ID] {
			t.Fatalf("metadata mismatch %+v", v)
		}
	}
	repo := bots.New(s.store)
	bot, _, err := repo.Create(sessionContext, bots.CreateRequest{Name: "独立凭据", Enabled: true, Capabilities: bots.Capabilities{Send: true}})
	if err != nil {
		t.Fatal(err)
	}
	// device_id 只能作为元数据，不得回退选择同安装任一会话。
	if err = s.RevokeDeviceSession(sessionContext, actor, a.DeviceID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("device fallback: %v", err)
	}
	if err = s.RevokeDeviceSession(sessionContext, actor, a.ID); err != nil {
		t.Fatal(err)
	}
	for _, method := range []func(context.Context, string, string) (Session, SessionSecrets, error){s.Inspect, s.Authenticate} {
		if _, _, err = method(sessionContext, sa.Token, "android"); !errors.Is(err, ErrInvalidSession) {
			t.Fatal("old token remained valid", err)
		}
	}
	r := httptest.NewRequest("DELETE", "https://relay.test/api/v1/devices/x/session", nil)
	r.Header.Set("Authorization", "Bearer "+sa.Token)
	w := httptest.NewRecorder()
	if _, ok := deviceGuard(t, s).Authorize(w, r, RequestWrite); ok || w.Code != 401 {
		t.Fatal("revoked request accepted")
	}
	fresh, sf := createSession(t, s, "android")
	if fresh.ID == a.ID || sf.Token == sa.Token || fresh.DeviceID != a.DeviceID {
		t.Fatal("relogin reused identity")
	}
	if err = s.RevokeDeviceSession(sessionContext, actor, a.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("old ID retry", err)
	}
	for _, v := range []struct {
		m      Session
		secret SessionSecrets
	}{{b, sb}, {c, sc}, {fresh, sf}, {web, sw}} {
		if got, _, err := s.Inspect(sessionContext, v.secret.Token, v.m.ClientType); err != nil || got.ID != v.m.ID {
			t.Fatal("collateral revoke", err)
		}
	}
	if _, err = repo.Authenticate(sessionContext, bot.Bot.ID, bots.APIToken, bot.APIToken.Token); err != nil {
		t.Fatal("bot affected", err)
	}
	r.Header.Set("Authorization", "Bearer "+bot.APIToken.Token)
	if _, ok := deviceGuard(t, s).Authorize(httptest.NewRecorder(), r, RequestWrite); ok {
		t.Fatal("bot became administrator")
	}
}

func TestDeviceSessionPagination(t *testing.T) {
	s, now := sessionFixture(t)
	_, secret := createSession(t, s, "android")
	actor := deviceActor(t, s, secret, "android")
	ids := []string{actor.Session.ID}
	for i := 0; i < 102; i++ {
		m, _ := createSession(t, s, "android")
		ids = append(ids, m.ID)
	}
	// 数据库真实状态过滤，边界恰好等于 now 也失效。
	for _, q := range []string{"UPDATE sessions SET revoked=1 WHERE id=?", "UPDATE sessions SET auth_generation=2 WHERE id=?", "UPDATE sessions SET idle_expires_at=? WHERE id=?", "UPDATE sessions SET idle_expires_at=?,absolute_expires_at=? WHERE id=?"} {
		m, _ := createSession(t, s, "android")
		switch strings.Count(q, "?") {
		case 1:
			deviceSQL(t, s, q, m.ID)
		case 2:
			deviceSQL(t, s, q, now.UnixNano(), m.ID)
		case 3:
			deviceSQL(t, s, q, now.UnixNano(), now.UnixNano(), m.ID)
		}
	}
	sort.Strings(ids)
	for _, tc := range []struct{ limit, want int }{{0, 50}, {1, 1}, {100, 100}, {101, 100}, {int(^uint(0) >> 1), 100}} {
		p, err := s.ListDeviceSessions(sessionContext, actor, "", tc.limit)
		if err != nil || len(p.Items) != tc.want || !p.HasMore || p.NextCursor == nil {
			t.Fatalf("limit %d: %+v %v", tc.limit, p, err)
		}
	}
	for _, cursor := range []string{"!", "v1:bad", base64.RawURLEncoding.EncodeToString([]byte("v2:" + ids[0])), base64.RawURLEncoding.EncodeToString([]byte("v1:bad"))} {
		if _, err := s.ListDeviceSessions(sessionContext, actor, cursor, 1); !errors.Is(err, ErrInvalidSessionPage) {
			t.Fatal("invalid cursor", err)
		}
	}
	if _, err := s.ListDeviceSessions(sessionContext, actor, "", -1); !errors.Is(err, ErrInvalidSessionPage) {
		t.Fatal("negative limit", err)
	}
	var got []string
	cursor := ""
	for {
		p, err := s.ListDeviceSessions(sessionContext, actor, cursor, 7)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(p)
		for _, bad := range []string{secret.Token, "token_hash", "CSRF", "provider"} {
			if strings.Contains(string(raw), bad) {
				t.Fatal("secret field in page")
			}
		}
		for _, m := range p.Items {
			if m.Username != "Admin" || m.DeviceID == "" || m.DeviceName == "" || m.ClientType != "android" || m.IdleExpiresAt.Location() != time.UTC || m.AbsoluteExpiresAt.Location() != time.UTC {
				t.Fatal("incomplete metadata", m)
			}
			got = append(got, m.ID)
		}
		if !p.HasMore {
			if p.NextCursor != nil {
				t.Fatal("tail cursor")
			}
			break
		}
		if p.NextCursor == nil || *p.NextCursor == cursor || strings.Contains(*p.NextCursor, p.Items[len(p.Items)-1].ID) {
			t.Fatal("cursor")
		}
		cursor = *p.NextCursor
	}
	if strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Fatalf("pagination filtered/order mismatch %d vs %d", len(got), len(ids))
	}
	p, err := s.ListDeviceSessions(sessionContext, actor, base64.RawURLEncoding.EncodeToString([]byte("v1:"+ids[len(ids)-1])), 7)
	if err != nil || len(p.Items) != 0 || p.Items == nil || p.HasMore || p.NextCursor != nil {
		t.Fatalf("empty tail %+v %v", p, err)
	}
}

func TestDeviceSessionExistingTxBinding(t *testing.T) {
	s, _ := sessionFixture(t)
	_, secret := createSession(t, s, "android")
	actor := deviceActor(t, s, secret, "android")
	other, _ := sessionFixture(t)
	_, os := createSession(t, other, "android")
	foreign := deviceActor(t, other, os, "android")
	// 连另库的真实行都复制成相同凭据/ID，仍须拒绝不同私有签发域。
	deviceSQL(t, other, `UPDATE sessions SET id=?,token_hash=(?) WHERE id=?`, actor.Session.ID, deviceHash(secret.Token), foreign.Session.ID)
	foreign = deviceActor(t, other, secret, "android")
	tampered := actor
	tampered.Session.DeviceID = "changed"
	wrongSecret := actor
	wrongSecret.secrets = os
	wrongIssuer := actor
	wrongIssuer.issuer = foreign.issuer
	noIssuer := actor
	noIssuer.issuer = nil
	cases := []AdminRequest{{Session: actor.Session}, tampered, wrongSecret, wrongIssuer, noIssuer, foreign}
	old := persistedIdle(t, s, actor.Session.ID)
	for i, bad := range cases {
		for _, op := range []func(context.Context, *sql.Tx, AdminRequest) (Session, error){s.AuthorizeTx, s.RenewTx} {
			err := s.store.Write(sessionContext, func(tx *sql.Tx) error { _, err := op(sessionContext, tx, bad); return err })
			if !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("binding case %d: %v", i, err)
			}
		}
		if err := s.RevokeDeviceSession(sessionContext, bad, actor.Session.ID); !errors.Is(err, ErrInvalidSession) {
			t.Fatal("bad actor revoked", err)
		}
		if _, err := s.ListDeviceSessions(sessionContext, bad, "", 1); !errors.Is(err, ErrInvalidSession) {
			t.Fatal("bad actor listed", err)
		}
	}
	if _, err := s.AuthorizeTx(sessionContext, nil, actor); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("nil tx")
	}
	if persistedIdle(t, s, actor.Session.ID) != old || deviceRevoked(t, s, actor.Session.ID) != 0 {
		t.Fatal("binding failure changed state")
	}
}

func TestDeviceSessionExistingTxAtomicity(t *testing.T) {
	for _, mode := range []string{"success", "business-error", "changes-error", "callback-error", "after-renew-error", "commit-error", "revoked", "generation", "expired", "absolute-cap"} {
		t.Run(mode, func(t *testing.T) {
			s, now := sessionFixture(t)
			meta, secret := createSession(t, s, "web")
			if mode == "absolute-cap" {
				*now = meta.AbsoluteExpiresAt.Add(-time.Hour)
				deviceSQL(t, s, "UPDATE sessions SET idle_expires_at=absolute_expires_at-1 WHERE id=?", meta.ID)
			} else {
				*now = now.Add(time.Hour)
			}
			actor := deviceActor(t, s, secret, "web")
			old := persistedIdle(t, s, meta.ID)
			deviceSQL(t, s, `CREATE TABLE tx_business(value INTEGER NOT NULL CHECK(value>0)); CREATE TABLE tx_parent(id INTEGER PRIMARY KEY); CREATE TABLE tx_child(id INTEGER REFERENCES tx_parent(id) DEFERRABLE INITIALLY DEFERRED)`)
			marker := errors.New("business callback failure")
			err := s.store.Write(sessionContext, func(tx *sql.Tx) error {
				if _, err := s.AuthorizeTx(sessionContext, tx, actor); err != nil {
					return err
				}
				value := 1
				if mode == "business-error" {
					value = 0
				}
				if _, err := tx.Exec("INSERT INTO tx_business VALUES(?)", value); err != nil {
					return err
				}
				revision := 1
				if mode == "changes-error" {
					revision = 0
				}
				if _, err := tx.Exec("INSERT INTO changes(type,entity_id,revision,created_at) VALUES('fixture.updated','fixture',?,'2026-10-23T01:00:00Z')", revision); err != nil {
					return err
				}
				switch mode {
				case "callback-error":
					return marker
				case "revoked":
					if _, err := tx.Exec("UPDATE sessions SET revoked=1 WHERE id=?", meta.ID); err != nil {
						return err
					}
				case "generation":
					if _, err := tx.Exec("UPDATE administrator SET auth_generation=2"); err != nil {
						return err
					}
				case "expired":
					*now = meta.IdleExpiresAt
				case "commit-error":
					if _, err := tx.Exec("INSERT INTO tx_child VALUES(7)"); err != nil {
						return err
					}
				}
				if _, err := s.RenewTx(sessionContext, tx, actor); err != nil {
					return err
				}
				if mode == "after-renew-error" {
					return marker
				}
				return nil
			})
			success := mode == "success" || mode == "absolute-cap"
			if success && err != nil || !success && err == nil {
				t.Fatalf("unexpected transaction result %v", err)
			}
			if mode == "revoked" || mode == "generation" || mode == "expired" {
				if !errors.Is(err, ErrInvalidSession) {
					t.Fatal("identity failure", err)
				}
			}
			want := int64(0)
			if success {
				want = 1
			}
			if deviceInt(t, s, "SELECT count(*) FROM tx_business") != want || deviceInt(t, s, "SELECT count(*) FROM changes") != want || deviceInt(t, s, "SELECT count(*) FROM tx_child") != 0 {
				t.Fatal("partial business/changes commit")
			}
			idle := persistedIdle(t, s, meta.ID)
			if success {
				expected := now.Add(24 * time.Hour).UnixNano()
				if expected > meta.AbsoluteExpiresAt.UnixNano() {
					expected = meta.AbsoluteExpiresAt.UnixNano()
				}
				if idle != expected {
					t.Fatal("successful renewal/cap", idle, expected)
				}
			} else if idle != old {
				t.Fatal("failed transaction renewed")
			}
			if deviceRevoked(t, s, meta.ID) != 0 || deviceInt(t, s, "SELECT auth_generation FROM administrator") != 1 {
				t.Fatal("failed identity mutation not rolled back")
			}
		})
	}
}

func deviceHash(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }

func TestDeviceSessionRevocationCommitBoundaries(t *testing.T) {
	for _, mode := range []string{"actor-entry-expired", "actor-tail-expired", "self-tail-expired", "self-final-expired", "target-update-expired", "target-tail-expired", "actor-revoked", "actor-generation", "self-generation", "target-revoked", "target-generation", "unknown", "update-error", "commit-error"} {
		t.Run(mode, func(t *testing.T) {
			s, now := sessionFixture(t)
			am, as := createSession(t, s, "web")
			target, _ := createSession(t, s, "android")
			if strings.HasPrefix(mode, "target-") && strings.HasSuffix(mode, "expired") {
				deviceSQL(t, s, "UPDATE sessions SET idle_expires_at=? WHERE id=?", now.Add(2*time.Hour).UnixNano(), target.ID)
			}
			actor := deviceActor(t, s, as, "web")
			old := persistedIdle(t, s, am.ID)
			id := target.ID
			want := ErrInvalidSession
			if strings.HasPrefix(mode, "self-") {
				id = am.ID
			}
			if strings.HasPrefix(mode, "target-") || mode == "unknown" {
				want = ErrSessionNotFound
			}
			switch mode {
			case "actor-entry-expired":
				*now = am.IdleExpiresAt
			case "actor-revoked":
				deviceSQL(t, s, "UPDATE sessions SET revoked=1 WHERE id=?", am.ID)
			case "actor-generation":
				deviceSQL(t, s, "UPDATE administrator SET auth_generation=2")
			case "target-revoked":
				deviceSQL(t, s, "UPDATE sessions SET revoked=1 WHERE id=?", target.ID)
			case "target-generation":
				deviceSQL(t, s, "UPDATE sessions SET auth_generation=2 WHERE id=?", target.ID)
			case "unknown":
				id = "unknown"
			case "self-generation":
				deviceSQL(t, s, `CREATE TRIGGER generation_at_self AFTER UPDATE OF revoked ON sessions WHEN NEW.id='`+am.ID+`' BEGIN UPDATE administrator SET auth_generation=2; END`)
			case "update-error":
				want = ErrStorage
				deviceSQL(t, s, `CREATE TRIGGER fail_revoke BEFORE UPDATE OF revoked ON sessions BEGIN SELECT RAISE(ABORT,'fixture'); END`)
			case "commit-error":
				want = ErrStorage
				deviceSQL(t, s, `CREATE TABLE hook_parent(id INTEGER PRIMARY KEY); CREATE TABLE hook_child(id INTEGER REFERENCES hook_parent(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_commit AFTER UPDATE OF revoked ON sessions BEGIN INSERT INTO hook_child VALUES(9); END`)
			}
			// 受控时间步进，无 sleep；严格区分入口、UPDATE 与末尾两个时刻。
			calls := 0
			base := *now
			if strings.HasSuffix(mode, "tail-expired") || mode == "self-final-expired" || mode == "target-update-expired" {
				s.now = func() time.Time {
					calls++
					threshold := 3
					if mode == "self-final-expired" {
						threshold = 4
					}
					if mode == "target-update-expired" {
						threshold = 2
					}
					if calls >= threshold {
						if strings.HasPrefix(mode, "target-") {
							return base.Add(2 * time.Hour)
						}
						return am.IdleExpiresAt
					}
					return base.Add(time.Hour)
				}
			}
			notifications := 0
			s.SetRevocationHook(func(string) { notifications++ })
			err := s.RevokeDeviceSession(sessionContext, actor, id)
			if !errors.Is(err, want) {
				t.Fatalf("want %v got %v (clock calls %d)", want, err, calls)
			}
			wantTarget := int64(0)
			if mode == "target-revoked" {
				wantTarget = 1
			}
			if deviceRevoked(t, s, target.ID) != wantTarget || persistedIdle(t, s, am.ID) != old || notifications != 0 {
				t.Fatal("failed revoke/renew/hook escaped rollback")
			}
			if mode == "self-tail-expired" || mode == "self-final-expired" || mode == "self-generation" {
				if deviceRevoked(t, s, am.ID) != 0 || deviceInt(t, s, "SELECT auth_generation FROM administrator") != 1 {
					t.Fatal("self revoke failed to roll back")
				}
			}
		})
	}
}

func TestDeviceSessionPostCommitHookAndLogout(t *testing.T) {
	s, now := sessionFixture(t)
	am, as := createSession(t, s, "web")
	target, _ := createSession(t, s, "android")
	actor := deviceActor(t, s, as, "web")
	*now = now.Add(time.Hour)
	old := persistedIdle(t, s, am.ID)
	var got []string
	s.SetRevocationHook(func(id string) {
		// 回调能从独立只读连接看到撤销与成功续期，且可重设 hook，不持有 hookMu。
		if deviceRevoked(t, s, id) != 1 || persistedIdle(t, s, am.ID) <= old {
			t.Error("hook before commit")
		}
		got = append(got, id)
		s.SetRevocationHook(nil)
	})
	if err := s.RevokeDeviceSession(sessionContext, actor, target.ID); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != target.ID {
		t.Fatal("wrong notification", got)
	}
	s.SetRevocationHook(func(id string) {
		if deviceRevoked(t, s, id) != 1 {
			t.Error("logout hook before commit")
		}
		got = append(got, id)
	})
	if err := s.Revoke(sessionContext, as.Token, "web"); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(sessionContext, as.Token, "web"); !errors.Is(err, ErrInvalidSession) {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1] != am.ID {
		t.Fatal("logout not shared exact ID", got)
	}
	// 成功自撤只通知一次，不续期。
	self, ss := createSession(t, s, "web")
	selfActor := deviceActor(t, s, ss, "web")
	before := persistedIdle(t, s, self.ID)
	if err := s.RevokeDeviceSession(sessionContext, selfActor, self.ID); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2] != self.ID || persistedIdle(t, s, self.ID) != before {
		t.Fatal("self revoke identity/renewal")
	}
}

func TestDeviceSessionHookConcurrentReplacement(t *testing.T) {
	s, _ := sessionFixture(t)
	var tokens []string
	for i := 0; i < 24; i++ {
		_, secret := createSession(t, s, "android")
		tokens = append(tokens, secret.Token)
	}
	var calls atomic.Int64
	h := func(string) { calls.Add(1) }
	s.SetRevocationHook(h)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			s.SetRevocationHook(h)
		}
	}()
	errs := make(chan error, 1)
	go func() {
		defer wg.Done()
		<-start
		for _, token := range tokens {
			if err := s.Revoke(sessionContext, token, "android"); err != nil {
				errs <- err
				return
			}
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if calls.Load() != int64(len(tokens)) {
		t.Fatal("lost or duplicate hook", calls.Load())
	}
}

func TestDeviceSessionQueuedInvalidation(t *testing.T) {
	for _, mode := range []string{"actor-revoked", "actor-generation", "target-revoked", "target-expired"} {
		t.Run(mode, func(t *testing.T) {
			s, now := sessionFixture(t)
			am, as := createSession(t, s, "web")
			target, _ := createSession(t, s, "android")
			other, _ := createSession(t, s, "android")
			actor := deviceActor(t, s, as, "web")
			old := persistedIdle(t, s, am.ID)
			var clock atomic.Int64
			clock.Store(now.UnixNano())
			s.now = func() time.Time { return time.Unix(0, clock.Load()).UTC() }
			entered, release := make(chan struct{}), make(chan struct{})
			writerDone := make(chan error, 1)
			// 已取得同一 Store.Write 的唯一写连接；排队请求一定在失效提交后复核。
			go func() {
				writerDone <- s.store.Write(sessionContext, func(tx *sql.Tx) error {
					var err error
					switch mode {
					case "actor-revoked":
						_, err = tx.Exec("UPDATE sessions SET revoked=1 WHERE id=?", am.ID)
					case "actor-generation":
						_, err = tx.Exec("UPDATE administrator SET auth_generation=2")
					case "target-revoked":
						_, err = tx.Exec("UPDATE sessions SET revoked=1 WHERE id=?", target.ID)
					case "target-expired":
						_, err = tx.Exec("UPDATE sessions SET idle_expires_at=? WHERE id=?", clock.Load(), target.ID)
					}
					close(entered)
					<-release
					return err
				})
			}()
			<-entered
			var hook atomic.Int64
			s.SetRevocationHook(func(string) { hook.Add(1) })
			started := make(chan struct{})
			done := make(chan error, 1)
			go func() { close(started); done <- s.RevokeDeviceSession(sessionContext, actor, target.ID) }()
			<-started
			close(release)
			if err := <-writerDone; err != nil {
				t.Fatal(err)
			}
			want := ErrInvalidSession
			if strings.HasPrefix(mode, "target-") {
				want = ErrSessionNotFound
			}
			if err := <-done; !errors.Is(err, want) {
				t.Fatalf("queued revoke: %v want %v", err, want)
			}
			expected := int64(0)
			if mode == "target-revoked" {
				expected = 1
			}
			if deviceRevoked(t, s, target.ID) != expected || deviceRevoked(t, s, other.ID) != 0 || persistedIdle(t, s, am.ID) != old || hook.Load() != 0 {
				t.Fatal("queued invalidation affected another session or renewed")
			}
		})
	}
}

func TestDeviceSessionRevokeSerializesResetAndRelogin(t *testing.T) {
	s, now := sessionFixture(t)
	_, as := createSession(t, s, "web")
	target, _ := createSession(t, s, "android")
	actor := deviceActor(t, s, as, "web")
	base := *now
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	// 首次事务内读完 actor 后，时钟屏障证明撤销已持有写连接。
	s.now = func() time.Time { once.Do(func() { close(entered); <-release }); return base }
	revoked := make(chan error, 1)
	go func() { revoked <- s.RevokeDeviceSession(sessionContext, actor, target.ID) }()
	<-entered
	resetStarted := make(chan struct{})
	reset := make(chan error, 1)
	go func() {
		close(resetStarted)
		reset <- s.store.Write(sessionContext, func(tx *sql.Tx) error { _, err := tx.Exec("UPDATE administrator SET auth_generation=2"); return err })
	}()
	<-resetStarted
	close(release)
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	if err := <-reset; err != nil {
		t.Fatal(err)
	}
	fresh, secret, err := s.Create(sessionContext, 2, SessionInput{"android", target.DeviceID, "重登"})
	if err != nil {
		t.Fatal(err)
	}
	freshActor := deviceActor(t, s, secret, "android")
	if err = s.RevokeDeviceSession(sessionContext, freshActor, target.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("late old id after reset", err)
	}
	if deviceRevoked(t, s, fresh.ID) != 0 || fresh.ID == target.ID {
		t.Fatal("late revoke reached fresh identity")
	}
}

func TestDeviceSessionLogoutRollbackDoesNotNotify(t *testing.T) {
	for _, mode := range []string{"update", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := sessionFixture(t)
			meta, secret := createSession(t, s, "web")
			if mode == "update" {
				deviceSQL(t, s, `CREATE TRIGGER fail_logout BEFORE UPDATE OF revoked ON sessions BEGIN SELECT RAISE(ABORT,'fixture'); END`)
			} else {
				deviceSQL(t, s, `CREATE TABLE logout_parent(id INTEGER PRIMARY KEY);CREATE TABLE logout_child(id INTEGER REFERENCES logout_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail_logout AFTER UPDATE OF revoked ON sessions BEGIN INSERT INTO logout_child VALUES(1);END`)
			}
			n := 0
			s.SetRevocationHook(func(string) { n++ })
			if err := s.Revoke(sessionContext, secret.Token, "web"); !errors.Is(err, ErrStorage) {
				t.Fatal(fmt.Sprintf("logout failure: %v", err))
			}
			if n != 0 || deviceRevoked(t, s, meta.ID) != 0 {
				t.Fatal("logout failure notified or committed")
			}
		})
	}
}

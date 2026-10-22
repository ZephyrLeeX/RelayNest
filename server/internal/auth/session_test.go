package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var sessionContext = context.Background()

func sessionFixture(t *testing.T) (*SessionStore, *time.Time) {
	t.Helper()
	db := testStore(t, filepath.Join(t.TempDir(), "session.sqlite"))
	if err := db.Write(sessionContext, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO administrator VALUES(1,'Admin','fixture',1)")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	service := NewSessionStore(db)
	service.now = func() time.Time { return now }
	return service, &now
}

func createSession(t *testing.T, s *SessionStore, client string) (Session, SessionSecrets) {
	t.Helper()
	in := SessionInput{ClientType: client, DeviceName: "测试设备"}
	if client == "android" {
		in.DeviceID = "12345678-1234-4123-8123-123456789abc"
	}
	meta, secret, err := s.Create(sessionContext, 1, in)
	if err != nil {
		t.Fatal(err)
	}
	return meta, secret
}

func persistedIdle(t *testing.T, s *SessionStore, id string) int64 {
	t.Helper()
	var idle int64
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT idle_expires_at FROM sessions WHERE id=?", id).Scan(&idle)
	}); err != nil {
		t.Fatal(err)
	}
	return idle
}

func TestSessionDeadlines(t *testing.T) {
	for _, client := range []string{"web", "android"} {
		t.Run(client, func(t *testing.T) {
			for _, kind := range []string{"idle", "absolute"} {
				for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
					t.Run(kind+delta.String(), func(t *testing.T) {
						s, now := sessionFixture(t)
						initial := *now
						meta, secret := createSession(t, s, client)
						ttl, absolute := sessionTTL(client)
						if !meta.IdleExpiresAt.Equal(initial.Add(ttl)) || !meta.AbsoluteExpiresAt.Equal(initial.Add(absolute)) {
							t.Fatal("wrong initial TTL")
						}
						target := meta.IdleExpiresAt
						if kind == "absolute" {
							target = meta.AbsoluteExpiresAt
							for now.Add(ttl / 2).Before(target) {
								*now = now.Add(ttl / 2)
								if _, _, err := s.Authenticate(sessionContext, secret.Token, client); err != nil {
									t.Fatal(err)
								}
							}
						}
						*now = target.Add(delta)
						before := persistedIdle(t, s, meta.ID)
						got, _, err := s.Authenticate(sessionContext, secret.Token, client)
						if delta < 0 {
							if err != nil {
								t.Fatal(err)
							}
							want := now.Add(ttl)
							if want.After(meta.AbsoluteExpiresAt) {
								want = meta.AbsoluteExpiresAt
							}
							if !got.IdleExpiresAt.Equal(want) {
								t.Fatal("renewal did not respect bound")
							}
						} else {
							if !errors.Is(err, ErrInvalidSession) {
								t.Fatal("accepted exact/past deadline")
							}
							if persistedIdle(t, s, meta.ID) != before {
								t.Fatal("failure renewed session")
							}
						}
					})
				}
			}
		})
	}
}

func TestSessionPersistenceSecretsAndIsolation(t *testing.T) {
	s, now := sessionFixture(t)
	meta, secret := createSession(t, s, "web")
	other, otherSecret := createSession(t, s, "web")
	_, android := createSession(t, s, "android")
	if meta.ID == other.ID || meta.DeviceID == other.DeviceID || secret.Token == otherSecret.Token || secret.CSRFToken == otherSecret.CSRFToken || secret.CSRFToken == "" || android.CSRFToken != "" {
		t.Fatal("credentials not isolated")
	}
	var stored []byte
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT token_hash FROM sessions WHERE id=?", meta.ID).Scan(&stored)
	}); err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(secret.Token))
	if string(stored) != string(expected[:]) {
		t.Fatal("credential digest mismatch")
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{secret.Token, secret.CSRFToken, string(stored)} {
		if strings.Contains(string(encoded), value) {
			t.Fatal("secret in metadata")
		}
	}
	*now = now.Add(time.Hour)
	before := persistedIdle(t, s, meta.ID)
	if _, _, err := s.Inspect(sessionContext, secret.Token, "web"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(sessionContext, secret.Token, "android"); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("cross-channel accepted")
	}
	if _, _, err := s.Authenticate(sessionContext, "bad", "web"); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("malformed accepted")
	}
	if persistedIdle(t, s, meta.ID) != before {
		t.Fatal("inspection/failure renewed")
	}
	// 第二连接模拟重启后全部状态从 SQLite 恢复，服务对象不保存凭据。
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
	reopened := NewSessionStore(testStore(t, path))
	s.store = reopened.store
	reopened.now = s.now
	got, boot, err := reopened.Inspect(sessionContext, secret.Token, "web")
	if err != nil || got.ID != meta.ID || boot.CSRFToken != secret.CSRFToken {
		t.Fatal("restart lost state/bootstrap")
	}
	if err := reopened.Revoke(sessionContext, secret.Token, "web"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(sessionContext, secret.Token, "web"); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("revocation not visible")
	}
	for _, pair := range []struct{ token, client string }{{otherSecret.Token, "web"}, {android.Token, "android"}} {
		if _, _, err := s.Authenticate(sessionContext, pair.token, pair.client); err != nil {
			t.Fatal("revoked other device", err)
		}
	}
}

type failingRandom struct{ remaining int }

func (r *failingRandom) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if len(p) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := rand.Read(p)
	r.remaining -= n
	return n, err
}

func TestSessionCreationFailures(t *testing.T) {
	s, _ := sessionFixture(t)
	for _, n := range []int{0, 16, 32, 63} {
		s.random = &failingRandom{n}
		if _, _, err := s.Create(sessionContext, 1, SessionInput{ClientType: "web", DeviceName: "browser"}); !errors.Is(err, ErrSessionRandom) {
			t.Fatal("random failure accepted")
		}
	}
	s.random = rand.Reader
	for _, in := range []SessionInput{{ClientType: "other", DeviceName: "x"}, {ClientType: "web"}, {ClientType: "android", DeviceName: "x", DeviceID: "invalid"}, {ClientType: "web", DeviceName: "x", DeviceID: "12345678-1234-4123-8123-123456789abc"}} {
		if _, _, err := s.Create(sessionContext, 1, in); !errors.Is(err, ErrInvalidSessionInput) {
			t.Fatal("invalid metadata accepted")
		}
	}
	if _, _, err := s.Create(sessionContext, 2, SessionInput{ClientType: "web", DeviceName: "x"}); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("stale generation accepted")
	}
	ctx, cancel := context.WithCancel(sessionContext)
	cancel()
	if _, _, err := s.Create(ctx, 1, SessionInput{ClientType: "web", DeviceName: "x"}); !errors.Is(err, ErrStorage) {
		t.Fatal("cancelled creation accepted")
	}
	var count int
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error { return tx.QueryRow("SELECT count(*) FROM sessions").Scan(&count) }); err != nil || count != 0 {
		t.Fatal("partial session survived")
	}
}

func TestSessionResetCreateCompetition(t *testing.T) {
	for i := 0; i < 4; i++ {
		s, _ := sessionFixture(t)
		_, old := createSession(t, s, "web")
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var createErr, resetErr error
		var created SessionSecrets
		go func() {
			defer wg.Done()
			<-start
			_, created, createErr = s.Create(sessionContext, 1, SessionInput{ClientType: "web", DeviceName: "racing"})
		}()
		go func() {
			defer wg.Done()
			<-start
			resetErr = ResetPassword(sessionContext, s.store, []byte("new password"))
		}()
		close(start)
		wg.Wait()
		if resetErr != nil || (createErr != nil && !errors.Is(createErr, ErrInvalidSession)) {
			t.Fatal("competition failed")
		}
		for _, token := range []string{old.Token, created.Token} {
			if token != "" {
				if _, _, err := s.Authenticate(sessionContext, token, "web"); !errors.Is(err, ErrInvalidSession) {
					t.Fatal("old generation survived reset")
				}
			}
		}
		if _, _, err := s.Create(sessionContext, 1, SessionInput{ClientType: "web", DeviceName: "stale"}); !errors.Is(err, ErrInvalidSession) {
			t.Fatal("late stale creation accepted")
		}
		admin, err := ReadAdministrator(sessionContext, s.store)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Create(sessionContext, admin.AuthGeneration, SessionInput{ClientType: "web", DeviceName: "new"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSessionRevokeRenewCompetition(t *testing.T) {
	s, _ := sessionFixture(t)
	var path string
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error {
		var seq int
		var name string
		return tx.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path)
	}); err != nil {
		t.Fatal(err)
	}
	other := NewSessionStore(testStore(t, path))
	other.now = s.now

	for i := 0; i < 24; i++ {
		meta, secret := createSession(t, s, "web")
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var renewal, revoke error
		go func() {
			defer wg.Done()
			<-start
			_, _, renewal = other.Authenticate(sessionContext, secret.Token, "web")
		}()
		go func() { defer wg.Done(); <-start; revoke = s.Revoke(sessionContext, secret.Token, "web") }()
		close(start)
		wg.Wait()
		if revoke != nil || (renewal != nil && !errors.Is(renewal, ErrInvalidSession)) {
			t.Fatal("competition failed")
		}
		before := persistedIdle(t, s, meta.ID)
		if _, _, err := other.Authenticate(sessionContext, secret.Token, "web"); !errors.Is(err, ErrInvalidSession) {
			t.Fatal("revoked session resurrected")
		}
		if persistedIdle(t, s, meta.ID) != before {
			t.Fatal("revoked failure renewed")
		}
	}
}

// 阻塞随机源让 reset 确定在创建事务之前提交，覆盖另一种竞争顺序。
type gatedRandom struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *gatedRandom) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.entered); <-r.release })
	return rand.Read(p)
}

func TestSessionResetBeforeCreateTransaction(t *testing.T) {
	s, _ := sessionFixture(t)
	gate := &gatedRandom{entered: make(chan struct{}), release: make(chan struct{})}
	s.random = gate
	done := make(chan error, 1)
	go func() {
		_, _, err := s.Create(sessionContext, 1, SessionInput{ClientType: "web", DeviceName: "blocked"})
		done <- err
	}()
	<-gate.entered
	resetErr := ResetPassword(sessionContext, s.store, []byte("changed password"))
	close(gate.release)
	if resetErr != nil {
		t.Fatal(resetErr)
	}
	if err := <-done; !errors.Is(err, ErrInvalidSession) {
		t.Fatal("reset-before-create accepted stale generation")
	}
	var count int
	if err := s.store.Read(sessionContext, func(tx *sql.Tx) error { return tx.QueryRow("SELECT count(*) FROM sessions").Scan(&count) }); err != nil || count != 0 {
		t.Fatal("stale create left session")
	}
}

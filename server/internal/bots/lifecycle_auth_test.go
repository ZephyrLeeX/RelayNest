package bots_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

// 通过真实 RequestSecurity 签发 actor，再由 WithAuthorizedWrite 执行生命周期写入。
// 仅缩短持久 idle 截止时间以观察续期，不依赖 sleep 或伪造私有 actor。
func TestLifecycleAuthorizedSessionTransaction(t *testing.T) {
	for _, operation := range []string{"api", "webhook", "patch"} {
		for _, mode := range []string{"success", "changes", "renew", "commit", "revoked"} {
			t.Run(operation+"/"+mode, func(t *testing.T) {
				c := context.Background()
				s, err := storage.Open(c, storage.Config{Path: filepath.Join(t.TempDir(), "lifecycle.sqlite")})
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				writeSQL := func(q string, args ...any) {
					t.Helper()
					if err := s.Write(c, func(tx *sql.Tx) error { _, err := tx.ExecContext(c, q, args...); return err }); err != nil {
						t.Fatal(err)
					}
				}
				integer := func(q string, args ...any) int64 {
					t.Helper()
					var n int64
					if err := s.Read(c, func(tx *sql.Tx) error { return tx.QueryRowContext(c, q, args...).Scan(&n) }); err != nil {
						t.Fatal(err)
					}
					return n
				}
				writeSQL("INSERT INTO administrator VALUES(1,'Admin','fixture',1)")
				sessions := auth.NewSessionStore(s)
				actorSession, secret, err := sessions.Create(c, 1, auth.SessionInput{ClientType: "android", DeviceID: "12345678-1234-4123-8123-123456789abc", DeviceName: "actor"})
				if err != nil {
					t.Fatal(err)
				}
				other, _, err := sessions.Create(c, 1, auth.SessionInput{ClientType: "web", DeviceName: "other"})
				if err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(time.Hour).UnixNano()
				writeSQL("UPDATE sessions SET idle_expires_at=?", deadline)
				policy, err := httpapi.NewOriginPolicy("https://relay.test", nil)
				if err != nil {
					t.Fatal(err)
				}
				guard := auth.NewRequestSecurity(sessions, policy)
				req := httptest.NewRequest("POST", "https://relay.test/api/v1/admin/bots/fixture", nil)
				req.Header.Set("Authorization", "Bearer "+secret.Token)
				response := httptest.NewRecorder()
				actor, ok := guard.Authorize(response, req, auth.RequestWrite)
				if !ok {
					t.Fatal("actor issuance", response.Code)
				}
				r := bots.New(s)
				created, _, err := r.Create(c, bots.CreateRequest{Name: "事务", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
				if err != nil {
					t.Fatal(err)
				}
				writeSQL("CREATE TABLE queue_marker(bot_id TEXT)")
				switch mode {
				case "changes":
					writeSQL(`CREATE TRIGGER failure BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'changes'); END`)
				case "renew":
					writeSQL(`CREATE TRIGGER failure BEFORE UPDATE ON sessions BEGIN SELECT RAISE(ABORT,'renew'); END`)
				case "commit":
					writeSQL("CREATE TABLE commit_failure(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)")
					writeSQL(`CREATE TRIGGER failure AFTER INSERT ON changes BEGIN INSERT INTO commit_failure VALUES('missing'); END`)
				case "revoked":
					writeSQL("UPDATE sessions SET revoked=1 WHERE id=?", actorSession.ID)
				}
				credentialSnapshot := func() string {
					t.Helper()
					var v string
					if err := s.Read(c, func(tx *sql.Tx) error {
						return tx.QueryRowContext(c, `SELECT group_concat(kind||':'||credential_id||':'||generation||':'||hex(digest),'|') FROM (SELECT * FROM bot_credentials ORDER BY kind)`).Scan(&v)
					}); err != nil {
						t.Fatal(err)
					}
					return v
				}
				before := credentialSnapshot()
				write := func(c context.Context, fn func(*sql.Tx) error) error {
					return sessions.WithAuthorizedWrite(c, actor, fn)
				}
				hooks := 0
				hook := func(c context.Context, tx *sql.Tx, e bots.LifecycleChange) error {
					hooks++
					if e.After.ID != created.Bot.ID {
						return errors.New("wrong hook Bot")
					}
					_, err := tx.ExecContext(c, "INSERT INTO queue_marker VALUES(?)", e.After.ID)
					return err
				}
				var released bool
				switch operation {
				case "api":
					v, seq, e := r.RotateAPITokenWithWrite(c, created.Bot.ID, write, hook)
					err = e
					released = v.Token != "" || seq != 0
				case "webhook":
					v, seq, e := r.RotateWebhookKeyWithWrite(c, created.Bot.ID, write)
					err = e
					released = v.Key != "" || seq != 0
				case "patch":
					off := false
					v, seq, e := r.PatchWithLifecycle(c, created.Bot.ID, bots.PatchRequest{ExpectedVersion: 1, Enabled: &off}, write, hook)
					err = e
					released = v.ID != "" || seq != 0
				}
				after, getErr := r.Get(c, created.Bot.ID)
				if getErr != nil {
					t.Fatal(getErr)
				}
				if mode == "success" {
					if err != nil || !released || after.Version != 2 || integer("SELECT count(*) FROM changes") != 2 || integer("SELECT idle_expires_at FROM sessions WHERE id=?", actorSession.ID) <= deadline {
						t.Fatal("atomic success", err)
					}
					expected := int64(1)
					if operation == "webhook" {
						expected = 0
					}
					if int64(hooks) != expected || integer("SELECT count(*) FROM queue_marker") != expected {
						t.Fatal("hook isolation")
					}
				} else {
					if err == nil || released || after != created.Bot || credentialSnapshot() != before || integer("SELECT count(*) FROM changes") != 1 || integer("SELECT count(*) FROM queue_marker") != 0 || integer("SELECT idle_expires_at FROM sessions WHERE id=?", actorSession.ID) != deadline {
						t.Fatal("partial session transaction", err)
					}
					if mode == "revoked" && hooks != 0 {
						t.Fatal("revoked actor reached business")
					}
				}
				if integer("SELECT idle_expires_at FROM sessions WHERE id=?", other.ID) != deadline {
					t.Fatal("other session renewed")
				}
			})
		}
	}
}

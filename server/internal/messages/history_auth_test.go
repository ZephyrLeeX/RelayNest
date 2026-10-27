package messages_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
)

type historySecurity struct {
	store, other                 *storage.Store
	sessions                     *auth.SessionStore
	repo                         *messages.Repository
	actor                        auth.AdminRequest
	botID, messageID, instanceID string
	entered                      bool
}

func historySQL(t *testing.T, store *storage.Store, query string, args ...any) {
	t.Helper()
	if err := store.Write(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec(query, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func newHistorySecurity(t *testing.T) *historySecurity {
	t.Helper()
	ctx := context.Background()
	cfg := storage.Config{Path: filepath.Join(t.TempDir(), "history-auth.sqlite"), BusyTimeout: 50 * time.Millisecond}
	s, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	other, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() })
	f := &historySecurity{store: s, other: other, sessions: auth.NewSessionStore(s), repo: messages.New(s)}
	historySQL(t, s, `INSERT INTO administrator VALUES(1,'admin','test-hash',1)`)
	_, secrets, err := f.sessions.Create(ctx, 1, auth.SessionInput{ClientType: "android", DeviceID: "11111111-1111-4111-8111-111111111111", DeviceName: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Read(ctx, func(tx *sql.Tx) error { var e error; f.instanceID, e = storage.ReadInstanceIDTx(ctx, tx); return e }); err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.NewOriginPolicy("https://relaynest.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "https://relaynest.test/client/history", nil)
	request.Header.Set("Authorization", "Bearer "+secrets.Token)
	request.Header.Set("X-RelayNest-Instance-ID", f.instanceID)
	var ok bool
	f.actor, ok = auth.NewRequestSecurity(f.sessions, policy).Authorize(httptest.NewRecorder(), request, auth.RequestRead)
	if !ok {
		t.Fatal("real Android actor not authorized")
	}
	bot, _, err := bots.New(s).Create(ctx, bots.CreateRequest{Name: "history", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.botID = bot.Bot.ID
	identity, err := bots.New(s).Authenticate(ctx, f.botID, bots.APIToken, bot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	created, err := f.repo.CreateApplication(ctx, identity, messages.TextInput{BotID: f.botID, Format: "plain", Text: "protected body"})
	if err != nil {
		t.Fatal(err)
	}
	f.messageID = created.Message.ID
	return f
}

// 这里只演示受信调用方的发布规则，不提供生产授权包装或公开 HTTP 入口。
func (f *historySecurity) read(ctx context.Context, actor auth.AdminRequest, write bool, work func(*sql.Tx) error) (messages.HistoryPage, error) {
	var candidate messages.HistoryPage
	f.entered = false
	callback := func(tx *sql.Tx) error {
		f.entered = true
		var err error
		candidate, err = f.repo.ListHistoryTx(ctx, tx, f.botID, messages.HistoryQuery{Limit: 2})
		if err != nil {
			return err
		}
		detail, err := f.repo.GetHistoryMessageTx(ctx, tx, f.botID, f.messageID)
		if err != nil {
			return err
		}
		if len(candidate.Items) != 1 || detail.ID != candidate.Items[0].ID {
			return errors.New("inconsistent history")
		}
		if err = f.repo.ValidateReplyTx(ctx, tx, f.botID, f.messageID); err != nil {
			return err
		}
		if work != nil {
			return work(tx)
		}
		return nil
	}
	var err error
	if write {
		err = f.sessions.WithAuthorizedWrite(ctx, actor, callback)
	} else {
		_, _, err = f.sessions.WithAuthorizedRead(ctx, actor, callback)
	}
	if err != nil {
		return messages.HistoryPage{}, err
	}
	return candidate, nil
}

func historyIdle(t *testing.T, f *historySecurity) int64 {
	t.Helper()
	var idle int64
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT idle_expires_at FROM sessions WHERE id=?`, f.actor.Session.ID).Scan(&idle)
	}); err != nil {
		t.Fatal(err)
	}
	return idle
}

func TestHistoryRealAuthorizationAndInvalidation(t *testing.T) {
	for _, write := range []bool{false, true} {
		for _, mode := range []string{"success", "revoked", "generation", "instance", "forged", "tampered"} {
			t.Run(mode+map[bool]string{false: "-read", true: "-write"}[write], func(t *testing.T) {
				f := newHistorySecurity(t)
				actor := f.actor
				switch mode {
				case "revoked":
					historySQL(t, f.other, `UPDATE sessions SET revoked=1`)
				case "generation":
					historySQL(t, f.other, `UPDATE administrator SET auth_generation=auth_generation+1`)
				case "instance":
					if err := f.other.Write(context.Background(), func(tx *sql.Tx) error {
						_, err := storage.RotateInstanceIDTx(context.Background(), tx, f.instanceID)
						return err
					}); err != nil {
						t.Fatal(err)
					}
				case "forged":
					actor = auth.AdminRequest{Session: f.actor.Session, Source: f.actor.Source}
				case "tampered":
					actor.Session.DeviceName = "other"
				}
				if mode == "success" {
					historySQL(t, f.store, `UPDATE sessions SET idle_expires_at=? WHERE id=?`, time.Now().Add(time.Hour).Unix(), f.actor.Session.ID)
				}
				before := historyIdle(t, f)
				entered := false
				page, err := f.read(context.Background(), actor, write, func(*sql.Tx) error { entered = true; return nil })
				if mode == "success" {
					if err != nil || !entered || len(page.Items) != 1 || page.Items[0].Text != "protected body" {
						t.Fatal("authorized history", err)
					}
					if historyIdle(t, f) <= before {
						t.Fatal("successful history did not renew")
					}
				} else {
					if err == nil || entered || f.entered || !reflect.DeepEqual(page, messages.HistoryPage{}) {
						t.Fatal("invalid actor entered body read", err)
					}
					if historyIdle(t, f) != before {
						t.Fatal("failed access renewed")
					}
				}
			})
		}
	}
}

func TestHistoryFinalAuthorizationAndCommitFailureDiscardsCandidate(t *testing.T) {
	for _, write := range []bool{false, true} {
		for _, mode := range []string{"revoke-in-work", "generation-in-work", "instance-in-work", "renew-trigger", "commit", "cancel-in-work"} {
			t.Run(mode+map[bool]string{false: "-read", true: "-write"}[write], func(t *testing.T) {
				f := newHistorySecurity(t)
				if mode == "renew-trigger" {
					historySQL(t, f.store, `CREATE TRIGGER history_renew AFTER UPDATE OF idle_expires_at ON sessions BEGIN UPDATE administrator SET auth_generation=auth_generation+1; END`)
				}
				if mode == "commit" {
					historySQL(t, f.store, `CREATE TABLE history_commit_failure(id INTEGER REFERENCES administrator(id) DEFERRABLE INITIALLY DEFERRED)`)
				}
				before := historyIdle(t, f)
				entered := false
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				page, err := f.read(ctx, f.actor, write, func(tx *sql.Tx) error {
					entered = true
					var err error
					switch mode {
					case "revoke-in-work":
						_, err = tx.Exec(`UPDATE sessions SET revoked=1`)
					case "generation-in-work":
						_, err = tx.Exec(`UPDATE administrator SET auth_generation=auth_generation+1`)
					case "instance-in-work":
						_, err = storage.RotateInstanceIDTx(context.Background(), tx, f.instanceID)
					case "commit":
						_, err = tx.Exec(`INSERT INTO history_commit_failure VALUES(2)`)
					case "cancel-in-work":
						cancel()
					}
					return err
				})
				if err == nil || !entered || !reflect.DeepEqual(page, messages.HistoryPage{}) {
					t.Fatal("failed final check published body", err)
				}
				if historyIdle(t, f) != before {
					t.Fatal("failed final check renewed")
				}
				// 业务内失效及续期触发器副作用均应完整回滚。
				if mode == "renew-trigger" {
					historySQL(t, f.store, `DROP TRIGGER history_renew`)
				}
				if _, err := f.read(context.Background(), f.actor, write, nil); err != nil {
					t.Fatal("failed final transaction changed authority", err)
				}
			})
		}
	}
}

func TestHistoryAuthorizedReadWaitCancellation(t *testing.T) {
	f := newHistorySecurity(t)
	held, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- f.other.Write(context.Background(), func(tx *sql.Tx) error { close(held); <-release; return nil })
	}()
	<-held
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	entered := false
	page, err := f.read(ctx, f.actor, false, func(*sql.Tx) error { entered = true; return nil })
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if !errors.Is(err, context.DeadlineExceeded) || entered || f.entered || !reflect.DeepEqual(page, messages.HistoryPage{}) {
		t.Fatal("cancelled gate wait read body", err)
	}
	// 新请求能继续读；取消没有滞留共享写准入。
	if _, err = f.read(context.Background(), f.actor, false, nil); err != nil {
		t.Fatal(err)
	}
}

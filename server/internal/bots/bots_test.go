package bots

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
)

var ctx = context.Background()

func fixture(t *testing.T) (*storage.Store, *Repository, storage.Config) {
	t.Helper()
	cfg := storage.Config{Path: filepath.Join(t.TempDir(), "bots.sqlite")}
	s, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, New(s), cfg
}
func create(t *testing.T, r *Repository, c Capabilities) Created {
	t.Helper()
	v, seq, err := r.Create(ctx, CreateRequest{"测试", true, c})
	if err != nil || seq <= 0 {
		t.Fatalf("create: seq=%d err=%v", seq, err)
	}
	return v
}
func sqlWrite(t *testing.T, s *storage.Store, q string, args ...any) {
	t.Helper()
	if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.ExecContext(ctx, q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}
func scalar(t *testing.T, s *storage.Store, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := s.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRowContext(ctx, q, args...).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestPersistenceAndBoundary(t *testing.T) {
	s, r, cfg := fixture(t)
	v := create(t, r, Capabilities{true, true})
	if v.APIToken.Token == v.WebhookKey.Key || v.APIToken.Credential == v.WebhookKey.Credential {
		t.Fatal("credentials not independent")
	}
	data, _ := json.Marshal(v)
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil || len(keys) != 3 || keys["bot"] == nil || keys["api_token"] == nil || keys["webhook_key"] == nil {
		t.Fatal("creation keys")
	}
	epoch := func(s *storage.Store) string {
		var e string
		if err := s.Read(ctx, func(tx *sql.Tx) error { var err error; e, err = changes.Epoch(ctx, tx); return err }); err != nil {
			t.Fatal(err)
		}
		return e
	}
	before := epoch(s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	r = New(reopened)
	if epoch(reopened) != before {
		t.Fatal("epoch changed")
	}
	b, err := r.Get(ctx, v.Bot.ID)
	if err != nil || b != v.Bot {
		t.Fatalf("reopened: %v", err)
	}
	public, _ := json.Marshal(b)
	if strings.Contains(string(public), v.APIToken.Token) || strings.Contains(string(public), "digest") || strings.Contains(string(public), v.WebhookKey.Key) {
		t.Fatal("public secret leak")
	}
	if b.Connection.State != "unknown" || b.Connection.LastPollAt != nil {
		t.Fatal("invented poll")
	}
	if err := reopened.Read(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT digest FROM bot_credentials")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d []byte
			if err := rows.Scan(&d); err != nil {
				return err
			}
			if len(d) != 32 || string(d) == v.APIToken.Token || string(d) == v.WebhookKey.Key {
				t.Fatal("not digest")
			}
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	for kind, secret := range map[Kind]string{APIToken: v.APIToken.Token, WebhookKey: v.WebhookKey.Key} {
		if _, err := r.Authenticate(ctx, b.ID, kind, secret); err != nil {
			t.Fatal(err)
		}
	}
	list, err := r.List(ctx)
	if err != nil || len(list) != 1 || list[0] != b {
		t.Fatal("list", err)
	}
}
func TestAuthorizationMatrix(t *testing.T) {
	s, r, _ := fixture(t)
	ops := []Operation{Send, Edit, Upload, Claim, ACK, Renew, Download, Manage, Operation("read"), Operation("create")}
	for _, caps := range []Capabilities{{true, false}, {false, true}, {true, true}, {false, false}} {
		v := create(t, r, caps)
		for _, kind := range []Kind{APIToken, WebhookKey} {
			secret := v.APIToken.Token
			if kind == WebhookKey {
				secret = v.WebhookKey.Key
			}
			i, err := r.Authenticate(ctx, v.Bot.ID, kind, secret)
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range ops {
				allowed := kind == WebhookKey && op == Send && caps.Send || kind == APIToken && ((caps.Send && (op == Send || op == Edit || op == Upload)) || (caps.Receive && (op == Claim || op == ACK || op == Renew || op == Download)))
				err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, v.Bot.ID, op) })
				if allowed && err != nil || !allowed && !errors.Is(err, ErrForbidden) {
					t.Fatalf("kind=%s caps=%v op=%s err=%v", kind, caps, op, err)
				}
			}
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, "other", Send) }); !errors.Is(err, ErrForbidden) {
				t.Fatal("cross bot", err)
			}
			stale := i
			stale.generation++
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, stale, v.Bot.ID, Send) }); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("generation", err)
			}
			stale = i
			if kind == APIToken {
				stale.kind = WebhookKey
			} else {
				stale.kind = APIToken
			}
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, stale, v.Bot.ID, Send) }); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("kind", err)
			}
		}
		for _, pair := range []struct {
			k Kind
			s string
		}{{APIToken, v.WebhookKey.Key}, {WebhookKey, v.APIToken.Token}, {APIToken, v.APIToken.Token + "x"}, {APIToken, ""}} {
			if _, err := r.Authenticate(ctx, v.Bot.ID, pair.k, pair.s); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("wrong credential", err)
			}
		}
		old, err := r.Authenticate(ctx, v.Bot.ID, APIToken, v.APIToken.Token)
		if err != nil {
			t.Fatal(err)
		}
		off := false
		if _, _, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1, Enabled: &off}); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Authenticate(ctx, v.Bot.ID, APIToken, v.APIToken.Token); !errors.Is(err, ErrForbidden) {
			t.Fatal("disabled authentication", err)
		}
		if _, err := r.Authenticate(ctx, v.Bot.ID, WebhookKey, v.WebhookKey.Key); !errors.Is(err, ErrForbidden) {
			t.Fatal("disabled webhook", err)
		}
		for _, op := range ops {
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, old, v.Bot.ID, op) }); !errors.Is(err, ErrForbidden) {
				t.Fatal("stale disabled", err)
			}
		}
	}
	if err := AuthorizeTx(ctx, nil, Identity{}, "", Send); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, Identity{}, "", Send) }); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}

func TestCASRollbackAndErrors(t *testing.T) {
	s, r, _ := fixture(t)
	v := create(t, r, Capabilities{true, true})
	name := "changed"
	if _, _, err := r.Create(ctx, CreateRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := r.Patch(ctx, "missing", PatchRequest{ExpectedVersion: 1}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := r.Patch(ctx, v.Bot.ID, PatchRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	b, seq, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1, Name: &name})
	if err != nil || b.Version != 2 || seq <= 1 {
		t.Fatal("patch", err)
	}
	if _, _, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for _, target := range []string{"bots", "bot_credentials", "changes"} {
		t.Run("create failure "+target, func(t *testing.T) {
			before := scalar(t, s, "SELECT count(*) FROM changes")
			sqlWrite(t, s, "CREATE TRIGGER failure BEFORE INSERT ON "+target+" BEGIN SELECT RAISE(ABORT,'injected'); END")
			out, seq, err := r.Create(ctx, CreateRequest{"new", true, Capabilities{true, true}})
			if err == nil || seq != 0 || out.APIToken.Token != "" || out.Bot.ID != "" {
				t.Fatal("failed create released result")
			}
			sqlWrite(t, s, "DROP TRIGGER failure")
			if scalar(t, s, "SELECT count(*) FROM bots") != 1 || scalar(t, s, "SELECT count(*) FROM bot_credentials") != 2 || scalar(t, s, "SELECT count(*) FROM changes") != before {
				t.Fatal("create partial commit")
			}
		})
	}
	for _, target := range []string{"bots", "changes"} {
		t.Run("patch failure "+target, func(t *testing.T) {
			event := "INSERT"
			if target == "bots" {
				event = "UPDATE"
			}
			sqlWrite(t, s, "CREATE TRIGGER failure BEFORE "+event+" ON "+target+" BEGIN SELECT RAISE(ABORT,'injected'); END")
			out, seq, err := r.Patch(ctx, b.ID, PatchRequest{ExpectedVersion: 2, Name: &name})
			if err == nil || seq != 0 || out.ID != "" {
				t.Fatal("failed patch released result")
			}
			sqlWrite(t, s, "DROP TRIGGER failure")
			got, err := r.Get(ctx, b.ID)
			if err != nil || got != b || scalar(t, s, "SELECT count(*) FROM changes") != 2 {
				t.Fatal("patch partial commit", err)
			}
		})
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, v.Bot.ID); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatal("storage failure not propagated")
	}
	if out, seq, err := r.Create(ctx, CreateRequest{"x", true, Capabilities{}}); err == nil || out.Bot.ID != "" || seq != 0 {
		t.Fatal("closed store create")
	}
}
func TestConcurrentCASAndSequences(t *testing.T) {
	s, r, _ := fixture(t)
	v := create(t, r, Capabilities{true, true})
	const workers = 12
	var wg sync.WaitGroup
	results := make(chan error, workers)
	seqs := make(chan changes.Sequence, workers+1)
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, seq, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1})
			results <- err
			if err == nil {
				seqs <- seq
			}
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal("CAS success count", successes)
	}
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, seq, err := r.Create(ctx, CreateRequest{"concurrent", true, Capabilities{}})
			if err != nil {
				t.Error(err)
				return
			}
			seqs <- seq
		}()
	}
	wg.Wait()
	close(seqs)
	seen := map[changes.Sequence]bool{}
	for seq := range seqs {
		if seq <= 1 || seen[seq] {
			t.Fatal("nonunique sequence", seq)
		}
		seen[seq] = true
	}
	if len(seen) != workers+1 {
		t.Fatal("missing sequences")
	}
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT change_seq,type,entity_id,revision,bot_id FROM changes ORDER BY change_seq")
		if err != nil {
			return err
		}
		defer rows.Close()
		var prev int64
		for rows.Next() {
			var seq, rev int64
			var kind, id, botID string
			if err := rows.Scan(&seq, &kind, &id, &rev, &botID); err != nil {
				return err
			}
			if seq <= prev || kind != "bot.upsert" || id != botID || rev < 1 {
				t.Fatal("event metadata")
			}
			prev = seq
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
}

// 并发更新先取得写锁，旧身份必须在等待后读到提交后的权限，拒绝后不得写入。
func TestCommitTimeReauthorization(t *testing.T) {
	for _, mode := range []string{"disable", "capability", "generation"} {
		t.Run(mode, func(t *testing.T) {
			s, r, cfg := fixture(t)
			other, err := storage.Open(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			v := create(t, r, Capabilities{true, true})
			identity, err := r.Authenticate(ctx, v.Bot.ID, APIToken, v.APIToken.Token)
			if err != nil {
				t.Fatal(err)
			}
			locked := make(chan struct{})
			release := make(chan struct{})
			updated := make(chan error, 1)
			go func() {
				updated <- s.Write(ctx, func(tx *sql.Tx) error {
					var err error
					switch mode {
					case "disable":
						_, err = tx.ExecContext(ctx, "UPDATE bots SET enabled=0 WHERE id=?", v.Bot.ID)
					case "capability":
						_, err = tx.ExecContext(ctx, "UPDATE bots SET can_send=0 WHERE id=?", v.Bot.ID)
					case "generation":
						_, err = tx.ExecContext(ctx, "UPDATE bot_credentials SET generation=generation+1 WHERE bot_id=?", v.Bot.ID)
					}
					close(locked)
					<-release
					return err
				})
			}()
			<-locked
			started := make(chan struct{})
			result := make(chan error, 1)
			go func() {
				close(started)
				result <- other.Write(ctx, func(tx *sql.Tx) error {
					if err := AuthorizeTx(ctx, tx, identity, v.Bot.ID, Send); err != nil {
						return err
					}
					_, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.BotUpsert, EntityID: v.Bot.ID, Revision: 1, BotID: &v.Bot.ID, CreatedAt: time.Now()})
					return err
				})
			}()
			<-started
			close(release)
			if err := <-updated; err != nil {
				t.Fatal(err)
			}
			err = <-result
			if !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrUnauthorized) {
				t.Fatal("stale identity committed", err)
			}
			if scalar(t, s, "SELECT count(*) FROM changes") != 1 {
				t.Fatal("unauthorized commit")
			}
		})
	}
}

func TestCommitFailureDoesNotReleaseResult(t *testing.T) {
	s, r, _ := fixture(t)
	sqlWrite(t, s, "CREATE TABLE commit_failure(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)")
	sqlWrite(t, s, `CREATE TRIGGER failure AFTER INSERT ON changes BEGIN INSERT INTO commit_failure VALUES('missing'); END`)
	out, seq, err := r.Create(ctx, CreateRequest{"commit failure", true, Capabilities{true, true}})
	if err == nil || seq != 0 || out.Bot.ID != "" || out.APIToken.Token != "" || out.WebhookKey.Key != "" {
		t.Fatal("failed commit released result")
	}
	if scalar(t, s, "SELECT count(*) FROM bots") != 0 || scalar(t, s, "SELECT count(*) FROM bot_credentials") != 0 || scalar(t, s, "SELECT count(*) FROM changes") != 0 {
		t.Fatal("failed commit retained writes")
	}
}

func TestIdentityCapabilitiesAreBound(t *testing.T) {
	s, r, _ := fixture(t)
	v := create(t, r, Capabilities{})
	i, err := r.Authenticate(ctx, v.Bot.ID, APIToken, v.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	caps := Capabilities{true, true}
	if _, _, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1, Capabilities: &caps}); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, v.Bot.ID, Send) }); !errors.Is(err, ErrForbidden) {
		t.Fatal("snapshot escalated", err)
	}
	i, err = r.Authenticate(ctx, v.Bot.ID, APIToken, v.APIToken.Token)
	if err != nil || i.Capabilities() != caps {
		t.Fatal("new identity", err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, v.Bot.ID, Send) }); err != nil {
		t.Fatal(err)
	}
}

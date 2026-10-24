package bots

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
	"strings"
	"testing"
	"time"
)

// 快照包含所有 Bot 的凭据摘要及计数，避免只检查公开投影而遗漏部分提交。
type lifecycleSnapshot struct {
	Bot             Bot
	Credentials     string
	Count, Sequence int64
}

func snapshot(t *testing.T, s *storage.Store, r *Repository, id string) lifecycleSnapshot {
	t.Helper()
	b, err := r.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var credentials string
	err = s.Read(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT group_concat(kind||':'||credential_id||':'||generation||':'||hex(digest),'|') FROM (SELECT * FROM bot_credentials ORDER BY bot_id,kind)`).Scan(&credentials)
	})
	if err != nil {
		t.Fatal(err)
	}
	return lifecycleSnapshot{b, credentials, scalar(t, s, "SELECT count(*) FROM changes"), scalar(t, s, "SELECT seq FROM sqlite_sequence WHERE name='changes'")}
}
func rotateForTest(r *Repository, id string, kind Kind, write WriteFunc, hook LifecycleFunc) (CredentialMetadata, string, changes.Sequence, error) {
	if kind == APIToken {
		v, seq, err := r.RotateAPITokenWithWrite(ctx, id, write, hook)
		return v.Credential, v.Token, seq, err
	}
	v, seq, err := r.RotateWebhookKeyWithWrite(ctx, id, write)
	return v.Credential, v.Key, seq, err
}
func identityForTest(t *testing.T, r *Repository, v Created, k Kind) Identity {
	t.Helper()
	secret := v.APIToken.Token
	if k == WebhookKey {
		secret = v.WebhookKey.Key
	}
	i, err := r.Authenticate(ctx, v.Bot.ID, k, secret)
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func TestRotationIsolationAndDisclosure(t *testing.T) {
	for _, kind := range []Kind{APIToken, WebhookKey} {
		t.Run(string(kind), func(t *testing.T) {
			s, r, _ := fixture(t)
			a := create(t, r, Capabilities{true, true})
			b := create(t, r, Capabilities{true, true})
			oldAPI := identityForTest(t, r, a, APIToken)
			oldWH := identityForTest(t, r, a, WebhookKey)
			before := snapshot(t, s, r, b.Bot.ID)
			called := 0
			hook := func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
				called++
				if e.Action != LifecycleRotateAPIToken || e.Before != a.Bot || e.After.Credentials.APIToken.Generation != 2 {
					t.Fatal("hook metadata")
				}
				current, err := get(c, tx, a.Bot.ID)
				if err != nil || current != e.After {
					t.Fatal("hook transaction", err)
				}
				return nil
			}
			meta, secret, seq, err := rotateForTest(r, a.Bot.ID, kind, s.Write, hook)
			if err != nil || seq != 3 || meta.Generation != 2 || secret == "" {
				t.Fatal("rotate", err)
			}
			if (kind == APIToken && called != 1) || (kind == WebhookKey && called != 0) {
				t.Fatal("API hook isolation")
			}
			current, err := r.Get(ctx, a.Bot.ID)
			if err != nil || current.Version != 2 || current.Capabilities != a.Bot.Capabilities {
				t.Fatal("bot metadata", err)
			}
			other, selected, original := oldWH, oldAPI, a.APIToken.Credential
			if kind == WebhookKey {
				other, selected, original = oldAPI, oldWH, a.WebhookKey.Credential
			}
			if meta.CredentialID == original.CredentialID {
				t.Fatal("credential ID reused")
			}
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, selected, a.Bot.ID, Send) }); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("old identity", err)
			}
			oldSecret := a.APIToken.Token
			if kind == WebhookKey {
				oldSecret = a.WebhookKey.Key
			}
			if _, err := r.Authenticate(ctx, a.Bot.ID, kind, oldSecret); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("old secret", err)
			}
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, other, a.Bot.ID, Send) }); err != nil {
				t.Fatal("other credential", err)
			}
			fresh, err := r.Authenticate(ctx, a.Bot.ID, kind, secret)
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range []Operation{Send, Claim} {
				err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, fresh, a.Bot.ID, op) })
				if kind == WebhookKey && op == Claim {
					if !errors.Is(err, ErrForbidden) {
						t.Fatal("webhook receive", err)
					}
				} else if err != nil {
					t.Fatal("new credential authorization", err)
				}
			}
			if err = s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, fresh, b.Bot.ID, Send) }); !errors.Is(err, ErrForbidden) {
				t.Fatal("cross Bot", err)
			}
			for _, k := range []Kind{APIToken, WebhookKey} {
				identityForTest(t, r, b, k)
			}
			after := snapshot(t, s, r, b.Bot.ID)
			if before.Bot != after.Bot {
				t.Fatal("other Bot changed")
			}
			oldRows, newRows := strings.Split(before.Credentials, "|"), strings.Split(after.Credentials, "|")
			changed := 0
			for n, row := range oldRows {
				if row != newRows[n] {
					changed++
					if !strings.Contains(row, original.CredentialID) {
						t.Fatal("other digest changed")
					}
				}
			}
			if changed != 1 {
				t.Fatal("changed credentials", changed)
			}
			data, _ := json.Marshal(current)
			if strings.Contains(string(data), secret) || strings.Contains(string(data), "digest") {
				t.Fatal("secret disclosure")
			}
			if scalar(t, s, `SELECT count(*) FROM changes WHERE type='bot.upsert' AND entity_id=? AND revision=2 AND bot_id=?`, a.Bot.ID, a.Bot.ID) != 1 {
				t.Fatal("typed change")
			}
			_, next, _, err := rotateForTest(r, a.Bot.ID, kind, s.Write, nil)
			if err != nil || next == secret {
				t.Fatal("secret reused", err)
			}
			if _, err := r.Authenticate(ctx, a.Bot.ID, kind, secret); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("previous secret survived", err)
			}
		})
	}
}
func TestLifecycleRollbackMatrix(t *testing.T) {
	injected := errors.New("injected")
	for _, kind := range []Kind{APIToken, WebhookKey} {
		for _, mode := range []string{"missing", "version", "generation", "changes-exhausted", "random", "credential-write", "bot-write", "credential-cas", "bot-cas", "changes", "hook", "renew", "commit"} {
			if kind == WebhookKey && mode == "hook" {
				continue
			}
			t.Run(string(kind)+"/"+mode, func(t *testing.T) {
				s, r, _ := fixture(t)
				a := create(t, r, Capabilities{true, true})
				create(t, r, Capabilities{true, true})
				id := a.Bot.ID
				switch mode {
				case "missing":
					id = "missing"
				case "version":
					sqlWrite(t, s, "UPDATE bots SET version=? WHERE id=?", int64(math.MaxInt64), id)
				case "generation":
					sqlWrite(t, s, "UPDATE bot_credentials SET generation=? WHERE bot_id=? AND kind=?", int64(math.MaxInt64), id, kind)
				case "changes-exhausted":
					sqlWrite(t, s, "UPDATE sqlite_sequence SET seq=? WHERE name='changes'", int64(math.MaxInt64))
				case "random":
					r.issueCredential = func() (CredentialMetadata, string, []byte, error) { return CredentialMetadata{}, "", nil, injected }
				case "credential-write":
					sqlWrite(t, s, `CREATE TRIGGER failure BEFORE UPDATE ON bot_credentials BEGIN SELECT RAISE(ABORT,'injected'); END`)
				case "bot-write":
					sqlWrite(t, s, `CREATE TRIGGER failure BEFORE UPDATE ON bots BEGIN SELECT RAISE(ABORT,'injected'); END`)
				case "credential-cas":
					sqlWrite(t, s, `CREATE TRIGGER failure BEFORE UPDATE ON bot_credentials BEGIN SELECT RAISE(IGNORE); END`)
				case "bot-cas":
					sqlWrite(t, s, `CREATE TRIGGER failure BEFORE UPDATE ON bots BEGIN SELECT RAISE(IGNORE); END`)
				case "changes":
					sqlWrite(t, s, `CREATE TRIGGER failure BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'injected'); END`)
				case "commit":
					sqlWrite(t, s, "CREATE TABLE commit_failure(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)")
					sqlWrite(t, s, `CREATE TRIGGER failure AFTER INSERT ON changes BEGIN INSERT INTO commit_failure VALUES('missing'); END`)
				}
				before := snapshot(t, s, r, a.Bot.ID)
				hook := LifecycleFunc(nil)
				if mode == "hook" {
					hook = func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
						_, err := tx.ExecContext(c, "UPDATE bots SET name='hook' WHERE id=?", e.After.ID)
						if err != nil {
							return err
						}
						return injected
					}
				}
				write := WriteFunc(s.Write)
				if mode == "renew" {
					write = func(c context.Context, fn func(*sql.Tx) error) error {
						return s.Write(c, func(tx *sql.Tx) error {
							if err := fn(tx); err != nil {
								return err
							}
							return injected
						})
					}
				}
				meta, secret, seq, err := rotateForTest(r, id, kind, write, hook)
				if err == nil || meta != (CredentialMetadata{}) || secret != "" || seq != 0 {
					t.Fatal("failed rotation released result", err)
				}
				if snapshot(t, s, r, a.Bot.ID) != before {
					t.Fatal("partial commit", mode)
				}
			})
		}
	}
}
func TestPatchLifecycleAtomicityAndRestore(t *testing.T) {
	for _, mode := range []string{"hook", "changes", "renew", "cas", "version", "changes-exhausted", "success"} {
		t.Run(mode, func(t *testing.T) {
			s, r, _ := fixture(t)
			a := create(t, r, Capabilities{true, false})
			api := identityForTest(t, r, a, APIToken)
			wh := identityForTest(t, r, a, WebhookKey)
			if mode == "version" {
				sqlWrite(t, s, "UPDATE bots SET version=? WHERE id=?", int64(math.MaxInt64), a.Bot.ID)
			}
			if mode == "changes-exhausted" {
				sqlWrite(t, s, "UPDATE sqlite_sequence SET seq=? WHERE name='changes'", int64(math.MaxInt64))
			}
			if mode == "changes" {
				sqlWrite(t, s, `CREATE TRIGGER failure BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'injected'); END`)
			}
			sqlWrite(t, s, "CREATE TABLE queue_marker(bot_id TEXT)")
			before := snapshot(t, s, r, a.Bot.ID)
			calls := 0
			off := false
			hook := func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
				calls++
				if e.Action != LifecyclePatch || e.Before != before.Bot || e.After.Enabled {
					t.Fatal("patch hook metadata")
				}
				if _, err := tx.ExecContext(c, "INSERT INTO queue_marker VALUES(?)", e.After.ID); err != nil {
					return err
				}
				if mode == "hook" {
					return errors.New("hook")
				}
				return nil
			}
			write := WriteFunc(s.Write)
			if mode == "renew" {
				write = func(c context.Context, fn func(*sql.Tx) error) error {
					return s.Write(c, func(tx *sql.Tx) error {
						if err := fn(tx); err != nil {
							return err
						}
						return errors.New("renew")
					})
				}
			}
			version := before.Bot.Version
			if mode == "cas" {
				version++
			}
			out, seq, err := r.PatchWithLifecycle(ctx, a.Bot.ID, PatchRequest{ExpectedVersion: version, Enabled: &off}, write, hook)
			if mode != "success" {
				if err == nil || out != (Bot{}) || seq != 0 || snapshot(t, s, r, a.Bot.ID) != before || scalar(t, s, "SELECT count(*) FROM queue_marker") != 0 {
					t.Fatal("patch partial commit", err)
				}
				return
			}
			if err != nil || seq != 2 || calls != 1 || scalar(t, s, "SELECT count(*) FROM queue_marker") != 1 {
				t.Fatal("patch hook", err)
			}
			for _, i := range []Identity{api, wh} {
				for _, op := range []Operation{Send, Claim, ACK, Renew, Download} {
					if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, a.Bot.ID, op) }); !errors.Is(err, ErrForbidden) {
						t.Fatal("disabled", err)
					}
				}
			}
			on := true
			restored, _, err := r.PatchWithLifecycle(ctx, a.Bot.ID, PatchRequest{ExpectedVersion: 2, Enabled: &on}, s.Write, func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
				if e.Before.Enabled || !e.After.Enabled {
					return errors.New("restore metadata")
				}
				return nil
			})
			if err != nil || restored.Capabilities != a.Bot.Capabilities || restored.Credentials != a.Bot.Credentials {
				t.Fatal("restore", err)
			}
			for _, i := range []Identity{api, wh} {
				if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, a.Bot.ID, Send) }); err != nil {
					t.Fatal("restored send", err)
				}
			}
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, api, a.Bot.ID, Claim) }); !errors.Is(err, ErrForbidden) {
				t.Fatal("restore escalated", err)
			}
		})
	}
}

// 显式事务屏障固定提交先后，不以 race 检测器或偶然竞争作为并发证据。
func TestLifecycleCommitOrdering(t *testing.T) {
	for _, mode := range []string{"api", "webhook", "disable", "business-first"} {
		t.Run(mode, func(t *testing.T) {
			s, r, cfg := fixture(t)
			other, err := storage.Open(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			a := create(t, r, Capabilities{true, true})
			kind := APIToken
			if mode == "webhook" {
				kind = WebhookKey
			}
			identity := identityForTest(t, r, a, kind)
			sqlWrite(t, s, "CREATE TABLE business_marker(bot_id TEXT)")
			locked, release := make(chan struct{}), make(chan struct{})
			first := make(chan error, 1)
			business := func(tx *sql.Tx) error {
				if err := AuthorizeTx(ctx, tx, identity, a.Bot.ID, Send); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO business_marker VALUES(?)", a.Bot.ID); err != nil {
					return err
				}
				_, err := changes.Append(ctx, tx, changes.Metadata{Type: changes.BotUpsert, EntityID: a.Bot.ID, Revision: 1, BotID: &a.Bot.ID, CreatedAt: time.Now()})
				return err
			}
			mutate := func(write WriteFunc) error {
				if mode == "disable" {
					off := false
					_, _, err := r.PatchWithLifecycle(ctx, a.Bot.ID, PatchRequest{ExpectedVersion: 1, Enabled: &off}, write, nil)
					return err
				}
				_, _, _, err := rotateForTest(r, a.Bot.ID, kind, write, nil)
				return err
			}
			gated := func(c context.Context, fn func(*sql.Tx) error) error {
				return s.Write(c, func(tx *sql.Tx) error {
					if err := fn(tx); err != nil {
						return err
					}
					close(locked)
					<-release
					return nil
				})
			}
			go func() {
				if mode == "business-first" {
					first <- gated(ctx, business)
				} else {
					first <- mutate(gated)
				}
			}()
			<-locked
			started := make(chan struct{})
			second := make(chan error, 1)
			go func() {
				close(started)
				if mode == "business-first" {
					second <- mutate(other.Write)
				} else {
					second <- other.Write(ctx, business)
				}
			}()
			<-started
			close(release)
			if err := <-first; err != nil {
				t.Fatal(err)
			}
			err = <-second
			if mode == "business-first" {
				if err != nil || scalar(t, s, "SELECT count(*) FROM business_marker") != 1 || scalar(t, s, "SELECT count(*) FROM changes") != 3 {
					t.Fatal("prior commit", err)
				}
			} else {
				expected := ErrUnauthorized
				if mode == "disable" {
					expected = ErrForbidden
				}
				if !errors.Is(err, expected) || scalar(t, s, "SELECT count(*) FROM business_marker") != 0 || scalar(t, s, "SELECT count(*) FROM changes") != 2 {
					t.Fatal("stale committed", err)
				}
			}
		})
	}
}

func TestLifecycleCounterBoundaryAndInputs(t *testing.T) {
	for _, kind := range []Kind{APIToken, WebhookKey} {
		t.Run(string(kind), func(t *testing.T) {
			s, r, _ := fixture(t)
			a := create(t, r, Capabilities{true, true})
			sqlWrite(t, s, "UPDATE bots SET version=? WHERE id=?", int64(math.MaxInt64-1), a.Bot.ID)
			sqlWrite(t, s, "UPDATE bot_credentials SET generation=? WHERE bot_id=? AND kind=?", int64(math.MaxInt64-1), a.Bot.ID, kind)
			sqlWrite(t, s, "UPDATE sqlite_sequence SET seq=? WHERE name='changes'", int64(math.MaxInt64-1))
			meta, secret, seq, err := rotateForTest(r, a.Bot.ID, kind, s.Write, nil)
			if err != nil || meta.Generation != math.MaxInt64 || seq != changes.Sequence(math.MaxInt64) || secret == "" {
				t.Fatal("last valid increment", err)
			}
			before := snapshot(t, s, r, a.Bot.ID)
			if before.Bot.Version != math.MaxInt64 {
				t.Fatal("version boundary")
			}
			if _, secret, seq, err = rotateForTest(r, a.Bot.ID, kind, s.Write, nil); !errors.Is(err, ErrConflict) || secret != "" || seq != 0 || snapshot(t, s, r, a.Bot.ID) != before {
				t.Fatal("boundary wrapped", err)
			}
			for _, input := range []struct {
				id    string
				write WriteFunc
			}{{"", s.Write}, {a.Bot.ID, nil}} {
				if _, secret, seq, err = rotateForTest(r, input.id, kind, input.write, nil); !errors.Is(err, ErrInvalid) || secret != "" || seq != 0 {
					t.Fatal("invalid input", err)
				}
			}
		})
	}
}

func TestEnableHookFailureRollsBack(t *testing.T) {
	s, r, _ := fixture(t)
	a := create(t, r, Capabilities{true, true})
	off := false
	if _, _, err := r.Patch(ctx, a.Bot.ID, PatchRequest{ExpectedVersion: 1, Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, s, r, a.Bot.ID)
	on := true
	called := false
	out, seq, err := r.PatchWithLifecycle(ctx, a.Bot.ID, PatchRequest{ExpectedVersion: 2, Enabled: &on}, s.Write, func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
		called = true
		if e.Before.Enabled || !e.After.Enabled {
			t.Fatal("enable event")
		}
		return errors.New("queue restore failed")
	})
	if !called || err == nil || out != (Bot{}) || seq != 0 || snapshot(t, s, r, a.Bot.ID) != before {
		t.Fatal("enable failure retained writes", err)
	}
}

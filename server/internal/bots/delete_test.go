package bots

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/changes"
	"relaynest/server/internal/storage"
)

func TestDeleteIsolationAndMarker(t *testing.T) {
	s, r, cfg := fixture(t)
	a := create(t, r, Capabilities{true, true})
	b := create(t, r, Capabilities{true, true})
	identities := []Identity{identityForTest(t, r, a, APIToken), identityForTest(t, r, a, WebhookKey)}
	beforeB := snapshot(t, s, r, b.Bot.ID)
	credentialB := func() string {
		t.Helper()
		var out string
		if err := s.Read(ctx, func(tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, `SELECT group_concat(kind||':'||credential_id||':'||generation||':'||hex(digest),'|') FROM (SELECT * FROM bot_credentials WHERE bot_id=? ORDER BY kind)`, b.Bot.ID).Scan(&out)
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	beforeCredentialsB := credentialB()
	// 删除不需要随机数；故障随机源不得影响其行为。
	r.issueCredential = func() (CredentialMetadata, string, []byte, error) {
		t.Fatal("delete requested randomness")
		return CredentialMetadata{}, "", nil, errors.New("random")
	}
	var stages []LifecycleAction
	hook := func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
		stages = append(stages, e.Action)
		if e.Before != a.Bot || e.After != (Bot{}) {
			t.Fatal("delete metadata")
		}
		data, err := json.Marshal(e)
		if err != nil || strings.Contains(string(data), a.APIToken.Token) || strings.Contains(string(data), a.WebhookKey.Key) || strings.Contains(string(data), "digest") {
			t.Fatal("delete hook leaked secrets")
		}
		current, err := get(c, tx, a.Bot.ID)
		if e.Action == LifecycleDeleteBefore && (err != nil || current != a.Bot) {
			t.Fatal("before state", err)
		}
		if e.Action == LifecycleDeleteAfter && !errors.Is(err, ErrNotFound) {
			t.Fatal("after state", err)
		}
		return nil
	}
	seq, err := r.DeleteWithWrite(ctx, a.Bot.ID, DeleteRequest{a.Bot.Name, 1}, s.Write, hook)
	if err != nil || seq != 3 || !reflect.DeepEqual(stages, []LifecycleAction{LifecycleDeleteBefore, LifecycleDeleteAfter}) {
		t.Fatal("delete", err)
	}
	if _, err := r.Get(ctx, a.Bot.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("bot survived", err)
	}
	if scalar(t, s, "SELECT count(*) FROM bot_credentials WHERE bot_id=?", a.Bot.ID) != 0 {
		t.Fatal("credentials survived")
	}
	for n, kind := range []Kind{APIToken, WebhookKey} {
		secret := a.APIToken.Token
		if kind == WebhookKey {
			secret = a.WebhookKey.Key
		}
		if _, err := r.Authenticate(ctx, a.Bot.ID, kind, secret); !errors.Is(err, ErrUnauthorized) {
			t.Fatal("secret survived", err)
		}
		for _, op := range []Operation{Send, Claim, ACK, Renew, Download} {
			if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, identities[n], a.Bot.ID, op) }); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("identity survived", err)
			}
		}
		identityForTest(t, r, b, kind)
	}
	afterB := snapshot(t, s, r, b.Bot.ID)
	if afterB.Bot != beforeB.Bot || credentialB() != beforeCredentialsB || scalar(t, s, "SELECT count(*) FROM changes WHERE entity_id=? AND type='bot.upsert' AND revision=1", b.Bot.ID) != 1 {
		t.Fatal("other bot changed")
	}
	// 再次打开真实数据库，读取公共 changes 元数据而非依赖已删除 Bot 投影。
	reopened, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Read(ctx, func(tx *sql.Tx) error {
		var typ changes.Type
		var id, botID, deletedAt string
		var revision int64
		if err := tx.QueryRowContext(ctx, "SELECT type,entity_id,revision,bot_id,created_at FROM changes WHERE change_seq=?", seq).Scan(&typ, &id, &revision, &botID, &deletedAt); err != nil {
			return err
		}
		if typ != changes.BotDeleted || id != a.Bot.ID || botID != a.Bot.ID || revision != 2 {
			t.Fatal("marker metadata")
		}
		if _, err := time.Parse(time.RFC3339Nano, deletedAt); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if seq, err := r.Delete(ctx, a.Bot.ID, DeleteRequest{a.Bot.Name, 1}); !errors.Is(err, ErrNotFound) || seq != 0 {
		t.Fatal("repeated delete", err)
	}
	if scalar(t, s, "SELECT count(*) FROM changes") != 3 {
		t.Fatal("repeated delete changed log")
	}
}

func TestDeleteRollbackMatrix(t *testing.T) {
	for _, mode := range []string{"name", "case", "space", "stale", "missing", "version", "changes-exhausted", "credential", "bot", "cas", "before", "after", "hook-cas", "changes", "renew", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, r, _ := fixture(t)
			a := create(t, r, Capabilities{true, true})
			b := create(t, r, Capabilities{true, true})
			sqlWrite(t, s, "CREATE TABLE delete_marker(stage TEXT)")
			req := DeleteRequest{a.Bot.Name, 1}
			id := a.Bot.ID
			switch mode {
			case "name":
				req.ConfirmName = "other"
			case "case":
				sqlWrite(t, s, "UPDATE bots SET name='ABC' WHERE id=?", id)
				req.ConfirmName = "abc"
			case "space":
				req.ConfirmName += " "
			case "stale":
				req.ExpectedVersion = 2
			case "missing":
				id = "missing"
			case "version":
				sqlWrite(t, s, "UPDATE bots SET version=? WHERE id=?", int64(math.MaxInt64), id)
				req.ExpectedVersion = math.MaxInt64
			case "changes-exhausted":
				sqlWrite(t, s, "UPDATE sqlite_sequence SET seq=? WHERE name='changes'", int64(math.MaxInt64))
			case "credential":
				sqlWrite(t, s, `CREATE TRIGGER failure BEFORE DELETE ON bot_credentials BEGIN SELECT RAISE(ABORT,'injected'); END`)
			case "bot":
				sqlWrite(t, s, `CREATE TRIGGER failure BEFORE DELETE ON bots BEGIN SELECT RAISE(ABORT,'injected'); END`)
			case "cas":
				sqlWrite(t, s, `CREATE TRIGGER failure BEFORE DELETE ON bots BEGIN SELECT RAISE(IGNORE); END`)
			case "changes":
				sqlWrite(t, s, `CREATE TRIGGER failure BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'injected'); END`)
			case "commit":
				sqlWrite(t, s, "CREATE TABLE commit_failure(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)")
				sqlWrite(t, s, `CREATE TRIGGER failure AFTER INSERT ON changes BEGIN INSERT INTO commit_failure VALUES('missing'); END`)
			}
			before := snapshot(t, s, r, a.Bot.ID)
			beforeB := snapshot(t, s, r, b.Bot.ID)
			hook := func(c context.Context, tx *sql.Tx, e LifecycleChange) error {
				if _, err := tx.ExecContext(c, "INSERT INTO delete_marker VALUES(?)", string(e.Action)); err != nil {
					return err
				}
				if (mode == "before" && e.Action == LifecycleDeleteBefore) || (mode == "after" && e.Action == LifecycleDeleteAfter) {
					return errors.New("injected hook")
				}
				if mode == "hook-cas" && e.Action == LifecycleDeleteBefore {
					_, err := tx.ExecContext(c, "UPDATE bots SET version=version+1 WHERE id=?", id)
					return err
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
			seq, err := r.DeleteWithWrite(ctx, id, req, write, hook)
			if err == nil || seq != 0 || snapshot(t, s, r, a.Bot.ID) != before || snapshot(t, s, r, b.Bot.ID) != beforeB || scalar(t, s, "SELECT count(*) FROM delete_marker") != 0 {
				t.Fatal("partial delete", err)
			}
			if mode == "stale" || mode == "version" || mode == "cas" || mode == "hook-cas" {
				if !errors.Is(err, ErrConflict) {
					t.Fatal("conflict", err)
				}
			}
			if mode == "name" || mode == "case" || mode == "space" {
				if !errors.Is(err, ErrInvalid) {
					t.Fatal("name error", err)
				}
			}
			if mode == "missing" && !errors.Is(err, ErrNotFound) {
				t.Fatal("missing error", err)
			}
		})
	}
}

func TestDeleteInputAndBoundary(t *testing.T) {
	s, r, _ := fixture(t)
	a := create(t, r, Capabilities{true, true})
	before := snapshot(t, s, r, a.Bot.ID)
	calls := 0
	write := func(context.Context, func(*sql.Tx) error) error { calls++; return errors.New("unexpected write") }
	for _, req := range []DeleteRequest{{}, {ConfirmName: a.Bot.Name}, {ExpectedVersion: 1}, {a.Bot.Name, -1}} {
		if seq, err := r.DeleteWithWrite(ctx, a.Bot.ID, req, write, nil); !errors.Is(err, ErrInvalid) || seq != 0 {
			t.Fatal("invalid", err)
		}
	}
	if _, err := r.DeleteWithWrite(ctx, "", DeleteRequest{a.Bot.Name, 1}, write, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := r.DeleteWithWrite(ctx, a.Bot.ID, DeleteRequest{a.Bot.Name, 1}, nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if calls != 0 || snapshot(t, s, r, a.Bot.ID) != before {
		t.Fatal("invalid request entered write")
	}
	// 名称原样匹配，包括既有允许的空白；最后一个有效终态版本/日志序号不回绕。
	sqlWrite(t, s, "UPDATE bots SET name=' ',version=? WHERE id=?", int64(math.MaxInt64-1), a.Bot.ID)
	sqlWrite(t, s, "UPDATE sqlite_sequence SET seq=? WHERE name='changes'", int64(math.MaxInt64-1))
	seq, err := r.Delete(ctx, a.Bot.ID, DeleteRequest{" ", math.MaxInt64 - 1})
	if err != nil || seq != changes.Sequence(math.MaxInt64) || scalar(t, s, "SELECT revision FROM changes WHERE change_seq=?", seq) != math.MaxInt64 {
		t.Fatal("boundary", err)
	}
}

// 两个真实 Store 的写屏障固定 edit/delete 提交顺序，避免依赖偶然竞争。
func TestDeleteCommitOrdering(t *testing.T) {
	for _, mode := range []string{"edit-first", "delete-first", "delete-delete"} {
		t.Run(mode, func(t *testing.T) {
			s, r, cfg := fixture(t)
			other, err := storage.Open(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			a := create(t, r, Capabilities{true, true})
			locked, release := make(chan struct{}), make(chan struct{})
			first, second := make(chan error, 1), make(chan error, 1)
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
			edit := func(w WriteFunc) error {
				name := "edited"
				_, _, err := r.PatchWithWrite(ctx, a.Bot.ID, PatchRequest{ExpectedVersion: 1, Name: &name}, w)
				return err
			}
			del := func(w WriteFunc) error {
				_, err := r.DeleteWithWrite(ctx, a.Bot.ID, DeleteRequest{a.Bot.Name, 1}, w, nil)
				return err
			}
			go func() {
				if mode == "edit-first" {
					first <- edit(gated)
				} else {
					first <- del(gated)
				}
			}()
			<-locked
			started := make(chan struct{})
			go func() {
				close(started)
				if mode == "delete-first" {
					second <- edit(other.Write)
				} else {
					second <- del(other.Write)
				}
			}()
			<-started
			close(release)
			if err := <-first; err != nil {
				t.Fatal(err)
			}
			expected := ErrNotFound
			if mode == "edit-first" {
				expected = ErrConflict
			}
			if err := <-second; !errors.Is(err, expected) {
				t.Fatal("stale operation", err)
			}
			if scalar(t, s, "SELECT count(*) FROM changes") != 2 {
				t.Fatal("stale operation wrote changes")
			}
			if mode == "edit-first" {
				b, err := r.Get(ctx, a.Bot.ID)
				if err != nil || b.Name != "edited" || b.Version != 2 {
					t.Fatal("edit lost", err)
				}
				// 新版本确认可以删除，旧版本随后仍不能复活或重复写墓碑。
				if _, err := r.Delete(ctx, b.ID, DeleteRequest{b.Name, 2}); err != nil {
					t.Fatal(err)
				}
				if seq, err := r.Delete(ctx, b.ID, DeleteRequest{a.Bot.Name, 1}); !errors.Is(err, ErrNotFound) || seq != 0 {
					t.Fatal("reverse stale", err)
				}
			}
		})
	}
}

func TestDeleteAuthorizationOrdering(t *testing.T) {
	for _, businessFirst := range []bool{false, true} {
		name := "delete-first"
		if businessFirst {
			name = "business-first"
		}
		t.Run(name, func(t *testing.T) {
			s, r, cfg := fixture(t)
			other, err := storage.Open(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			a := create(t, r, Capabilities{true, true})
			identity := identityForTest(t, r, a, APIToken)
			sqlWrite(t, s, "CREATE TABLE business_marker(bot_id TEXT)")
			locked, release := make(chan struct{}), make(chan struct{})
			first, second := make(chan error, 1), make(chan error, 1)
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
			business := func(tx *sql.Tx) error {
				if err := AuthorizeTx(ctx, tx, identity, a.Bot.ID, Send); err != nil {
					return err
				}
				_, err := tx.ExecContext(ctx, "INSERT INTO business_marker VALUES(?)", a.Bot.ID)
				return err
			}
			del := func(w WriteFunc) error {
				_, err := r.DeleteWithWrite(ctx, a.Bot.ID, DeleteRequest{a.Bot.Name, 1}, w, nil)
				return err
			}
			go func() {
				if businessFirst {
					first <- gated(ctx, business)
				} else {
					first <- del(gated)
				}
			}()
			<-locked
			started := make(chan struct{})
			go func() {
				close(started)
				if businessFirst {
					second <- del(other.Write)
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
			expectedCount := int64(0)
			if businessFirst {
				expectedCount = 1
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrUnauthorized) {
				t.Fatal("stale identity committed", err)
			}
			if scalar(t, s, "SELECT count(*) FROM business_marker") != expectedCount || scalar(t, s, "SELECT count(*) FROM changes") != 2 {
				t.Fatal("ordering effects")
			}
		})
	}
}

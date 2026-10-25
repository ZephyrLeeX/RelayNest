package bots

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/storage"
)

func discover(t *testing.T, r *Repository, secret string) Identity {
	t.Helper()
	i, match, err := r.DiscoverAPIIdentity(ctx, secret)
	if err != nil || match != APIMatched || i.BotID() == "" {
		t.Fatalf("identity discovery: match=%d err=%v", match, err)
	}
	return i
}

func TestDiscoverAPIIdentityIsolation(t *testing.T) {
	s, r, _ := fixture(t)
	first := create(t, r, Capabilities{true, false})
	second := create(t, r, Capabilities{false, true})
	for _, v := range []Created{first, second} {
		i := discover(t, r, v.APIToken.Token)
		if i.BotID() != v.Bot.ID || i.Kind() != APIToken || i.Generation() != v.APIToken.Credential.Generation || i.Capabilities() != v.Bot.Capabilities {
			t.Fatal("incorrect identity binding")
		}
		original, err := r.Authenticate(ctx, v.Bot.ID, APIToken, v.APIToken.Token)
		if err != nil || original != i {
			t.Fatal("explicit authentication changed")
		}
		other := first.Bot.ID
		if other == i.BotID() {
			other = second.Bot.ID
		}
		if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, other, Upload) }); !errors.Is(err, ErrForbidden) {
			t.Fatal("cross Bot authorization", err)
		}
	}
	id, tail, _ := strings.Cut(first.APIToken.Token, ".")
	unknownID := strings.Repeat("0", 64)
	if id == unknownID {
		unknownID = strings.Repeat("1", 64)
	}
	replacement := "0"
	if tail[:1] == replacement {
		replacement = "1"
	}
	for name, secret := range map[string]string{
		"empty": "", "admin candidate": strings.Repeat("a", 43), "missing separator": id + tail,
		"short": id + ".x", "extra separator": first.APIToken.Token + ".",
		"unknown id":   unknownID + "." + tail,
		"wrong digest": id + "." + replacement + tail[1:], "webhook": first.WebhookKey.Key,
	} {
		t.Run(name, func(t *testing.T) {
			i, match, err := r.DiscoverAPIIdentity(ctx, secret)
			if match != APINoMatch || err != nil || i != (Identity{}) {
				t.Fatalf("expected definite miss: match=%d err=%v", match, err)
			}
		})
	}
	// API 发现不能接受 Webhook，也不能改变显式 Webhook 认证政策。
	if _, err := r.Authenticate(ctx, first.Bot.ID, WebhookKey, first.WebhookKey.Key); err != nil {
		t.Fatal(err)
	}
	old := discover(t, r, first.APIToken.Token)
	rotated, _, err := r.RotateAPIToken(ctx, first.Bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if i, match, err := r.DiscoverAPIIdentity(ctx, first.APIToken.Token); match != APINoMatch || err != nil || i != (Identity{}) {
		t.Fatal("old token must be a definite miss")
	}
	current := discover(t, r, rotated.Token)
	if current.BotID() != old.BotID() || current.Generation() != old.Generation()+1 {
		t.Fatal("rotation binding")
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, old, old.BotID(), Upload) }); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old identity survived rotation", err)
	}
}

func TestDiscoverAPIIdentityMatchedRemainsTerminal(t *testing.T) {
	for _, mode := range []string{"disabled", "capability", "rotation", "revoke", "generation", "delete", "dependency", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s, r, _ := fixture(t)
			v := create(t, r, Capabilities{true, true})
			i := discover(t, r, v.APIToken.Token)
			want := ErrUnauthorized
			switch mode {
			case "disabled":
				off := false
				if _, _, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1, Enabled: &off}); err != nil {
					t.Fatal(err)
				}
				rejected, match, err := r.DiscoverAPIIdentity(ctx, v.APIToken.Token)
				if match != APIMatched || !errors.Is(err, ErrForbidden) || rejected != (Identity{}) {
					t.Fatal("disabled Bot lost matched branch")
				}
				want = ErrForbidden
			case "capability":
				caps := Capabilities{false, true}
				if _, _, err := r.Patch(ctx, v.Bot.ID, PatchRequest{ExpectedVersion: 1, Capabilities: &caps}); err != nil {
					t.Fatal(err)
				}
				current := discover(t, r, v.APIToken.Token)
				if current.Capabilities().Send {
					t.Fatal("stale discovered capabilities")
				}
				want = ErrForbidden
			case "rotation":
				if _, _, err := r.RotateAPIToken(ctx, v.Bot.ID); err != nil {
					t.Fatal(err)
				}
			case "revoke":
				sqlWrite(t, s, "DELETE FROM bot_credentials WHERE bot_id=? AND kind=?", v.Bot.ID, APIToken)
			case "generation":
				sqlWrite(t, s, "UPDATE bot_credentials SET generation=generation+1 WHERE bot_id=? AND kind=?", v.Bot.ID, APIToken)
			case "delete":
				if _, err := r.Delete(ctx, v.Bot.ID, DeleteRequest{ConfirmName: v.Bot.Name, ExpectedVersion: 1}); err != nil {
					t.Fatal(err)
				}
			case "dependency":
				sqlWrite(t, s, "DROP TABLE bot_credentials")
			}
			callCtx := ctx
			if mode == "cancel" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				callCtx = c
			}
			err := s.Write(callCtx, func(tx *sql.Tx) error {
				if err := AuthorizeTx(callCtx, tx, i, v.Bot.ID, Upload); err != nil {
					return err
				}
				_, err := tx.ExecContext(callCtx, "UPDATE bots SET name='unauthorized write' WHERE id=?", v.Bot.ID)
				return err
			})
			if mode == "dependency" || mode == "cancel" {
				if err == nil {
					t.Fatal("dependency failure authorized")
				}
			} else if !errors.Is(err, want) {
				t.Fatal("transaction recheck", err)
			}
			if mode != "delete" && scalar(t, s, "SELECT count(*) FROM bots WHERE name='unauthorized write'") != 0 {
				t.Fatal("rejected write escaped")
			}
			// 后续401/403/依赖错误不能重新解释发现结果；资源404留真实附件调用方验证。
		})
	}
}

func TestDiscoverAPIIdentityUnavailable(t *testing.T) {
	for _, mode := range []string{"closed", "query", "instance", "canceled", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			s, r, _ := fixture(t)
			v := create(t, r, Capabilities{true, true})
			callCtx := ctx
			switch mode {
			case "closed":
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			case "query":
				sqlWrite(t, s, "DROP TABLE bot_credentials")
			case "instance":
				sqlWrite(t, s, "DELETE FROM instance_identity")
			case "canceled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				callCtx = c
			case "deadline":
				c, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
				callCtx = c
			}
			i, match, err := r.DiscoverAPIIdentity(callCtx, v.APIToken.Token)
			if match != APIUnavailable || err != ErrAPIIdentityUnavailable || i != (Identity{}) {
				t.Fatalf("failure fell through: match=%d err=%v", match, err)
			}
			if strings.Contains(err.Error(), v.APIToken.Token) || strings.Contains(err.Error(), v.APIToken.Credential.CredentialID) {
				t.Fatal("error leaked credential")
			}
		})
	}
}

func TestDiscoverAPIIdentityWaitingDeadline(t *testing.T) {
	s, err := storage.Open(ctx, storage.Config{Path: filepath.Join(t.TempDir(), "waiting.sqlite"), ReadConnections: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := New(s)
	v := create(t, r, Capabilities{true, true})
	held, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() { finished <- s.Read(ctx, func(tx *sql.Tx) error { close(held); <-release; return nil }) }()
	<-held
	// 占满真实读连接池，验证依赖等待超时，不用查询桩或 sleep。
	c, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	i, match, err := r.DiscoverAPIIdentity(c, v.APIToken.Token)
	cancel()
	close(release)
	if heldErr := <-finished; heldErr != nil {
		t.Fatal(heldErr)
	}
	if match != APIUnavailable || err != ErrAPIIdentityUnavailable || i != (Identity{}) {
		t.Fatal("waiting timeout fell through")
	}
	discover(t, r, v.APIToken.Token)
}

func TestDiscoverAPIIdentityConcurrentRotation(t *testing.T) {
	s, r, _ := fixture(t)
	v := create(t, r, Capabilities{true, true})
	original := discover(t, r, v.APIToken.Token)
	start := make(chan struct{})
	failures := make(chan error, 8)
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 12; j++ {
				i, match, err := r.DiscoverAPIIdentity(ctx, v.APIToken.Token)
				if err != nil || (match != APIMatched && match != APINoMatch) {
					failures <- errors.New("unexpected concurrent discovery result")
					return
				}
				if match == APINoMatch {
					if i != (Identity{}) {
						failures <- errors.New("miss released identity")
						return
					}
					continue
				}
				if i.BotID() != v.Bot.ID || i.Generation() != original.Generation() {
					failures <- errors.New("concurrent binding changed")
					return
				}
				err = s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, i, v.Bot.ID, Upload) })
				if err != nil && !errors.Is(err, ErrUnauthorized) {
					failures <- errors.New("unexpected transaction result")
					return
				}
			}
		}()
	}
	close(start)
	rotated, _, err := r.RotateAPIToken(ctx, v.Bot.ID)
	if err != nil {
		t.Error(err)
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	if t.Failed() {
		return
	}
	if i, match, err := r.DiscoverAPIIdentity(ctx, v.APIToken.Token); match != APINoMatch || err != nil || i != (Identity{}) {
		t.Fatal("rotation did not revoke discovery")
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error { return AuthorizeTx(ctx, tx, original, v.Bot.ID, Upload) }); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("post rotation write authorized", err)
	}
	discover(t, r, rotated.Token)
}

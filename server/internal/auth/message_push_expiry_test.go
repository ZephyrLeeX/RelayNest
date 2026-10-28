package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/messages"
)

// 只通过真实会话创建和既有测试时钟产生历史期限，不手写期限或身份。
func TestMessagePushTargetRealSessionExpiry(t *testing.T) {
	for _, days := range []int{0, 31, 91} {
		for _, idempotent := range []bool{false, true} {
			name := fmt.Sprintf("age_%d_days", days)
			if idempotent {
				name += "/idempotent"
			} else {
				name += "/application"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				db := testStore(t, filepath.Join(t.TempDir(), "push-expiry.sqlite"))
				if err := InitAdministrator(ctx, db, "Regression", []byte("local-regression-only-password")); err != nil {
					t.Fatal(err)
				}
				var generation int64
				if err := db.Read(ctx, func(tx *sql.Tx) error {
					return tx.QueryRowContext(ctx, "SELECT auth_generation FROM administrator WHERE id=1").Scan(&generation)
				}); err != nil {
					t.Fatal(err)
				}
				sessions := NewSessionStore(db)
				wall := time.Now().UTC()
				createdAt := wall.Add(-time.Duration(days) * 24 * time.Hour)
				sessions.now = func() time.Time { return createdAt }
				session, secrets, err := sessions.Create(ctx, generation, SessionInput{ClientType: "android", DeviceID: "12345678-1234-4123-8123-123456789abc", DeviceName: "regression"})
				if err != nil {
					t.Fatal(err)
				}
				if !session.IdleExpiresAt.Equal(createdAt.Add(30*24*time.Hour)) || !session.AbsoluteExpiresAt.Equal(createdAt.Add(90*24*time.Hour)) {
					t.Fatal("unexpected real session TTL")
				}
				var idle, absolute int64
				if err = db.Read(ctx, func(tx *sql.Tx) error {
					return tx.QueryRowContext(ctx, "SELECT idle_expires_at,absolute_expires_at FROM sessions WHERE id=?", session.ID).Scan(&idle, &absolute)
				}); err != nil {
					t.Fatal(err)
				}
				if idle != session.IdleExpiresAt.UnixNano() || absolute != session.AbsoluteExpiresAt.UnixNano() {
					t.Fatal("persistence differs from real Create")
				}
				sessions.now = time.Now
				_, _, inspectErr := sessions.Inspect(ctx, secrets.Token, "android")
				expired := days != 0
				if expired && !errors.Is(inspectErr, ErrInvalidSession) || !expired && inspectErr != nil {
					t.Fatalf("unexpected Inspect result expired=%v err=%v", expired, inspectErr)
				}
				br := bots.New(db)
				bot, _, err := br.Create(ctx, bots.CreateRequest{Name: "regression", Enabled: true, Capabilities: bots.Capabilities{Send: true, Receive: true}})
				if err != nil {
					t.Fatal(err)
				}
				identity, err := br.Authenticate(ctx, bot.Bot.ID, bots.APIToken, bot.APIToken.Token)
				if err != nil {
					t.Fatal(err)
				}
				snapshot := func() []int64 {
					t.Helper()
					state := []int64{}
					err := db.Read(ctx, func(tx *sql.Tx) error {
						for _, q := range []string{"SELECT count(*) FROM messages", "SELECT count(*) FROM messages WHERE created_change_seq IS NULL", "SELECT count(*) FROM deliveries", "SELECT count(*) FROM push_jobs", "SELECT count(*) FROM changes", "SELECT coalesce(max(change_seq),0) FROM changes", "SELECT coalesce(sum(bot_seq),0) FROM bots", "SELECT coalesce(max(seq),0) FROM sqlite_sequence WHERE name='changes'", "SELECT count(*) FROM idempotency"} {
							var n int64
							if err := tx.QueryRowContext(ctx, q).Scan(&n); err != nil {
								return err
							}
							state = append(state, n)
						}
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					return state
				}
				before := snapshot()
				repo := messages.New(db)
				targets := []messages.PushTarget{{SessionID: session.ID, DeviceID: session.DeviceID}}
				var out messages.Created
				if idempotent {
					out, err = repo.CreateIdempotentApplication(ctx, identity, "abcdefff-1234-4567-89ab-123456789abc", messages.CreateRequest{Format: "plain", Text: "regression"}, targets)
				} else {
					out, err = repo.CreateApplication(ctx, identity, messages.TextInput{BotID: bot.Bot.ID, Format: "plain", Text: "regression", PushTargets: targets})
				}
				after := snapshot()
				t.Logf("age_days=%d inspect_err=%v create_err=%v push_count=%d before=%v after=%v", days, inspectErr, err, len(out.Pushes), before, after)
				if expired {
					if !errors.Is(err, messages.ErrPushTarget) || !reflect.DeepEqual(out, messages.Created{}) || !reflect.DeepEqual(before, after) {
						t.Errorf("expired real session accepted or business transaction changed; want ErrPushTarget, empty result and zero state delta")
					}
				} else if err != nil || len(out.Pushes) != 1 || after[0]-before[0] != 1 || after[3]-before[3] != 1 {
					t.Errorf("valid real session control failed: %v", err)
				}
			})
		}
	}
}

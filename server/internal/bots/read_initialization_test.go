package bots

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestCreateReadStateAtomicInitialization(t *testing.T) {
	for _, mode := range []string{"success", "read-row", "credential", "change", "commit", "reauth"} {
		t.Run(mode, func(t *testing.T) {
			s, r, _ := fixture(t)
			switch mode {
			case "read-row":
				sqlWrite(t, s, `CREATE TRIGGER create_read_fail BEFORE INSERT ON bot_read_states BEGIN SELECT RAISE(ABORT,'read fault'); END`)
			case "credential":
				sqlWrite(t, s, `CREATE TRIGGER create_read_fail BEFORE INSERT ON bot_credentials BEGIN SELECT RAISE(ABORT,'credential fault'); END`)
			case "change":
				sqlWrite(t, s, `CREATE TRIGGER create_read_fail BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'change fault'); END`)
			case "commit":
				sqlWrite(t, s, `CREATE TABLE create_read_commit(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)`)
			}
			callbackOK := false
			out, seq, err := r.CreateWithWrite(ctx, CreateRequest{Name: "read", Enabled: true}, func(c context.Context, fn func(*sql.Tx) error) error {
				return s.Write(c, func(tx *sql.Tx) error {
					if err := fn(tx); err != nil {
						return err
					}
					callbackOK = true
					switch mode {
					case "commit":
						_, err := tx.Exec(`INSERT INTO create_read_commit VALUES('absent')`)
						return err
					case "reauth":
						return errors.New("final authorization denied")
					}
					return nil
				})
			})
			if mode != "success" {
				if err == nil || !reflect.DeepEqual(out, Created{}) || seq != 0 {
					t.Fatal("failed create published credentials", err)
				}
				if mode == "commit" && !callbackOK {
					t.Fatal("not COMMIT fault", err)
				}
				for _, table := range []string{"bots", "bot_credentials", "bot_read_states", "changes"} {
					if scalar(t, s, "SELECT count(*) FROM "+table) != 0 {
						t.Fatal("partial create", table)
					}
				}
				return
			}
			if err != nil || seq == 0 || out.APIToken.Token == "" || out.WebhookKey.Key == "" {
				t.Fatal("create failed", err)
			}
			if scalar(t, s, `SELECT count(*) FROM bot_credentials`) != 2 || scalar(t, s, `SELECT count(*) FROM bot_read_states`) != 1 || scalar(t, s, `SELECT count(*) FROM changes WHERE type='bot.upsert'`) != 1 || scalar(t, s, `SELECT count(*) FROM changes`) != 1 {
				t.Fatal("initial entities/events")
			}
			if err := s.Read(ctx, func(tx *sql.Tx) error {
				var n, v int64
				var stamp string
				if err := tx.QueryRow(`SELECT read_seq,state_version,updated_at FROM bot_read_states WHERE bot_id=?`, out.Bot.ID).Scan(&n, &v, &stamp); err != nil {
					return err
				}
				at, err := time.Parse(time.RFC3339Nano, stamp)
				if err != nil || n != 0 || v != 1 || at.IsZero() || at.Location() != time.UTC {
					t.Fatal("bad initialization", n, v, stamp)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

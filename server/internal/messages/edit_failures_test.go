package messages

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/storage"
)

func TestEditTextButtonsAuthorization(t *testing.T) {
	modes := []string{"webhook", "android-no-bot-identity", "cross-domain", "cross-message", "administrator-message", "missing", "generation", "revoked", "disabled", "send-removed", "snapshot-no-gain"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			identity := f.identity
			switch mode {
			case "webhook":
				var err error
				identity, err = f.b.Authenticate(messageContext, f.a.Bot.ID, bots.WebhookKey, f.a.WebhookKey.Key)
				if err != nil {
					t.Fatal(err)
				}
			case "android-no-bot-identity":
				identity = bots.Identity{}
			case "cross-domain":
				in.BotID = f.bbot.Bot.ID
			case "cross-message":
				foreign := f.input()
				foreign.BotID = f.bbot.Bot.ID
				out, err := adminWrite(f, foreign)
				if err != nil {
					t.Fatal(err)
				}
				in.MessageID = out.Message.ID
			case "administrator-message":
				out, err := adminWrite(f, f.input())
				if err != nil {
					t.Fatal(err)
				}
				in.MessageID = out.Message.ID
			case "missing":
				in.MessageID = "99999999-9999-4999-8999-999999999999"
			case "generation":
				execMessages(t, f.s, `UPDATE bot_credentials SET generation=generation+1 WHERE bot_id=?`, f.a.Bot.ID)
			case "revoked":
				execMessages(t, f.s, `DELETE FROM bot_credentials WHERE bot_id=? AND kind='api_token'`, f.a.Bot.ID)
			case "disabled":
				execMessages(t, f.s, `UPDATE bots SET enabled=0 WHERE id=?`, f.a.Bot.ID)
			case "send-removed":
				execMessages(t, f.s, `UPDATE bots SET can_send=0 WHERE id=?`, f.a.Bot.ID)
			case "snapshot-no-gain":
				execMessages(t, f.s, `UPDATE bots SET can_send=0 WHERE id=?`, f.a.Bot.ID)
				var err error
				identity, err = f.b.Authenticate(messageContext, f.a.Bot.ID, bots.APIToken, f.a.APIToken.Token)
				if err != nil {
					t.Fatal(err)
				}
				execMessages(t, f.s, `UPDATE bots SET can_send=1 WHERE id=?`, f.a.Bot.ID)
			}
			before := editSnapshot(t, f.other)
			out, err := editOwned(messageContext, f.s, f.r, identity, in)
			want := bots.ErrForbidden
			switch mode {
			case "cross-message", "missing":
				want = ErrHistoryNotFound
			case "generation", "revoked":
				want = bots.ErrUnauthorized
			}
			if !errors.Is(err, want) || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("authorization/read leaked or mutated", mode, err)
			}
		})
	}
}

func TestEditTextButtonsSQLAndCommitRollback(t *testing.T) {
	cases := map[string]string{
		"update":           `CREATE TRIGGER edit_fault BEFORE UPDATE OF text ON messages BEGIN SELECT RAISE(ABORT,'fault');END`,
		"affected":         `CREATE TRIGGER edit_fault BEFORE UPDATE OF text ON messages BEGIN SELECT RAISE(IGNORE);END`,
		"changes":          `CREATE TRIGGER edit_fault AFTER INSERT ON changes BEGIN SELECT RAISE(ABORT,'fault');END`,
		"final-generation": `CREATE TRIGGER edit_fault AFTER UPDATE OF text ON messages BEGIN UPDATE bot_credentials SET generation=generation+1 WHERE bot_id=NEW.bot_id;END`,
		"final-disabled":   `CREATE TRIGGER edit_fault AFTER INSERT ON changes BEGIN UPDATE bots SET enabled=0 WHERE id=NEW.bot_id;END`,
		"commit":           `CREATE TABLE edit_fault_parent(id PRIMARY KEY);CREATE TABLE edit_fault_child(id REFERENCES edit_fault_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER edit_fault AFTER INSERT ON changes BEGIN INSERT INTO edit_fault_child VALUES(1);END`,
	}
	for mode, ddl := range cases {
		t.Run(mode, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			execMessages(t, f.s, ddl)
			before := editSnapshot(t, f.other)
			var candidate Edited
			callbackOK := false
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.EditApplicationTx(messageContext, tx, f.identity, in)
				callbackOK = err == nil
				if mode == "commit" && err == nil {
					var revision, child int64
					if err := tx.QueryRow(`SELECT revision FROM messages WHERE id=?`, in.MessageID).Scan(&revision); err != nil {
						return err
					}
					if err := tx.QueryRow(`SELECT count(*) FROM edit_fault_child`).Scan(&child); err != nil {
						return err
					}
					if revision != 2 || child != 1 {
						t.Fatal("COMMIT fault not staged")
					}
				}
				return err
			})
			if err == nil {
				t.Fatal("fault committed")
			}
			if mode == "commit" {
				if !callbackOK || candidate.Message.Revision != 2 || candidate.LastChangeSeq == 0 {
					t.Fatal("not a real deferred FK COMMIT failure", err)
				}
			} else if callbackOK || !reflect.DeepEqual(candidate, Edited{}) {
				t.Fatal("primitive published partial candidate", err)
			}
			if mode == "final-generation" && !errors.Is(err, bots.ErrUnauthorized) {
				t.Fatal("missing final generation check", err)
			}
			if mode == "final-disabled" && !errors.Is(err, bots.ErrForbidden) {
				t.Fatal("missing final enable check", err)
			}
			if !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("rollback kept partial data")
			}
			// 同一真实故障下，提交拥有者必须丢弃primitive先返回的候选。
			out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
			if err == nil || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("owner leaked COMMIT/SQL candidate", err)
			}
			reopened, err := storage.Open(messageContext, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if !reflect.DeepEqual(before, editSnapshot(t, reopened)) {
				t.Fatal("reopen saw partial transaction")
			}
			execMessages(t, f.s, `DROP TRIGGER edit_fault`)
			out, err = editOwned(messageContext, f.s, f.r, f.identity, in)
			if err != nil || out.Message.Revision != 2 {
				t.Fatal("failure poisoned recovery", err)
			}
		})
	}
}

func TestEditTextButtonsCallerRollbackAndCancellation(t *testing.T) {
	f := setupMessages(t)
	_, in := editSeed(t, f)
	before := editSnapshot(t, f.other)
	sentinel := errors.New("later caller stage failed")
	var candidate Edited
	err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		var err error
		candidate, err = f.r.EditApplicationTx(messageContext, tx, f.identity, in)
		if err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) || candidate.Message.Revision != 2 || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
		t.Fatal("owner rollback changed persistent data", err)
	}
	if out, err := f.r.EditApplicationTx(messageContext, nil, f.identity, in); !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(out, Edited{}) {
		t.Fatal("nil tx accepted", err)
	}
	for _, differentStore := range []bool{false, true} {
		held, release := make(chan struct{}), make(chan struct{})
		finished := make(chan error, 1)
		go func() {
			finished <- f.s.Write(messageContext, func(tx *sql.Tx) error {
				_, err := tx.Exec(`UPDATE bots SET name=name`)
				close(held)
				<-release
				return err
			})
		}()
		<-held
		s, r := f.s, f.r
		if differentStore {
			s, r = f.other, New(f.other)
		}
		ctx, cancel := context.WithTimeout(messageContext, 20*time.Millisecond)
		out, err := editOwned(ctx, s, r, f.identity, in)
		cancel()
		close(release)
		if e := <-finished; e != nil {
			t.Fatal(e)
		}
		if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
			t.Fatal("cancelled queued edit published", err)
		}
	}
}

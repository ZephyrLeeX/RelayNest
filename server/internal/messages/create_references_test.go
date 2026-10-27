package messages

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/idempotency"
)

func replyRequest(id string) CreateRequest { in := request(); in.ReplyToMessageID = &id; return in }

func TestReferenceCreateRecoveryAndDomains(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := setupMessages(t)
			source, err := f.r.CreateApplication(messageContext, f.identity, f.input())
			if err != nil {
				t.Fatal(err)
			}
			in := replyRequest(source.Message.ID)
			create := func(key string, in CreateRequest) (Created, error) {
				if admin {
					return idemAdmin(f, f.a.Bot.ID, key, in)
				}
				return f.r.CreateIdempotentApplication(messageContext, f.identity, key, in, nil)
			}
			out, err := create(idemKey, in)
			if err != nil {
				t.Fatal(err)
			}
			if out.Message.ReplyToMessageID == nil || *out.Message.ReplyToMessageID != source.Message.ID || !reflect.DeepEqual(out.Message.ReplyToMessageID, out.Message.OriginalReplyToMessageID) {
				t.Fatal("missing reference")
			}
			if admin {
				var event struct {
					Message struct {
						Reply string `json:"reply_to_message_id"`
					} `json:"message"`
				}
				if err := json.Unmarshal(out.Delivery.PayloadJSON, &event); err != nil || event.Message.Reply != source.Message.ID {
					t.Fatal("snapshot lost reference", err)
				}
			}
			execMessages(t, f.s, "DELETE FROM messages WHERE id=?", source.Message.ID)
			err = f.s.Read(messageContext, func(tx *sql.Tx) error {
				var live, original sql.NullString
				if err := tx.QueryRow("SELECT reply_to_message_id,original_reply_to_message_id FROM messages WHERE id=?", out.Message.ID).Scan(&live, &original); err != nil {
					return err
				}
				if live.Valid || !original.Valid || original.String != source.Message.ID {
					t.Fatal("SET NULL lost original")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			before := idemState(t, f.other)
			replay, err := create(idemKey, in)
			if err != nil || !reflect.DeepEqual(out, replay) {
				t.Fatal("deleted source changed replay", err)
			}
			if _, err := create("11111111-1111-4111-8111-111111111111", in); !errors.Is(err, ErrReplyUnavailable) {
				t.Fatal("deleted source accepted", err)
			}
			if _, err := create(idemKey, replyRequest("22222222-2222-4222-8222-222222222222")); !errors.Is(err, idempotency.ErrConflict) {
				t.Fatal("missing different reply masked conflict", err)
			}
			other, err := f.b.Authenticate(messageContext, f.bbot.Bot.ID, bots.APIToken, f.bbot.APIToken.Token)
			if err != nil {
				t.Fatal(err)
			}
			otherIn := f.input()
			otherIn.BotID = f.bbot.Bot.ID
			foreign, err := f.r.CreateApplication(messageContext, other, otherIn)
			if err != nil {
				t.Fatal(err)
			}
			before = idemState(t, f.other)
			if _, err := create("33333333-3333-4333-8333-333333333333", replyRequest(foreign.Message.ID)); !errors.Is(err, ErrReplyUnavailable) {
				t.Fatal("foreign source accepted", err)
			}
			if !reflect.DeepEqual(before, idemState(t, f.other)) {
				t.Fatal("rejection wrote records")
			}
		})
	}
}

func TestReferenceWebhookRemainsUnsupported(t *testing.T) {
	f := setupMessages(t)
	source, err := f.r.CreateApplication(messageContext, f.identity, f.input())
	if err != nil {
		t.Fatal(err)
	}
	webhook, err := f.b.Authenticate(messageContext, f.a.Bot.ID, bots.WebhookKey, f.a.WebhookKey.Key)
	if err != nil {
		t.Fatal(err)
	}
	before := idemState(t, f.s)
	if _, err = f.r.CreateIdempotentApplication(messageContext, webhook, idemKey, replyRequest(source.Message.ID), nil); !errors.Is(err, ErrUnsupported) {
		t.Fatal("Webhook reply enabled", err)
	}
	in := f.input()
	in.ReplyToMessageID = &source.Message.ID
	if _, err = f.r.CreateApplication(messageContext, webhook, in); !errors.Is(err, ErrUnsupported) {
		t.Fatal("primitive Webhook reply enabled", err)
	}
	if !reflect.DeepEqual(before, idemState(t, f.s)) {
		t.Fatal("Webhook rejection wrote")
	}
}

func TestReferenceFaultsAreAtomic(t *testing.T) {
	cases := map[string]string{
		"sequence":    "CREATE TRIGGER fail BEFORE UPDATE OF bot_seq ON bots BEGIN SELECT RAISE(ABORT,'fault');END",
		"message":     "CREATE TRIGGER fail AFTER INSERT ON messages BEGIN SELECT RAISE(ABORT,'fault');END",
		"changes":     "CREATE TRIGGER fail AFTER INSERT ON changes BEGIN SELECT RAISE(ABORT,'fault');END",
		"delivery":    "CREATE TRIGGER fail AFTER INSERT ON deliveries BEGIN SELECT RAISE(ABORT,'fault');END",
		"push":        "CREATE TRIGGER fail AFTER INSERT ON push_jobs BEGIN SELECT RAISE(ABORT,'fault');END",
		"idempotency": "CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'fault');END",
		"commit":      "CREATE TABLE fault_parent(id PRIMARY KEY);CREATE TABLE fault_child(id REFERENCES fault_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN INSERT INTO fault_child VALUES(1);END",
	}
	for name, ddl := range cases {
		t.Run(name, func(t *testing.T) {
			f := setupMessages(t)
			source, err := f.r.CreateApplication(messageContext, f.identity, f.input())
			if err != nil {
				t.Fatal(err)
			}
			execMessages(t, f.s, ddl)
			before := idemState(t, f.other)
			var candidate Created
			callbackOK := false
			err = f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.CreateIdempotentAdministratorTx(messageContext, tx, f.a.Bot.ID, idemKey, replyRequest(source.Message.ID), f.input().PushTargets)
				callbackOK = err == nil
				return err
			})
			if err == nil {
				t.Fatal("fault accepted")
			}
			if name == "commit" && (!callbackOK || candidate.Message.ID == "") {
				t.Fatal("not a real COMMIT failure")
			}
			if !reflect.DeepEqual(before, idemState(t, f.other)) {
				t.Fatal("fault kept partial writes")
			}
			if name != "delivery" {
				out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, replyRequest(source.Message.ID), f.input().PushTargets)
				if err == nil || !reflect.DeepEqual(out, Created{}) {
					t.Fatal("wrapper leaked candidate", err)
				}
			}
		})
	}
}

func TestReferenceConcurrentKeysAndDelete(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f := setupMessages(t)
			source, err := f.r.CreateApplication(messageContext, f.identity, f.input())
			if err != nil {
				t.Fatal(err)
			}
			before := idemState(t, f.s)
			var wg sync.WaitGroup
			results := make(chan Created, 12)
			errs := make(chan error, 12)
			for n := 0; n < 12; n++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					r := f.r
					if n%2 == 0 {
						r = New(f.other)
					}
					key := idemKey
					if !same {
						key = fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
					}
					out, err := r.CreateIdempotentApplication(messageContext, f.identity, key, replyRequest(source.Message.ID), nil)
					results <- out
					errs <- err
				}(n)
			}
			wg.Wait()
			close(errs)
			close(results)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			ids := map[string]bool{}
			for out := range results {
				ids[out.Message.ID] = true
			}
			want := int64(12)
			if same {
				want = 1
			}
			if int64(len(ids)) != want {
				t.Fatal("wrong distinct receipts")
			}
			after := idemState(t, f.s)
			for _, i := range []int{0, 4, 5, 6, 7, 8} {
				if after[i]-before[i] != want {
					t.Fatalf("counter %d: %v -> %v", i, before, after)
				}
			}
		})
	}
	// 同库双 Store 的删源与首次创建竞争只允许完整成功或首次 404。
	for n := 0; n < 12; n++ {
		t.Run(fmt.Sprintf("delete-%d", n), func(t *testing.T) {
			f := setupMessages(t)
			source, err := f.r.CreateApplication(messageContext, f.identity, f.input())
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			deleted := make(chan error, 1)
			go func() {
				<-start
				deleted <- f.other.Write(messageContext, func(tx *sql.Tx) error {
					_, err := tx.Exec("DELETE FROM messages WHERE id=?", source.Message.ID)
					return err
				})
			}()
			close(start)
			out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, replyRequest(source.Message.ID), nil)
			if e := <-deleted; e != nil {
				t.Fatal(e)
			}
			if err != nil && !errors.Is(err, ErrReplyUnavailable) {
				t.Fatal(err)
			}
			if err == nil {
				if out.Message.ID == "" {
					t.Fatal("empty success")
				}
				if e := f.s.Read(messageContext, func(tx *sql.Tx) error {
					var live sql.NullString
					var original string
					if e := tx.QueryRow("SELECT reply_to_message_id,original_reply_to_message_id FROM messages WHERE id=?", out.Message.ID).Scan(&live, &original); e != nil {
						return e
					}
					if live.Valid || original != source.Message.ID {
						t.Fatal("delete race lost FK invariant")
					}
					return nil
				}); e != nil {
					t.Fatal(e)
				}
			} else if out.Message.ID != "" {
				t.Fatal("failed result leaked")
			}
		})
	}
}

func TestReferenceLookupSQLFailureDoesNotAllocate(t *testing.T) {
	f := setupMessages(t)
	source, err := f.r.CreateApplication(messageContext, f.identity, f.input())
	if err != nil {
		t.Fatal(err)
	}
	before := idemState(t, f.other)
	// 暂时用会在引用 SELECT 求值时报错的视图代替表；恢复后检查业务全量状态。
	execMessages(t, f.s, `ALTER TABLE messages RENAME TO saved_messages;CREATE VIEW messages AS SELECT id,bot_id FROM saved_messages WHERE json_extract('invalid-json','$')=1`)
	out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, replyRequest(source.Message.ID), nil)
	execMessages(t, f.s, `DROP VIEW messages;ALTER TABLE saved_messages RENAME TO messages`)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") || !reflect.DeepEqual(out, Created{}) {
		t.Fatal("reference SQL failure was not propagated", err)
	}
	if !reflect.DeepEqual(before, idemState(t, f.other)) {
		t.Fatal("reference SQL failure allocated side effects")
	}
}

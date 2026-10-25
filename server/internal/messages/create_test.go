package messages

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/idempotency"
	"relaynest/server/internal/storage"
)

const idemKey = "abcdefff-1234-4567-89ab-123456789abc"

func request() CreateRequest { return CreateRequest{Format: "markdown", Text: " 正文 α\n"} }
func idemState(t *testing.T, s *storage.Store) []int64 {
	t.Helper()
	state := persisted(t, s)
	if err := s.Read(messageContext, func(tx *sql.Tx) error {
		var n int64
		err := tx.QueryRow("SELECT count(*) FROM idempotency").Scan(&n)
		state = append(state, n)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return state
}

// 管理员测试驱动仅模拟受信外层拥有事务，不冒充真实 Session/G1 的验收。
func idemAdmin(f *fixture, botID, key string, in CreateRequest) (Created, error) {
	var out Created
	err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		var err error
		out, err = f.r.CreateIdempotentAdministratorTx(messageContext, tx, botID, key, in, f.input().PushTargets)
		return err
	})
	if err != nil {
		return Created{}, err
	}
	return out, nil
}
func TestIdempotentRecoveryAndOriginalSnapshot(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := setupMessages(t)
			create := func(r *Repository, key string, in CreateRequest) (Created, error) {
				if admin {
					return idemAdmin(f, f.a.Bot.ID, key, in)
				}
				return r.CreateIdempotentApplication(messageContext, f.identity, key, in, f.input().PushTargets)
			}
			original, err := create(f.r, idemKey, request())
			if err != nil {
				t.Fatal(err)
			}
			before := idemState(t, f.other)
			in := request()
			in.Attachments = []AttachmentReference{}
			in.Buttons = [][]Button{}
			replay, err := create(f.r, strings.ToUpper(idemKey), in)
			if err != nil || !reflect.DeepEqual(original, replay) {
				t.Fatalf("replay changed result: %v", err)
			}
			if !reflect.DeepEqual(before, idemState(t, f.other)) {
				t.Fatal("replay wrote side effects")
			}
			execMessages(t, f.s, "UPDATE messages SET text='edited',revision=2 WHERE id=?", original.Message.ID)
			if err = f.s.Close(); err != nil {
				t.Fatal(err)
			}
			if err = f.other.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := storage.Open(messageContext, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			f.s = reopened
			f.r = New(reopened)
			replay, err = create(f.r, idemKey, request())
			if err != nil || !reflect.DeepEqual(original, replay) {
				t.Fatal("lost receipt/reopen did not restore original snapshot", err)
			}
			in = request()
			in.Text += " "
			if _, err = create(f.r, idemKey, in); !errors.Is(err, idempotency.ErrConflict) {
				t.Fatal("same key different content accepted", err)
			}
			in = request()
			in.Format = "plain"
			if _, err = create(f.r, idemKey, in); !errors.Is(err, idempotency.ErrConflict) {
				t.Fatal("format omitted from digest", err)
			}
		})
	}
}
func TestIdempotentConcurrentSameKey(t *testing.T) {
	f := setupMessages(t)
	other := New(f.other)
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
				r = other
			}
			out, err := r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), f.input().PushTargets)
			results <- out
			errs <- err
		}(n)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first Created
	for out := range results {
		if first.Message.ID == "" {
			first = out
		}
		if !reflect.DeepEqual(first, out) {
			t.Fatal("concurrent duplicate resource")
		}
	}
	after := idemState(t, f.s)
	// application 创建一条消息、change、push及幂等记录，Bot序号只增加一次。
	for index, delta := range map[int]int64{0: 1, 1: 0, 2: 0, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1} {
		if after[index]-before[index] != delta {
			t.Fatalf("counter %d: %v -> %v", index, before, after)
		}
	}
}
func TestIdempotentAuthorizationAndScopes(t *testing.T) {
	f := setupMessages(t)
	original, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil)
	if err != nil {
		t.Fatal(err)
	}
	rotated, _, err := f.b.RotateAPIToken(messageContext, f.a.Bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := idemState(t, f.s)
	if out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil); !errors.Is(err, bots.ErrUnauthorized) || out.Message.ID != "" {
		t.Fatal("old generation replay accepted", err)
	}
	fresh, err := f.b.Authenticate(messageContext, f.a.Bot.ID, bots.APIToken, rotated.Token)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.r.CreateIdempotentApplication(messageContext, fresh, idemKey, request(), nil)
	if err != nil || !reflect.DeepEqual(original, replay) {
		t.Fatal("reset changed scope", err)
	}
	if !reflect.DeepEqual(before, idemState(t, f.s)) {
		t.Fatal("reset retry allocated resource")
	}
	execMessages(t, f.s, "UPDATE bots SET can_send=0 WHERE id=?", f.a.Bot.ID)
	if _, err = f.r.CreateIdempotentApplication(messageContext, fresh, idemKey, request(), nil); !errors.Is(err, bots.ErrForbidden) {
		t.Fatal("revoked capability replay", err)
	}
	execMessages(t, f.s, "UPDATE bots SET can_send=1 WHERE id=?", f.a.Bot.ID)
	second, err := f.b.Authenticate(messageContext, f.bbot.Bot.ID, bots.APIToken, f.bbot.APIToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.r.CreateIdempotentApplication(messageContext, second, idemKey, request(), nil)
	if err != nil || out.Message.ID == original.Message.ID {
		t.Fatal("cross Bot scope", err)
	}
	webhook, err := f.b.Authenticate(messageContext, f.a.Bot.ID, bots.WebhookKey, f.a.WebhookKey.Key)
	if err != nil {
		t.Fatal(err)
	}
	out, err = f.r.CreateIdempotentApplication(messageContext, webhook, idemKey, request(), nil)
	if err != nil || out.Message.ID == original.Message.ID {
		t.Fatal("operation scope", err)
	}
	admin, err := idemAdmin(f, f.a.Bot.ID, idemKey, request())
	if err != nil || admin.Message.ID == original.Message.ID {
		t.Fatal("administrator identity scope", err)
	}
	if _, err = idemAdmin(f, f.bbot.Bot.ID, idemKey, request()); !errors.Is(err, idempotency.ErrConflict) {
		t.Fatal("target Bot omitted from digest", err)
	}
	execMessages(t, f.s, "UPDATE bots SET can_receive=0 WHERE id=?", f.a.Bot.ID)
	if _, err = idemAdmin(f, f.a.Bot.ID, idemKey, request()); !errors.Is(err, bots.ErrForbidden) {
		t.Fatal("administrator replay bypassed capability", err)
	}
}
func TestIdempotentFaultsAreAtomic(t *testing.T) {
	cases := map[string]string{
		"counter":        "CREATE TRIGGER fail BEFORE UPDATE OF bot_seq ON bots BEGIN SELECT RAISE(IGNORE);END",
		"message":        "CREATE TRIGGER fail AFTER INSERT ON messages BEGIN SELECT RAISE(ABORT,'fault');END",
		"change":         "CREATE TRIGGER fail AFTER INSERT ON changes BEGIN SELECT RAISE(ABORT,'fault');END",
		"backfill":       "CREATE TRIGGER fail BEFORE UPDATE OF created_change_seq ON messages BEGIN SELECT RAISE(IGNORE);END",
		"delivery":       "CREATE TRIGGER fail AFTER INSERT ON deliveries BEGIN SELECT RAISE(ABORT,'fault');END",
		"push":           "CREATE TRIGGER fail AFTER INSERT ON push_jobs BEGIN SELECT RAISE(ABORT,'fault');END",
		"record":         "CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'fault');END",
		"ignored_record": "CREATE TRIGGER fail BEFORE INSERT ON idempotency BEGIN SELECT RAISE(IGNORE);END",
		"commit":         "CREATE TABLE fault_parent(id INTEGER PRIMARY KEY);CREATE TABLE fault_child(id REFERENCES fault_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN INSERT INTO fault_child VALUES(42);END",
	}
	for name, ddl := range cases {
		t.Run(name, func(t *testing.T) {
			f := setupMessages(t)
			execMessages(t, f.s, ddl)
			before := idemState(t, f.other)
			var candidate Created
			callbackOK := false
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.CreateIdempotentAdministratorTx(messageContext, tx, f.a.Bot.ID, idemKey, request(), f.input().PushTargets)
				callbackOK = err == nil
				return err
			})
			if err == nil {
				t.Fatal("fault accepted")
			}
			if name == "commit" && (!callbackOK || candidate.Message.ID == "") {
				t.Fatal("did not reach real commit failure")
			}
			if !reflect.DeepEqual(before, idemState(t, f.other)) {
				t.Fatal("partial transaction retained")
			}
			if name != "delivery" {
				out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), f.input().PushTargets)
				if err == nil || !reflect.DeepEqual(out, Created{}) {
					t.Fatal("failed wrapper leaked candidate", err)
				}
				if !reflect.DeepEqual(before, idemState(t, f.other)) {
					t.Fatal("wrapper retained partial records")
				}
			}
		})
	}
	f := setupMessages(t)
	execMessages(t, f.s, "UPDATE bots SET bot_seq=? WHERE id=?", int64(math.MaxInt64), f.a.Bot.ID)
	before := idemState(t, f.s)
	if _, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil); !errors.Is(err, ErrSequenceExhausted) {
		t.Fatal("overflow accepted", err)
	}
	if !reflect.DeepEqual(before, idemState(t, f.s)) {
		t.Fatal("overflow retained partial records")
	}
}
func TestIdempotentValidationAndTombstone(t *testing.T) {
	f := setupMessages(t)
	invalid := []CreateRequest{{Format: "bad", Text: "x"}, {Format: "plain"}, {Format: "plain", Text: string([]byte{255})}, {Format: "plain", Text: strings.Repeat("界", 21846)}, {Format: "plain", Text: "x", Attachments: []AttachmentReference{{ID: "x", Kind: "file"}}}, {Format: "plain", Text: "x", Buttons: [][]Button{{{ID: "x"}}}}, {Format: "plain", Text: "x", ReplyToMessageID: new(string)}}
	for _, in := range invalid {
		if _, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, in, nil); err == nil {
			t.Fatal("invalid or unsupported input accepted")
		}
	}
	if _, err := f.r.CreateIdempotentApplication(messageContext, f.identity, "bad", request(), nil); err == nil {
		t.Fatal("invalid key accepted")
	}
	in := request()
	in.Text = strings.Repeat("a", 65536)
	out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, in, nil)
	if err != nil {
		t.Fatal("64 KiB boundary", err)
	}
	scope := idempotency.Scope{Identity: "bot:" + f.a.Bot.ID, Operation: CreateBotMessage, Key: idemKey}
	err = f.s.Write(messageContext, func(tx *sql.Tx) error { return idempotency.TombstoneTx(messageContext, tx, scope, time.Now()) })
	if !errors.Is(err, idempotency.ErrProtected) {
		t.Fatal("live record expired", err)
	}
	err = f.s.Write(messageContext, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM messages WHERE id=?", out.Message.ID); err != nil {
			return err
		}
		return idempotency.TombstoneTx(messageContext, tx, scope, time.Now())
	})
	if err != nil {
		t.Fatal(err)
	}
	before := idemState(t, f.s)
	if _, err = f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, in, nil); !errors.Is(err, idempotency.ErrExpired) {
		t.Fatal("tombstone recreated resource", err)
	}
	if !reflect.DeepEqual(before, idemState(t, f.s)) {
		t.Fatal("tombstone retry changed data")
	}
	if err = f.s.Read(messageContext, func(tx *sql.Tx) error {
		var empty bool
		err := tx.QueryRow("SELECT result_json IS NULL FROM idempotency").Scan(&empty)
		if !empty {
			t.Fatal("tombstone retained body")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateDigestFieldsAndNormalization(t *testing.T) {
	base := request()
	sum, err := createDigest("bot", base)
	if err != nil {
		t.Fatal(err)
	}
	empty := base
	empty.Attachments = []AttachmentReference{}
	empty.Buttons = [][]Button{}
	normalized, err := createDigest("bot", empty)
	if err != nil || normalized != sum {
		t.Fatal("empty normalization", err)
	}
	url := "https://example.invalid"
	callback := "x"
	reply := "id"
	variants := []CreateRequest{base, base, base, base, base, base}
	variants[0].Text += " "
	variants[1].Format = "plain"
	variants[2].Attachments = []AttachmentReference{{ID: "a", Kind: "image"}}
	variants[3].ReplyToMessageID = &reply
	variants[4].Buttons = [][]Button{{{ID: "b", Label: "go", URL: &url}}}
	variants[5].Buttons = [][]Button{{{ID: "b", Label: "go", CallbackData: &callback}}}
	for _, in := range variants {
		digest, err := createDigest("bot", in)
		if err != nil || digest == sum {
			t.Fatal("field absent from digest", err)
		}
	}
	target, _ := createDigest("another", base)
	if target == sum {
		t.Fatal("target omitted")
	}
	ordered := base
	ordered.Attachments = []AttachmentReference{{ID: "a", Kind: "file"}, {ID: "b", Kind: "image"}}
	order1, _ := createDigest("bot", ordered)
	ordered.Attachments[0], ordered.Attachments[1] = ordered.Attachments[1], ordered.Attachments[0]
	order2, _ := createDigest("bot", ordered)
	if order1 == order2 {
		t.Fatal("array reordered")
	}
	f := setupMessages(t)
	if _, err = f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, base, nil); err != nil {
		t.Fatal(err)
	}
	for _, in := range variants {
		if _, err = f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, in, nil); !errors.Is(err, idempotency.ErrConflict) {
			t.Fatal("field mismatch did not conflict", err)
		}
	}
}

func TestProtectedAdministratorReceiptCannotExpire(t *testing.T) {
	f := setupMessages(t)
	out, err := idemAdmin(f, f.a.Bot.ID, idemKey, request())
	if err != nil {
		t.Fatal(err)
	}
	scope := idempotency.Scope{Identity: "administrator", Operation: CreateClientMessage, Key: idemKey}
	// 消息已清理但投递仍存活时，完整回执仍受保护，不能按七天窗口删除。
	execMessages(t, f.s, "DELETE FROM messages WHERE id=?", out.Message.ID)
	err = f.s.Write(messageContext, func(tx *sql.Tx) error {
		return idempotency.TombstoneTx(messageContext, tx, scope, out.Message.CreatedAt.Add(30*24*time.Hour))
	})
	if !errors.Is(err, idempotency.ErrProtected) {
		t.Fatal("live delivery expired", err)
	}
	replay, err := idemAdmin(f, f.a.Bot.ID, idemKey, request())
	if err != nil || !reflect.DeepEqual(out, replay) {
		t.Fatal("protected receipt lost", err)
	}
}

func TestPermanentBotDeleteTombstonesAtomically(t *testing.T) {
	for _, mode := range []string{"success", "statement", "commit"} {
		t.Run(mode, func(t *testing.T) {
			f := setupMessages(t)
			application, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil)
			if err != nil {
				t.Fatal(err)
			}
			administrator, err := idemAdmin(f, f.a.Bot.ID, idemKey, request())
			if err != nil {
				t.Fatal(err)
			}
			// 另一 Bot 的快照不受删除影响。
			second, err := f.b.Authenticate(messageContext, f.bbot.Bot.ID, bots.APIToken, f.bbot.APIToken.Token)
			if err != nil {
				t.Fatal(err)
			}
			other, err := f.r.CreateIdempotentApplication(messageContext, second, idemKey, request(), nil)
			if err != nil {
				t.Fatal(err)
			}
			before := idemState(t, f.s)
			switch mode {
			case "statement":
				execMessages(t, f.s, "CREATE TRIGGER fail AFTER DELETE ON bots BEGIN SELECT RAISE(ABORT,'delete failure');END")
			case "commit":
				execMessages(t, f.s, "CREATE TABLE fault_parent(id PRIMARY KEY);CREATE TABLE fault_child(id REFERENCES fault_parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail AFTER DELETE ON bots BEGIN INSERT INTO fault_child VALUES(1);END")
			}
			_, err = f.b.Delete(messageContext, f.a.Bot.ID, bots.DeleteRequest{ExpectedVersion: f.a.Bot.Version, ConfirmName: f.a.Bot.Name})
			if mode != "success" {
				if err == nil {
					t.Fatal("delete fault accepted")
				}
				if mode == "commit" && !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
					t.Fatal("expected deferred foreign key commit failure", err)
				}
				if !reflect.DeepEqual(before, idemState(t, f.s)) {
					t.Fatal("delete fault changed resources")
				}
				restored, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil)
				if err != nil || !reflect.DeepEqual(application, restored) {
					t.Fatal("delete rollback lost application snapshot", err)
				}
				restored, err = idemAdmin(f, f.a.Bot.ID, idemKey, request())
				if err != nil || !reflect.DeepEqual(administrator, restored) {
					t.Fatal("delete rollback lost administrator receipt", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			err = f.s.Read(messageContext, func(tx *sql.Tx) error {
				var n int
				if err := tx.QueryRow("SELECT count(*) FROM idempotency WHERE message_id IN (?,?) AND result_json IS NULL AND expired_at IS NOT NULL", application.Message.ID, administrator.Message.ID).Scan(&n); err != nil {
					return err
				}
				if n != 2 {
					t.Fatal("deleted Bot retained body or lost tombstone")
				}
				digest, err := createDigest(f.a.Bot.ID, request())
				if err != nil {
					return err
				}
				_, _, err = idempotency.LookupTx(messageContext, tx, idempotency.Scope{Identity: "bot:" + f.a.Bot.ID, Operation: CreateBotMessage, Key: idemKey}, digest)
				if !errors.Is(err, idempotency.ErrExpired) {
					t.Fatal("deleted resource tombstone did not expire", err)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil); !errors.Is(err, bots.ErrUnauthorized) {
				t.Fatal("deleted credential accessed tombstone", err)
			}
			restored, err := f.r.CreateIdempotentApplication(messageContext, second, idemKey, request(), nil)
			if err != nil || !reflect.DeepEqual(other, restored) {
				t.Fatal("cross Bot delete altered snapshot", err)
			}
		})
	}
}

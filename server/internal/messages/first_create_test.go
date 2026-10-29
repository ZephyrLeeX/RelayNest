package messages

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestFirstCreateCallbackOrderingReplayAndRollback(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(map[bool]string{false: "bot", true: "administrator"}[admin], func(t *testing.T) {
			f := setupMessages(t)
			calls := 0
			reject := errors.New("synthetic first-create failure")
			fail := true
			f.r = NewWithFirstCreate(f.s, func(ctx context.Context, tx *sql.Tx, out Created) error {
				calls++
				var count int
				if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM idempotency").Scan(&count); err != nil {
					return err
				}
				if count != 0 {
					t.Fatal("callback ran after receipt")
				}
				if out.LastChangeSeq < 1 || (admin && (out.Delivery == nil || out.LastChangeSeq != out.Delivery.EnqueueSeq || out.LastChangeSeq <= *out.Message.CreatedChangeSeq)) {
					t.Fatal("callback missed final delivery watermark")
				}
				if fail {
					return reject
				}
				return nil
			})
			create := func() (Created, error) {
				if admin {
					return idemAdmin(f, f.a.Bot.ID, idemKey, request())
				}
				return f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil)
			}
			before := idemState(t, f.s)
			if out, err := create(); !errors.Is(err, reject) || out.Message.ID != "" {
				t.Fatal("callback failure exposed candidate", out, err)
			}
			if !reflect.DeepEqual(before, idemState(t, f.s)) {
				t.Fatal("callback failure was not atomic")
			}
			fail = false
			original, err := create()
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatal(calls)
			}
			replay, err := create()
			if err != nil || !reflect.DeepEqual(original, replay) || calls != 2 {
				t.Fatal("replay reran callback", err, calls)
			}
		})
	}
}

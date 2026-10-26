package attachments

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func begin(t *testing.T, x *fixture, in Input) Attempt {
	t.Helper()
	var a Attempt
	e := x.s.Write(context.Background(), func(tx *sql.Tx) error { var e error; a, e = x.r.BeginTx(context.Background(), tx, in); return e })
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestExpiredAndLateAttemptFenced(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	in := input()
	now := time.Now()
	x.r.now = func() time.Time { return now }
	old := begin(t, x, in)
	if e := x.s.Read(ctx, func(tx *sql.Tx) error { _, e := x.r.ReadyTx(ctx, tx, old.Attachment.ID); return e }); !errors.Is(e, ErrNotReady) {
		t.Fatal(e)
	}
	now = now.Add(2 * x.r.lease)
	if e := x.r.Renew(ctx, old); !errors.Is(e, ErrFenced) {
		t.Fatal(e)
	}
	current := begin(t, x, in)
	if old.Token == current.Token || current.Generation != old.Generation+1 || current.Attachment.ID != old.Attachment.ID {
		t.Fatal(old, current)
	}
	newDigest := sha256.Sum256([]byte("new"))
	oldDigest := sha256.Sum256([]byte("old"))
	if e := x.r.Renew(ctx, old); !errors.Is(e, ErrFenced) {
		t.Fatal(e)
	}
	for _, a := range []Attempt{current, old} {
		_, _, e := x.f.write(ctx, a.Token, strings.NewReader(map[bool]string{true: "new", false: "old"}[a.Token == current.Token]), 100)
		if e != nil {
			t.Fatal(e)
		}
		if e = x.f.publish(a.Token); e != nil {
			t.Fatal(e)
		}
	}
	e := x.s.Write(ctx, func(tx *sql.Tx) error { _, e := x.r.CompleteTx(ctx, tx, current, 3, newDigest[:]); return e })
	if e != nil {
		t.Fatal(e)
	}
	e = x.s.Write(ctx, func(tx *sql.Tx) error { _, e := x.r.CompleteTx(ctx, tx, old, 3, oldDigest[:]); return e })
	if !errors.Is(e, ErrFenced) {
		t.Fatal(e)
	}
	e = x.s.Read(ctx, func(tx *sql.Tx) error {
		a, e := x.r.ReadyTx(ctx, tx, current.Attachment.ID)
		if e == nil && a.StorageKey != current.Token+".ready" {
			t.Fatal(a)
		}
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
}
func TestStableScopeAndMetadataConflict(t *testing.T) {
	x := setup(t)
	in := input()
	a, e := x.u.Upload(context.Background(), in, strings.NewReader("body"))
	if e != nil {
		t.Fatal(e)
	}
	changed := in
	changed.Filename = "different"
	if _, e = x.u.Upload(context.Background(), changed, strings.NewReader("body")); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	changed = in
	changed.Identity = "administrator:stable"
	b, e := x.u.Upload(context.Background(), changed, strings.NewReader("body"))
	if e != nil || a.ID == b.ID {
		t.Fatal(b, e)
	}
}

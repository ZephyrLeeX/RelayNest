package bots

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// 受控事务的未提交投影必须可见，且授权/提交失败时不能释放候选结果。
func TestListPageUsesAuthorizedTransaction(t *testing.T) {
	s, r, _ := fixture(t)
	created := create(t, r, Capabilities{true, true})
	rejected := errors.New("authorization rejected")
	items, err := r.ListPageWithRead(ctx, "", 101, func(ctx context.Context, fn func(*sql.Tx) error) error {
		return s.Write(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, "UPDATE bots SET name='transaction projection' WHERE id=?", created.Bot.ID); err != nil {
				return err
			}
			if err := fn(tx); err != nil {
				return err
			}
			return rejected
		})
	})
	if !errors.Is(err, rejected) || items != nil {
		t.Fatal("failed authorization released projection", err)
	}
	items, err = r.ListPageWithRead(ctx, "", 101, func(ctx context.Context, fn func(*sql.Tx) error) error {
		return s.Write(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, "UPDATE bots SET name='transaction projection' WHERE id=?", created.Bot.ID); err != nil {
				return err
			}
			return fn(tx)
		})
	})
	if err != nil || len(items) != 1 || items[0].Name != "transaction projection" {
		t.Fatal("list opened a separate transaction", items, err)
	}
}

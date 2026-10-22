package auth

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"sync"
	"testing"

	"relaynest/server/internal/storage"
)

func testStore(t *testing.T, path string) *storage.Store {
	t.Helper()
	s, err := storage.Open(context.Background(), storage.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestConcurrentInitIndependentStores(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "admin.sqlite")
	a, b := testStore(t, path), testStore(t, path)
	start, results := make(chan struct{}), make(chan error, 2)
	var wg sync.WaitGroup
	for _, s := range []*storage.Store{a, b} {
		wg.Add(1)
		go func(s *storage.Store) {
			defer wg.Done()
			<-start
			results <- InitAdministrator(ctx, s, " 管理员 ", []byte("secret"))
		}(s)
	}
	close(start)
	wg.Wait()
	close(results)
	success, duplicate := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrAlreadyInitialized) {
			duplicate++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || duplicate != 1 {
		t.Fatalf("success=%d duplicate=%d", success, duplicate)
	}
	admin, err := ReadAdministrator(ctx, b)
	if err != nil || admin.Username != " 管理员 " || admin.AuthGeneration != 1 || !VerifyPassword([]byte("secret"), admin.PasswordHash) {
		t.Fatal("unexpected administrator", err)
	}
	if err := InitAdministrator(ctx, a, "different", []byte("new")); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatal("duplicate init accepted", err)
	}
	for _, statement := range []string{
		"INSERT INTO administrator VALUES(2,'other','hash',1)",
		"INSERT INTO administrator VALUES(1,'other','hash',1)",
		"UPDATE administrator SET auth_generation=9223372036854775808",
		"UPDATE administrator SET auth_generation=0",
	} {
		if err := a.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(statement); return err }); err == nil {
			t.Fatal("constraint bypassed")
		}
	}
}

func TestResetAtomicAndExhaustion(t *testing.T) {
	ctx := context.Background()
	s := testStore(t, filepath.Join(t.TempDir(), "reset.sqlite"))
	if _, err := ReadAdministrator(ctx, s); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
	if err := ResetPassword(ctx, s, []byte("new")); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
	if err := InitAdministrator(ctx, s, "Admin", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := ResetPassword(ctx, s, []byte("new")); err != nil {
		t.Fatal(err)
	}
	before, err := ReadAdministrator(ctx, s)
	if err != nil || before.AuthGeneration != 2 || before.Username != "Admin" || !VerifyPassword([]byte("new"), before.PasswordHash) || VerifyPassword([]byte("old"), before.PasswordHash) {
		t.Fatal("reset invariant", err)
	}
	// 延迟外键故障在提交阶段触发，证明摘要和代次均不能部分提交。
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE reset_fault(id INTEGER REFERENCES administrator(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_reset AFTER UPDATE ON administrator BEGIN INSERT INTO reset_fault VALUES(2); END;")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := ResetPassword(ctx, s, []byte("should-not-commit")); !errors.Is(err, ErrStorage) {
		t.Fatal("commit failure accepted", err)
	}
	after, err := ReadAdministrator(ctx, s)
	if err != nil || before != after {
		t.Fatal("failed reset changed administrator", err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DROP TRIGGER fail_reset"); err != nil {
			return err
		}
		_, err := tx.Exec("UPDATE administrator SET auth_generation=?", int64(math.MaxInt64-1))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := ResetPassword(ctx, s, []byte("last")); err != nil {
		t.Fatal(err)
	}
	before, err = ReadAdministrator(ctx, s)
	if err != nil || before.AuthGeneration != math.MaxInt64 {
		t.Fatal("generation boundary", err)
	}
	if err := ResetPassword(ctx, s, []byte("exhausted")); !errors.Is(err, ErrGenerationExhausted) {
		t.Fatal("generation wrapped", err)
	}
	after, err = ReadAdministrator(ctx, s)
	if err != nil || before != after {
		t.Fatal("exhaustion changed digest or generation", err)
	}
}

func TestInvalidInitAndCancelledReset(t *testing.T) {
	ctx := context.Background()
	s := testStore(t, filepath.Join(t.TempDir(), "invalid.sqlite"))
	if err := InitAdministrator(ctx, s, "", []byte("secret")); err == nil {
		t.Fatal("empty username accepted")
	}
	if err := InitAdministrator(ctx, s, "admin", nil); err == nil {
		t.Fatal("empty password accepted")
	}
	if err := InitAdministrator(ctx, s, "admin", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	before, err := ReadAdministrator(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := ResetPassword(cancelled, s, []byte("new")); err == nil {
		t.Fatal("cancelled reset succeeded")
	}
	if err := ResetPassword(ctx, s, nil); err == nil {
		t.Fatal("empty reset accepted")
	}
	after, err := ReadAdministrator(ctx, s)
	if err != nil || before != after {
		t.Fatal("rejected reset changed record", err)
	}
}

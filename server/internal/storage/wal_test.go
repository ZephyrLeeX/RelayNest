package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// 先生成真实业务库，再转为 rollback journal 并持有 SHARED 读锁。
// 此锁阻止 WAL 模式切换，尚未执行迁移 BEGIN，明确覆盖 WAL 阶段。
func lockedWALDatabase(t *testing.T) (Config, *sql.DB, func(), string) {
	t.Helper()
	s, cfg := openTest(t)
	var epoch string
	if err := s.reader.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.BusyTimeout = 150 * time.Millisecond
	db, err := openPool(cfg.Path, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode=DELETE").Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("journal=%s err=%v", mode, err)
	}
	if _, err := db.Exec("BEGIN; SELECT * FROM system_state"); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			if _, err := db.Exec("ROLLBACK"); err != nil {
				t.Error(err)
			}
		})
	}
	t.Cleanup(release)
	return cfg, db, release, epoch
}

func assertWALRecovery(t *testing.T, cfg Config, epoch string) {
	t.Helper()
	s, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var got string
	if err := s.reader.QueryRow("SELECT sync_epoch FROM system_state").Scan(&got); err != nil || got != epoch {
		t.Fatalf("epoch=%s err=%v", got, err)
	}
	var timeout int
	if err := s.writer.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != int(cfg.BusyTimeout.Milliseconds()) {
		t.Fatalf("busy_timeout=%d err=%v", timeout, err)
	}
}

func TestWALWaitBoundaries(t *testing.T) {
	for _, name := range []string{"release", "budget", "cancel", "deadline", "canceled"} {
		t.Run(name, func(t *testing.T) {
			cfg, _, release, epoch := lockedWALDatabase(t)
			probe, err := openPool(cfg.Path, cfg, false)
			if err != nil {
				t.Fatal(err)
			}
			defer probe.Close()
			conn, err := probe.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if _, err := conn.ExecContext(context.Background(), "PRAGMA busy_timeout=0"); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			budget := cfg.BusyTimeout
			if name == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 60*time.Millisecond)
				defer stop()
				budget = time.Second
			}
			if name == "canceled" {
				cancel()
			}
			busy := make(chan struct{})
			attempts := 0
			result := make(chan error, 1)
			started := time.Now()
			go func() {
				result <- waitForWAL(ctx, budget, func(ctx context.Context) (string, error) {
					attempts++
					var mode string
					err := conn.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode)
					var e *sqlite.Error
					if errors.As(err, &e) && e.Code()&0xff == sqlite3.SQLITE_BUSY && attempts == 1 {
						close(busy)
					}
					return mode, err
				})
			}()
			if name != "canceled" {
				select {
				case <-busy:
				case err := <-result:
					t.Fatalf("did not enter WAL BUSY: %v", err)
				case <-time.After(2 * time.Second):
					t.Fatal("no WAL BUSY signal")
				}
			}
			actionAt := time.Now()
			if name == "release" {
				release()
			}
			if name == "cancel" {
				cancel()
			}
			select {
			case err = <-result:
			case <-time.After(2 * time.Second):
				t.Fatal("WAL wait did not finish")
			}
			elapsed := time.Since(started)
			switch name {
			case "release":
				if err != nil || attempts < 2 {
					t.Fatalf("attempts=%d err=%v", attempts, err)
				}
			case "budget":
				var e *sqlite.Error
				if !errors.As(err, &e) || e.Code()&0xff != sqlite3.SQLITE_BUSY || elapsed < budget || elapsed > budget+200*time.Millisecond || attempts < 2 {
					t.Fatalf("budget wait=%s attempts=%d err=%v", elapsed, attempts, err)
				}
			case "cancel", "canceled":
				if !errors.Is(err, context.Canceled) || time.Since(actionAt) > 200*time.Millisecond {
					t.Fatalf("cancel wait=%s err=%v", elapsed, err)
				}
				if name == "canceled" && attempts != 0 {
					t.Fatalf("canceled context attempted %d queries", attempts)
				}
			case "deadline":
				if !errors.Is(err, context.DeadlineExceeded) || elapsed > 260*time.Millisecond {
					t.Fatalf("deadline wait=%s err=%v", elapsed, err)
				}
			}
			t.Logf("%s: attempts=%d elapsed=%s after_action=%s err=%v", name, attempts, elapsed, time.Since(actionAt), err)
			conn.Close()
			probe.Close()
			release()
			assertWALRecovery(t, cfg, epoch)
		})
	}
}

func TestWALOpenBudgetAndRecovery(t *testing.T) {
	cfg, _, release, epoch := lockedWALDatabase(t)
	start := time.Now()
	s, err := Open(context.Background(), cfg)
	elapsed := time.Since(start)
	var e *sqlite.Error
	if s != nil || !errors.As(err, &e) || e.Code()&0xff != sqlite3.SQLITE_BUSY || elapsed < cfg.BusyTimeout || elapsed > cfg.BusyTimeout+200*time.Millisecond {
		t.Fatalf("Open wait=%s store=%v err=%v", elapsed, s, err)
	}
	t.Logf("Open budget elapsed=%s err=%v", elapsed, err)
	release()
	assertWALRecovery(t, cfg, epoch)
}

func TestWALNonLockError(t *testing.T) {
	cfg, _, release, epoch := lockedWALDatabase(t)
	release()
	db, err := openPool(cfg.Path, cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	start := time.Now()
	err = enableWAL(context.Background(), db, time.Second)
	var e *sqlite.Error
	if !errors.As(err, &e) || e.Code()&0xff != sqlite3.SQLITE_READONLY || time.Since(start) > 200*time.Millisecond {
		t.Fatalf("non-lock elapsed=%s err=%v", time.Since(start), err)
	}
	assertWALRecovery(t, cfg, epoch)
}

func TestWALOpenContext(t *testing.T) {
	for _, name := range []string{"canceled", "deadline"} {
		t.Run(name, func(t *testing.T) {
			cfg, _, release, epoch := lockedWALDatabase(t)
			cfg.BusyTimeout = time.Second
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
			defer cancel()
			want := context.DeadlineExceeded
			if name == "canceled" {
				cancel()
				want = context.Canceled
			}
			start := time.Now()
			s, err := Open(ctx, cfg)
			if s != nil || !errors.Is(err, want) || time.Since(start) > 260*time.Millisecond {
				t.Fatalf("Open elapsed=%s store=%v err=%v", time.Since(start), s, err)
			}
			t.Logf("Open %s elapsed=%s err=%v", name, time.Since(start), err)
			release()
			assertWALRecovery(t, cfg, epoch)
		})
	}
}

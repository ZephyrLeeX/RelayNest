package storage

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func gateStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(context.Background(), Config{Path: path, BusyTimeout: 50 * time.Millisecond})
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

// 保持真实写事务未提交，让排队、取消、快照与独立数据库行为都能被观察。
func holdGateWrite(t *testing.T, s *Store) func() {
	t.Helper()
	entered, release, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		result <- s.Write(context.Background(), func(tx *sql.Tx) error {
			if _, err := tx.Exec("UPDATE system_state SET config_version=config_version+1"); err != nil {
				return err
			}
			close(entered)
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case err := <-result:
		t.Fatalf("holder failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("holder did not start")
	}
	var once sync.Once
	finish := func() {
		once.Do(func() {
			close(release)
			if err := <-result; err != nil {
				t.Errorf("held write: %v", err)
			}
		})
	}
	t.Cleanup(finish)
	return finish
}

func TestWriteGateAliasesReadsAndIndependentDatabase(t *testing.T) {
	for _, alias := range []string{"same", "symlink", "hardlink"} {
		t.Run(alias, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.sqlite")
			a := gateStore(t, path)
			otherPath := path
			if alias != "same" {
				otherPath = filepath.Join(filepath.Dir(path), alias+".sqlite")
				var err error
				if alias == "symlink" {
					err = os.Symlink(path, otherPath)
				} else {
					err = os.Link(path, otherPath)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			b := gateStore(t, otherPath)
			if a.gate != b.gate || a.gate.path != b.gate.path {
				t.Fatal("same file split SQLite/WAL path")
			}
			independent := gateStore(t, filepath.Join(t.TempDir(), "other.sqlite"))
			finish := holdGateWrite(t, a)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var version int
			if err := b.Read(ctx, func(tx *sql.Tx) error {
				return tx.QueryRowContext(ctx, "SELECT config_version FROM system_state").Scan(&version)
			}); err != nil || version != 1 {
				t.Fatalf("reader blocked or saw uncommitted state: %d %v", version, err)
			}
			if err := independent.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec("UPDATE system_state SET config_version=2"); return err }); err != nil {
				t.Fatalf("other DB serialized: %v", err)
			}
			// 等待超过SQLite原50ms仍由调用方context取消，且回调没有进入。
			queued, cancelQueued := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancelQueued()
			if err := b.Write(queued, func(*sql.Tx) error { t.Error("canceled callback ran"); return nil }); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("queued cancellation: %v", err)
			}
			for _, opener := range []func(context.Context, Config) (*Store, error){Open, OpenOffline} {
				openCtx, cancelOpen := context.WithTimeout(context.Background(), 10*time.Millisecond)
				opened, err := opener(openCtx, Config{Path: otherPath, BusyTimeout: 50 * time.Millisecond})
				cancelOpen()
				if opened != nil || !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("open bypassed holder: %v %v", opened, err)
				}
			}
			finish()
			if err := b.Write(context.Background(), func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE system_state SET config_version=config_version+1")
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if err := b.reader.QueryRow("SELECT config_version FROM system_state").Scan(&version); err != nil || version != 3 {
				t.Fatalf("alias lost commit: %d %v", version, err)
			}
		})
	}
}

func TestWriteGateSymlinkRetargetDoesNotPoisonOtherStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	a := gateStore(t, path)
	other := gateStore(t, filepath.Join(t.TempDir(), "other.sqlite"))
	alias := filepath.Join(filepath.Dir(path), "alias.sqlite")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	b := gateStore(t, alias)
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other.path, alias); err != nil {
		t.Fatal(err)
	}
	for _, op := range []func(context.Context, func(*sql.Tx) error) error{b.Read, b.Write} {
		if err := op(context.Background(), func(*sql.Tx) error { t.Error("retarget callback ran"); return nil }); !errors.Is(err, ErrInstanceUnavailable) {
			t.Fatalf("retarget accepted: %v", err)
		}
	}
	if err := a.Write(context.Background(), func(*sql.Tx) error { return nil }); err != nil {
		t.Fatalf("bad alias blocked good Store: %v", err)
	}
}

func TestWriteGateFileReplacementFailsClosed(t *testing.T) {
	for _, when := range []string{"before", "callback"} {
		t.Run(when, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.sqlite")
			s := gateStore(t, path)
			replace := func() error {
				if err := os.Rename(path, path+".moved"); err != nil {
					return err
				}
				return os.WriteFile(path, nil, 0600)
			}
			if when == "before" {
				if err := replace(); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			err := s.Write(context.Background(), func(tx *sql.Tx) error {
				calls++
				if _, err := tx.Exec("UPDATE system_state SET config_version=7"); err != nil {
					return err
				}
				return replace()
			})
			want := 0
			if when == "callback" {
				want = 1
			}
			if !errors.Is(err, ErrInstanceUnavailable) || calls != want {
				t.Fatalf("replacement boundary calls=%d err=%v", calls, err)
			}
			if err := s.Read(context.Background(), func(*sql.Tx) error { t.Error("replacement read ran"); return nil }); !errors.Is(err, ErrInstanceUnavailable) {
				t.Fatalf("replacement read: %v", err)
			}
			for _, p := range []string{path, path + ".moved"} {
				if opened, err := Open(context.Background(), Config{Path: p}); opened != nil || !errors.Is(err, ErrInstanceUnavailable) {
					t.Fatalf("rebound/replacement pool: %v %v", opened, err)
				}
			}
		})
	}
}

func TestWriteGateCloseAndReferenceLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	a, b := gateStore(t, path), gateStore(t, path)
	finish := holdGateWrite(t, a)
	waiting := make(chan error, 1)
	go func() {
		waiting <- b.Write(context.Background(), func(*sql.Tx) error { t.Error("closed callback ran"); return nil })
	}()
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-waiting; !errors.Is(err, errStoreClosed) {
		t.Fatalf("closed waiter: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if opened, err := Open(ctx, Config{Path: path}); opened != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("remaining holder bypassed: %v %v", opened, err)
	}
	finish()
	gate := a.gate
	c := gateStore(t, path)
	if c.gate != gate {
		t.Fatal("live reference detached")
	}
	finishC := holdGateWrite(t, c)
	closed := make(chan error, 1)
	go func() { closed <- c.Close() }()
	<-c.lifecycle.closedCh
	select {
	case err := <-closed:
		t.Fatalf("Close ignored active transaction: %v", err)
	default:
	}
	finishC()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	writeGates.Lock()
	for _, entry := range writeGates.entries {
		if entry == gate {
			t.Error("last reference leaked")
		}
	}
	writeGates.Unlock()
	if _, err := gate.file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("anchor not closed: %v", err)
	}
	d := gateStore(t, path)
	if d.gate == gate {
		t.Fatal("closed gate reused")
	}
}

func TestWriteGateErrorsDoNotReplayAndAlwaysRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	a, b := gateStore(t, path), gateStore(t, path)
	ctx := context.Background()
	if err := a.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE gate_parent(id INTEGER PRIMARY KEY); CREATE TABLE gate_child(id INTEGER REFERENCES gate_parent(id) DEFERRABLE INITIALLY DEFERRED)")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	abort := errors.New("injected callback failure")
	for _, stage := range []string{"callback", "commit", "panic", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			callCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			var err error
			panicked := false
			func() {
				defer func() {
					if got := recover(); got != nil {
						panicked = true
						if stage != "panic" || got != abort {
							t.Fatalf("unexpected panic: %v", got)
						}
					}
				}()
				err = a.Write(callCtx, func(tx *sql.Tx) error {
					calls++
					if _, err := tx.Exec("INSERT INTO gate_child VALUES(1)"); err != nil {
						return err
					}
					switch stage {
					case "callback":
						return abort
					case "panic":
						panic(abort)
					case "cancel":
						cancel()
					}
					return nil
				})
			}()
			if calls != 1 || (stage == "panic" && !panicked) || (stage != "panic" && err == nil) || (stage == "callback" && !errors.Is(err, abort)) {
				t.Fatalf("replayed/swallowed: calls=%d panic=%v err=%v", calls, panicked, err)
			}
			if err := b.Write(ctx, func(tx *sql.Tx) error {
				var n int
				if err := tx.QueryRow("SELECT count(*) FROM gate_child").Scan(&n); err != nil {
					return err
				}
				if n != 0 {
					return fmt.Errorf("partial commit: %d", n)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWriteGateConcurrentStartupAndFailureCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "startup.sqlite")
	var wg sync.WaitGroup
	stores := make(chan *Store, 8)
	for i := 0; i < cap(stores); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(context.Background(), Config{Path: path, BusyTimeout: 50 * time.Millisecond})
			if err != nil {
				t.Error(err)
				return
			}
			stores <- s
		}()
	}
	wg.Wait()
	close(stores)
	var first *writeGate
	for s := range stores {
		if first == nil {
			first = s.gate
		}
		if first != s.gate {
			t.Error("startup split gate")
		}
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}
	badPath := filepath.Join(t.TempDir(), "bad.sqlite")
	if err := os.WriteFile(badPath, []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, opener := range []func(context.Context, Config) (*Store, error){Open, OpenOffline} {
		if s, err := opener(context.Background(), Config{Path: badPath}); s != nil || err == nil {
			t.Fatalf("corrupt accepted: %v %v", s, err)
		}
		writeGates.Lock()
		for _, gate := range writeGates.entries {
			if gate.path == badPath {
				t.Error("failed Open leaked gate")
			}
		}
		writeGates.Unlock()
	}
}

// 子进程没有父进程注册表，直接SQLite事务确实持有进程外文件写锁。
func TestWriteGateExternalHolder(t *testing.T) {
	path := os.Getenv("RELAYNEST_WRITE_GATE_HOLDER_PATH")
	if path == "" {
		return
	}
	db, err := openPool(path, Config{BusyTimeout: 50 * time.Millisecond}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	fmt.Fprintln(os.Stdout, "READY")
	var one [1]byte
	if _, err := os.Stdin.Read(one[:]); err != nil {
		t.Fatal(err)
	}
}

func TestWriteGateExternalLockKeepsSQLiteBusyBudget(t *testing.T) {
	s := gateStore(t, filepath.Join(t.TempDir(), "external.sqlite"))
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestWriteGateExternalHolder$")
	cmd.Env = append(os.Environ(), "RELAYNEST_WRITE_GATE_HOLDER_PATH="+s.path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = stdin.Write([]byte{1})
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("holder: %v %s", err, stderr.String())
		}
	}()
	reader := bufio.NewScanner(stdout)
	if !reader.Scan() || reader.Text() != "READY" {
		t.Fatalf("holder not ready: %s %s", reader.Text(), stderr.String())
	}
	var budget int
	if err := s.writer.QueryRow("PRAGMA busy_timeout").Scan(&budget); err != nil || budget != 50 {
		t.Fatalf("budget changed: %d %v", budget, err)
	}
	called := false
	start := time.Now()
	err = s.Write(context.Background(), func(*sql.Tx) error { called = true; return nil })
	elapsed := time.Since(start)
	var sqliteErr *sqlite.Error
	if called || !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_BUSY || elapsed > 300*time.Millisecond {
		t.Fatalf("external busy hidden/retried: called=%v elapsed=%v err=%v", called, elapsed, err)
	}
}

// 只向 SQLite 注册一个常驻分发器；每例故障按临时库规范路径隔离并在
// cleanup 删除，不随测试次数累积驱动钩子，也不捕获已结束用例的 t。
var initializationFaults struct {
	once sync.Once
	sync.Mutex
	byPath map[string]*initializationFault
}

type initializationFault struct {
	once   sync.Once
	fired  atomic.Bool
	mutate func() error
	err    error
}

func injectInitializationFault(t *testing.T, path string, mutate func() error) *initializationFault {
	t.Helper()
	initializationFaults.once.Do(func() {
		sqlite.RegisterConnectionHook(func(_ sqlite.ExecQuerierContext, dsn string) error {
			u, err := url.Parse(dsn)
			if err != nil {
				return nil
			}
			initializationFaults.Lock()
			fault := initializationFaults.byPath[u.Path]
			initializationFaults.Unlock()
			if fault == nil {
				return nil
			}
			fault.once.Do(func() { fault.err = fault.mutate(); fault.fired.Store(true) })
			return fault.err
		})
	})
	fault := &initializationFault{mutate: mutate}
	initializationFaults.Lock()
	if initializationFaults.byPath == nil {
		initializationFaults.byPath = make(map[string]*initializationFault)
	}
	if initializationFaults.byPath[path] != nil {
		initializationFaults.Unlock()
		t.Fatal("duplicate initialization fault path")
	}
	initializationFaults.byPath[path] = fault
	initializationFaults.Unlock()
	t.Cleanup(func() {
		initializationFaults.Lock()
		delete(initializationFaults.byPath, path)
		initializationFaults.Unlock()
	})
	return fault
}

func TestWriteGateInitializationPathChangeFailsClosed(t *testing.T) {
	for _, offline := range []bool{false, true} {
		for _, change := range []string{"symlink", "replacement"} {
			t.Run(fmt.Sprintf("offline=%v/%s", offline, change), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "original.sqlite")
				otherPath := filepath.Join(t.TempDir(), "other.sqlite")
				original, other := gateStore(t, path), gateStore(t, otherPath)
				for store, value := range map[*Store]int{original: 101, other: 202} {
					if err := store.Write(context.Background(), func(tx *sql.Tx) error {
						_, err := tx.Exec("UPDATE system_state SET config_version=?", value)
						return err
					}); err != nil {
						t.Fatal(err)
					}
				}
				// 预先打开观察连接，使之后的读保持原文件句柄，不重新选路径。
				readMarker := func(store *Store) int {
					var n int
					if err := store.reader.QueryRow("SELECT config_version FROM system_state").Scan(&n); err != nil {
						t.Fatal(err)
					}
					return n
				}
				beforeOriginal, beforeOther := readMarker(original), readMarker(other)
				request := path
				if change == "symlink" {
					request = filepath.Join(filepath.Dir(path), "alias.sqlite")
					if err := os.Symlink(path, request); err != nil {
						t.Fatal(err)
					}
				}
				fault := injectInitializationFault(t, original.gate.path, func() error {
					if change == "symlink" {
						if err := os.Remove(request); err != nil {
							return err
						}
						return os.Symlink(otherPath, request)
					}
					if err := os.Rename(path, path+".moved"); err != nil {
						return err
					}
					return os.Link(otherPath, path)
				})
				// 故障后先恢复路径，再关闭原观察 Store；不把活库重命名当受支持维护。
				t.Cleanup(func() {
					if !fault.fired.Load() || fault.err != nil {
						return
					}
					if change == "symlink" {
						if err := os.Remove(request); err != nil {
							t.Error(err)
							return
						}
						if err := os.Symlink(path, request); err != nil {
							t.Error(err)
						}
					} else {
						if err := os.Remove(path); err != nil {
							t.Error(err)
							return
						}
						if err := os.Rename(path+".moved", path); err != nil {
							t.Error(err)
						}
					}
				})
				opener := Open
				if offline {
					opener = OpenOffline
				}
				candidate, err := opener(context.Background(), Config{Path: request, BusyTimeout: 50 * time.Millisecond})
				if candidate != nil {
					_ = candidate.Close()
				}
				if !fault.fired.Load() || fault.err != nil {
					t.Fatalf("fault did not run: %v %v", fault.fired.Load(), fault.err)
				}
				if candidate != nil || !errors.Is(err, ErrInstanceUnavailable) {
					t.Fatalf("published mismatched Store: %v %v", candidate, err)
				}
				if got := readMarker(original); got != beforeOriginal {
					t.Fatalf("original marker changed: %d->%d", beforeOriginal, got)
				}
				if got := readMarker(other); got != beforeOther {
					t.Fatalf("other marker changed: %d->%d", beforeOther, got)
				}
				writeGates.Lock()
				refs := original.gate.refs
				writeGates.Unlock()
				if refs != 1 {
					t.Fatalf("failed initialization retained %d references", refs)
				}
			})
		}
	}
}

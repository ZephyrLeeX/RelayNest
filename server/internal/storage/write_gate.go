package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// 锚定句柄防止仍有 Store 时 inode 被回收复用；SameFile 也覆盖硬链接。
// 注册表只保留存活 Store 的引用，不按路径永久缓存身份。
var writeGates struct {
	sync.Mutex
	entries []*writeGate
}

type writeGate struct {
	file   *os.File
	info   os.FileInfo
	path   string
	refs   int
	permit chan struct{}
}

type storeLifecycle struct {
	mu       sync.Mutex
	active   sync.WaitGroup
	once     sync.Once
	closed   bool
	closedCh chan struct{}
	closeErr error
}

var errStoreClosed = errors.New("storage: store closed")

func newStore() *Store {
	return &Store{lifecycle: storeLifecycle{closedCh: make(chan struct{})}}
}

func retainWriteGate(path string, create bool) (*writeGate, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*writeGate, error) { return nil, errors.Join(err, file.Close()) }
	info, err := file.Stat()
	if err != nil {
		return fail(err)
	}
	if !info.Mode().IsRegular() {
		return fail(ErrInstanceUnavailable)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fail(errors.Join(ErrInstanceUnavailable, err))
	}
	if err := sameDatabaseFile(info, canonical); err != nil {
		return fail(err)
	}

	writeGates.Lock()
	defer writeGates.Unlock()
	for _, gate := range writeGates.entries {
		if canonical == gate.path && !os.SameFile(info, gate.info) {
			// 旧池仍可能持有该名字的 WAL/SHM，不能为替换文件再建立第二个池组。
			return fail(ErrInstanceUnavailable)
		}
		if os.SameFile(info, gate.info) {
			// 硬链接必须共用一个 SQLite 路径，否则 WAL/SHM 会使用不同文件名。
			// 已失效的实际连接路径不自动迁往别名，避免旧、新文件混进同一池。
			if err := sameDatabaseFile(gate.info, gate.path); err != nil {
				return fail(err)
			}
			if err := file.Close(); err != nil {
				return nil, err
			}
			gate.refs++
			return gate, nil
		}
	}
	gate := &writeGate{file: file, info: info, path: canonical, refs: 1, permit: make(chan struct{}, 1)}
	gate.permit <- struct{}{}
	writeGates.entries = append(writeGates.entries, gate)
	return gate, nil
}

func (g *writeGate) releaseReference() error {
	writeGates.Lock()
	defer writeGates.Unlock()
	g.refs--
	if g.refs != 0 {
		return nil
	}
	for i, entry := range writeGates.entries {
		if entry == g {
			writeGates.entries = append(writeGates.entries[:i], writeGates.entries[i+1:]...)
			break
		}
	}
	return g.file.Close()
}

// 准入只协调当前进程，不替代 SQLite 外部锁或离线维护的进程排他锁。
// 等待受调用方 context 和该 Store 的 Close 控制，不重置 SQLite busy 预算。
func (g *writeGate) acquire(ctx context.Context, closed <-chan struct{}) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-closed:
		return nil, errStoreClosed
	default:
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-closed:
		return nil, errStoreClosed
	case <-g.permit:
	}
	release := func() { g.permit <- struct{}{} }
	// 多个就绪分支并发时仍优先保留取消/关闭，不让已取消回调进入事务。
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	select {
	case <-closed:
		release()
		return nil, errStoreClosed
	default:
		return release, nil
	}
}

func sameDatabaseFile(want os.FileInfo, path string) error {
	got, err := os.Stat(path)
	if err != nil {
		return errors.Join(ErrInstanceUnavailable, err)
	}
	if !os.SameFile(want, got) {
		return ErrInstanceUnavailable
	}
	return nil
}

func (s *Store) checkDatabaseFile() error {
	// 请求路径保留 symlink 本身，以便其重指向只使相应 Store 失败关闭。
	if err := sameDatabaseFile(s.gate.info, s.path); err != nil {
		return err
	}
	return sameDatabaseFile(s.gate.info, s.gate.path)
}

func (s *Store) beginOperation() (func(), error) {
	s.lifecycle.mu.Lock()
	defer s.lifecycle.mu.Unlock()
	if s.lifecycle.closed {
		return nil, errStoreClosed
	}
	s.lifecycle.active.Add(1)
	return s.lifecycle.active.Done, nil
}

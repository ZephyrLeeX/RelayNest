// Package storage 管理 SQLite 连接、启动迁移及短事务边界。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
	"relaynest/server/migrations"
)

// Config 仅接受本地文件路径，避免调用方注入驱动参数或意外使用内存库。
type Config struct {
	Path            string
	ReadConnections int
	BusyTimeout     time.Duration
}

// Store 的写池固定一个连接，读池有界且在连接级禁止写入。
// 调用方不得在事务中等待网络、长轮询或文件上传。
type Store struct {
	writer     *sql.DB
	reader     *sql.DB
	instanceID string
	gate       *writeGate
	path       string
	lifecycle  storeLifecycle
}

// Open 打开本地数据库并完成迁移。失败时关闭连接，保留原库供人工诊断。
func Open(ctx context.Context, cfg Config) (_ *Store, err error) {
	if cfg.Path == "" || cfg.Path == ":memory:" {
		return nil, errors.New("storage: require local database file path")
	}
	if cfg.ReadConnections == 0 {
		cfg.ReadConnections = 4
	}
	if cfg.ReadConnections < 1 || cfg.ReadConnections > 16 {
		return nil, errors.New("storage: read connections must be 1..16")
	}
	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = 5 * time.Second
	}
	if cfg.BusyTimeout < time.Millisecond || cfg.BusyTimeout > time.Minute {
		return nil, errors.New("storage: busy timeout must be 1ms..1m")
	}
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("storage path: %w", err)
	}
	s := newStore()
	s.path = path
	s.gate, err = retainWriteGate(path, true)
	if err != nil {
		return nil, err
	}
	release, err := s.gate.acquire(ctx, s.lifecycle.closedCh)
	if err != nil {
		return nil, errors.Join(err, s.Close())
	}
	// 失败清理必须先释放准入，再关闭并撤销引用，不能留下第二个活跃 gate。
	defer func() {
		release()
		if err != nil {
			err = errors.Join(err, s.Close())
		}
	}()
	if err = s.checkDatabaseFile(); err != nil {
		return nil, err
	}
	s.writer, err = openPool(s.gate.path, cfg, false)
	if err != nil {
		return nil, err
	}
	if err = s.writer.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("storage connect: %w", err)
	}
	// journal_mode 是文件持久设置；先开启 WAL，再允许迁移事务获取写锁。
	if err = enableWAL(ctx, s.writer, cfg.BusyTimeout); err != nil {
		return nil, fmt.Errorf("storage WAL: %w", err)
	}
	if err = migrate(ctx, s.writer, migrations.Files); err != nil {
		return nil, err
	}
	s.reader, err = openPool(s.gate.path, cfg, true)
	if err != nil {
		return nil, err
	}
	if err = s.reader.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("storage reader: %w", err)
	}
	if err = transact(ctx, s.reader, func(tx *sql.Tx) error {
		var readErr error
		s.instanceID, readErr = ReadInstanceIDTx(ctx, tx)
		return readErr
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func openPool(path string, cfg Config, readOnly bool) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout("+strconv.FormatInt(cfg.BusyTimeout.Milliseconds(), 10)+")")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Set("_txlock", "immediate")
	n := 1
	if readOnly {
		q.Set("mode", "ro")
		q.Set("_txlock", "deferred")
		q.Add("_pragma", "query_only(ON)")
		n = cfg.ReadConnections
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("storage pool: %w", err)
	}
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n)
	return db, nil
}

// Read 在只读事务中提供一致快照；回调不得保留事务或未关闭的 Rows。
func (s *Store) Read(ctx context.Context, fn func(*sql.Tx) error) error {
	done, err := s.beginOperation()
	if err != nil {
		return err
	}
	defer done()
	return s.transact(ctx, s.reader, fn)
}

// Write 将写操作合并到单个短事务，提交失败原样返回，不宣称写入成功。
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	done, err := s.beginOperation()
	if err != nil {
		return err
	}
	defer done()
	release, err := s.gate.acquire(ctx, s.lifecycle.closedCh)
	if err != nil {
		return err
	}
	defer release()
	return s.transact(ctx, s.writer, fn)
}

// 每次事务重新读取身份；启动值只用于比较，不能替代数据库读取。
// 离线换 ID 后必须重启 Store；仍运行的旧 Store 失败关闭。
func (s *Store) transact(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	if err := s.checkDatabaseFile(); err != nil {
		return err
	}
	return transact(ctx, db, func(tx *sql.Tx) error {
		id, err := ReadInstanceIDTx(ctx, tx)
		if err != nil {
			return err
		}
		if id != s.instanceID {
			return ErrInstanceUnavailable
		}
		if err := fn(tx); err != nil {
			return err
		}
		// 不能把请求期间替换/移走的文件当作仍绑定原部署的可提交结果。
		return s.checkDatabaseFile()
	})
}

func transact(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// EngineVersion 返回实际运行的 SQLite 引擎版本，供诊断与兼容性核对。
func (s *Store) EngineVersion(ctx context.Context) (string, error) {
	done, err := s.beginOperation()
	if err != nil {
		return "", err
	}
	defer done()
	if err := s.checkDatabaseFile(); err != nil {
		return "", err
	}
	var version string
	err = s.reader.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	return version, err
}

// Close 关闭两个连接池，等待已开始的查询结束。
func (s *Store) Close() error {
	s.lifecycle.once.Do(func() {
		s.lifecycle.mu.Lock()
		s.lifecycle.closed = true
		if s.lifecycle.closedCh != nil {
			close(s.lifecycle.closedCh)
		}
		s.lifecycle.mu.Unlock()
		// 停止新准入并唤醒本 Store 排队者；已运行事务结束后才能释放文件锚。
		s.lifecycle.active.Wait()
		var r, w, g error
		if s.reader != nil {
			r = s.reader.Close()
		}
		if s.writer != nil {
			w = s.writer.Close()
		}
		if s.gate != nil {
			g = s.gate.releaseReference()
		}
		s.lifecycle.closeErr = errors.Join(r, w, g)
	})
	return s.lifecycle.closeErr
}

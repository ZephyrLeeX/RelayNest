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
	writer *sql.DB
	reader *sql.DB
}

// Open 打开本地数据库并完成迁移。失败时关闭连接，保留原库供人工诊断。
func Open(ctx context.Context, cfg Config) (*Store, error) {
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
	s := &Store{}
	s.writer, err = openPool(path, cfg, false)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Store, error) { return nil, errors.Join(err, s.Close()) }
	if err = s.writer.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("storage connect: %w", err))
	}
	// journal_mode 是文件持久设置；先开启 WAL，再允许迁移事务获取写锁。
	var journal string
	if err = s.writer.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journal); err != nil {
		return fail(fmt.Errorf("storage WAL: %w", err))
	}
	if journal != "wal" {
		return fail(fmt.Errorf("storage: expected WAL, got %q", journal))
	}
	if err = migrate(ctx, s.writer, migrations.Files); err != nil {
		return fail(err)
	}
	s.reader, err = openPool(path, cfg, true)
	if err != nil {
		return fail(err)
	}
	if err = s.reader.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("storage reader: %w", err))
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
	return transact(ctx, s.reader, fn)
}

// Write 将写操作合并到单个短事务，提交失败原样返回，不宣称写入成功。
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	return transact(ctx, s.writer, fn)
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
	var version string
	err := s.reader.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	return version, err
}

// Close 关闭两个连接池，等待已开始的查询结束。
func (s *Store) Close() error {
	var r, w error
	if s.reader != nil {
		r = s.reader.Close()
	}
	if s.writer != nil {
		w = s.writer.Close()
	}
	return errors.Join(r, w)
}

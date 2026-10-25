package storage

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"relaynest/server/migrations"
)

// OpenOffline 只打开迁移完整的现存库；不初始化身份、不更换 journal 模式，
// 不调用正常启动迁移。调用方须另持进程排他锁并确认全部服务已经停机。
func OpenOffline(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.Path == "" || cfg.Path == ":memory:" {
		return nil, ErrInstanceUnavailable
	}
	if cfg.ReadConnections == 0 {
		cfg.ReadConnections = 1
	}
	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = time.Millisecond
	}
	if cfg.ReadConnections < 1 || cfg.ReadConnections > 16 || cfg.BusyTimeout < time.Millisecond || cfg.BusyTimeout > time.Minute {
		return nil, errors.New("storage: invalid offline configuration")
	}
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInstanceUnavailable
	}
	all, err := loadMigrations(migrations.Files)
	if err != nil {
		return nil, err
	}
	// mode=rw 防止检查文件后到连接建立间意外新建库；驱动参数不来自用户路径。
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{}
	q.Set("mode", "rw")
	q.Set("_txlock", "immediate")
	q.Add("_pragma", "busy_timeout("+strconv.FormatInt(cfg.BusyTimeout.Milliseconds(), 10)+")")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "synchronous(FULL)")
	u.RawQuery = q.Encode()
	s := &Store{}
	s.writer, err = sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	s.writer.SetMaxOpenConns(1)
	s.writer.SetMaxIdleConns(1)
	fail := func(err error) (*Store, error) { return nil, errors.Join(err, s.Close()) }
	if err = s.writer.PingContext(ctx); err != nil {
		return fail(err)
	}
	s.reader, err = openPool(path, cfg, true)
	if err != nil {
		return fail(err)
	}
	err = transact(ctx, s.reader, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT version,name,checksum FROM schema_migrations ORDER BY version")
		if err != nil {
			return errors.Join(ErrInstanceUnavailable, err)
		}
		applied := 0
		for rows.Next() {
			var version int
			var name, checksum string
			if err = rows.Scan(&version, &name, &checksum); err != nil {
				rows.Close()
				return errors.Join(ErrInstanceUnavailable, err)
			}
			if applied >= len(all) || version != applied+1 || name != all[applied].name || checksum != all[applied].checksum {
				rows.Close()
				return ErrInstanceUnavailable
			}
			applied++
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return err
		}
		if applied != len(all) {
			return ErrInstanceUnavailable
		}
		s.instanceID, err = ReadInstanceIDTx(ctx, tx)
		return err
	})
	if err != nil {
		return fail(err)
	}
	return s, nil
}

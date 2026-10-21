package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func enableWAL(ctx context.Context, db *sql.DB, timeout time.Duration) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// journal_mode 的锁升级可能绕过 SQLite busy handler。独占该物理连接，
	// 暂停驱动等待，由同一个总预算控制全部尝试，避免每次叠加完整超时。
	if _, err = conn.ExecContext(ctx, "PRAGMA busy_timeout=0"); err != nil {
		return err
	}
	defer func() {
		// 即使调用方取消，也要恢复连接设置；失败会由 Open 关闭整个池。
		_, restoreErr := conn.ExecContext(context.Background(), "PRAGMA busy_timeout="+strconv.FormatInt(timeout.Milliseconds(), 10))
		err = errors.Join(err, restoreErr)
	}()
	return waitForWAL(ctx, timeout, func(ctx context.Context) (string, error) {
		var journal string
		err := conn.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journal)
		return journal, err
	})
}

// query 只执行一次真实 WAL 设置；拆出等待边界便于观测实际锁竞争后的取消。
func waitForWAL(ctx context.Context, timeout time.Duration, query func(context.Context) (string, error)) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var lastBusy error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := waitCtx.Err(); err != nil {
			return errors.Join(err, lastBusy)
		}
		journal, err := query(waitCtx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if waitCtx.Err() != nil {
			return errors.Join(waitCtx.Err(), lastBusy, err)
		}
		if err == nil {
			if journal != "wal" {
				return fmt.Errorf("storage: expected WAL, got %q", journal)
			}
			return nil
		}
		var sqliteErr *sqlite.Error
		// 仅重试 SQLite 数值码 BUSY（包含扩展码）；LOCKED、I/O、损坏等直接返回。
		if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_BUSY {
			return err
		}
		lastBusy = err
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-waitCtx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
)

type migration struct{ name, sql, checksum string }

var migrationName = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

func loadMigrations(source fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, err
	}
	var all []migration
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "embed.go" {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("storage: invalid migration filename %q", entry.Name())
		}
		number, _ := strconv.Atoi(matches[1])
		if number != len(all)+1 {
			return nil, fmt.Errorf("storage: non-contiguous migration %q", entry.Name())
		}
		data, err := fs.ReadFile(source, entry.Name())
		if err != nil {
			return nil, err
		}
		all = append(all, migration{entry.Name(), string(data), fmt.Sprintf("%x", sha256.Sum256(data))})
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("storage: no migrations")
	}
	return all, nil
}

func migrate(ctx context.Context, db *sql.DB, source fs.FS) error {
	all, err := loadMigrations(source)
	if err != nil {
		return err
	}
	// 写池的 BEGIN IMMEDIATE 在校验前取得 SQLite 独占写入权；并发启动必须
	// 等前一个迁移提交/回滚后才重新读取台账。WAL 下读者仍可完成旧快照。
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage migration lock: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY CHECK (version >= 1),
        name TEXT NOT NULL,
        checksum TEXT NOT NULL,
        applied_at TEXT NOT NULL
    )`)
	if err != nil {
		return fmt.Errorf("storage migration ledger: %w", err)
	}
	rows, err := tx.QueryContext(ctx, "SELECT version, name, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return err
	}
	applied := 0
	for rows.Next() {
		var version int
		var name, checksum string
		if err = rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			return err
		}
		if version != applied+1 || version > len(all) {
			rows.Close()
			return fmt.Errorf("storage: incompatible migration version %d", version)
		}
		known := all[version-1]
		if name != known.name || checksum != known.checksum {
			rows.Close()
			return fmt.Errorf("storage: migration %d name/checksum mismatch", version)
		}
		applied++
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// 同一次启动的全部 DDL 和版本台账原子提交，任一步失败不留半次升级。
	for i := applied; i < len(all); i++ {
		m := all[i]
		if _, err = tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("storage migration %s: %w", m.name, err)
		}
		if m.name == instanceMigration {
			if _, err = InitInstanceIDTx(ctx, tx); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))", i+1, m.name, m.checksum); err != nil {
			return err
		}
	}
	// 已完成身份升级的库严格校验；缺失行不能视作首次安装。
	for _, m := range all {
		if m.name == instanceMigration {
			if _, err = ReadInstanceIDTx(ctx, tx); err != nil {
				return err
			}
			break
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("storage migration commit: %w", err)
	}
	return nil
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"relaynest/server/internal/storage"
)

var (
	ErrAlreadyInitialized  = errors.New("auth: administrator already initialized")
	ErrNotInitialized      = errors.New("auth: administrator not initialized")
	ErrGenerationExhausted = errors.New("auth: generation exhausted")
	ErrStorage             = errors.New("auth: storage operation failed")
)

// Administrator 的代次用于后续会话认证；用户名按原始文本、区分大小写保存。
type Administrator struct {
	Username       string
	PasswordHash   string
	AuthGeneration int64
}

func ReadAdministrator(ctx context.Context, store *storage.Store) (Administrator, error) {
	var admin Administrator
	err := store.Read(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT username, password_hash, auth_generation FROM administrator WHERE id=1").Scan(&admin.Username, &admin.PasswordHash, &admin.AuthGeneration)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Administrator{}, ErrNotInitialized
	}
	if err != nil {
		return Administrator{}, ErrStorage
	}
	return admin, nil
}

func InitAdministrator(ctx context.Context, store *storage.Store, username string, password []byte) error {
	if username == "" {
		return errors.New("auth: username required")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	err = store.Write(ctx, func(tx *sql.Tx) error {
		// 单写事务序列化检查与插入；DDL 仍独立阻止其他进程写入第二条记录。
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM administrator").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return ErrAlreadyInitialized
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO administrator(id,username,password_hash,auth_generation) VALUES(1,?,?,1)", username, hash)
		return err
	})
	if errors.Is(err, ErrAlreadyInitialized) {
		return ErrAlreadyInitialized
	}
	if err != nil {
		return ErrStorage
	}
	return nil
}

func ResetPassword(ctx context.Context, store *storage.Store, password []byte) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	err = store.Write(ctx, func(tx *sql.Tx) error {
		var generation int64
		if err := tx.QueryRowContext(ctx, "SELECT auth_generation FROM administrator WHERE id=1").Scan(&generation); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotInitialized
			}
			return err
		}
		// 先检查耗尽，避免 SQLite INTEGER 溢出转 REAL 或摘要发生部分更新。
		if generation == math.MaxInt64 {
			return ErrGenerationExhausted
		}
		_, err := tx.ExecContext(ctx, "UPDATE administrator SET password_hash=?,auth_generation=? WHERE id=1", hash, generation+1)
		return err
	})
	if errors.Is(err, ErrNotInitialized) || errors.Is(err, ErrGenerationExhausted) {
		return err
	}
	if err != nil {
		return ErrStorage
	}
	return nil
}

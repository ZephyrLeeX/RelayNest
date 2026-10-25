package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"regexp"
)

// ErrInstanceUnavailable 表示不能可靠读取部署身份；调用方必须失败关闭。
var ErrInstanceUnavailable = errors.New("storage: instance identity unavailable")

// ErrInstanceMismatch 表示事务中的部署身份与预期身份不一致。
var ErrInstanceMismatch = errors.New("storage: instance identity mismatch")

const instanceMigration = "0005_instance_identity.sql"

var instanceIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func newInstanceID() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", fmt.Errorf("storage instance random: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// ReadInstanceIDTx 严格读取当前事务快照，不接受缺失、额外行或格式损坏。
func ReadInstanceIDTx(ctx context.Context, tx *sql.Tx) (string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, instance_id FROM instance_identity")
	if err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	defer rows.Close()
	var identity string
	count := 0
	for rows.Next() {
		var key int
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return "", errors.Join(ErrInstanceUnavailable, err)
		}
		count++
		if count != 1 || key != 1 || !instanceIDPattern.MatchString(value) {
			return "", ErrInstanceUnavailable
		}
		identity = value
	}
	if err := rows.Err(); err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	if err := rows.Close(); err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	if count == 0 {
		return "", errors.Join(ErrInstanceUnavailable, sql.ErrNoRows)
	}
	return identity, nil
}

// InitInstanceIDTx 仅供首次编号迁移；完成台账存在时绝不重建丢失的身份。
// 返回值是候选值，外层提交成功前不得对外发布。
func InitInstanceIDTx(ctx context.Context, tx *sql.Tx) (string, error) {
	id, err := ReadInstanceIDTx(ctx, tx)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	var completed bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version >= 5)").Scan(&completed); err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	if completed {
		return "", ErrInstanceUnavailable
	}
	id, err = newInstanceID()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO instance_identity(id,instance_id) VALUES(1,?)", id); err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	return id, nil
}

// RotateInstanceIDTx 只替换身份，不提交事务；撤销会话等离线维护步骤由调用方
// 在同一事务完成，任一步失败必须整体回滚，提交后才可使用新身份。
func RotateInstanceIDTx(ctx context.Context, tx *sql.Tx, expectedID string) (string, error) {
	current, err := ReadInstanceIDTx(ctx, tx)
	if err != nil {
		return "", err
	}
	if current != expectedID {
		return "", ErrInstanceMismatch
	}
	id, err := newInstanceID()
	if err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, "UPDATE instance_identity SET instance_id=? WHERE id=1 AND instance_id=?", id, expectedID)
	if err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", errors.Join(ErrInstanceUnavailable, err)
	}
	if n != 1 {
		return "", ErrInstanceUnavailable
	}
	return id, nil
}

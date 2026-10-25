// Package idempotency 提供受信业务在同一写事务内使用的去重记录。
package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalid   = errors.New("idempotency: invalid record")
	ErrConflict  = errors.New("idempotency: conflicting request") // HTTP 映射 409 IDEMPOTENCY_CONFLICT。
	ErrExpired   = errors.New("idempotency: resource expired")    // HTTP 映射 410 RESOURCE_EXPIRED。
	ErrProtected = errors.New("idempotency: resource still exists")
)

type Scope struct{ Identity, Operation, Key string }
type Record struct {
	Digest     [32]byte
	MessageID  string
	DeliveryID *string
	Result     []byte
}

// ValidKey 接受标准 UUIDv4 大小写表示；持久化统一小写，避免大小写绕过去重。
func ValidKey(key string) bool {
	if len(key) != 36 || key[8] != '-' || key[13] != '-' || key[18] != '-' || key[23] != '-' || key[14] != '4' {
		return false
	}
	for n, c := range strings.ToLower(key) {
		if n == 8 || n == 13 || n == 18 || n == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return strings.ContainsRune("89ab", rune(strings.ToLower(key)[19]))
}
func (s Scope) valid() bool { return s.Identity != "" && s.Operation != "" && ValidKey(s.Key) }

// LookupTx 不代替当前授权。调用方必须先复核权限，再查重，错误必须回滚。
func LookupTx(ctx context.Context, tx *sql.Tx, scope Scope, digest [32]byte) (Record, bool, error) {
	if tx == nil || !scope.valid() {
		return Record{}, false, ErrInvalid
	}
	var r Record
	var stored []byte
	var expired sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT digest,message_id,delivery_id,result_json,expired_at FROM idempotency WHERE identity=? AND operation=? AND key=?`, scope.Identity, scope.Operation, strings.ToLower(scope.Key)).Scan(&stored, &r.MessageID, &r.DeliveryID, &r.Result, &expired)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	if string(stored) != string(digest[:]) {
		return Record{}, false, ErrConflict
	}
	if expired.Valid {
		return Record{}, false, ErrExpired
	}
	copy(r.Digest[:], stored)
	return r, true, nil
}

// InsertTx 只写创建候选的稳定快照；不执行提交、不发布候选、不保留 Authorization。
func InsertTx(ctx context.Context, tx *sql.Tx, scope Scope, r Record) error {
	if tx == nil || !scope.valid() || r.MessageID == "" || len(r.Result) == 0 {
		return ErrInvalid
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO idempotency(identity,operation,key,digest,message_id,delivery_id,result_json) VALUES(?,?,?,?,?,?,?)`, scope.Identity, scope.Operation, strings.ToLower(scope.Key), r.Digest[:], r.MessageID, r.DeliveryID, string(r.Result))
	return one(result, err)
}

// TombstoneTx 仅供未来清理在删除资源的同一事务内调用。保留最小无正文元数据，
// 不设置活记录 TTL；墓碑至少保留七天的调度由未来清理任务负责。
func TombstoneTx(ctx context.Context, tx *sql.Tx, scope Scope, now time.Time) error {
	if tx == nil || !scope.valid() || now.IsZero() {
		return ErrInvalid
	}
	var exists bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages m WHERE m.id=i.message_id) OR EXISTS(SELECT 1 FROM deliveries d WHERE d.delivery_id=i.delivery_id) FROM idempotency i WHERE identity=? AND operation=? AND key=?`, scope.Identity, scope.Operation, strings.ToLower(scope.Key)).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrProtected
	}
	result, err := tx.ExecContext(ctx, `UPDATE idempotency SET result_json=NULL,expired_at=? WHERE identity=? AND operation=? AND key=? AND expired_at IS NULL`, now.UTC().Format(time.RFC3339Nano), scope.Identity, scope.Operation, strings.ToLower(scope.Key))
	return one(result, err)
}
func one(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInvalid
	}
	return nil
}

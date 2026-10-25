package auth

import (
	"context"
	"database/sql"
	"errors"
	"math"
)

// RevokeInstanceSessionsTx 仅供停服换实例身份使用；调用方负责把身份变更与
// 本撤销操作放进同一 Store.Write，且在提交后关闭 Store、重启服务。
func RevokeInstanceSessionsTx(ctx context.Context, tx *sql.Tx) error {
	var generation int64
	err := tx.QueryRowContext(ctx, "SELECT auth_generation FROM administrator WHERE id=1").Scan(&generation)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// 尚未初始化管理员的完整数据库也可以恢复；此时没有代次需要推进。
	if err == nil {
		if generation == math.MaxInt64 {
			return ErrGenerationExhausted
		}
		if _, err := tx.ExecContext(ctx, "UPDATE administrator SET auth_generation=? WHERE id=1", generation+1); err != nil {
			return err
		}
	}
	// 删除所有状态、所有客户端的会话，避免恢复的旧撤销记录重新有效。
	_, err = tx.ExecContext(ctx, "DELETE FROM sessions")
	return err
}

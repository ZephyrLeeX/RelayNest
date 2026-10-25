package bots

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
)

// APIMatch 表示身份发现的分支归属。只有 APINoMatch 允许调用方尝试其他认证。
// 零值失败关闭；APIMatched 一经摘要核验成立，后续拒绝或故障都不得回退。
type APIMatch uint8

const (
	// APIUnavailable 表示尚未确认匹配且依赖失败；禁止尝试其他认证。
	APIUnavailable APIMatch = iota
	// APINoMatch 表示当前解析确定无法匹配、无 API 凭据或摘要不符。
	APINoMatch
	// APIMatched 表示摘要已匹配，包括随后停用拒绝或事务故障。
	APIMatched
)

// ErrAPIIdentityUnavailable 不携带存储错误或凭据；调用方须映射为固定 503 外壳。
var ErrAPIIdentityUnavailable = errors.New("bot API identity unavailable")

// DiscoverAPIIdentity 无需预知 Bot ID，只发现 API 凭据，不读取附件或认证管理员。
// 返回的 Identity 仍须经业务同事务 AuthorizeTx 复核；Download 能力不是附件资源授权。
// 调用方必须先检查 match，不能把任意认证错误当作 APINoMatch。
// 只有 err == nil 且 match == APIMatched 才可使用身份；匹配后的故障仍返回
// APIMatched 与 ErrAPIIdentityUnavailable，匹配后的停用返回 ErrForbidden。
func (r *Repository) DiscoverAPIIdentity(ctx context.Context, secret string) (Identity, APIMatch, error) {
	if ctx.Err() != nil {
		return Identity{}, APIUnavailable, ErrAPIIdentityUnavailable
	}
	id, tail, ok := strings.Cut(secret, ".")
	// 这里只解析当前内部发行格式，不把长度/形状作为稳定 wire 协议或身份凭证。
	if !ok || len(id) != 64 || len(tail) != 64 {
		return Identity{}, APINoMatch, nil
	}
	match := APIUnavailable
	var identity Identity
	var denied bool
	err := r.store.Read(ctx, func(tx *sql.Tx) error {
		var digest []byte
		var enabled bool
		var candidate Identity
		candidate.kind = APIToken
		candidate.credentialID = id
		err := tx.QueryRowContext(ctx, `SELECT c.bot_id,c.digest,c.generation,b.enabled,b.can_send,b.can_receive FROM bot_credentials c JOIN bots b ON b.id=c.bot_id WHERE c.kind=? AND c.credential_id=?`, APIToken, id).Scan(&candidate.botID, &digest, &candidate.generation, &enabled, &candidate.capabilities.Send, &candidate.capabilities.Receive)
		if errors.Is(err, sql.ErrNoRows) {
			match = APINoMatch
			return nil
		}
		if err != nil {
			return err
		}
		supplied := sha256.Sum256([]byte(secret))
		if subtle.ConstantTimeCompare(digest, supplied[:]) != 1 {
			match = APINoMatch
			return nil
		}
		// 必须先固定归属再判断停用；读事务提交失败也不能释放身份或回退管理员。
		match = APIMatched
		denied = !enabled
		identity = candidate
		return nil
	})
	if err != nil {
		if match != APIMatched {
			match = APIUnavailable
		}
		return Identity{}, match, ErrAPIIdentityUnavailable
	}
	if denied {
		return Identity{}, APIMatched, ErrForbidden
	}
	return identity, match, nil
}

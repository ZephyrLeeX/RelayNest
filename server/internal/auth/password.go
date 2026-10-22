// Package auth 提供唯一管理员与密码摘要基础，不处理会话或 HTTP。
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
)

const hashPrefix = "$argon2id$v=19$m=65536,t=3,p=1$"

// 限制校验并发，避免每次 64 MiB 内存开销被放大；登录限流由上层负责。
var verificationSlots = make(chan struct{}, 2)

// HashPassword 使用独立随机盐。调用方只持有摘要，不记录密码。
func HashPassword(password []byte) (string, error) {
	if len(password) == 0 {
		return "", errors.New("auth: password required")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("auth: salt generation failed")
	}
	key := argon2.IDKey(password, salt, 3, 64*1024, 1, 32)
	return hashPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

// VerifyPassword 仅接受本版本固定参数的 PHC 摘要，避免恶意参数造成资源放大。
// 后续需要调整算法参数时，应显式扩充受支持版本。
func VerifyPassword(password []byte, encoded string) bool {
	if len(password) == 0 || !strings.HasPrefix(encoded, hashPrefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, hashPrefix), "$")
	if len(parts) != 2 || len(parts[0]) != 22 || len(parts[1]) != 43 {
		return false
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[0])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[1])
	if err != nil || len(expected) != 32 {
		return false
	}
	verificationSlots <- struct{}{}
	defer func() { <-verificationSlots }()
	actual := argon2.IDKey(password, salt, 3, 64*1024, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

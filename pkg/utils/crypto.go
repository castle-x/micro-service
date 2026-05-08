package utils

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DefaultBcryptCost 控制 bcrypt 计算强度；10 是典型安全与性能平衡点。
const DefaultBcryptCost = 10

// HashPassword 对明文密码进行 bcrypt 散列；适合 IDP 用户密码存储。
// 失败返回空串与 error。
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", fmt.Errorf("utils: HashPassword empty password")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), DefaultBcryptCost)
	if err != nil {
		return "", fmt.Errorf("utils: HashPassword failed: %w", err)
	}
	return string(b), nil
}

// VerifyPassword 对比明文与散列，匹配返回 nil，否则返回 bcrypt 错误（可用 errors.Is 判断）。
func VerifyPassword(hashed, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
}

// RandomBytes 使用 crypto/rand 生成 n 字节安全随机数据。
func RandomBytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("utils: RandomBytes n must be > 0, got %d", n)
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("utils: RandomBytes failed: %w", err)
	}
	return buf, nil
}

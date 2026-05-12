package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
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

// EncryptAESGCM 使用 AES-256-GCM 加密 plaintext，key 必须为 32 字节。
// 返回 base64url 编码的 nonce+ciphertext。
func EncryptAESGCM(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("utils: EncryptAESGCM key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("utils: EncryptAESGCM new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("utils: EncryptAESGCM new GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("utils: EncryptAESGCM rand nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(sealed), nil
}

// DecryptAESGCM 解密 EncryptAESGCM 生成的密文，key 必须为 32 字节。
func DecryptAESGCM(key []byte, ciphertext string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("utils: DecryptAESGCM key must be 32 bytes, got %d", len(key))
	}
	data, err := base64.URLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("utils: DecryptAESGCM base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("utils: DecryptAESGCM new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("utils: DecryptAESGCM new GCM: %w", err)
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", fmt.Errorf("utils: DecryptAESGCM ciphertext too short")
	}
	plain, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("utils: DecryptAESGCM open: %w", err)
	}
	return string(plain), nil
}

package biz_test

import (
	"testing"

	"github.com/castlexu/micro-service/pkg/utils"
)

// TestEncryptDecryptRoundtrip 验证 AES-256-GCM 加解密往返一致。
func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := []byte("test-encrypt-key-32-bytes-padXXX")[:32]
	plain := "sk-test-api-key-12345"

	cipher, err := utils.EncryptAESGCM(key, plain)
	if err != nil {
		t.Fatalf("EncryptAESGCM failed: %v", err)
	}

	// 密文不应等于明文
	if cipher == plain {
		t.Error("ciphertext should not equal plaintext")
	}

	// 解密后应等于明文
	got, err := utils.DecryptAESGCM(key, cipher)
	if err != nil {
		t.Fatalf("DecryptAESGCM failed: %v", err)
	}
	if got != plain {
		t.Errorf("decrypted %q, want %q", got, plain)
	}
}

// TestEncryptNonDeterministic 相同明文每次加密结果不同（随机 nonce）。
func TestEncryptNonDeterministic(t *testing.T) {
	key := []byte("test-encrypt-key-32-bytes-padXXX")[:32]
	plain := "sk-test-api-key-12345"

	c1, _ := utils.EncryptAESGCM(key, plain)
	c2, _ := utils.EncryptAESGCM(key, plain)
	if c1 == c2 {
		t.Error("two encryptions of same plaintext should produce different ciphertexts (random nonce)")
	}
}

// TestEncryptEmptyPlaintext 空字符串加密返回非空密文（biz 层的 encryptAPIKey 处理 bypass，utils 层不处理）。
func TestEncryptEmptyPlaintext(t *testing.T) {
	key := []byte("test-encrypt-key-32-bytes-padXXX")[:32]
	// utils.EncryptAESGCM 对空字符串也正常加密（GCM 支持 0 字节 plaintext）
	c, err := utils.EncryptAESGCM(key, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 解密后应为空字符串
	plain, err := utils.DecryptAESGCM(key, c)
	if err != nil {
		t.Fatalf("DecryptAESGCM failed: %v", err)
	}
	if plain != "" {
		t.Errorf("expected empty string, got %q", plain)
	}
}

// TestDecryptWrongKey 用错误的 key 解密应返回错误。
func TestDecryptWrongKey(t *testing.T) {
	key1 := []byte("test-encrypt-key-32-bytes-padXXX")[:32]
	key2 := []byte("wrong-encrypt-key-32-bytes-padYY")[:32]

	cipher, _ := utils.EncryptAESGCM(key1, "secret")
	_, err := utils.DecryptAESGCM(key2, cipher)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

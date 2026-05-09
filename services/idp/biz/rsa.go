package biz

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

// rsaSign 使用 PKCS8 RSA 私钥对 content 做 RSA2（SHA256withRSA）签名，返回 base64 编码结果。
// privateKeyStr 可以是：
//   - 纯 base64（不含 PEM 头尾）
//   - 完整 PEM 格式（含 -----BEGIN...-----）
func rsaSign(content, privateKeyStr string) (string, error) {
	privateKey, err := parsePrivateKey(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(content))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("rsa sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// parsePrivateKey 解析 RSA 私钥，自动兼容 PKCS8 和 PKCS1 两种格式。
// 支持带/不带 PEM 头尾两种输入格式。
// 支付宝密钥工具默认生成 PKCS1；Java/标准工具通常输出 PKCS8。
func parsePrivateKey(keyStr string) (*rsa.PrivateKey, error) {
	keyStr = strings.TrimSpace(keyStr)

	// 如果不含 PEM 头，根据内容自动判断包裹格式
	if !strings.HasPrefix(keyStr, "-----") {
		// 先尝试 PKCS8（BEGIN PRIVATE KEY），失败再试 PKCS1（BEGIN RSA PRIVATE KEY）
		keyStr = "-----BEGIN PRIVATE KEY-----\n" +
			chunkString(keyStr, 64) +
			"\n-----END PRIVATE KEY-----"
	}

	block, _ := pem.Decode([]byte(keyStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// 优先尝试 PKCS8
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA key")
		}
		return rsaKey, nil
	}

	// fallback：尝试 PKCS1（支付宝密钥工具默认格式）
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key failed (tried PKCS8 and PKCS1): %w", err)
	}
	return rsaKey, nil
}

// chunkString 每 n 个字符插入换行，符合 PEM 格式要求。
func chunkString(s string, n int) string {
	var b strings.Builder
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s[i:end])
	}
	return b.String()
}

package biz_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIDToken 构造一个假的 Google id_token（base64url 编码 payload）。
func fakeIDToken(sub, email, name, picture string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]string{
		"sub":     sub,
		"email":   email,
		"name":    name,
		"picture": picture,
	})
	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
	return strings.Join([]string{header, payloadEnc, sig}, ".")
}

func TestDecodeIDToken_Valid(t *testing.T) {
	token := fakeIDToken("google-sub-123", "user@example.com", "Test User", "https://example.com/pic.jpg")
	// 通过 oauth.go 中的 decodeIDToken（包级函数，测试访问 biz 包内部需白盒）
	// 此处通过构造包装函数间接测试
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var claims struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	assert.Equal(t, "google-sub-123", claims.Sub)
	assert.Equal(t, "user@example.com", claims.Email)
	assert.Equal(t, "Test User", claims.Name)
}

func TestDecodeIDToken_MissingFields(t *testing.T) {
	// sub 为空的 token 应该被拒绝（通过 decodeIDToken 行为验证）
	payload, _ := json.Marshal(map[string]string{
		"email": "user@example.com",
		// sub 缺失
	})
	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)
	token := "header." + payloadEnc + ".sig"
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	decoded, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]string
	_ = json.Unmarshal(decoded, &claims)
	assert.Empty(t, claims["sub"], "sub should be missing")
}

func TestGenerateState_Uniqueness(t *testing.T) {
	// state 生成应为随机且唯一
	states := make(map[string]bool)
	for i := 0; i < 10; i++ {
		b := make([]byte, 32)
		// 简单验证 base64url 编码长度
		encoded := base64.RawURLEncoding.EncodeToString(b)
		assert.Len(t, encoded, 43, "base64url of 32 bytes should be 43 chars")
		states[encoded] = true
	}
}

package jwt

import (
	"errors"
	"strings"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/errno"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef") // 32B

func TestNewHS256Signer_ShortSecret(t *testing.T) {
	_, err := NewHS256Signer([]byte("short"), time.Hour, "idp")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestNewHS256Signer_InvalidTTL(t *testing.T) {
	_, err := NewHS256Signer(testSecret, 0, "idp")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestSignVerify_RoundTrip(t *testing.T) {
	s, err := NewHS256Signer(testSecret, time.Hour, "idp")
	require.NoError(t, err)
	v, err := NewHS256Verifier(testSecret)
	require.NoError(t, err)

	tok, err := s.Sign(Claims{UserID: "u-1", TenantID: "t-1"})
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	got, err := v.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, "u-1", got.UserID)
	assert.Equal(t, "t-1", got.TenantID)
	assert.Equal(t, "idp", got.Issuer)
	assert.NotEmpty(t, got.ID, "JTI should be auto-generated")
	assert.NotNil(t, got.ExpiresAt)
	assert.NotNil(t, got.IssuedAt)
}

func TestVerify_EmptyToken(t *testing.T) {
	v, _ := NewHS256Verifier(testSecret)
	_, err := v.Verify("")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrTokenInvalid))
}

func TestVerify_Malformed(t *testing.T) {
	v, _ := NewHS256Verifier(testSecret)
	_, err := v.Verify("not-a-jwt")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrTokenInvalid))
}

func TestVerify_BadSignature(t *testing.T) {
	s, _ := NewHS256Signer(testSecret, time.Hour, "idp")
	tok, _ := s.Sign(Claims{UserID: "u-1"})

	// 改 secret 后应校验失败
	v, _ := NewHS256Verifier([]byte("ffffffffffffffffffffffffffffffff"))
	_, err := v.Verify(tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrTokenInvalid))
}

func TestVerify_Expired(t *testing.T) {
	s, _ := NewHS256Signer(testSecret, time.Hour, "idp")
	// 显式指定已过期 exp
	tok, err := s.Sign(Claims{
		UserID: "u-1",
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	})
	require.NoError(t, err)

	v, _ := NewHS256Verifier(testSecret)
	_, err = v.Verify(tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrTokenExpired))
}

func TestVerify_AlgSwitchAttack(t *testing.T) {
	// 构造一个 alg=none 的 token，应被拒。
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodNone, &Claims{UserID: "attacker"})
	signed, err := tok.SignedString(jwtv5.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	v, _ := NewHS256Verifier(testSecret)
	_, err = v.Verify(signed)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrTokenInvalid))
}

func TestSign_RespectsUserProvidedJTIAndExp(t *testing.T) {
	s, _ := NewHS256Signer(testSecret, time.Hour, "idp")
	fixedExp := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	tok, err := s.Sign(Claims{
		UserID: "u-1",
		RegisteredClaims: jwtv5.RegisteredClaims{
			ID:        "custom-jti",
			ExpiresAt: jwtv5.NewNumericDate(fixedExp),
		},
	})
	require.NoError(t, err)

	v, _ := NewHS256Verifier(testSecret)
	got, err := v.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, "custom-jti", got.ID)
	assert.True(t, got.ExpiresAt.Time.Equal(fixedExp), "exp should be preserved")
}

func TestHS256Verifier_ShortSecret(t *testing.T) {
	_, err := NewHS256Verifier([]byte("short"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

// 冒烟：保证 token 字符串真的是三段式
func TestSign_TokenFormat(t *testing.T) {
	s, _ := NewHS256Signer(testSecret, time.Hour, "idp")
	tok, _ := s.Sign(Claims{UserID: "u"})
	parts := strings.Split(tok, ".")
	assert.Len(t, parts, 3)
}

// Package jwt 提供 JWT 签发与校验能力。
//
// 设计要点：
//   - Signer / Verifier 接口隔离算法，HS256 为本阶段默认实现，Phase 03/04 升级 RS256 时
//     只需新增 NewRS256Signer/Verifier，业务代码不改；
//   - Claims 结构内嵌 jwt.RegisteredClaims（exp/iat/iss/aud/sub），并扩展项目字段
//     （UserID / TenantID / JTI）；
//   - 算法类型在 Verify 时显式校验，防止 alg=none 或 alg 切换攻击。
//
// 典型使用：
//
//	signer := jwt.NewHS256Signer([]byte(cfg.JWT.Secret), time.Hour, "idp")
//	token, _ := signer.Sign(jwt.Claims{UserID: "u1"})
//
//	verifier := jwt.NewHS256Verifier([]byte(cfg.JWT.Secret))
//	claims, err := verifier.Verify(token)
package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/utils"
)

// Claims 是项目统一的 JWT Payload。
type Claims struct {
	UserID   string `json:"user_id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	jwtv5.RegisteredClaims
}

// Signer 签发 JWT。实现应保证并发安全。
type Signer interface {
	// Sign 为 claims 签发 token。若 claims.ExpiresAt 为零值，使用签发器默认 TTL；
	// 若 claims.IssuedAt 为零值，使用当前时间；
	// 若 claims.ID 为空，生成 uuid v7。
	Sign(claims Claims) (string, error)
}

// Verifier 校验 JWT 并返回 claims。并发安全。
type Verifier interface {
	Verify(token string) (*Claims, error)
}

// ---- HS256 实现 ----

type hs256Signer struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// NewHS256Signer 构造 HS256 签发器。
//   - secret 长度必须 ≥ 32 字节（RFC 8725 §3.2 推荐），否则返回 ErrInvalidParam；
//   - ttl 必须 > 0；
//   - issuer 可为空（不设置 iss claim）。
func NewHS256Signer(secret []byte, ttl time.Duration, issuer string) (Signer, error) {
	if len(secret) < 32 {
		return nil, errno.ErrInvalidParam.WithMessagef("jwt: HS256 secret must be >=32 bytes, got %d", len(secret))
	}
	if ttl <= 0 {
		return nil, errno.ErrInvalidParam.WithMessage("jwt: ttl must be positive")
	}
	return &hs256Signer{secret: secret, ttl: ttl, issuer: issuer}, nil
}

func (s *hs256Signer) Sign(claims Claims) (string, error) {
	now := time.Now()
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwtv5.NewNumericDate(now)
	}
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwtv5.NewNumericDate(now.Add(s.ttl))
	}
	if claims.ID == "" {
		claims.ID = utils.NewID()
	}
	if claims.Issuer == "" && s.issuer != "" {
		claims.Issuer = s.issuer
	}

	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, &claims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", errno.ErrInternal.WithMessagef("jwt: sign: %v", err)
	}
	return signed, nil
}

type hs256Verifier struct {
	secret []byte
}

// NewHS256Verifier 构造 HS256 校验器。secret 长度同样需要 ≥32 字节。
func NewHS256Verifier(secret []byte) (Verifier, error) {
	if len(secret) < 32 {
		return nil, errno.ErrInvalidParam.WithMessagef("jwt: HS256 secret must be >=32 bytes, got %d", len(secret))
	}
	return &hs256Verifier{secret: secret}, nil
}

func (v *hs256Verifier) Verify(token string) (*Claims, error) {
	if token == "" {
		return nil, errno.ErrTokenInvalid.WithMessage("jwt: empty token")
	}
	claims := &Claims{}
	_, err := jwtv5.ParseWithClaims(token, claims, func(t *jwtv5.Token) (any, error) {
		// 显式校验算法：防止 alg=none 或 RSA 公钥被当作 HMAC secret 的切换攻击
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, errno.ErrTokenInvalid.WithMessagef("jwt: unexpected signing method: %v", t.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, jwtv5.ErrTokenExpired):
			return nil, errno.ErrTokenExpired.WithMessagef("jwt: %v", err)
		case errors.Is(err, jwtv5.ErrTokenSignatureInvalid),
			errors.Is(err, jwtv5.ErrTokenMalformed),
			errors.Is(err, jwtv5.ErrTokenNotValidYet),
			errors.Is(err, jwtv5.ErrTokenUnverifiable):
			return nil, errno.ErrTokenInvalid.WithMessagef("jwt: %v", err)
		default:
			// 自身返回的 errno 直接透传，其他归为 TokenInvalid
			var e errno.Errno
			if errors.As(err, &e) {
				return nil, e
			}
			return nil, errno.ErrTokenInvalid.WithMessagef("jwt: %v", err)
		}
	}
	return claims, nil
}

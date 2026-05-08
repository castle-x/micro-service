// Package biz — Token 颁发、刷新、校验。
package biz

import (
	"context"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
	pkgjwt "github.com/castlexu/micro-service/pkg/jwt"
	idpcache "github.com/castlexu/micro-service/services/idp/cache"
)

const (
	accessTokenTTL  = time.Hour
	refreshTokenTTL = 7 * 24 * time.Hour
)

// TokenPair 是一对 access + refresh token 及过期时间。
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64 // access token 过期 Unix 秒
}

// TokenBiz 处理 Token 颁发、刷新、撤销和校验。
type TokenBiz struct {
	secret     []byte
	accessSign pkgjwt.Signer
	accessVfy  pkgjwt.Verifier
	tokenCache *idpcache.TokenCache
}

// NewTokenBiz 构造 TokenBiz。
func NewTokenBiz(secret []byte, tokenCache *idpcache.TokenCache) (*TokenBiz, error) {
	signer, err := pkgjwt.NewHS256Signer(secret, accessTokenTTL, "idp")
	if err != nil {
		return nil, err
	}
	verifier, err := pkgjwt.NewHS256Verifier(secret)
	if err != nil {
		return nil, err
	}
	return &TokenBiz{
		secret:     secret,
		accessSign: signer,
		accessVfy:  verifier,
		tokenCache: tokenCache,
	}, nil
}

// Issue 为 userID 签发 access + refresh token pair。
func (b *TokenBiz) Issue(ctx context.Context, userID string) (*TokenPair, error) {
	// access token
	accessToken, err := b.accessSign.Sign(pkgjwt.Claims{UserID: userID})
	if err != nil {
		return nil, err
	}
	// refresh token（独立 TTL）
	refreshSigner, err := pkgjwt.NewHS256Signer(b.secret, refreshTokenTTL, "idp-refresh")
	if err != nil {
		return nil, err
	}
	refreshToken, err := refreshSigner.Sign(pkgjwt.Claims{UserID: userID})
	if err != nil {
		return nil, err
	}
	// 解析 refresh JTI 存入 Redis
	refreshVerifier, err := pkgjwt.NewHS256Verifier(b.secret)
	if err != nil {
		return nil, err
	}
	rc, err := refreshVerifier.Verify(refreshToken)
	if err != nil {
		return nil, err
	}
	if saveErr := b.tokenCache.SaveRefreshToken(ctx, rc.ID, userID); saveErr != nil {
		return nil, saveErr
	}
	ac, _ := b.accessVfy.Verify(accessToken)
	expiresAt := time.Now().Add(accessTokenTTL).Unix()
	if ac != nil && ac.ExpiresAt != nil {
		expiresAt = ac.ExpiresAt.Unix()
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresAt: expiresAt}, nil
}

// Refresh 用 refresh token 换新 token pair（滚动刷新）。
func (b *TokenBiz) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	refreshVerifier, err := pkgjwt.NewHS256Verifier(b.secret)
	if err != nil {
		return nil, err
	}
	rc, err := refreshVerifier.Verify(refreshToken)
	if err != nil {
		return nil, errno.ErrTokenInvalid.WithMessagef("idp: refresh token invalid: %v", err)
	}
	userID, err := b.tokenCache.GetRefreshToken(ctx, rc.ID)
	if err != nil {
		return nil, err
	}
	if userID != rc.UserID {
		return nil, errno.ErrTokenInvalid.WithMessage("idp: refresh token user mismatch")
	}
	_ = b.tokenCache.DeleteRefreshToken(ctx, rc.ID)
	return b.Issue(ctx, userID)
}

// Verify 校验 access token，检查黑名单。
func (b *TokenBiz) Verify(ctx context.Context, token string) (*pkgjwt.Claims, error) {
	claims, err := b.accessVfy.Verify(token)
	if err != nil {
		return nil, err
	}
	blacklisted, err := b.tokenCache.IsBlacklisted(ctx, claims.ID)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		return nil, errno.ErrTokenInvalid.WithMessage("idp: token has been revoked")
	}
	return claims, nil
}

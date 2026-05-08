// Package cache 封装 idp 的 Redis 缓存访问。
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
)

const (
	refreshTokenTTL = 7 * 24 * time.Hour
	blacklistTTL    = 2 * time.Hour // 略大于 access token TTL
)

// TokenCache 管理 refresh token 存储和 access token 黑名单。
type TokenCache struct {
	client *pkgredis.Client
}

// NewTokenCache 构造 TokenCache。
func NewTokenCache(client *pkgredis.Client) *TokenCache {
	return &TokenCache{client: client}
}

// SaveRefreshToken 保存 refresh token → userID 映射，TTL 7 天。
func (c *TokenCache) SaveRefreshToken(ctx context.Context, jti, userID string) error {
	key := pkgredis.Key("idp", "refresh", jti)
	return c.client.Set(ctx, key, userID, refreshTokenTTL)
}

// GetRefreshToken 查 refresh token 对应的 userID，不存在返回 ErrCacheMiss。
func (c *TokenCache) GetRefreshToken(ctx context.Context, jti string) (string, error) {
	key := pkgredis.Key("idp", "refresh", jti)
	v, err := c.client.Get(ctx, key)
	if err != nil {
		if errors.Is(err, errno.ErrCacheMiss) {
			return "", errno.ErrTokenInvalid.WithMessage("idp: refresh token not found")
		}
		return "", err
	}
	return v, nil
}

// DeleteRefreshToken 撤销 refresh token（登出/刷新后旧 token 作废）。
func (c *TokenCache) DeleteRefreshToken(ctx context.Context, jti string) error {
	key := pkgredis.Key("idp", "refresh", jti)
	_, err := c.client.Del(ctx, key)
	return err
}

// BlacklistAccessToken 将 access token 的 JTI 加入黑名单，remainTTL 为剩余有效期。
func (c *TokenCache) BlacklistAccessToken(ctx context.Context, jti string, remainTTL time.Duration) error {
	if remainTTL <= 0 {
		return nil // 已过期，无需加黑名单
	}
	key := pkgredis.Key("idp", "blacklist", jti)
	return c.client.Set(ctx, key, "1", remainTTL)
}

// IsBlacklisted 检查 access token JTI 是否在黑名单中。
func (c *TokenCache) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := pkgredis.Key("idp", "blacklist", jti)
	_, err := c.client.Get(ctx, key)
	if err != nil {
		if errors.Is(err, errno.ErrCacheMiss) {
			return false, nil
		}
		return false, fmt.Errorf("cache: is blacklisted: %w", err)
	}
	return true, nil
}

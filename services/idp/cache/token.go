// Package cache 封装 idp 的 Redis 缓存访问。
package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// SaveRefreshToken 保存 refresh token → "userID|role" 映射，TTL 7 天。
// 同时维护 user → JTI 反向索引集合，用于批量撤销。
func (c *TokenCache) SaveRefreshToken(ctx context.Context, jti, userID, role string) error {
	key := pkgredis.Key("idp", "refresh", jti)
	if err := c.client.Set(ctx, key, userID+"|"+role, refreshTokenTTL); err != nil {
		return err
	}
	indexKey := pkgredis.Key("idp", "user", "refresh", userID)
	return c.client.SAdd(ctx, indexKey, jti, refreshTokenTTL)
}

// GetRefreshToken 查 refresh token 对应的 userID 和 role，不存在返回 ErrTokenInvalid。
func (c *TokenCache) GetRefreshToken(ctx context.Context, jti string) (userID, role string, err error) {
	key := pkgredis.Key("idp", "refresh", jti)
	v, getErr := c.client.Get(ctx, key)
	if getErr != nil {
		if errors.Is(getErr, errno.ErrCacheMiss) {
			return "", "", errno.ErrTokenInvalid.WithMessage("idp: refresh token not found")
		}
		return "", "", getErr
	}
	parts := strings.SplitN(v, "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return v, "", nil // 兼容旧格式
}

// DeleteRefreshToken 撤销 refresh token（登出/刷新后旧 token 作废）。
func (c *TokenCache) DeleteRefreshToken(ctx context.Context, jti string) error {
	key := pkgredis.Key("idp", "refresh", jti)
	_, err := c.client.Del(ctx, key)
	return err
}

// RevokeAllUserTokens 撤销指定用户的所有 refresh token。
// 管理员修改用户角色后调用，用户下次刷新时会被踢出，强制重新登录获取新 role。
func (c *TokenCache) RevokeAllUserTokens(ctx context.Context, userID string) error {
	indexKey := pkgredis.Key("idp", "user", "refresh", userID)
	jtis, err := c.client.SMembers(ctx, indexKey)
	if err != nil {
		return err
	}
	for _, jti := range jtis {
		_, _ = c.client.Del(ctx, pkgredis.Key("idp", "refresh", jti))
	}
	_, _ = c.client.Del(ctx, indexKey)
	return nil
}

// ---- 封禁标记 ----

// BanUser 在 Redis 写入封禁标记，永久有效。
func (c *TokenCache) BanUser(ctx context.Context, userID string) error {
	key := pkgredis.Key("idp", "banned", userID)
	return c.client.Set(ctx, key, "1", 0) // TTL=0 永久
}

// UnbanUser 移除封禁标记。
func (c *TokenCache) UnbanUser(ctx context.Context, userID string) error {
	_, err := c.client.Del(ctx, pkgredis.Key("idp", "banned", userID))
	return err
}

// IsBanned 检查用户是否被封禁。
func (c *TokenCache) IsBanned(ctx context.Context, userID string) (bool, error) {
	_, err := c.client.Get(ctx, pkgredis.Key("idp", "banned", userID))
	if err != nil {
		if errors.Is(err, errno.ErrCacheMiss) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (c *TokenCache) BlacklistAccessToken(ctx context.Context, jti string, remainTTL time.Duration) error {
	if remainTTL <= 0 {
		return nil
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

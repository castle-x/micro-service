package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/errno"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	idpcache "github.com/castlexu/micro-service/services/idp/cache"
)

func setup(t *testing.T) (*idpcache.TokenCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redisv9.NewClient(&redisv9.Options{Addr: mr.Addr()})
	pkgredis.InitWithClient(rdb)
	t.Cleanup(func() { _ = pkgredis.Close() })
	return idpcache.NewTokenCache(pkgredis.GetClient()), mr
}

func TestTokenCache_SaveAndGet(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	err := c.SaveRefreshToken(ctx, "jti-001", "user-123")
	require.NoError(t, err)

	userID, err := c.GetRefreshToken(ctx, "jti-001")
	require.NoError(t, err)
	assert.Equal(t, "user-123", userID)
}

func TestTokenCache_GetNotFound(t *testing.T) {
	c, _ := setup(t)
	_, err := c.GetRefreshToken(context.Background(), "nonexistent")
	require.Error(t, err)
	var e errno.Errno
	assert.ErrorAs(t, err, &e)
}

func TestTokenCache_DeleteRefreshToken(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	require.NoError(t, c.SaveRefreshToken(ctx, "jti-002", "user-456"))
	require.NoError(t, c.DeleteRefreshToken(ctx, "jti-002"))

	_, err := c.GetRefreshToken(ctx, "jti-002")
	require.Error(t, err, "token should be gone after delete")
}

func TestTokenCache_Blacklist(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	// 初始不在黑名单
	blacklisted, err := c.IsBlacklisted(ctx, "jti-003")
	require.NoError(t, err)
	assert.False(t, blacklisted)

	// 加入黑名单
	require.NoError(t, c.BlacklistAccessToken(ctx, "jti-003", time.Hour))

	blacklisted, err = c.IsBlacklisted(ctx, "jti-003")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestTokenCache_Blacklist_ZeroTTL(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	// TTL <= 0 时跳过加入黑名单
	err := c.BlacklistAccessToken(ctx, "jti-004", 0)
	require.NoError(t, err)

	blacklisted, err := c.IsBlacklisted(ctx, "jti-004")
	require.NoError(t, err)
	assert.False(t, blacklisted, "zero TTL should not add to blacklist")
}

func TestTokenCache_Blacklist_Expiry(t *testing.T) {
	c, mr := setup(t)
	ctx := context.Background()

	require.NoError(t, c.BlacklistAccessToken(ctx, "jti-005", 1*time.Second))

	// 快进 miniredis 时间 2 秒
	mr.FastForward(2 * time.Second)

	blacklisted, err := c.IsBlacklisted(ctx, "jti-005")
	require.NoError(t, err)
	assert.False(t, blacklisted, "should expire after TTL")
}

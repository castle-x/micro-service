package biz_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	idpbiz "github.com/castlexu/micro-service/services/idp/biz"
	idpcache "github.com/castlexu/micro-service/services/idp/cache"
)

var testSecret = []byte("test-secret-must-be-32-bytes-long!!")

func newTokenBiz(t *testing.T) (*idpbiz.TokenBiz, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redisv9.NewClient(&redisv9.Options{Addr: mr.Addr()})
	pkgredis.InitWithClient(rdb)
	t.Cleanup(func() { _ = pkgredis.Close() })

	cache := idpcache.NewTokenCache(pkgredis.GetClient())
	biz, err := idpbiz.NewTokenBiz(testSecret, cache)
	require.NoError(t, err)
	return biz, mr
}

func TestTokenBiz_Issue_Verify(t *testing.T) {
	biz, _ := newTokenBiz(t)
	ctx := context.Background()

	pair, err := biz.Issue(ctx, "user-123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Greater(t, pair.ExpiresAt, time.Now().Unix())

	// 校验 access token
	claims, err := biz.Verify(ctx, pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
}

func TestTokenBiz_Verify_EmptyToken(t *testing.T) {
	biz, _ := newTokenBiz(t)
	_, err := biz.Verify(context.Background(), "")
	require.Error(t, err)
}

func TestTokenBiz_Verify_InvalidToken(t *testing.T) {
	biz, _ := newTokenBiz(t)
	_, err := biz.Verify(context.Background(), "not.a.jwt")
	require.Error(t, err)
}

func TestTokenBiz_Refresh(t *testing.T) {
	biz, _ := newTokenBiz(t)
	ctx := context.Background()

	pair, err := biz.Issue(ctx, "user-456")
	require.NoError(t, err)

	// 用 refresh token 换新 pair
	newPair, err := biz.Refresh(ctx, pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newPair.AccessToken)
	assert.NotEqual(t, pair.AccessToken, newPair.AccessToken, "new access token should differ")

	// 旧 refresh token 应已失效（Redis 已删）
	_, err = biz.Refresh(ctx, pair.RefreshToken)
	require.Error(t, err, "old refresh token should be invalidated after use")
}

func TestTokenBiz_NewTokenBiz_ShortSecret(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redisv9.NewClient(&redisv9.Options{Addr: mr.Addr()})
	pkgredis.InitWithClient(rdb)
	defer func() { _ = pkgredis.Close() }()

	cache := idpcache.NewTokenCache(pkgredis.GetClient())
	_, err := idpbiz.NewTokenBiz([]byte("short"), cache)
	require.Error(t, err, "should fail with secret < 32 bytes")
}

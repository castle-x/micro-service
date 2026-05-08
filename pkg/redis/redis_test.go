package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/errno"
)

// setup 启动 miniredis 并返回一个 Client（注册为全局单例）。
func setup(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redisv9.NewClient(&redisv9.Options{Addr: mr.Addr()})
	InitWithClient(rdb)
	t.Cleanup(func() {
		_ = Close()
	})
	return GetClient(), mr
}

func TestKey(t *testing.T) {
	assert.Equal(t, "idp:token:blacklist:abc", Key("idp", "token", "blacklist", "abc"))
	assert.Equal(t, "a:b", Key("a", "", "b"))
	assert.Equal(t, "", Key())
	assert.Equal(t, "", Key("", ""))
}

func TestInit_InvalidAddr(t *testing.T) {
	err := Init(Config{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestInit_PingFailure(t *testing.T) {
	// 指向一个肯定关闭的端口
	err := Init(Config{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrServiceUnavailable))
}

func TestSetGetDel(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "foo", "bar", 0))

	v, err := c.Get(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", v)

	n, err := c.Del(ctx, "foo")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
}

func TestGet_CacheMiss(t *testing.T) {
	c, _ := setup(t)
	_, err := c.Get(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrCacheMiss))
}

func TestSetNX(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	ok, err := c.SetNX(ctx, "k", "v1", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.SetNX(ctx, "k", "v2", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "second SetNX on same key should fail")
}

func TestObtainLock_Basic(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()

	l1, err := c.ObtainLock(ctx, "credits:lock:consume:o-1", 2*time.Second)
	require.NoError(t, err)
	require.NotNil(t, l1)

	// 第二次抢占同一 key 应失败 -> ErrRateLimit
	_, err = c.ObtainLock(ctx, "credits:lock:consume:o-1", 2*time.Second)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrRateLimit))

	require.NoError(t, l1.Release(ctx))

	// 释放后可再次抢占
	l2, err := c.ObtainLock(ctx, "credits:lock:consume:o-1", 2*time.Second)
	require.NoError(t, err)
	require.NoError(t, l2.Release(ctx))
}

func TestObtainLock_InvalidArgs(t *testing.T) {
	c, _ := setup(t)
	_, err := c.ObtainLock(context.Background(), "", time.Second)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))

	_, err = c.ObtainLock(context.Background(), "k", 0)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestObtainLock_NilClient(t *testing.T) {
	var c *Client
	_, err := c.ObtainLock(context.Background(), "k", time.Second)
	assert.True(t, errors.Is(err, errno.ErrServiceUnavailable))
}

func TestLock_ReleaseIdempotent(t *testing.T) {
	c, _ := setup(t)
	ctx := context.Background()
	l, err := c.ObtainLock(ctx, "k", 2*time.Second)
	require.NoError(t, err)
	require.NoError(t, l.Release(ctx))
	// 再次 release 应幂等
	require.NoError(t, l.Release(ctx))
}

func TestGetClient_NotInitialized(t *testing.T) {
	// 强制清空
	_ = Close()
	assert.Nil(t, GetClient())
}

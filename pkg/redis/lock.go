package redis

import (
	"context"
	"errors"
	"time"

	"github.com/bsm/redislock"

	"github.com/castlexu/micro-service/pkg/errno"
)

// Lock 是一次性获取的分布式锁句柄。获取失败时方法返回 (nil, err)。
// 持锁期间业务完成后必须调用 Release，或依赖 ttl 自动过期。
type Lock struct {
	inner *redislock.Lock
}

// LockOptions 控制锁行为。零值对应默认：LinearBackoff(100ms)，不重试（立即返回）。
type LockOptions struct {
	// RetryBackoff 持锁冲突时的重试间隔，0 表示不重试。
	RetryBackoff time.Duration
	// RetryCount 最大重试次数，仅 RetryBackoff > 0 时生效；0 表示不重试。
	RetryCount int
	// Metadata 可选，写入锁 value，便于排查（如调用方名）。
	Metadata string
}

// ObtainLock 抢占分布式锁。
//
//   - 抢占失败且为 redislock.ErrNotObtained 时，返回 (nil, errno.ErrRateLimit)；
//     业务可 errors.Is(err, errno.ErrRateLimit) 判定并做排队 / 降级。
//   - 其他错误包装为 errno.ErrInternal。
func (c *Client) ObtainLock(ctx context.Context, key string, ttl time.Duration, opts ...LockOptions) (*Lock, error) {
	ctx, end := startRedisOperation(ctx, "LOCK")
	var opErr error
	defer func() { end(opErr) }()

	if c == nil || c.locker == nil {
		opErr = errno.ErrServiceUnavailable.WithMessage("redis: client not initialized")
		return nil, opErr
	}
	if key == "" || ttl <= 0 {
		opErr = errno.ErrInvalidParam.WithMessage("redis: ObtainLock requires non-empty key and positive ttl")
		return nil, opErr
	}

	var o LockOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	lockOpts := &redislock.Options{}
	if o.RetryBackoff > 0 && o.RetryCount > 0 {
		lockOpts.RetryStrategy = redislock.LimitRetry(redislock.LinearBackoff(o.RetryBackoff), o.RetryCount)
	}
	if o.Metadata != "" {
		lockOpts.Metadata = o.Metadata
	}

	l, err := c.locker.Obtain(ctx, key, ttl, lockOpts)
	if err != nil {
		if errors.Is(err, redislock.ErrNotObtained) {
			opErr = errno.ErrRateLimit.WithMessagef("redis: lock held: %s", key)
			return nil, opErr
		}
		opErr = errno.ErrInternal.WithMessagef("redis: obtain lock %s: %v", key, err)
		return nil, opErr
	}
	return &Lock{inner: l}, nil
}

// Release 释放锁。重复释放或锁已过期返回 nil（幂等），其他错误包装为 ErrInternal。
func (l *Lock) Release(ctx context.Context) error {
	ctx, end := startRedisOperation(ctx, "UNLOCK")
	var opErr error
	defer func() { end(opErr) }()

	if l == nil || l.inner == nil {
		return nil
	}
	if err := l.inner.Release(ctx); err != nil {
		if errors.Is(err, redislock.ErrLockNotHeld) {
			return nil
		}
		opErr = errno.ErrInternal.WithMessagef("redis: release lock: %v", err)
		return opErr
	}
	return nil
}

// TTL 返回锁剩余有效时间。锁不存在或失败返回 0。
func (l *Lock) TTL(ctx context.Context) time.Duration {
	if l == nil || l.inner == nil {
		return 0
	}
	d, err := l.inner.TTL(ctx)
	if err != nil {
		return 0
	}
	return d
}

// Refresh 延长锁 TTL。通常在长任务中定期续期。
func (l *Lock) Refresh(ctx context.Context, ttl time.Duration) error {
	ctx, end := startRedisOperation(ctx, "LOCK_REFRESH")
	var opErr error
	defer func() { end(opErr) }()

	if l == nil || l.inner == nil {
		opErr = errno.ErrInvalidParam.WithMessage("redis: lock is nil")
		return opErr
	}
	if err := l.inner.Refresh(ctx, ttl, nil); err != nil {
		opErr = errno.ErrInternal.WithMessagef("redis: refresh lock: %v", err)
		return opErr
	}
	return nil
}

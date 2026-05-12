// Package redis 封装 go-redis v9 客户端与分布式锁。
//
// 设计要点（见 SPEC.md §8）：
//   - 单例 Client：进程内共享连接池，通过 Init(cfg) 初始化一次，GetClient() 获取；
//   - 键名规范统一：Key("idp", "token", "blacklist", jti) -> "idp:token:blacklist:<jti>"；
//   - 分布式锁基于 github.com/bsm/redislock，默认 LinearBackoff(100ms) 重试；
//   - 未初始化 GetClient() 返回 nil（不 panic），由调用方决定降级策略。
package redis

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/bsm/redislock"
	redisv9 "github.com/redis/go-redis/v9"

	"github.com/castlexu/micro-service/pkg/errno"
)

// Config 是 Redis 客户端配置。所有字段可通过 pkg/config 从 yaml + env 加载。
type Config struct {
	Addr         string        `mapstructure:"addr"`           // host:port
	Password     string        `mapstructure:"password"`       // 敏感，建议从 env 注入
	DB           int           `mapstructure:"db"`             // 库号，默认 0
	PoolSize     int           `mapstructure:"pool_size"`      // 连接池，默认 10*CPU（go-redis 内部默认）
	MinIdleConns int           `mapstructure:"min_idle_conns"` // 最小空闲连接
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`   // 建连超时，默认 5s
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`   // 读超时，默认 3s
	WriteTimeout time.Duration `mapstructure:"write_timeout"`  // 写超时，默认 3s
}

// Client 是对 *redisv9.Client 的薄封装 + 分布式锁 locker。
type Client struct {
	rdb    *redisv9.Client
	locker *redislock.Client
}

var (
	gClient *Client
	gMu     sync.RWMutex
)

// Init 初始化全局 Client 并 Ping 验证连接。重复调用以最后一次为准，老连接会被 Close。
//
// 典型用法：
//
//	if err := redis.Init(cfg); err != nil { log.Fatal(err) }
//	defer redis.Close()
func Init(cfg Config) error {
	if cfg.Addr == "" {
		return errno.ErrInvalidParam.WithMessage("redis: Addr is required")
	}
	opt := &redisv9.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	rdb := redisv9.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return errno.ErrServiceUnavailable.WithMessagef("redis: ping %s: %v", cfg.Addr, err)
	}

	c := &Client{
		rdb:    rdb,
		locker: redislock.New(rdb),
	}

	gMu.Lock()
	old := gClient
	gClient = c
	gMu.Unlock()
	if old != nil {
		_ = old.rdb.Close()
	}
	return nil
}

// InitWithClient 允许直接注入 *redisv9.Client（测试或自定义场景用）。
// 不做 Ping 校验；由调用方保证连通性。
func InitWithClient(rdb *redisv9.Client) {
	c := &Client{rdb: rdb, locker: redislock.New(rdb)}
	gMu.Lock()
	old := gClient
	gClient = c
	gMu.Unlock()
	if old != nil {
		_ = old.rdb.Close()
	}
}

// GetClient 返回全局 Client。未 Init 时返回 nil（调用方自行判空或降级）。
func GetClient() *Client {
	gMu.RLock()
	defer gMu.RUnlock()
	return gClient
}

// Close 关闭全局 Client，幂等。
func Close() error {
	gMu.Lock()
	defer gMu.Unlock()
	if gClient == nil {
		return nil
	}
	err := gClient.rdb.Close()
	gClient = nil
	return err
}

// Raw 返回底层 *redisv9.Client，便于调用 go-redis 原生 API（Pipeline / Script / PubSub 等）。
func (c *Client) Raw() *redisv9.Client { return c.rdb }

// Ping 探活。
func (c *Client) Ping(ctx context.Context) error {
	ctx, end := startRedisOperation(ctx, "PING")
	err := c.rdb.Ping(ctx).Err()
	end(err)
	return err
}

// ---- 高频常用方法（薄封装，直接 error 语义）----

// Set 写入字符串值，expiration = 0 表示不过期。
func (c *Client) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	ctx, end := startRedisOperation(ctx, "SET")
	err := c.rdb.Set(ctx, key, value, expiration).Err()
	end(err)
	return err
}

// Get 读取字符串值。key 不存在时返回 (""、errno.ErrCacheMiss)，便于业务层统一处理。
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	ctx, end := startRedisOperation(ctx, "GET")
	v, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			end(errno.ErrCacheMiss)
			return "", errno.ErrCacheMiss
		}
		err = errno.ErrInternal.WithMessagef("redis get %s: %v", key, err)
		end(err)
		return "", err
	}
	end(nil)
	return v, nil
}

// Del 删除一个或多个 key，返回被删除的数量。
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	ctx, end := startRedisOperation(ctx, "DEL")
	n, err := c.rdb.Del(ctx, keys...).Result()
	if err != nil {
		err = errno.ErrInternal.WithMessagef("redis del: %v", err)
		end(err)
		return 0, err
	}
	end(nil)
	return n, nil
}

// SetNX 原子写入（key 不存在时才写）。返回是否写入成功。
func (c *Client) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	ctx, end := startRedisOperation(ctx, "SETNX")
	ok, err := c.rdb.SetNX(ctx, key, value, expiration).Result()
	if err != nil {
		err = errno.ErrInternal.WithMessagef("redis setnx %s: %v", key, err)
		end(err)
		return false, err
	}
	end(nil)
	return ok, nil
}

// Key 拼接符合 SPEC §8.1 规范的键名：{a}:{b}:{c}...
// 空段会被忽略（如 Key("idp", "", "x") -> "idp:x"），但通常不建议传空段。
func Key(parts ...string) string {
	filtered := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ":")
}

// SAdd 向 Set 类型的 key 中添加成员，并设置 TTL（若 TTL > 0）。
func (c *Client) SAdd(ctx context.Context, key string, member string, expiration time.Duration) error {
	ctx, end := startRedisOperation(ctx, "SADD")
	var opErr error
	defer func() { end(opErr) }()

	if err := c.rdb.SAdd(ctx, key, member).Err(); err != nil {
		opErr = errno.ErrInternal.WithMessagef("redis sadd %s: %v", key, err)
		return opErr
	}
	if expiration > 0 {
		if err := c.rdb.Expire(ctx, key, expiration).Err(); err != nil {
			opErr = errno.ErrInternal.WithMessagef("redis expire %s: %v", key, err)
			return opErr
		}
	}
	return nil
}

// SMembers 返回 Set key 中所有成员。key 不存在时返回空切片。
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	ctx, end := startRedisOperation(ctx, "SMEMBERS")
	members, err := c.rdb.SMembers(ctx, key).Result()
	if err != nil {
		err = errno.ErrInternal.WithMessagef("redis smembers %s: %v", key, err)
		end(err)
		return nil, err
	}
	end(nil)
	return members, nil
}

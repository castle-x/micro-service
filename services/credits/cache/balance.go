// Package cache 封装 credits 的 Redis 缓存访问。
package cache

// BalanceCache 管理用户积分余额缓存（credits:balance:{user_id}）。
type BalanceCache struct{}

// NewBalanceCache 构造 BalanceCache。
func NewBalanceCache() *BalanceCache { return &BalanceCache{} }

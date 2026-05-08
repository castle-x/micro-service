// Package cache 封装 iam 的 Redis 缓存访问。
package cache

// PermissionCache 管理用户权限缓存（iam:permissions:{user_id}）。
type PermissionCache struct{}

// NewPermissionCache 构造 PermissionCache。
func NewPermissionCache() *PermissionCache { return &PermissionCache{} }

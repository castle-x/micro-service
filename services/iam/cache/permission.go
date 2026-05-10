// Package cache 封装 iam 的 Redis 缓存访问。
package cache

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
)

const rolePermsTTL = 5 * time.Minute

// RoleCache 缓存角色权限列表，key = iam:role:perms:{roleName}，TTL=5min。
// 管理员修改角色权限后应调用 Delete 主动失效。
type RoleCache struct {
	client *pkgredis.Client
}

// NewRoleCache 构造 RoleCache。
func NewRoleCache(client *pkgredis.Client) *RoleCache {
	return &RoleCache{client: client}
}

func rolePermsKey(roleName string) string {
	return pkgredis.Key("iam", "role", "perms", roleName)
}

// Get 查询角色权限列表。未命中返回 ErrCacheMiss。
func (c *RoleCache) Get(ctx context.Context, roleName string) ([]string, error) {
	v, err := c.client.Get(ctx, rolePermsKey(roleName))
	if err != nil {
		if errors.Is(err, errno.ErrCacheMiss) {
			return nil, errno.ErrCacheMiss
		}
		return nil, err
	}
	if v == "" {
		return []string{}, nil
	}
	return strings.Split(v, ","), nil
}

// Set 缓存角色权限列表，TTL=5min。
func (c *RoleCache) Set(ctx context.Context, roleName string, permissions []string) error {
	return c.client.Set(ctx, rolePermsKey(roleName), strings.Join(permissions, ","), rolePermsTTL)
}

// Delete 主动失效角色权限缓存（管理员修改角色后调用）。
func (c *RoleCache) Delete(ctx context.Context, roleName string) error {
	_, err := c.client.Del(ctx, rolePermsKey(roleName))
	return err
}

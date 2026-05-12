package biz

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	iamcache "github.com/castlexu/micro-service/services/iam/cache"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
)

// RoleBiz 处理角色管理业务。
type RoleBiz struct {
	roleRepo  *iammongo.RoleRepo
	permRepo  *iammongo.PermissionRepo
	roleCache *iamcache.RoleCache
}

// NewRoleBiz 构造 RoleBiz。
func NewRoleBiz(roleRepo *iammongo.RoleRepo, permRepo *iammongo.PermissionRepo, roleCache *iamcache.RoleCache) *RoleBiz {
	return &RoleBiz{roleRepo: roleRepo, permRepo: permRepo, roleCache: roleCache}
}

// ListRoles 返回所有角色。
func (b *RoleBiz) ListRoles(ctx context.Context) ([]*iammodel.Role, error) {
	return b.roleRepo.ListAll(ctx)
}

// CreateRole 创建自定义角色。
func (b *RoleBiz) CreateRole(ctx context.Context, name, displayName string, permissions []string) (*iammodel.Role, error) {
	if name == "" || displayName == "" {
		return nil, errno.ErrInvalidParam.WithMessage("iam: name and display_name are required")
	}
	// 校验 permission code 都存在
	if missing, err := b.permRepo.ExistsByCodes(ctx, permissions); err != nil {
		return nil, err
	} else if len(missing) > 0 {
		return nil, errno.ErrInvalidParam.WithMessagef("iam: unknown permission codes: %v", missing)
	}

	role := &iammodel.Role{
		BaseDoc:     db.BaseDoc{ID: primitive.NewObjectID()},
		Name:        name,
		DisplayName: displayName,
		Permissions: permissions,
		IsSystem:    false,
	}
	if err := b.roleRepo.Insert(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

// UpdateRole 更新角色展示名和权限（super_admin 权限列表不受限制，可由 super_admin 修改）。
func (b *RoleBiz) UpdateRole(ctx context.Context, roleID, displayName string, permissions []string) error {
	oid, err := primitive.ObjectIDFromHex(roleID)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("iam: invalid role_id")
	}
	role, err := b.roleRepo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	// 校验 permission codes 存在
	if missing, err := b.permRepo.ExistsByCodes(ctx, permissions); err != nil {
		return err
	} else if len(missing) > 0 {
		return errno.ErrInvalidParam.WithMessagef("iam: unknown permission codes: %v", missing)
	}

	if err := b.roleRepo.UpdatePermissions(ctx, oid, displayName, permissions); err != nil {
		return err
	}
	// 主动失效缓存
	_ = b.roleCache.Delete(ctx, role.Name)
	return nil
}

// DeleteRole 删除角色，内置角色不可删。
func (b *RoleBiz) DeleteRole(ctx context.Context, roleID string) error {
	oid, err := primitive.ObjectIDFromHex(roleID)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("iam: invalid role_id")
	}
	role, err := b.roleRepo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return errno.ErrForbidden.WithMessage("iam: cannot delete system role")
	}
	if err := b.roleRepo.Delete(ctx, oid); err != nil {
		return err
	}
	_ = b.roleCache.Delete(ctx, role.Name)
	return nil
}

// GetRolePermissions 获取角色权限列表，优先读缓存。
func (b *RoleBiz) GetRolePermissions(ctx context.Context, roleName string) ([]string, error) {
	// super_admin bypass：所有权限由调用方处理
	if roleName == "super_admin" {
		return []string{"*"}, nil
	}

	// 先查缓存
	if perms, err := b.roleCache.Get(ctx, roleName); err == nil {
		return perms, nil
	}

	// 缓存未命中，查库
	role, err := b.roleRepo.FindByName(ctx, roleName)
	if err != nil {
		return nil, err
	}
	_ = b.roleCache.Set(ctx, roleName, role.Permissions)
	return role.Permissions, nil
}

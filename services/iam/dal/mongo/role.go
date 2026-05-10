package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
)

// RoleRepo 封装 roles 集合操作。
type RoleRepo struct {
	repo *db.Repository[iammodel.Role]
}

// NewRoleRepo 构造 RoleRepo。
func NewRoleRepo(client *db.Client) *RoleRepo {
	return &RoleRepo{repo: db.NewRepository[iammodel.Role](client, iammodel.RoleCollection)}
}

// EnsureIndexes 建立 name 唯一索引。
func (r *RoleRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, iammodel.RoleCollection, []string{"name"}, true)
}

// FindByName 按名称查角色。
func (r *RoleRepo) FindByName(ctx context.Context, name string) (*iammodel.Role, error) {
	role, err := r.repo.FindOne(ctx, bson.D{{Key: "name", Value: name}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrNotFound.WithMessagef("iam: role %q not found", name)
		}
		return nil, errno.ErrInternal.WithMessagef("iam: find role: %v", err)
	}
	return role, nil
}

// FindByID 按 ID 查角色。
func (r *RoleRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*iammodel.Role, error) {
	role, err := r.repo.FindByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrNotFound.WithMessage("iam: role not found")
		}
		return nil, errno.ErrInternal.WithMessagef("iam: find role by id: %v", err)
	}
	return role, nil
}

// ListAll 返回所有角色。
func (r *RoleRepo) ListAll(ctx context.Context) ([]*iammodel.Role, error) {
	roles, err := r.repo.Find(ctx, bson.D{}, db.FindOptions{
		Sort: bson.D{{Key: "created_at", Value: 1}},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("iam: list roles: %v", err)
	}
	return roles, nil
}

// Insert 插入新角色，name 重复返回 ErrDuplicateKey。
func (r *RoleRepo) Insert(ctx context.Context, role *iammodel.Role) error {
	_, err := r.repo.InsertOne(ctx, role)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return errno.ErrDuplicateKey.WithMessagef("iam: role %q already exists", role.Name)
		}
		return errno.ErrInternal.WithMessagef("iam: insert role: %v", err)
	}
	return nil
}

// UpdatePermissions 更新角色的展示名和权限列表。
func (r *RoleRepo) UpdatePermissions(ctx context.Context, id primitive.ObjectID, displayName string, permissions []string) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "display_name", Value: displayName},
			{Key: "permissions", Value: permissions},
		}}},
	)
	if err != nil {
		return errno.ErrInternal.WithMessagef("iam: update role: %v", err)
	}
	return nil
}

// Delete 删除角色（is_system=true 时调用方应先检查）。
func (r *RoleRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if err := r.repo.HardDeleteOne(ctx, bson.D{{Key: "_id", Value: id}}); err != nil {
		return errno.ErrInternal.WithMessagef("iam: delete role: %v", err)
	}
	return nil
}

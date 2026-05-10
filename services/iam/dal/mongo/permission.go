package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
)

// PermissionRepo 封装 permissions 集合操作。
type PermissionRepo struct {
	repo *db.Repository[iammodel.Permission]
}

// NewPermissionRepo 构造 PermissionRepo。
func NewPermissionRepo(client *db.Client) *PermissionRepo {
	return &PermissionRepo{repo: db.NewRepository[iammodel.Permission](client, iammodel.PermissionCollection)}
}

// EnsureIndexes 建立 code 唯一索引。
func (r *PermissionRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, iammodel.PermissionCollection, []string{"code"}, true)
}

// ListAll 返回所有权限。
func (r *PermissionRepo) ListAll(ctx context.Context) ([]*iammodel.Permission, error) {
	perms, err := r.repo.Find(ctx, bson.D{}, db.FindOptions{
		Sort: bson.D{{Key: "code", Value: 1}},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("iam: list permissions: %v", err)
	}
	return perms, nil
}

// FindByCode 按 code 查权限。
func (r *PermissionRepo) FindByCode(ctx context.Context, code string) (*iammodel.Permission, error) {
	p, err := r.repo.FindOne(ctx, bson.D{{Key: "code", Value: code}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrNotFound.WithMessagef("iam: permission %q not found", code)
		}
		return nil, errno.ErrInternal.WithMessagef("iam: find permission: %v", err)
	}
	return p, nil
}

// Insert 插入新权限，code 重复返回 ErrDuplicateKey。
func (r *PermissionRepo) Insert(ctx context.Context, p *iammodel.Permission) error {
	_, err := r.repo.InsertOne(ctx, p)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return errno.ErrDuplicateKey.WithMessagef("iam: permission %q already exists", p.Code)
		}
		return errno.ErrInternal.WithMessagef("iam: insert permission: %v", err)
	}
	return nil
}

// ExistsByCodes 检查给定 code 列表是否都存在，返回不存在的 codes。
func (r *PermissionRepo) ExistsByCodes(ctx context.Context, codes []string) ([]string, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	existing, err := r.repo.Find(ctx,
		bson.D{{Key: "code", Value: bson.D{{Key: "$in", Value: codes}}}},
		db.FindOptions{},
	)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("iam: check permissions: %v", err)
	}
	found := make(map[string]bool, len(existing))
	for _, p := range existing {
		found[p.Code] = true
	}
	var missing []string
	for _, c := range codes {
		if !found[c] {
			missing = append(missing, c)
		}
	}
	return missing, nil
}

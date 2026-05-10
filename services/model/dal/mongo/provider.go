package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	mdlmodel "github.com/castlexu/micro-service/services/model/dal/model"
)

// ProviderRepo 封装 model_providers 集合的 CRUD。
type ProviderRepo struct {
	repo *db.Repository[mdlmodel.Provider]
}

// NewProviderRepo 构造 ProviderRepo。
func NewProviderRepo(client *db.Client) *ProviderRepo {
	return &ProviderRepo{repo: db.NewRepository[mdlmodel.Provider](client, mdlmodel.ProviderCollection)}
}

// EnsureIndexes 建立 slug 唯一索引。
func (r *ProviderRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, mdlmodel.ProviderCollection, []string{"slug"}, true)
}

// Insert 插入新 provider。
func (r *ProviderRepo) Insert(ctx context.Context, p *mdlmodel.Provider) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, p)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return primitive.NilObjectID, errno.ErrDuplicateKey.WithMessage("model: provider slug already exists")
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("model: insert provider: %v", err)
	}
	return id, nil
}

// FindByID 按 ID 查 provider。
func (r *ProviderRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*mdlmodel.Provider, error) {
	p, err := r.repo.FindByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrProviderNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("model: find provider: %v", err)
	}
	return p, nil
}

// FindBySlug 按 slug 查 provider。
func (r *ProviderRepo) FindBySlug(ctx context.Context, slug string) (*mdlmodel.Provider, error) {
	p, err := r.repo.FindOne(ctx, bson.D{{Key: "slug", Value: slug}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrProviderNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("model: find provider by slug: %v", err)
	}
	return p, nil
}

// List 列出所有 provider（不分页，数量预期很小）。
func (r *ProviderRepo) List(ctx context.Context) ([]*mdlmodel.Provider, error) {
	items, err := r.repo.Find(ctx, bson.D{}, db.FindOptions{
		Sort: bson.D{{Key: "created_at", Value: 1}},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("model: list providers: %v", err)
	}
	return items, nil
}

// UpdateEnabled 切换 enabled 状态。
func (r *ProviderRepo) UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "enabled", Value: enabled}}}},
	)
	if err != nil {
		return errno.ErrInternal.WithMessagef("model: update provider enabled: %v", err)
	}
	return nil
}

// UpdateAPIKey 更新 API key。
func (r *ProviderRepo) UpdateAPIKey(ctx context.Context, id primitive.ObjectID, apiKey string) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "api_key", Value: apiKey}}}},
	)
	if err != nil {
		return errno.ErrInternal.WithMessagef("model: update provider api_key: %v", err)
	}
	return nil
}

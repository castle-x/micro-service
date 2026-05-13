package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// AssetCategoryRepo 封装 asset_categories 集合的索引与仓储入口。
type AssetCategoryRepo struct {
	repo *db.Repository[assetmodel.AssetCategory]
}

// NewAssetCategoryRepo 构造 AssetCategoryRepo。
func NewAssetCategoryRepo(client *db.Client) *AssetCategoryRepo {
	return &AssetCategoryRepo{repo: db.NewRepository[assetmodel.AssetCategory](client, assetmodel.AssetCategoryCollection)}
}

// EnsureIndexes 建立 asset_categories 必要索引。
func (r *AssetCategoryRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, assetmodel.AssetCategoryCollection, []string{"workspace_id", "parent_id", "sort_order"}, false); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, assetmodel.AssetCategoryCollection, []string{"workspace_id", "name"}, false)
}

// CreateAssetCategory 插入资产分类。
func (r *AssetCategoryRepo) CreateAssetCategory(ctx context.Context, doc *assetmodel.AssetCategory) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("asset: create category: %v", err)
	}
	return id, nil
}

// FindAssetCategoryByID 按 workspace + id 查询分类。
func (r *AssetCategoryRepo) FindAssetCategoryByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetCategory, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrAssetCategoryNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find category: %v", err)
	}
	return doc, nil
}

// ListAssetCategories 查询当前个人资产库的分类。
func (r *AssetCategoryRepo) ListAssetCategories(ctx context.Context, workspaceID string) ([]*assetmodel.AssetCategory, error) {
	docs, err := r.repo.Find(ctx, bson.D{{Key: "workspace_id", Value: workspaceID}}, db.FindOptions{
		Sort: bson.D{{Key: "parent_id", Value: 1}, {Key: "sort_order", Value: 1}, {Key: "updated_at", Value: -1}},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("asset: list categories: %v", err)
	}
	return docs, nil
}

// UpdateAssetCategory 更新分类。
func (r *AssetCategoryRepo) UpdateAssetCategory(ctx context.Context, doc *assetmodel.AssetCategory) error {
	set := bson.D{
		{Key: "name", Value: doc.Name},
		{Key: "sort_order", Value: doc.SortOrder},
	}
	update := bson.D{{Key: "$set", Value: set}}
	if doc.ParentID.IsZero() {
		update = append(update, bson.E{Key: "$unset", Value: bson.D{{Key: "parent_id", Value: ""}}})
	} else {
		set = append(set, bson.E{Key: "parent_id", Value: doc.ParentID})
		update[0].Value = set
	}
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: doc.ID}, {Key: "workspace_id", Value: doc.WorkspaceID}},
		update,
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetCategoryNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: update category: %v", err)
	}
	return nil
}

// DeleteAssetCategory 物理删除空分类。
func (r *AssetCategoryRepo) DeleteAssetCategory(ctx context.Context, workspaceID string, id primitive.ObjectID) error {
	err := r.repo.HardDeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetCategoryNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: delete category: %v", err)
	}
	return nil
}

// CountChildCategories 统计分类子节点数量。
func (r *AssetCategoryRepo) CountChildCategories(ctx context.Context, workspaceID string, parentID primitive.ObjectID) (int64, error) {
	count, err := r.repo.Count(ctx, bson.D{{Key: "workspace_id", Value: workspaceID}, {Key: "parent_id", Value: parentID}})
	if err != nil {
		return 0, errno.ErrInternal.WithMessagef("asset: count child categories: %v", err)
	}
	return count, nil
}

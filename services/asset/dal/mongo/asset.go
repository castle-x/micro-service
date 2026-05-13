package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// AssetRepo 封装 assets 集合的索引与仓储入口。
type AssetRepo struct {
	repo *db.Repository[assetmodel.Asset]
}

// NewAssetRepo 构造 AssetRepo。
func NewAssetRepo(client *db.Client) *AssetRepo {
	return &AssetRepo{repo: db.NewRepository[assetmodel.Asset](client, assetmodel.AssetCollection)}
}

// EnsureIndexes 建立 assets 必要索引。
func (r *AssetRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, assetmodel.AssetCollection, []string{"workspace_id", "saved_to_library", "updated_at:-1"}, false); err != nil {
		return err
	}
	if err := client.CreateIndexes(ctx, assetmodel.AssetCollection, []string{"workspace_id", "type_id", "updated_at:-1"}, false); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, assetmodel.AssetCollection, []string{"workspace_id", "category_id", "updated_at:-1"}, false)
}

// CreateAsset 插入资产实例。
func (r *AssetRepo) CreateAsset(ctx context.Context, doc *assetmodel.Asset) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("asset: create asset: %v", err)
	}
	return id, nil
}

// FindAssetByID 按 workspace + id 查询资产。
func (r *AssetRepo) FindAssetByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.Asset, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrAssetNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find asset: %v", err)
	}
	return doc, nil
}

// ListAssets 分页查询资产。
func (r *AssetRepo) ListAssets(ctx context.Context, workspaceID string, pageNum, pageSize int32, typeID, categoryID primitive.ObjectID, savedToLibrary *bool) ([]*assetmodel.Asset, int64, error) {
	filter := bson.D{{Key: "workspace_id", Value: workspaceID}}
	if !typeID.IsZero() {
		filter = append(filter, bson.E{Key: "type_id", Value: typeID})
	}
	if !categoryID.IsZero() {
		filter = append(filter, bson.E{Key: "category_id", Value: categoryID})
	}
	if savedToLibrary != nil {
		filter = append(filter, bson.E{Key: "saved_to_library", Value: *savedToLibrary})
	}
	total, err := r.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: count assets: %v", err)
	}
	skip, limit := normalizePage(pageNum, pageSize)
	docs, err := r.repo.Find(ctx, filter, db.FindOptions{
		Sort:  bson.D{{Key: "updated_at", Value: -1}},
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: list assets: %v", err)
	}
	return docs, total, nil
}

// UpdateAsset 更新资产基础信息。
func (r *AssetRepo) UpdateAsset(ctx context.Context, doc *assetmodel.Asset) error {
	set := bson.D{
		{Key: "name", Value: doc.Name},
		{Key: "description", Value: doc.Description},
		{Key: "source", Value: doc.Source},
		{Key: "provenance", Value: doc.Provenance},
	}
	unset := bson.D{}
	if doc.CategoryID.IsZero() {
		unset = append(unset, bson.E{Key: "category_id", Value: ""})
	} else {
		set = append(set, bson.E{Key: "category_id", Value: doc.CategoryID})
	}
	if doc.CoverMediaID.IsZero() {
		unset = append(unset, bson.E{Key: "cover_media_id", Value: ""})
	} else {
		set = append(set, bson.E{Key: "cover_media_id", Value: doc.CoverMediaID})
	}
	update := bson.D{{Key: "$set", Value: set}}
	if len(unset) > 0 {
		update = append(update, bson.E{Key: "$unset", Value: unset})
	}
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: doc.ID}, {Key: "workspace_id", Value: doc.WorkspaceID}},
		update,
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: update asset: %v", err)
	}
	return nil
}

// SetAssetLibraryState 更新资产库保存状态。
func (r *AssetRepo) SetAssetLibraryState(ctx context.Context, workspaceID string, id primitive.ObjectID, saved bool) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "saved_to_library", Value: saved}}}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: set library state: %v", err)
	}
	return nil
}

// SetAssetCurrentVersion 更新资产当前版本指针。
func (r *AssetRepo) SetAssetCurrentVersion(ctx context.Context, workspaceID string, id primitive.ObjectID, version int32) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "current_version", Value: version}}}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: set current version: %v", err)
	}
	return nil
}

// DeleteAsset 软删除资产实例。
func (r *AssetRepo) DeleteAsset(ctx context.Context, workspaceID string, id primitive.ObjectID) error {
	err := r.repo.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: delete asset: %v", err)
	}
	return nil
}

// CountAssetsByType 统计使用某资产类型的未删除资产数量。
func (r *AssetRepo) CountAssetsByType(ctx context.Context, workspaceID string, typeID primitive.ObjectID) (int64, error) {
	count, err := r.repo.Count(ctx, bson.D{{Key: "workspace_id", Value: workspaceID}, {Key: "type_id", Value: typeID}})
	if err != nil {
		return 0, errno.ErrInternal.WithMessagef("asset: count assets by type: %v", err)
	}
	return count, nil
}

// CountAssetsByCategory 统计某分类下的未删除资产数量。
func (r *AssetRepo) CountAssetsByCategory(ctx context.Context, workspaceID string, categoryID primitive.ObjectID) (int64, error) {
	count, err := r.repo.Count(ctx, bson.D{{Key: "workspace_id", Value: workspaceID}, {Key: "category_id", Value: categoryID}})
	if err != nil {
		return 0, errno.ErrInternal.WithMessagef("asset: count assets by category: %v", err)
	}
	return count, nil
}

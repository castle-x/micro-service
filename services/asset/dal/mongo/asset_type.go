package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// AssetTypeRepo 封装 asset_types 集合的索引与仓储入口。
type AssetTypeRepo struct {
	repo *db.Repository[assetmodel.AssetType]
}

// NewAssetTypeRepo 构造 AssetTypeRepo。
func NewAssetTypeRepo(client *db.Client) *AssetTypeRepo {
	return &AssetTypeRepo{repo: db.NewRepository[assetmodel.AssetType](client, assetmodel.AssetTypeCollection)}
}

// EnsureIndexes 建立 asset_types 必要索引。
func (r *AssetTypeRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, assetmodel.AssetTypeCollection, []string{"workspace_id", "code"}, true); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, assetmodel.AssetTypeCollection, []string{"workspace_id", "updated_at:-1"}, false)
}

// CreateAssetType 插入资产类型。
func (r *AssetTypeRepo) CreateAssetType(ctx context.Context, doc *assetmodel.AssetType) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return primitive.NilObjectID, errno.ErrAssetConflict.WithMessage("asset: asset type code already exists")
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("asset: create asset type: %v", err)
	}
	return id, nil
}

// FindAssetTypeByID 按 workspace + id 查询资产类型。
func (r *AssetTypeRepo) FindAssetTypeByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetType, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrAssetTypeNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find asset type: %v", err)
	}
	return doc, nil
}

// ListAssetTypes 分页查询资产类型。
func (r *AssetTypeRepo) ListAssetTypes(ctx context.Context, workspaceID string, pageNum, pageSize int32) ([]*assetmodel.AssetType, int64, error) {
	filter := bson.D{{Key: "workspace_id", Value: workspaceID}}
	total, err := r.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: count asset types: %v", err)
	}
	skip, limit := normalizePage(pageNum, pageSize)
	docs, err := r.repo.Find(ctx, filter, db.FindOptions{
		Sort:  bson.D{{Key: "updated_at", Value: -1}},
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: list asset types: %v", err)
	}
	return docs, total, nil
}

// UpdateAssetType 更新资产类型基础信息与 part schema。
func (r *AssetTypeRepo) UpdateAssetType(ctx context.Context, doc *assetmodel.AssetType) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: doc.ID}, {Key: "workspace_id", Value: doc.WorkspaceID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: doc.Name},
			{Key: "description", Value: doc.Description},
			{Key: "part_schemas", Value: doc.PartSchemas},
		}}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetTypeNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: update asset type: %v", err)
	}
	return nil
}

// DeleteAssetType 物理删除未使用资产类型。
func (r *AssetTypeRepo) DeleteAssetType(ctx context.Context, workspaceID string, id primitive.ObjectID) error {
	err := r.repo.HardDeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetTypeNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: delete asset type: %v", err)
	}
	return nil
}

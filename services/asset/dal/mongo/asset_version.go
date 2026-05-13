package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// AssetVersionRepo 封装 asset_versions 集合的索引与仓储入口。
type AssetVersionRepo struct {
	repo *db.Repository[assetmodel.AssetVersion]
}

// NewAssetVersionRepo 构造 AssetVersionRepo。
func NewAssetVersionRepo(client *db.Client) *AssetVersionRepo {
	return &AssetVersionRepo{repo: db.NewRepository[assetmodel.AssetVersion](client, assetmodel.AssetVersionCollection)}
}

// EnsureIndexes 建立 asset_versions 必要索引。
func (r *AssetVersionRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, assetmodel.AssetVersionCollection, []string{"asset_id", "version"}, true); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, assetmodel.AssetVersionCollection, []string{"asset_id", "created_at:-1"}, false)
}

// CreateAssetVersion 插入资产版本快照。
func (r *AssetVersionRepo) CreateAssetVersion(ctx context.Context, doc *assetmodel.AssetVersion) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return primitive.NilObjectID, errno.ErrAssetConflict.WithMessage("asset: version already exists")
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("asset: create asset version: %v", err)
	}
	return id, nil
}

// FindAssetVersion 按 asset_id + version 查询版本快照。
func (r *AssetVersionRepo) FindAssetVersion(ctx context.Context, assetID primitive.ObjectID, version int32) (*assetmodel.AssetVersion, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "asset_id", Value: assetID}, {Key: "version", Value: version}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrAssetVersionNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find asset version: %v", err)
	}
	return doc, nil
}

// ListAssetVersions 分页查询某资产的版本快照。
func (r *AssetVersionRepo) ListAssetVersions(ctx context.Context, assetID primitive.ObjectID, pageNum, pageSize int32) ([]*assetmodel.AssetVersion, int64, error) {
	filter := bson.D{{Key: "asset_id", Value: assetID}}
	total, err := r.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: count asset versions: %v", err)
	}
	skip, limit := normalizePage(pageNum, pageSize)
	docs, err := r.repo.Find(ctx, filter, db.FindOptions{
		Sort:  bson.D{{Key: "version", Value: -1}},
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: list asset versions: %v", err)
	}
	return docs, total, nil
}

// NextAssetVersionNumber 返回某资产下一个版本号。
func (r *AssetVersionRepo) NextAssetVersionNumber(ctx context.Context, assetID primitive.ObjectID) (int32, error) {
	docs, err := r.repo.Find(ctx,
		bson.D{{Key: "asset_id", Value: assetID}},
		db.FindOptions{Sort: bson.D{{Key: "version", Value: -1}}, Limit: 1},
	)
	if err != nil {
		return 0, errno.ErrInternal.WithMessagef("asset: next asset version: %v", err)
	}
	if len(docs) == 0 {
		return 1, nil
	}
	return docs[0].Version + 1, nil
}

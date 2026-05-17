package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// MediaObjectRepo 封装 media_objects 集合的索引与仓储入口。
type MediaObjectRepo struct {
	repo *db.Repository[assetmodel.MediaObject]
}

// NewMediaObjectRepo 构造 MediaObjectRepo。
func NewMediaObjectRepo(client *db.Client) *MediaObjectRepo {
	return &MediaObjectRepo{repo: db.NewRepository[assetmodel.MediaObject](client, assetmodel.MediaObjectCollection)}
}

// EnsureIndexes 建立 media_objects 必要索引。
func (r *MediaObjectRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, assetmodel.MediaObjectCollection, []string{"provider", "bucket", "object_key"}, true); err != nil {
		return err
	}
	if err := client.CreateIndexes(ctx, assetmodel.MediaObjectCollection, []string{"workspace_id", "created_at:-1"}, false); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, assetmodel.MediaObjectCollection, []string{"sha256"}, false)
}

// CreateMediaObject 插入媒体对象。
func (r *MediaObjectRepo) CreateMediaObject(ctx context.Context, doc *assetmodel.MediaObject) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return primitive.NilObjectID, errno.ErrDuplicateKey.WithMessage("asset: media object already exists")
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("asset: create media object: %v", err)
	}
	return id, nil
}

// FindMediaObjectByID 按 workspace + id 查询媒体对象。
func (r *MediaObjectRepo) FindMediaObjectByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.MediaObject, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrMediaObjectNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find media object: %v", err)
	}
	return doc, nil
}

// FindMediaObjectByObjectKey 按 provider + bucket + object key 查询媒体对象。
func (r *MediaObjectRepo) FindMediaObjectByObjectKey(ctx context.Context, provider assetmodel.StorageProvider, bucket, objectKey string) (*assetmodel.MediaObject, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{
		{Key: "provider", Value: provider},
		{Key: "bucket", Value: bucket},
		{Key: "object_key", Value: objectKey},
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrMediaObjectNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find media object by key: %v", err)
	}
	return doc, nil
}

// ListMediaObjects 分页查询当前 workspace 下的媒体对象。
func (r *MediaObjectRepo) ListMediaObjects(ctx context.Context, workspaceID string, pageNum, pageSize int32, source assetmodel.AssetSource, contentType string) ([]*assetmodel.MediaObject, int64, error) {
	filter := bson.D{{Key: "workspace_id", Value: workspaceID}}
	if source != assetmodel.AssetSourceUnknown {
		filter = append(filter, bson.E{Key: "source", Value: source})
	}
	if contentType != "" {
		filter = append(filter, bson.E{Key: "content_type", Value: contentType})
	}
	total, err := r.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: count media objects: %v", err)
	}
	skip, limit := normalizePage(pageNum, pageSize)
	docs, err := r.repo.Find(ctx, filter, db.FindOptions{
		Sort:  bson.D{{Key: "created_at", Value: -1}},
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("asset: list media objects: %v", err)
	}
	return docs, total, nil
}

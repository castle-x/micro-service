package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// StorageUploadSessionRepo 封装 storage_upload_sessions 集合的索引与仓储入口。
type StorageUploadSessionRepo struct {
	repo *db.Repository[assetmodel.StorageUploadSession]
}

// NewStorageUploadSessionRepo 构造 StorageUploadSessionRepo。
func NewStorageUploadSessionRepo(client *db.Client) *StorageUploadSessionRepo {
	return &StorageUploadSessionRepo{repo: db.NewRepository[assetmodel.StorageUploadSession](client, assetmodel.StorageUploadSessionCollection)}
}

// EnsureIndexes 建立 storage_upload_sessions 必要索引。
func (r *StorageUploadSessionRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, assetmodel.StorageUploadSessionCollection, []string{"workspace_id", "status", "expires_at"}, false); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, assetmodel.StorageUploadSessionCollection, []string{"provider", "bucket", "object_key"}, true)
}

// CreateStorageUploadSession 插入上传会话。
func (r *StorageUploadSessionRepo) CreateStorageUploadSession(ctx context.Context, doc *assetmodel.StorageUploadSession) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return primitive.NilObjectID, errno.ErrAssetConflict.WithMessage("asset: upload session object key already exists")
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("asset: create upload session: %v", err)
	}
	return id, nil
}

// FindStorageUploadSessionByID 按 workspace + id 查询上传会话。
func (r *StorageUploadSessionRepo) FindStorageUploadSessionByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.StorageUploadSession, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "workspace_id", Value: workspaceID}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrAssetUploadSessionNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("asset: find upload session: %v", err)
	}
	return doc, nil
}

// UpdateStorageUploadSession 更新上传会话状态和幂等字段。
func (r *StorageUploadSessionRepo) UpdateStorageUploadSession(ctx context.Context, doc *assetmodel.StorageUploadSession) error {
	set := bson.D{
		{Key: "provider", Value: doc.Provider},
		{Key: "bucket", Value: doc.Bucket},
		{Key: "object_key", Value: doc.ObjectKey},
		{Key: "status", Value: doc.Status},
		{Key: "expires_at", Value: doc.ExpiresAt},
		{Key: "content_type", Value: doc.ContentType},
		{Key: "size", Value: doc.Size},
		{Key: "sha256", Value: doc.SHA256},
	}
	if doc.FinalizedAt > 0 {
		set = append(set, bson.E{Key: "finalized_at", Value: doc.FinalizedAt})
	}
	if !doc.MediaID.IsZero() {
		set = append(set, bson.E{Key: "media_id", Value: doc.MediaID})
	}
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: doc.ID}, {Key: "workspace_id", Value: doc.WorkspaceID}},
		bson.D{{Key: "$set", Value: set}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetUploadSessionNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: update upload session: %v", err)
	}
	return nil
}

// SetUploadSessionFinalized 标记上传会话已完成。
func (r *StorageUploadSessionRepo) SetUploadSessionFinalized(ctx context.Context, workspaceID string, sessionID, mediaID primitive.ObjectID, finalizedAt int64) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: sessionID}, {Key: "workspace_id", Value: workspaceID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: assetmodel.UploadSessionStatusFinalized},
			{Key: "media_id", Value: mediaID},
			{Key: "finalized_at", Value: finalizedAt},
		}}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetUploadSessionNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: finalize upload session: %v", err)
	}
	return nil
}

// SetUploadSessionExpired 标记上传会话过期。
func (r *StorageUploadSessionRepo) SetUploadSessionExpired(ctx context.Context, workspaceID string, sessionID primitive.ObjectID) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: sessionID}, {Key: "workspace_id", Value: workspaceID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: assetmodel.UploadSessionStatusExpired}}}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrAssetUploadSessionNotFound
		}
		return errno.ErrInternal.WithMessagef("asset: expire upload session: %v", err)
	}
	return nil
}

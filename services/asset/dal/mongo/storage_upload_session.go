package mongo

import (
	"context"

	"github.com/castlexu/micro-service/pkg/db"
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

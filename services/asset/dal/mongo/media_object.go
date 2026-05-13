package mongo

import (
	"context"

	"github.com/castlexu/micro-service/pkg/db"
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

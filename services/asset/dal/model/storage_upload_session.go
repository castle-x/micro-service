package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// StorageUploadSessionCollection 是上传会话集合名。
const StorageUploadSessionCollection = "storage_upload_sessions"

// StorageUploadSession 保存前端直传或后续上传流程的临时会话。
type StorageUploadSession struct {
	db.BaseDoc `bson:",inline"`

	WorkspaceID string              `bson:"workspace_id"`
	Provider    StorageProvider     `bson:"provider"`
	Bucket      string              `bson:"bucket"`
	ObjectKey   string              `bson:"object_key"`
	Status      UploadSessionStatus `bson:"status"`
	ExpiresAt   int64               `bson:"expires_at"`
	CreatedBy   primitive.ObjectID  `bson:"created_by"`
	FinalizedAt int64               `bson:"finalized_at,omitempty"`
	ContentType string              `bson:"content_type,omitempty"`
	Size        int64               `bson:"size,omitempty"`
	SHA256      string              `bson:"sha256,omitempty"`
	MediaID     primitive.ObjectID  `bson:"media_id,omitempty"`
}

package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// MediaObjectCollection 是媒体对象集合名。
const MediaObjectCollection = "media_objects"

// MediaObject 记录对象存储中的媒体资源。
type MediaObject struct {
	db.BaseDoc `bson:",inline"`

	WorkspaceID   string             `bson:"workspace_id"`
	Provider      StorageProvider    `bson:"provider"`
	Bucket        string             `bson:"bucket"`
	ObjectKey     string             `bson:"object_key"`
	CDNURL        string             `bson:"cdn_url,omitempty"`
	URLVisibility URLVisibility      `bson:"url_visibility"`
	ContentType   string             `bson:"content_type"`
	Size          int64              `bson:"size"`
	Width         int32              `bson:"width,omitempty"`
	Height        int32              `bson:"height,omitempty"`
	SHA256        string             `bson:"sha256,omitempty"`
	Variants      []MediaVariant     `bson:"variants,omitempty"`
	Source        AssetSource        `bson:"source"`
	Provenance    *Provenance        `bson:"provenance,omitempty"`
	CreatedBy     primitive.ObjectID `bson:"created_by"`
}

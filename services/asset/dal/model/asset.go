package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// AssetCollection 是资产实例集合名。
const AssetCollection = "assets"

// Asset 是用户沉淀或生成的资产实例。
type Asset struct {
	db.BaseDoc `bson:",inline"`

	WorkspaceID    string             `bson:"workspace_id"`
	TypeID         primitive.ObjectID `bson:"type_id"`
	Name           string             `bson:"name"`
	Description    string             `bson:"description,omitempty"`
	SavedToLibrary bool               `bson:"saved_to_library"`
	CategoryID     primitive.ObjectID `bson:"category_id,omitempty"`
	CurrentVersion int32              `bson:"current_version"`
	CoverMediaID   primitive.ObjectID `bson:"cover_media_id,omitempty"`
	Source         AssetSource        `bson:"source"`
	Provenance     *Provenance        `bson:"provenance,omitempty"`
	CreatedBy      primitive.ObjectID `bson:"created_by"`
}

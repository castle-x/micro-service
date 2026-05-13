package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// AssetTypeCollection 是资产类型集合名。
const AssetTypeCollection = "asset_types"

// AssetType 定义某类资产的组成部分 schema。
type AssetType struct {
	db.BaseDoc `bson:",inline"`

	WorkspaceID string             `bson:"workspace_id"`
	Name        string             `bson:"name"`
	Code        string             `bson:"code"`
	Description string             `bson:"description,omitempty"`
	PartSchemas []AssetPartSchema  `bson:"part_schemas"`
	CreatedBy   primitive.ObjectID `bson:"created_by"`
}

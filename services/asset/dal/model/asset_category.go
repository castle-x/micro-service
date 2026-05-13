package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// AssetCategoryCollection 是资产分类集合名。
const AssetCategoryCollection = "asset_categories"

// AssetCategory 保存用户资产库的分类树节点。
type AssetCategory struct {
	db.BaseDoc `bson:",inline"`

	WorkspaceID string             `bson:"workspace_id"`
	Name        string             `bson:"name"`
	ParentID    primitive.ObjectID `bson:"parent_id,omitempty"`
	SortOrder   int32              `bson:"sort_order"`
	CreatedBy   primitive.ObjectID `bson:"created_by"`
}

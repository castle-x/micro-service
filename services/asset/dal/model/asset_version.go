package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// AssetVersionCollection 是资产版本集合名。
const AssetVersionCollection = "asset_versions"

// AssetVersion 保存资产某个版本的 parts 快照。
type AssetVersion struct {
	db.BaseDoc `bson:",inline"`

	AssetID      primitive.ObjectID        `bson:"asset_id"`
	Version      int32                     `bson:"version"`
	Parts        map[string]AssetPartValue `bson:"parts"`
	ChangeReason string                    `bson:"change_reason,omitempty"`
	Provenance   *Provenance               `bson:"provenance,omitempty"`
	CreatedBy    primitive.ObjectID        `bson:"created_by"`
}

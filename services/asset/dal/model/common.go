package model

import "go.mongodb.org/mongo-driver/bson/primitive"

// Provenance 记录资产或媒体对象的生产来源。
type Provenance struct {
	WorkflowRunID   string            `bson:"workflow_run_id,omitempty"`
	StepRunID       string            `bson:"step_run_id,omitempty"`
	GenerationJobID string            `bson:"generation_job_id,omitempty"`
	PromptID        string            `bson:"prompt_id,omitempty"`
	Extra           map[string]string `bson:"extra,omitempty"`
}

// AssetPartSchema 描述资产类型要求的组成部分。
type AssetPartSchema struct {
	Key               string           `bson:"key"`
	Name              string           `bson:"name"`
	Description       string           `bson:"description,omitempty"`
	AllowedValueKinds []AssetValueKind `bson:"allowed_value_kinds"`
	Multiple          bool             `bson:"multiple"`
	Required          bool             `bson:"required"`
	SortOrder         int32            `bson:"sort_order"`
}

// AssetPartValue 保存资产版本中的单个组成部分值。
type AssetPartValue struct {
	ValueKind AssetValueKind       `bson:"value_kind"`
	Text      string               `bson:"text,omitempty"`
	JSON      string               `bson:"json,omitempty"`
	MediaIDs  []primitive.ObjectID `bson:"media_ids,omitempty"`
}

// MediaVariant 保存媒体对象的派生版本信息。
type MediaVariant struct {
	Kind      string `bson:"kind"`
	ObjectKey string `bson:"object_key"`
	CDNURL    string `bson:"cdn_url,omitempty"`
	Width     int32  `bson:"width,omitempty"`
	Height    int32  `bson:"height,omitempty"`
	Size      int64  `bson:"size,omitempty"`
}

package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// ModelCollection is the llm model collection name.
const ModelCollection = "llm_models"

// Model stores a concrete model exposed by an upstream provider.
type Model struct {
	db.BaseDoc `bson:",inline"`

	ProviderID            primitive.ObjectID `bson:"provider_id" json:"provider_id"`
	ProviderSlug          string             `bson:"provider_slug" json:"provider_slug"`
	Model                 string             `bson:"model" json:"model"`
	ModelRef              string             `bson:"model_ref" json:"model_ref"`
	DisplayName           string             `bson:"display_name,omitempty" json:"display_name,omitempty"`
	Capabilities          []string           `bson:"capabilities" json:"capabilities"`
	ContextWindow         int                `bson:"context_window,omitempty" json:"context_window,omitempty"`
	MaxOutputTokens       int                `bson:"max_output_tokens,omitempty" json:"max_output_tokens,omitempty"`
	DefaultParametersJSON string             `bson:"default_parameters_json,omitempty" json:"default_parameters_json,omitempty"`
	Enabled               bool               `bson:"enabled" json:"enabled"`
}

// ModelUpdatePatch contains writable model fields except identity and enabled.
type ModelUpdatePatch struct {
	DisplayName           *string
	Capabilities          *[]string
	ContextWindow         *int
	MaxOutputTokens       *int
	DefaultParametersJSON *string
}

// Package model defines MongoDB documents for the llm service.
package model

import "github.com/castlexu/micro-service/pkg/db"

// ProviderCollection is the llm provider collection name.
const ProviderCollection = "llm_providers"

// Provider stores an upstream LLM provider configuration.
type Provider struct {
	db.BaseDoc `bson:",inline"`

	Name            string `bson:"name" json:"name"`
	Slug            string `bson:"slug" json:"slug"`
	Vendor          string `bson:"vendor" json:"vendor"`
	BaseURL         string `bson:"base_url" json:"base_url"`
	APIKeyCipher    string `bson:"api_key_cipher" json:"-"`
	Enabled         bool   `bson:"enabled" json:"enabled"`
	DefaultModelRef string `bson:"default_model_ref,omitempty" json:"default_model_ref,omitempty"`
	ExtraJSON       string `bson:"extra_json,omitempty" json:"extra_json,omitempty"`
}

// ProviderUpdatePatch contains writable provider fields except API key and enabled.
type ProviderUpdatePatch struct {
	Name            *string
	Vendor          *string
	BaseURL         *string
	DefaultModelRef *string
	ExtraJSON       *string
}

// Package model 定义 model service 的 MongoDB 文档模型。
package model

import "github.com/castlexu/micro-service/pkg/db"

// ProviderCollection 是 model_providers 集合名。
const ProviderCollection = "model_providers"

// ProviderType 供应商类型。
type ProviderType string

const (
	ProviderTypeLLM   ProviderType = "llm"
	ProviderTypeImage ProviderType = "image"
)

// Provider 记录一个 AI 模型供应商的接入配置。
type Provider struct {
	db.BaseDoc `bson:",inline"`

	Name         string       `bson:"name"`            // 展示名称，如 "DeepSeek"
	Slug         string       `bson:"slug"`            // 唯一标识符，如 "deepseek"
	Type         ProviderType `bson:"type"`            // "llm" | "image"
	BaseURL      string       `bson:"base_url"`        // 上游 API 地址
	APIKey       string       `bson:"api_key"`         // 加密存储的 API Key
	DefaultModel string       `bson:"default_model"`   // 默认模型名，如 "deepseek-chat"
	Enabled      bool         `bson:"enabled"`         // 是否启用
	Extra        string       `bson:"extra,omitempty"` // 扩展 JSON，供适配器自定义
}

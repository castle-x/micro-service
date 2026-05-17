package model

import "github.com/castlexu/micro-service/pkg/db"

// RequestLogCollection is the llm request log collection name.
const RequestLogCollection = "llm_request_logs"

// RequestUsage stores token usage for one LLM request.
type RequestUsage struct {
	PromptTokens     int `bson:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int `bson:"completion_tokens" json:"completion_tokens"`
	TotalTokens      int `bson:"total_tokens" json:"total_tokens"`
}

// RequestLog records one LLM request without storing full prompts.
type RequestLog struct {
	db.BaseDoc `bson:",inline"`

	RequestID      string       `bson:"request_id" json:"request_id"`
	Caller         string       `bson:"caller,omitempty" json:"caller,omitempty"`
	UserID         string       `bson:"user_id,omitempty" json:"user_id,omitempty"`
	TenantID       string       `bson:"tenant_id,omitempty" json:"tenant_id,omitempty"`
	ModelRef       string       `bson:"model_ref" json:"model_ref"`
	ProviderSlug   string       `bson:"provider_slug,omitempty" json:"provider_slug,omitempty"`
	Stream         bool         `bson:"stream" json:"stream"`
	Status         string       `bson:"status" json:"status"`
	Usage          RequestUsage `bson:"usage" json:"usage"`
	IdempotencyKey string       `bson:"idempotency_key,omitempty" json:"idempotency_key,omitempty"`
	ResponseJSON   string       `bson:"response_json,omitempty" json:"-"`
	ErrorCode      int32        `bson:"error_code,omitempty" json:"error_code,omitempty"`
	ErrorMessage   string       `bson:"error_message,omitempty" json:"error_message,omitempty"`
}

// Package component builds Eino components for the llm service.
package component

import (
	"context"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
)

const (
	ProviderTypeLLM = "llm"

	VendorOpenAICompatible = "openai_compatible"

	CapabilityToolCalling = "tool_calling"
)

// ResolvedModel is the minimum model/provider view needed to build a ChatModel.
type ResolvedModel struct {
	Ref             string
	Model           string
	ProviderSlug    string
	ProviderVendor  string
	ProviderType    string
	ProviderBaseURL string
	EncryptedAPIKey string
	Capabilities    []string
	MaxOutputTokens int

	Temperature         *float32
	MaxCompletionTokens *int
}

func (m *ResolvedModel) SupportsCapability(capability string) bool {
	if m == nil {
		return false
	}
	for _, item := range m.Capabilities {
		if strings.EqualFold(item, capability) {
			return true
		}
	}
	return false
}

type ModelResolver interface {
	ResolveModel(ctx context.Context, modelRef string) (*ResolvedModel, error)
}

type APIKeyDecrypter interface {
	Decrypt(ctx context.Context, ciphertext string) (string, error)
}

type APIKeyDecrypterFunc func(ctx context.Context, ciphertext string) (string, error)

func (fn APIKeyDecrypterFunc) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	return fn(ctx, ciphertext)
}

type ChatModelBuilder interface {
	Build(ctx context.Context, resolved *ResolvedModel, apiKey string) (einomodel.ToolCallingChatModel, error)
}

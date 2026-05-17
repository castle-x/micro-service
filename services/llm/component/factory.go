package component

import (
	"context"
	"fmt"

	"github.com/castlexu/micro-service/pkg/errno"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Factory struct {
	resolver  ModelResolver
	decrypter APIKeyDecrypter
	builders  map[string]ChatModelBuilder
}

type Option func(*Factory)

func NewFactory(resolver ModelResolver, decrypter APIKeyDecrypter, opts ...Option) *Factory {
	f := &Factory{
		resolver:  resolver,
		decrypter: decrypter,
		builders: map[string]ChatModelBuilder{
			VendorOpenAICompatible: OpenAICompatibleBuilder{},
		},
	}
	if f.decrypter == nil {
		f.decrypter = APIKeyDecrypterFunc(func(ctx context.Context, ciphertext string) (string, error) {
			return ciphertext, nil
		})
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func WithBuilder(vendor string, builder ChatModelBuilder) Option {
	return func(f *Factory) {
		if f.builders == nil {
			f.builders = make(map[string]ChatModelBuilder)
		}
		f.builders[vendor] = builder
	}
}

func (f *Factory) Build(ctx context.Context, modelRef string) (einomodel.ToolCallingChatModel, *ResolvedModel, error) {
	if f == nil || f.resolver == nil {
		return nil, nil, errno.ErrInvalidParam.WithMessage("llm component resolver required")
	}
	resolved, err := f.resolver.ResolveModel(ctx, modelRef)
	if err != nil {
		return nil, nil, err
	}
	if resolved == nil {
		return nil, nil, errno.ErrLLMModelNotFound.WithMessagef("model %s not found", modelRef)
	}
	if resolved.ProviderType != "" && resolved.ProviderType != ProviderTypeLLM {
		return nil, resolved, errno.ErrLLMAdapterUnsupported.WithMessagef("provider %s is not llm type", resolved.ProviderSlug)
	}

	builder, ok := f.builders[resolved.ProviderVendor]
	if !ok || builder == nil {
		return nil, resolved, errno.ErrLLMAdapterUnsupported.WithMessagef("llm vendor %s unsupported", resolved.ProviderVendor)
	}

	apiKey, err := f.decrypter.Decrypt(ctx, resolved.EncryptedAPIKey)
	if err != nil {
		return nil, resolved, errno.ErrInternal.WithMessage("decrypt provider api_key failed")
	}
	chatModel, err := builder.Build(ctx, resolved, apiKey)
	if err != nil {
		return nil, resolved, fmt.Errorf("build llm chat model: %w", err)
	}
	return chatModel, resolved, nil
}

func BindTools(chatModel einomodel.ToolCallingChatModel, resolved *ResolvedModel, tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return chatModel, nil
	}
	if chatModel == nil {
		return nil, errno.ErrInvalidParam.WithMessage("chat model required")
	}
	if !resolved.SupportsCapability(CapabilityToolCalling) {
		return nil, errno.ErrLLMModelCapabilityUnsupported.WithMessage("llm model capability unsupported: tool_calling")
	}
	return chatModel.WithTools(tools)
}

package component

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
)

type OpenAICompatibleBuilder struct{}

func (OpenAICompatibleBuilder) Build(ctx context.Context, resolved *ResolvedModel, apiKey string) (einomodel.ToolCallingChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:              apiKey,
		Model:               resolved.Model,
		BaseURL:             resolved.ProviderBaseURL,
		Temperature:         resolved.Temperature,
		MaxCompletionTokens: resolved.MaxCompletionTokens,
	})
}

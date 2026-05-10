package biz

import (
	"context"
	"fmt"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/services/model/adapter"
)

// ChatMessage 是对话消息，与 adapter.Message 对齐。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions 是可选调用参数。
type ChatOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
}

// ChatBiz 处理 LLM 对话请求。
type ChatBiz struct {
	providerBiz    *ProviderBiz
	adapterFactory func(baseURL, apiKey, defaultModel string) adapter.LLMAdapter
}

// NewChatBiz 构造 ChatBiz。
func NewChatBiz(providerBiz *ProviderBiz) *ChatBiz {
	return &ChatBiz{providerBiz: providerBiz, adapterFactory: adapter.NewDeepSeek}
}

// toAdapterMessages 转换 biz 消息格式到 adapter 消息格式。
func toAdapterMessages(messages []ChatMessage) []adapter.Message {
	msgs := make([]adapter.Message, len(messages))
	for i, m := range messages {
		msgs[i] = adapter.Message{Role: m.Role, Content: m.Content}
	}
	return msgs
}

// ChatWithAdapter 直接使用给定适配器对话，跳过 provider 查找（测试用）。
func (b *ChatBiz) ChatWithAdapter(ctx context.Context, adp adapter.LLMAdapter, messages []ChatMessage) (string, error) {
	content, err := adp.Chat(ctx, adapter.ChatRequest{Messages: toAdapterMessages(messages)})
	if err != nil {
		return "", errno.ErrUpstreamLLM.WithMessagef("chat failed: %v", err)
	}
	return content, nil
}

// Chat 按 slug 找到 provider，发起非流式对话，返回 assistant 内容。
func (b *ChatBiz) Chat(ctx context.Context, slug string, messages []ChatMessage, opts *ChatOptions) (string, error) {
	p, err := b.providerBiz.GetBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	if p.Type != "llm" {
		return "", errno.ErrAdapterUnsupported.WithMessagef("provider %s is not an llm provider", slug)
	}

	req := adapter.ChatRequest{
		Model:    p.DefaultModel,
		Messages: toAdapterMessages(messages),
	}
	if opts != nil {
		req.Temperature = opts.Temperature
		req.MaxTokens = opts.MaxTokens
	}

	adp := b.adapterFactory(p.BaseURL, p.APIKey, p.DefaultModel)
	content, err := adp.Chat(ctx, req)
	if err != nil {
		return "", errno.ErrUpstreamLLM.WithMessagef("chat with %s failed: %v", slug, err)
	}
	if content == "" {
		return "", fmt.Errorf("chat: empty response from provider %s", slug)
	}
	return content, nil
}

// ChatStream 按 slug 找到 provider，发起流式对话，返回 chunk channel。
// 调用方消费 channel 直到 ChatChunk.Done=true 或 channel 关闭。
func (b *ChatBiz) ChatStream(ctx context.Context, slug string, messages []ChatMessage, opts *ChatOptions) (<-chan adapter.ChatChunk, error) {
	p, err := b.providerBiz.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if p.Type != "llm" {
		return nil, errno.ErrAdapterUnsupported.WithMessagef("provider %s is not an llm provider", slug)
	}

	req := adapter.ChatRequest{
		Model:    p.DefaultModel,
		Messages: toAdapterMessages(messages),
	}
	if opts != nil {
		req.Temperature = opts.Temperature
		req.MaxTokens = opts.MaxTokens
	}

	adp := b.adapterFactory(p.BaseURL, p.APIKey, p.DefaultModel)
	return adp.ChatStream(ctx, req)
}

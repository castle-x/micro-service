package biz

import (
	"context"
	"fmt"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/services/model/adapter"
	mdlmodel "github.com/castlexu/micro-service/services/model/dal/model"
)

// ChatMessage 是对话消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat mirrors adapter.ResponseFormat for biz-layer use.
type ResponseFormat = adapter.ResponseFormat

// Tool mirrors adapter.Tool for biz-layer use.
type Tool = adapter.Tool

// ThinkingConfig mirrors adapter.ThinkingConfig for biz-layer use.
type ThinkingConfig = adapter.ThinkingConfig

// ChatOptions 是可选调用参数，覆盖所有 OpenAI 兼容参数。
type ChatOptions struct {
	// 采样
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
	Stop        []string
	// 输出格式
	ResponseFormat *ResponseFormat
	// Tool Calling
	Tools      []Tool
	ToolChoice any
	// Thinking（DeepSeek 私有）
	Thinking *ThinkingConfig
}

// ChatBiz 处理 LLM 对话请求。
type ChatBiz struct {
	providerBiz *ProviderBiz
}

// NewChatBiz 构造 ChatBiz。
func NewChatBiz(providerBiz *ProviderBiz) *ChatBiz {
	return &ChatBiz{providerBiz: providerBiz}
}

// toAdapterMessages 转换 biz 消息格式到 adapter 消息格式。
func toAdapterMessages(messages []ChatMessage) []adapter.Message {
	msgs := make([]adapter.Message, len(messages))
	for i, m := range messages {
		msgs[i] = adapter.Message{Role: m.Role, Content: m.Content}
	}
	return msgs
}

// applyOpts 把 ChatOptions 写入 adapter.ChatRequest。
func applyOpts(req *adapter.ChatRequest, opts *ChatOptions) {
	if opts == nil {
		return
	}
	req.Temperature = opts.Temperature
	req.MaxTokens = opts.MaxTokens
	req.TopP = opts.TopP
	req.Stop = opts.Stop
	req.ResponseFormat = opts.ResponseFormat
	req.Tools = opts.Tools
	req.ToolChoice = opts.ToolChoice
	req.Thinking = opts.Thinking
}
func buildLLMAdapter(p *mdlmodel.Provider) (adapter.LLMAdapter, error) {
	if p.Type != mdlmodel.ProviderTypeLLM {
		return nil, errno.ErrAdapterUnsupported.WithMessagef("provider %s is not llm type", p.Slug)
	}
	return adapter.BuildLLM(adapter.ProviderInfo{
		Slug:         p.Slug,
		Type:         string(p.Type),
		BaseURL:      p.BaseURL,
		APIKey:       p.APIKey,
		DefaultModel: p.DefaultModel,
	})
}

// ChatWithAdapter 直接使用给定适配器对话（测试用）。
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
	adp, err := buildLLMAdapter(p)
	if err != nil {
		return "", err
	}

	req := adapter.ChatRequest{Model: p.DefaultModel, Messages: toAdapterMessages(messages)}
	applyOpts(&req, opts)

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
func (b *ChatBiz) ChatStream(ctx context.Context, slug string, messages []ChatMessage, opts *ChatOptions) (<-chan adapter.ChatChunk, error) {
	p, err := b.providerBiz.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	adp, err := buildLLMAdapter(p)
	if err != nil {
		return nil, err
	}

	req := adapter.ChatRequest{Model: p.DefaultModel, Messages: toAdapterMessages(messages)}
	applyOpts(&req, opts)

	ch, err := adp.ChatStream(ctx, req)
	if err != nil {
		return nil, errno.ErrUpstreamLLM.WithMessagef("chat stream with %s failed: %v", slug, err)
	}
	return ch, nil
}

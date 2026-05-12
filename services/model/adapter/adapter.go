// Package adapter 定义 model service 的适配器接口与所有厂商实现。
//
// 设计原则：
//   - 接口（LLMAdapter、ImageAdapter）与实现放在同一包，避免循环依赖
//   - OpenAI 兼容厂商（DeepSeek、Moonshot、Qwen 等）共用 openaiAdapter，只需不同 base_url
//   - biz 层通过 BuildLLM / BuildImage 工厂函数获取适配器，不感知具体厂商
//   - 新增厂商：在 model_providers 集合插入记录 + 必要时在 BuildLLM/BuildImage 加 case
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/httpclient"
	"go.opentelemetry.io/otel/attribute"
)

// ---- 公共类型 ----

// Message 是对话消息（与 OpenAI Chat API 对齐）。
// Tool call 场景下 Content 可为空，ToolCalls 或 ToolCallID 会被填充。
type Message struct {
	Role       string     `json:"role"` // "system"|"user"|"assistant"|"tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant 调用工具时填充
	ToolCallID string     `json:"tool_call_id,omitempty"` // role=tool 时填充，对应 ToolCall.ID
}

// ToolCall 是 assistant 消息里的工具调用请求。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // 目前固定 "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall 是工具调用的函数信息。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// Tool 是可供模型调用的工具定义（Function Calling）。
type Tool struct {
	Type     string       `json:"type"` // 固定 "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction 描述一个可调用函数。
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"` // JSON Schema object
}

// ResponseFormat 控制模型输出格式。
type ResponseFormat struct {
	Type string `json:"type"` // "text" | "json_object"
}

// ThinkingConfig 控制 DeepSeek thinking 模式（其他厂商忽略未知字段）。
type ThinkingConfig struct {
	Type            string `json:"type"`                       // "enabled" | "disabled"
	BudgetTokens    *int   `json:"budget_tokens,omitempty"`    // deepseek-v4-pro
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // "low"|"medium"|"high"
}

// ChatRequest 是 LLM 对话请求参数（覆盖所有 OpenAI 兼容厂商通用参数）。
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`

	// 采样参数（通用）
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`

	// 输出格式（通用）
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	// Tool / Function Calling（通用）
	Tools      []Tool `json:"tools,omitempty"`
	ToolChoice any    `json:"tool_choice,omitempty"` // "none"|"auto"|"required"|{type,function}

	// Thinking / Reasoning（DeepSeek 私有，其他厂商忽略）
	Thinking *ThinkingConfig `json:"thinking,omitempty"`

	// 流式 usage 统计（部分厂商支持）
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

// StreamOptions 控制流式响应的附加选项。
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatChunk 是流式响应的一个增量 chunk。
// ReasoningContent 是 thinking 推理 token（如 deepseek-reasoner）。
// ToolCalls 是 function call 的增量（需调用方自行拼接 arguments 字符串）。
// Usage 仅在 Done=true 时有值（由 stream_options.include_usage 触发）。
type ChatChunk struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	Done             bool
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ImageRequest 是图像生成请求。
type ImageRequest struct {
	Prompt string `json:"prompt"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ImageResult 是图像生成结果。
type ImageResult struct {
	URL string `json:"url"`
}

// ---- 接口 ----

// LLMAdapter 是 LLM 适配器统一接口。
type LLMAdapter interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// ImageAdapter 是图像生成适配器统一接口。
type ImageAdapter interface {
	Generate(ctx context.Context, req ImageRequest) (*ImageResult, error)
}

// ---- ProviderInfo（供 registry 使用，避免循环依赖 dal/model）----

// ProviderInfo 是 registry 所需的 provider 最小信息。
type ProviderInfo struct {
	Slug         string
	Type         string // "llm" | "image"
	BaseURL      string
	APIKey       string // 已解密的明文
	DefaultModel string
}

// ---- Registry ----

// BuildLLM 根据 provider 信息构造 LLMAdapter。
// 所有 OpenAI 兼容厂商统一使用 openaiAdapter；未来新增非兼容协议时在此扩展 switch。
func BuildLLM(p ProviderInfo) (LLMAdapter, error) {
	if p.Type != "llm" {
		return nil, fmt.Errorf("adapter: provider %s is not llm type", p.Slug)
	}
	return newOpenAI(p.Slug, p.BaseURL, p.APIKey, p.DefaultModel), nil
}

// BuildImage 根据 provider 信息构造 ImageAdapter。
func BuildImage(p ProviderInfo) (ImageAdapter, error) {
	if p.Type != "image" {
		return nil, fmt.Errorf("adapter: provider %s is not image type", p.Slug)
	}
	return newSeedream(p.BaseURL, p.APIKey), nil
}

// ---- OpenAI 兼容实现 ----

type openaiAdapter struct {
	client   *httpclient.Client
	provider string
	model    string
}

func newOpenAI(provider, baseURL, apiKey, defaultModel string) LLMAdapter {
	if provider == "" {
		provider = "openai"
	}
	return &openaiAdapter{
		client: httpclient.New(baseURL, 120*time.Second, map[string]string{
			"Authorization": "Bearer " + apiKey,
		}),
		provider: provider,
		model:    defaultModel,
	}
}

type completionResp struct {
	Choices []struct {
		Message struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *openaiAdapter) Chat(ctx context.Context, req ChatRequest) (string, error) {
	if req.Model == "" {
		req.Model = a.model
	}
	req.Stream = false
	ctx, span, finish, _ := startLLMRequest(ctx, a.provider, req.Model, false)
	defer func() {
		finish(nil, nil, 0)
		span.End()
	}()

	var resp completionResp
	if err := a.client.Do(ctx, http.MethodPost, "/v1/chat/completions", req, &resp); err != nil {
		finish(err, nil, 0)
		return "", fmt.Errorf("openai chat: %w", err)
	}
	if resp.Error != nil {
		err := fmt.Errorf("openai error: %s", resp.Error.Message)
		finish(err, nil, 0)
		return "", err
	}
	if len(resp.Choices) == 0 {
		err := fmt.Errorf("openai: empty choices")
		finish(err, nil, 0)
		return "", err
	}
	if resp.Usage != nil {
		usage := streamUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		finish(nil, &usage, 0)
		return resp.Choices[0].Message.Content, nil
	}
	return resp.Choices[0].Message.Content, nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	// usage chunk：choices 为空时由 stream_options.include_usage 触发
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (a *openaiAdapter) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	if req.Model == "" {
		req.Model = a.model
	}
	req.Stream = true
	// 自动注入 include_usage，让上游在流末尾返回 token 用量（为将来统计做准备）
	if req.StreamOptions == nil {
		req.StreamOptions = &StreamOptions{IncludeUsage: true}
	}

	ctx, span, finish, start := startLLMRequest(ctx, a.provider, req.Model, true)

	resp, err := a.client.DoStream(ctx, http.MethodPost, "/v1/chat/completions", req)
	if err != nil {
		finish(err, nil, 0)
		span.End()
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	ch := make(chan ChatChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		defer span.End()

		send := func(out ChatChunk) bool {
			select {
			case ch <- out:
				return true
			case <-ctx.Done():
				return false
			}
		}

		var pendingUsage *streamUsage
		var firstTokenDurationMS int64

		err := httpclient.ReadSSELines(resp.Body, func(data string) error {
			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return nil
			}
			if chunk.Usage != nil {
				pendingUsage = &streamUsage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}
			// usage-only chunk（choices 为空，usage 有值）— 暂存，等 finish 后一起发
			if len(chunk.Choices) == 0 {
				return nil
			}
			c := chunk.Choices[0]
			out := ChatChunk{
				Content:          c.Delta.Content,
				ReasoningContent: c.Delta.ReasoningContent,
				ToolCalls:        c.Delta.ToolCalls,
			}
			if out.Content != "" || out.ReasoningContent != "" || len(out.ToolCalls) > 0 {
				if firstTokenDurationMS == 0 {
					firstTokenDurationMS = time.Since(start).Milliseconds()
					span.SetAttributes(attribute.Int64("llm.first_token.duration_ms", firstTokenDurationMS))
				}
				if !send(out) {
					return ctx.Err()
				}
			}
			if c.FinishReason != nil && (*c.FinishReason == "stop" || *c.FinishReason == "tool_calls") {
				// finish_reason 到达时 usage chunk 可能还没来，先不发 done
				// 让 SSE 流继续读完，usage chunk 会更新 pendingUsage，最后统一发
			}
			return nil
		})
		// 流读完后发 done（含 usage 如果有）
		doneChunk := ChatChunk{Done: true}
		if pendingUsage != nil {
			doneChunk.PromptTokens = pendingUsage.PromptTokens
			doneChunk.CompletionTokens = pendingUsage.CompletionTokens
			doneChunk.TotalTokens = pendingUsage.TotalTokens
		}
		send(doneChunk)
		if err != nil && err != io.EOF {
			finish(err, pendingUsage, firstTokenDurationMS)
			return
		}
		finish(nil, pendingUsage, firstTokenDurationMS)
	}()

	return ch, nil
}

// ---- Seedream 图像生成实现（占位）----

type seedreamAdapter struct {
	baseURL string
	apiKey  string
}

func newSeedream(baseURL, apiKey string) ImageAdapter {
	return &seedreamAdapter{baseURL: baseURL, apiKey: apiKey}
}

func (a *seedreamAdapter) Generate(_ context.Context, req ImageRequest) (*ImageResult, error) {
	// TODO: 接入 Seedream 正式 API（待官方文档发布）
	_ = req
	return nil, errno.ErrNotImplemented.WithMessage("seedream adapter not yet implemented")
}

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
)

// ---- 公共类型 ----

// Message 是对话消息（与 OpenAI Chat API 对齐）。
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// ChatRequest 是 LLM 对话请求参数。
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// ChatChunk 是流式响应的一个增量 chunk。
// ReasoningContent 是 thinking 推理 token（如 deepseek-reasoner 模型）。
type ChatChunk struct {
	Content          string
	ReasoningContent string
	Done             bool
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
	return newOpenAI(p.BaseURL, p.APIKey, p.DefaultModel), nil
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
	client *httpclient.Client
	model  string
}

func newOpenAI(baseURL, apiKey, defaultModel string) LLMAdapter {
	return &openaiAdapter{
		client: httpclient.New(baseURL, 120*time.Second, map[string]string{
			"Authorization": "Bearer " + apiKey,
		}),
		model: defaultModel,
	}
}

type completionResp struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *openaiAdapter) Chat(ctx context.Context, req ChatRequest) (string, error) {
	if req.Model == "" {
		req.Model = a.model
	}
	req.Stream = false

	var resp completionResp
	if err := a.client.Do(ctx, http.MethodPost, "/v1/chat/completions", req, &resp); err != nil {
		return "", fmt.Errorf("openai chat: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("openai error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: empty choices")
	}
	return resp.Choices[0].Message.Content, nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (a *openaiAdapter) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	if req.Model == "" {
		req.Model = a.model
	}
	req.Stream = true

	resp, err := a.client.DoStream(ctx, http.MethodPost, "/v1/chat/completions", req)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	ch := make(chan ChatChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		send := func(out ChatChunk) bool {
			select {
			case ch <- out:
				return true
			case <-ctx.Done():
				return false
			}
		}

		err := httpclient.ReadSSELines(resp.Body, func(data string) error {
			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Choices) == 0 {
				return nil
			}
			c := chunk.Choices[0]
			out := ChatChunk{Content: c.Delta.Content, ReasoningContent: c.Delta.ReasoningContent}
			if out.Content != "" || out.ReasoningContent != "" {
				if !send(out) {
					return ctx.Err()
				}
			}
			if c.FinishReason != nil && *c.FinishReason == "stop" {
				send(ChatChunk{Done: true})
			}
			return nil
		})
		if err != nil && err != io.EOF {
			select {
			case ch <- ChatChunk{Done: true}:
			default:
			}
		}
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

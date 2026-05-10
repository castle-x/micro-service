// Package adapter 提供 AI 模型适配器接口与 DeepSeek 实现。
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/castlexu/micro-service/pkg/httpclient"
)

// Message 是对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 是 LLM 对话请求。
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// ChatChunk 是流式响应的一个 chunk。
// Content 是普通回复 token；ReasoningContent 是 thinking 模式的推理 token。
type ChatChunk struct {
	Content          string
	ReasoningContent string
	Done             bool
}

// LLMAdapter 是 LLM 适配器接口。
type LLMAdapter interface {
	// Chat 发起非流式对话，返回完整 assistant 消息。
	Chat(ctx context.Context, req ChatRequest) (string, error)
	// ChatStream 发起流式对话，通过 chan 逐块返回内容，调用方负责消费直到 Done=true 或 chan 关闭。
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// deepseekAdapter 实现 OpenAI 兼容的 DeepSeek LLM 调用。
type deepseekAdapter struct {
	client *httpclient.Client
	model  string
}

// NewDeepSeek 构造 DeepSeek 适配器。baseURL 示例："https://api.deepseek.com"。
func NewDeepSeek(baseURL, apiKey, defaultModel string) LLMAdapter {
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	return &deepseekAdapter{
		client: httpclient.New(baseURL, 120*time.Second, headers),
		model:  defaultModel,
	}
}

// openAIResp 是 OpenAI 兼容接口的非流式响应。
type openAIResp struct {
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

func (a *deepseekAdapter) Chat(ctx context.Context, req ChatRequest) (string, error) {
	if req.Model == "" {
		req.Model = a.model
	}
	req.Stream = false

	var resp openAIResp
	if err := a.client.Do(ctx, http.MethodPost, "/v1/chat/completions", req, &resp); err != nil {
		return "", fmt.Errorf("deepseek chat: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("deepseek error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("deepseek: empty choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// openAIStreamChunk 是 OpenAI 兼容接口的流式 chunk。
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (a *deepseekAdapter) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	if req.Model == "" {
		req.Model = a.model
	}
	req.Stream = true

	resp, err := a.client.DoStream(ctx, http.MethodPost, "/v1/chat/completions", req)
	if err != nil {
		return nil, fmt.Errorf("deepseek stream: %w", err)
	}

	ch := make(chan ChatChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		err := httpclient.ReadSSELines(resp.Body, func(data string) error {
			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return nil
			}
			if len(chunk.Choices) == 0 {
				return nil
			}
			c := chunk.Choices[0]
			out := ChatChunk{
				Content:          c.Delta.Content,
				ReasoningContent: c.Delta.ReasoningContent,
			}
			if out.Content != "" || out.ReasoningContent != "" {
				select {
				case ch <- out:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if c.FinishReason != nil && *c.FinishReason == "stop" {
				select {
				case ch <- ChatChunk{Done: true}:
				case <-ctx.Done():
				}
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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	mdlbiz "github.com/castlexu/micro-service/services/model/biz"
)

// ChatHandler 处理 LLM 对话请求。
type ChatHandler struct {
	biz *mdlbiz.ChatBiz
}

// NewChatHandler 构造 ChatHandler。
func NewChatHandler(biz *mdlbiz.ChatBiz) *ChatHandler {
	return &ChatHandler{biz: biz}
}

type chatReq struct {
	Slug     string               `json:"slug"`
	Messages []mdlbiz.ChatMessage `json:"messages"`

	// 采样参数
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`

	// 输出格式
	ResponseFormat *mdlbiz.ResponseFormat `json:"response_format,omitempty"`

	// Tool / Function Calling
	Tools      []mdlbiz.Tool `json:"tools,omitempty"`
	ToolChoice any           `json:"tool_choice,omitempty"`

	// Thinking（DeepSeek 私有）
	Thinking *mdlbiz.ThinkingConfig `json:"thinking,omitempty"`
}

func (r *chatReq) toOpts() *mdlbiz.ChatOptions {
	return &mdlbiz.ChatOptions{
		Temperature:    r.Temperature,
		MaxTokens:      r.MaxTokens,
		TopP:           r.TopP,
		Stop:           r.Stop,
		ResponseFormat: r.ResponseFormat,
		Tools:          r.Tools,
		ToolChoice:     r.ToolChoice,
		Thinking:       r.Thinking,
	}
}

// Chat POST /api/v1/model/chat — 非流式
func (h *ChatHandler) Chat(c context.Context, ctx *app.RequestContext) {
	var req chatReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, map[string]any{"code": errno.ErrInvalidParam.Code, "message": err.Error()})
		return
	}
	if req.Slug == "" {
		ctx.JSON(http.StatusBadRequest, map[string]any{"code": errno.ErrInvalidParam.Code, "message": "slug required"})
		return
	}
	if len(req.Messages) == 0 {
		ctx.JSON(http.StatusBadRequest, map[string]any{"code": errno.ErrInvalidParam.Code, "message": "messages required"})
		return
	}
	content, err := h.biz.Chat(c, req.Slug, req.Messages, req.toOpts())
	if err != nil {
		logger.Ctx(c).Error("chat failed", zap.String("slug", req.Slug), zap.Error(err))
		code, msg := errCode(err)
		ctx.JSON(code, map[string]any{"code": code, "message": msg})
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"code": 0, "data": map[string]string{"content": content}})
}

// streamChunk 是 SSE 事件的 JSON payload。
type streamChunk struct {
	Type    string `json:"type"`              // "reasoning" | "content" | "done" | "error"
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"` // error 时使用
}

// ChatStream POST /api/v1/model/chat/stream — SSE 流式输出
//
// 使用 io.Pipe + SetBodyStream 实现真正的逐 chunk 推送。
// ctx.BodyWriter() + ctx.Flush() 在 Hertz netpoll 下是缓冲写法，不能保证逐帧推送。
//
// SSE 事件格式：
//
//	data: {"type":"reasoning","content":"..."}  ← thinking token（reasoner 模型）
//	data: {"type":"content","content":"..."}    ← 普通回复 token
//	data: {"type":"done"}                       ← 流结束
//	data: {"type":"error","message":"..."}      ← 错误
func (h *ChatHandler) ChatStream(c context.Context, ctx *app.RequestContext) {
	var req chatReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, map[string]any{"code": errno.ErrInvalidParam.Code, "message": err.Error()})
		return
	}
	if req.Slug == "" {
		ctx.JSON(http.StatusBadRequest, map[string]any{"code": errno.ErrInvalidParam.Code, "message": "slug required"})
		return
	}
	if len(req.Messages) == 0 {
		ctx.JSON(http.StatusBadRequest, map[string]any{"code": errno.ErrInvalidParam.Code, "message": "messages required"})
		return
	}

	ch, err := h.biz.ChatStream(c, req.Slug, req.Messages, req.toOpts())
	if err != nil {
		logger.Ctx(c).Error("chat stream init failed", zap.String("slug", req.Slug), zap.Error(err))
		code, msg := errCode(err)
		ctx.JSON(code, map[string]any{"code": code, "message": msg})
		return
	}

	ctx.Response.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	ctx.Response.Header.Set("X-Accel-Buffering", "no")
	ctx.SetStatusCode(http.StatusOK)

	// io.Pipe：写端每次 fmt.Fprintf(pw) 都会阻塞直到读端消费，
	// 框架通过 SetBodyStream 读取 pr，每读一块就立即发送给客户端。
	pr, pw := io.Pipe()
	ctx.Response.SetBodyStream(pr, -1)

	go func() {
		defer pw.Close()

		writeEvent := func(chunk streamChunk) {
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(pw, "data: %s\n\n", b) //nolint:errcheck
		}

		for {
			select {
			case <-c.Done():
				return
			case out, ok := <-ch:
				if !ok {
					writeEvent(streamChunk{Type: "done"})
					return
				}
				if out.Done {
					writeEvent(streamChunk{Type: "done"})
					return
				}
				if out.ReasoningContent != "" {
					writeEvent(streamChunk{Type: "reasoning", Content: out.ReasoningContent})
				}
				if out.Content != "" {
					writeEvent(streamChunk{Type: "content", Content: out.Content})
				}
			}
		}
	}()
}

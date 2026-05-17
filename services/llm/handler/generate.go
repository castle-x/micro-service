package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	llmbiz "github.com/castlexu/micro-service/services/llm/biz"
)

const (
	headerCaller   = "X-Caller"
	headerUserID   = "X-User-ID"
	headerTenantID = "X-Tenant-ID"
)

// GenerateService is the generate business seam used by HTTP handlers.
type GenerateService interface {
	Generate(context.Context, llmbiz.GenerateReq) (*llmbiz.GenerateResp, error)
	Stream(context.Context, llmbiz.GenerateReq) (<-chan llmbiz.StreamEvent, error)
}

// GenerateHandler handles Generate and Stream calls.
type GenerateHandler struct {
	biz GenerateService
}

// NewGenerateHandler constructs GenerateHandler.
func NewGenerateHandler(biz GenerateService) *GenerateHandler {
	return &GenerateHandler{biz: biz}
}

// Generate handles POST /api/v1/llm/generate.
func (h *GenerateHandler) Generate(c context.Context, ctx *app.RequestContext) {
	var req llmbiz.GenerateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	c = bindTrustedMetadata(c, ctx, &req)
	resp, err := h.biz.Generate(c, req)
	if err != nil {
		logger.Ctx(c).Error("llm generate failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, resp, nil)
}

// Stream handles POST /api/v1/llm/stream.
func (h *GenerateHandler) Stream(c context.Context, ctx *app.RequestContext) {
	var req llmbiz.GenerateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	c = bindTrustedMetadata(c, ctx, &req)
	events, err := h.biz.Stream(c, req)
	if err != nil {
		logger.Ctx(c).Error("llm stream init failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}

	ctx.Response.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	ctx.Response.Header.Set("X-Accel-Buffering", "no")
	ctx.SetStatusCode(http.StatusOK)

	pr, pw := io.Pipe()
	ctx.Response.SetBodyStream(pr, -1)

	go func() {
		defer pw.Close()
		for event := range events {
			if err := writeSSEEvent(pw, event); err != nil {
				return
			}
		}
	}()
}

func bindTrustedMetadata(c context.Context, ctx *app.RequestContext, req *llmbiz.GenerateReq) context.Context {
	req.Caller = strings.TrimSpace(string(ctx.GetHeader(headerCaller)))
	req.UserID = strings.TrimSpace(string(ctx.GetHeader(headerUserID)))
	req.TenantID = strings.TrimSpace(string(ctx.GetHeader(headerTenantID)))
	if req.Caller != "" {
		c = logger.WithCaller(c, req.Caller)
	}
	if req.UserID != "" {
		c = logger.WithUserID(c, req.UserID)
	}
	if req.TenantID != "" {
		c = logger.WithTenantID(c, req.TenantID)
	}
	return c
}

func writeSSEEvent(w io.Writer, event llmbiz.StreamEvent) error {
	payload := streamPayload(event)
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := llmbiz.ValidateStreamEventLimit(raw, llmbiz.LimitConfig{}); err != nil {
		raw, _ = json.Marshal(streamPayload(llmbiz.StreamEvent{
			Type:      llmbiz.StreamEventError,
			RequestID: event.RequestID,
			Error: &llmbiz.ErrorBody{
				Code:      errno.ErrLLMInvalidMessage.Code,
				Message:   "stream event exceeds size limit",
				RequestID: event.RequestID,
			},
		}))
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, raw)
	return err
}

func streamPayload(event llmbiz.StreamEvent) any {
	switch event.Type {
	case llmbiz.StreamEventReasoningDelta, llmbiz.StreamEventContentDelta:
		return map[string]any{"request_id": event.RequestID, "content": event.Content}
	case llmbiz.StreamEventToolCallDelta:
		return map[string]any{
			"request_id":      event.RequestID,
			"index":           event.Index,
			"id":              event.ID,
			"name":            event.Name,
			"arguments_delta": event.ArgumentsDelta,
		}
	case llmbiz.StreamEventMessageCompleted:
		return map[string]any{"request_id": event.RequestID, "message": event.Message}
	case llmbiz.StreamEventUsage:
		return event.Usage
	case llmbiz.StreamEventDone:
		return map[string]any{"request_id": event.RequestID, "finish_reason": event.FinishReason, "model_ref": event.ModelRef}
	case llmbiz.StreamEventError:
		if event.Error != nil {
			return event.Error
		}
		return map[string]any{"code": errno.ErrInternal.Code, "message": "internal server error", "request_id": event.RequestID}
	default:
		return map[string]any{"request_id": event.RequestID}
	}
}

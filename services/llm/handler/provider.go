// Package handler provides Hertz HTTP handlers for the llm service.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	llmbiz "github.com/castlexu/micro-service/services/llm/biz"
)

// ProviderService is the provider business seam used by HTTP handlers.
type ProviderService interface {
	List(context.Context) ([]llmbiz.ProviderDTO, error)
	Create(context.Context, llmbiz.ProviderCreateReq) (*llmbiz.Provider, error)
	Update(context.Context, string, llmbiz.ProviderUpdateReq) (*llmbiz.ProviderDTO, error)
	UpdateAPIKey(context.Context, string, string) error
	SetEnabled(context.Context, string, bool) (*llmbiz.ProviderDTO, error)
	Delete(context.Context, string) error
	Test(context.Context, string, string) (*llmbiz.ProviderTestResult, error)
}

// ProviderHandler handles provider CRUD.
type ProviderHandler struct {
	biz ProviderService
}

// NewProviderHandler constructs ProviderHandler.
func NewProviderHandler(biz ProviderService) *ProviderHandler {
	return &ProviderHandler{biz: biz}
}

func reply(ctx *app.RequestContext, data any, err error) {
	if err != nil {
		code, msg := errCode(err)
		ctx.JSON(code, map[string]any{"code": code, "message": msg})
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"code": 0, "data": data})
}

func errCode(err error) (int, string) {
	var e errno.Errno
	if errors.As(err, &e) {
		switch e.Code {
		case errno.ErrInvalidParam.Code:
			return http.StatusBadRequest, e.Message
		case errno.ErrDuplicateKey.Code:
			return http.StatusConflict, e.Message
		case errno.ErrLLMProviderNotFound.Code, errno.ErrLLMModelNotFound.Code:
			return http.StatusNotFound, e.Message
		case errno.ErrLLMProviderDisabled.Code, errno.ErrLLMModelDisabled.Code:
			return http.StatusForbidden, e.Message
		case errno.ErrLLMAdapterUnsupported.Code, errno.ErrLLMModelCapabilityUnsupported.Code:
			return http.StatusUnprocessableEntity, e.Message
		case errno.ErrLLMInvalidMessage.Code:
			return http.StatusBadRequest, e.Message
		case errno.ErrLLMRateLimited.Code:
			return http.StatusTooManyRequests, e.Message
		case errno.ErrLLMUpstream.Code:
			return http.StatusBadGateway, e.Message
		case errno.ErrNotImplemented.Code:
			return http.StatusNotImplemented, e.Message
		}
		return http.StatusInternalServerError, e.Message
	}
	return http.StatusInternalServerError, "internal server error"
}

// List handles GET /api/v1/llm/providers.
func (h *ProviderHandler) List(c context.Context, ctx *app.RequestContext) {
	items, err := h.biz.List(c)
	if err != nil {
		logger.Ctx(c).Error("list llm providers failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, items, nil)
}

// ListProviders is kept for the current router interface.
func (h *ProviderHandler) ListProviders(c context.Context, ctx *app.RequestContext) {
	h.List(c, ctx)
}

// Create handles POST /api/v1/llm/providers.
func (h *ProviderHandler) Create(c context.Context, ctx *app.RequestContext) {
	var req llmbiz.ProviderCreateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	p, err := h.biz.Create(c, req)
	if err != nil {
		logger.Ctx(c).Error("create llm provider failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]string{"id": p.ID.Hex()}, nil)
}

// CreateProvider is kept for the current router interface.
func (h *ProviderHandler) CreateProvider(c context.Context, ctx *app.RequestContext) {
	h.Create(c, ctx)
}

// Update handles PUT /api/v1/llm/providers/:id.
func (h *ProviderHandler) Update(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var req llmbiz.ProviderUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	dto, err := h.biz.Update(c, id, req)
	if err != nil {
		logger.Ctx(c).Error("update llm provider failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, dto, nil)
}

// UpdateAPIKey handles PATCH /api/v1/llm/providers/:id/api-key.
func (h *ProviderHandler) UpdateAPIKey(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var body struct {
		APIKey string `json:"api_key"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	if err := h.biz.UpdateAPIKey(c, id, body.APIKey); err != nil {
		logger.Ctx(c).Error("update llm provider api_key failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]bool{"ok": true}, nil)
}

// SetEnabled handles PATCH /api/v1/llm/providers/:id/enabled.
func (h *ProviderHandler) SetEnabled(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	dto, err := h.biz.SetEnabled(c, id, body.Enabled)
	if err != nil {
		logger.Ctx(c).Error("set llm provider enabled failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, dto, nil)
}

// Delete handles DELETE /api/v1/llm/providers/:id.
func (h *ProviderHandler) Delete(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	if err := h.biz.Delete(c, id); err != nil {
		logger.Ctx(c).Error("delete llm provider failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]bool{"ok": true}, nil)
}

// Test handles POST /api/v1/llm/providers/:id/test.
func (h *ProviderHandler) Test(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var body struct {
		ModelRef string `json:"model_ref"`
	}
	if len(ctx.Request.Body()) > 0 {
		if err := ctx.BindJSON(&body); err != nil {
			reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
			return
		}
	}
	result, err := h.biz.Test(c, id, body.ModelRef)
	if err != nil {
		logger.Ctx(c).Error("test llm provider failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, result, nil)
}

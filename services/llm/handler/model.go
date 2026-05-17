package handler

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	llmbiz "github.com/castlexu/micro-service/services/llm/biz"
)

// ModelService is the model business seam used by HTTP handlers.
type ModelService interface {
	List(context.Context, string, *bool) ([]llmbiz.ModelDTO, error)
	Create(context.Context, llmbiz.ModelCreateReq) (*llmbiz.Model, error)
	Update(context.Context, string, llmbiz.ModelUpdateReq) (*llmbiz.ModelDTO, error)
	SetEnabled(context.Context, string, bool) (*llmbiz.ModelDTO, error)
	Delete(context.Context, string) error
}

// ModelHandler handles model CRUD.
type ModelHandler struct {
	biz ModelService
}

// NewModelHandler constructs ModelHandler.
func NewModelHandler(biz ModelService) *ModelHandler {
	return &ModelHandler{biz: biz}
}

// List handles GET /api/v1/llm/models.
func (h *ModelHandler) List(c context.Context, ctx *app.RequestContext) {
	providerSlug := string(ctx.QueryArgs().Peek("provider_slug"))
	var enabled *bool
	if raw := string(ctx.QueryArgs().Peek("enabled")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			reply(ctx, nil, errno.ErrInvalidParam.WithMessage("enabled must be boolean"))
			return
		}
		enabled = &parsed
	}
	items, err := h.biz.List(c, providerSlug, enabled)
	if err != nil {
		logger.Ctx(c).Error("list llm models failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, items, nil)
}

// ListModels is a named alias for router variants.
func (h *ModelHandler) ListModels(c context.Context, ctx *app.RequestContext) {
	h.List(c, ctx)
}

// Create handles POST /api/v1/llm/models.
func (h *ModelHandler) Create(c context.Context, ctx *app.RequestContext) {
	var req llmbiz.ModelCreateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	m, err := h.biz.Create(c, req)
	if err != nil {
		logger.Ctx(c).Error("create llm model failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]string{"id": m.ID.Hex(), "model_ref": m.ModelRef}, nil)
}

// CreateModel is a named alias for router variants.
func (h *ModelHandler) CreateModel(c context.Context, ctx *app.RequestContext) {
	h.Create(c, ctx)
}

// Update handles PUT /api/v1/llm/models/:id.
func (h *ModelHandler) Update(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var req llmbiz.ModelUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	dto, err := h.biz.Update(c, id, req)
	if err != nil {
		logger.Ctx(c).Error("update llm model failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, dto, nil)
}

// UpdateModel is a named alias for router variants.
func (h *ModelHandler) UpdateModel(c context.Context, ctx *app.RequestContext) {
	h.Update(c, ctx)
}

// SetEnabled handles PATCH /api/v1/llm/models/:id/enabled.
func (h *ModelHandler) SetEnabled(c context.Context, ctx *app.RequestContext) {
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
		logger.Ctx(c).Error("set llm model enabled failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, dto, nil)
}

// SetModelEnabled is a named alias for router variants.
func (h *ModelHandler) SetModelEnabled(c context.Context, ctx *app.RequestContext) {
	h.SetEnabled(c, ctx)
}

// Delete handles DELETE /api/v1/llm/models/:id.
func (h *ModelHandler) Delete(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	if err := h.biz.Delete(c, id); err != nil {
		logger.Ctx(c).Error("delete llm model failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]bool{"ok": true}, nil)
}

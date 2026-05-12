// Package handler 提供 model service 的 Hertz HTTP handler。
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	mdlbiz "github.com/castlexu/micro-service/services/model/biz"
	mdlmodel "github.com/castlexu/micro-service/services/model/dal/model"
)

// ProviderHandler 处理 provider CRUD。
type ProviderHandler struct {
	biz *mdlbiz.ProviderBiz
}

// NewProviderHandler 构造 ProviderHandler。
func NewProviderHandler(biz *mdlbiz.ProviderBiz) *ProviderHandler {
	return &ProviderHandler{biz: biz}
}

// reply 统一响应格式。
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
		case errno.ErrProviderNotFound.Code:
			return http.StatusNotFound, e.Message
		case errno.ErrProviderDisabled.Code:
			return http.StatusForbidden, e.Message
		case errno.ErrAdapterUnsupported.Code:
			return http.StatusUnprocessableEntity, e.Message
		case errno.ErrNotImplemented.Code:
			return http.StatusNotImplemented, e.Message
		case errno.ErrUpstreamLLM.Code:
			// 上游 LLM 错误：透传原始 message，用 502 区别于内部错误
			return http.StatusBadGateway, e.Message
		}
		return http.StatusInternalServerError, e.Message
	}
	return http.StatusInternalServerError, "internal server error"
}

// ListProviders GET /api/v1/model/providers
func (h *ProviderHandler) ListProviders(c context.Context, ctx *app.RequestContext) {
	providers, err := h.biz.List(c)
	if err != nil {
		logger.Ctx(c).Error("list providers failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	// 不暴露 api_key
	type item struct {
		ID           string                `json:"id"`
		Name         string                `json:"name"`
		Slug         string                `json:"slug"`
		Type         mdlmodel.ProviderType `json:"type"`
		BaseURL      string                `json:"base_url"`
		DefaultModel string                `json:"default_model"`
		Enabled      bool                  `json:"enabled"`
		CreatedAt    int64                 `json:"created_at"`
	}
	result := make([]item, 0, len(providers))
	for _, p := range providers {
		result = append(result, item{
			ID:           p.ID.Hex(),
			Name:         p.Name,
			Slug:         p.Slug,
			Type:         p.Type,
			BaseURL:      p.BaseURL,
			DefaultModel: p.DefaultModel,
			Enabled:      p.Enabled,
			CreatedAt:    p.CreatedAt,
		})
	}
	reply(ctx, result, nil)
}

// CreateProvider POST /api/v1/model/providers
func (h *ProviderHandler) CreateProvider(c context.Context, ctx *app.RequestContext) {
	var req mdlbiz.CreateReq
	if err := ctx.BindJSON(&req); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	p, err := h.biz.Create(c, req)
	if err != nil {
		logger.Ctx(c).Error("create provider failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]string{"id": p.ID.Hex()}, nil)
}

// SetEnabled PATCH /api/v1/model/providers/:id/enabled
func (h *ProviderHandler) SetEnabled(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	if err := h.biz.SetEnabled(c, id, body.Enabled); err != nil {
		logger.Ctx(c).Error("set provider enabled failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]bool{"ok": true}, nil)
}

// UpdateAPIKey PATCH /api/v1/model/providers/:id/api_key
func (h *ProviderHandler) UpdateAPIKey(c context.Context, ctx *app.RequestContext) {
	id := ctx.Param("id")
	var body struct {
		APIKey string `json:"api_key"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage(err.Error()))
		return
	}
	if body.APIKey == "" {
		reply(ctx, nil, errno.ErrInvalidParam.WithMessage("api_key required"))
		return
	}
	if err := h.biz.UpdateAPIKey(c, id, body.APIKey); err != nil {
		logger.Ctx(c).Error("update provider api_key failed", zap.Error(err))
		reply(ctx, nil, err)
		return
	}
	reply(ctx, map[string]bool{"ok": true}, nil)
}

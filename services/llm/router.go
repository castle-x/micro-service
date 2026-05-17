package main

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

	mwhertz "github.com/castlexu/micro-service/pkg/middleware/hertz"
)

// ProviderHandler 定义 provider 管理路由所需的 handler 方法。
type ProviderHandler interface {
	ListProviders(context.Context, *app.RequestContext)
	CreateProvider(context.Context, *app.RequestContext)
	Update(context.Context, *app.RequestContext)
	SetEnabled(context.Context, *app.RequestContext)
	UpdateAPIKey(context.Context, *app.RequestContext)
	Delete(context.Context, *app.RequestContext)
	Test(context.Context, *app.RequestContext)
}

// ModelHandler 定义 model 管理路由所需的 handler 方法。
type ModelHandler interface {
	ListModels(context.Context, *app.RequestContext)
	CreateModel(context.Context, *app.RequestContext)
	UpdateModel(context.Context, *app.RequestContext)
	SetModelEnabled(context.Context, *app.RequestContext)
	Delete(context.Context, *app.RequestContext)
}

// GenerateHandler 定义 LLM 调用路由所需的 handler 方法。
type GenerateHandler interface {
	Generate(context.Context, *app.RequestContext)
	Stream(context.Context, *app.RequestContext)
}

// RegisterRoutes 注册 llm service 所有路由。
func RegisterRoutes(h *server.Hertz, ph ProviderHandler, mh ModelHandler, gh GenerateHandler) {
	h.Use(mwhertz.Trace(), mwhertz.Recovery(), mwhertz.Logging())

	v1 := h.Group("/api/v1/llm")
	{
		if ph != nil {
			providers := v1.Group("/providers")
			{
				providers.GET("", ph.ListProviders)
				providers.POST("", ph.CreateProvider)
				providers.PUT("/:id", ph.Update)
				providers.DELETE("/:id", ph.Delete)
				providers.PATCH("/:id/enabled", ph.SetEnabled)
				providers.PATCH("/:id/api-key", ph.UpdateAPIKey)
				providers.POST("/:id/test", ph.Test)
			}
		}
		if mh != nil {
			models := v1.Group("/models")
			{
				models.GET("", mh.ListModels)
				models.POST("", mh.CreateModel)
				models.PUT("/:id", mh.UpdateModel)
				models.DELETE("/:id", mh.Delete)
				models.PATCH("/:id/enabled", mh.SetModelEnabled)
			}
		}
		if gh != nil {
			v1.POST("/generate", gh.Generate)
			v1.POST("/stream", gh.Stream)
		}
	}
}

package main

import (
	"github.com/cloudwego/hertz/pkg/app/server"

	mwhertz "github.com/castlexu/micro-service/pkg/middleware/hertz"
	mdlhandler "github.com/castlexu/micro-service/services/model/handler"
)

// RegisterRoutes 注册 model service 所有路由。
func RegisterRoutes(h *server.Hertz, ph *mdlhandler.ProviderHandler, ch *mdlhandler.ChatHandler) {
	h.Use(mwhertz.Recovery(), mwhertz.Trace(), mwhertz.Logging())

	v1 := h.Group("/api/v1/model")
	{
		providers := v1.Group("/providers")
		{
			providers.GET("", ph.ListProviders)
			providers.POST("", ph.CreateProvider)
			providers.PATCH("/:id/enabled", ph.SetEnabled)
			providers.PATCH("/:id/api_key", ph.UpdateAPIKey)
		}
		v1.POST("/chat", ch.Chat)
		v1.POST("/chat/stream", ch.ChatStream)
	}
}

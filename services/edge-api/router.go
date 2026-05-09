package main

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

	mwhertz "github.com/castlexu/micro-service/pkg/middleware/hertz"
	"github.com/castlexu/micro-service/services/edge-api/handler"
)

// corsMiddleware 为前端开发服务器添加 CORS 支持。
// 允许 frontendURL origin 的跨域请求，并处理 OPTIONS preflight。
func corsMiddleware(frontendURL string) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		origin := string(ctx.GetHeader("Origin"))
		if origin == frontendURL {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Credentials", "true")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		// 处理 OPTIONS preflight，直接返回 204
		if string(ctx.Method()) == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next(c)
	}
}

// RegisterRoutes 注册所有 HTTP 路由。
func RegisterRoutes(h *server.Hertz, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, frontendURL string) {
	// 全局中间件
	h.Use(mwhertz.Recovery(), mwhertz.Trace(), mwhertz.Logging())
	// CORS 中间件
	h.Use(corsMiddleware(frontendURL))

	v1 := h.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			// GET /api/v1/auth/google/url — 获取 Google 授权 URL
			auth.GET("/google/url", authHandler.GetGoogleAuthURL)
			// GET /api/v1/auth/google/callback — Google OAuth2 回调（重定向到前端）
			auth.GET("/google/callback", authHandler.GoogleCallback)
			// GET /api/v1/auth/alipay/url — 获取支付宝扫码授权 URL
			auth.GET("/alipay/url", authHandler.GetAlipayAuthURL)
			// GET /api/v1/auth/alipay/callback — 支付宝回调（重定向到前端）
			auth.GET("/alipay/callback", authHandler.AlipayCallback)
			// POST /api/v1/auth/token/refresh — 刷新 access token
			auth.POST("/token/refresh", authHandler.RefreshToken)
			// POST /api/v1/auth/logout — 登出
			auth.POST("/logout", authHandler.Logout)
		}

		user := v1.Group("/user")
		{
			// GET /api/v1/user/me — 获取当前用户信息
			user.GET("/me", userHandler.GetMe)
		}
	}
}

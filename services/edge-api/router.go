package main

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

	mwhertz "github.com/castlexu/micro-service/pkg/middleware/hertz"
	"github.com/castlexu/micro-service/services/edge-api/handler"
	iamclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam/iamservice"
	idpclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp/idpservice"
	edgemw "github.com/castlexu/micro-service/services/edge-api/middleware"
)

func corsMiddleware(frontendURL string) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		origin := string(ctx.GetHeader("Origin"))
		if origin == frontendURL {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Credentials", "true")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if string(ctx.Method()) == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next(c)
	}
}

// RegisterRoutes 注册所有 HTTP 路由。
func RegisterRoutes(
	h *server.Hertz,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	adminHandler *handler.AdminHandler,
	modelProxy *handler.ModelProxy,
	idpCli idpclient.Client,
	iamCli iamclient.Client,
	jwtSecret []byte,
	frontendURL string,
) {
	h.Use(mwhertz.Trace(), mwhertz.Recovery(), mwhertz.Logging())
	h.Use(corsMiddleware(frontendURL))

	authMw := edgemw.Auth(jwtSecret, idpCli)

	v1 := h.Group("/api/v1")
	{
		// ---- 公开认证路由（无需登录）----
		auth := v1.Group("/auth")
		{
			auth.GET("/google/url", authHandler.GetGoogleAuthURL)
			auth.GET("/google/callback", authHandler.GoogleCallback)
			auth.GET("/alipay/url", authHandler.GetAlipayAuthURL)
			auth.GET("/alipay/callback", authHandler.AlipayCallback)
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.LoginByPassword)
			auth.POST("/token/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authHandler.Logout)
		}

		// ---- 需要登录的用户路由 ----
		user := v1.Group("/user", authMw)
		{
			user.GET("/me", userHandler.GetMe)
		}

		// ---- 管理后台路由（需要登录 + 对应权限）----
		admin := v1.Group("/admin", authMw)
		{
			// 用户管理
			admin.GET("/users", edgemw.RequirePermission("user:read", iamCli), adminHandler.ListUsers)
			admin.PUT("/users/:id/role", edgemw.RequirePermission("user:role:assign", iamCli), adminHandler.UpdateUserRole)
			admin.PUT("/users/:id/status", edgemw.RequirePermission("user:status:update", iamCli), adminHandler.UpdateUserStatus)

			// 角色管理
			admin.GET("/roles", edgemw.RequirePermission("role:read", iamCli), adminHandler.ListRoles)
			admin.POST("/roles", edgemw.RequirePermission("role:write", iamCli), adminHandler.CreateRole)
			admin.PUT("/roles/:id", edgemw.RequirePermission("role:write", iamCli), adminHandler.UpdateRole)
			admin.DELETE("/roles/:id", edgemw.RequirePermission("role:write", iamCli), adminHandler.DeleteRole)

			// 权限管理
			admin.GET("/permissions", edgemw.RequirePermission("permission:read", iamCli), adminHandler.ListPermissions)
			admin.POST("/permissions", edgemw.RequirePermission("permission:write", iamCli), adminHandler.CreatePermission)

			// Model Providers（转发到 model service）
			models := admin.Group("/models", edgemw.RequirePermission("model:admin", iamCli))
			{
				models.GET("/providers", modelProxy.ProxyModels)
				models.POST("/providers", modelProxy.ProxyModels)
				models.PATCH("/providers/:id/enabled", modelProxy.ProxyModels)
				models.PATCH("/providers/:id/api_key", modelProxy.ProxyModels)
				models.POST("/chat", modelProxy.ProxyModels)
				models.POST("/chat/stream", modelProxy.ProxyStream)
			}
		}
	}
}

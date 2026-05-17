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
	assetHandler *handler.AssetHandler,
	llmProxy *handler.LLMProxy,
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

		// ---- 个人资产库路由 ----
		assets := v1.Group("/assets", authMw)
		{
			assets.POST("/types", assetHandler.CreateAssetType)
			assets.GET("/types", assetHandler.ListAssetTypes)
			assets.GET("/types/:id", assetHandler.GetAssetType)
			assets.PUT("/types/:id", assetHandler.UpdateAssetType)
			assets.DELETE("/types/:id", assetHandler.DeleteAssetType)

			assets.POST("/categories", assetHandler.CreateAssetCategory)
			assets.GET("/categories", assetHandler.ListAssetCategories)
			assets.PUT("/categories/:id", assetHandler.UpdateAssetCategory)
			assets.DELETE("/categories/:id", assetHandler.DeleteAssetCategory)

			assets.POST("/media/upload-sessions", assetHandler.CreateStorageUploadSession)
			assets.POST("/media/upload-sessions/:session_id/finalize", assetHandler.FinalizeStorageUploadSession)
			assets.GET("/media", assetHandler.ListMediaObjects)
			assets.GET("/media/:id/access-url", assetHandler.GetMediaObjectAccessURL)
			assets.GET("/media/:id", assetHandler.GetMediaObject)

			assets.POST("/", assetHandler.CreateAsset)
			assets.GET("/", assetHandler.ListAssets)
			assets.POST("/:id/versions", assetHandler.CreateAssetVersion)
			assets.GET("/:id/versions", assetHandler.ListAssetVersions)
			assets.GET("/:id/versions/current", assetHandler.GetCurrentAssetVersion)
			assets.PUT("/:id/versions/current", assetHandler.SetCurrentAssetVersion)
			assets.GET("/:id/versions/:version", assetHandler.GetAssetVersion)
			assets.POST("/:id/versions/:version/copy", assetHandler.CopyAssetVersion)
			assets.GET("/:id", assetHandler.GetAsset)
			assets.PUT("/:id", assetHandler.UpdateAsset)
			assets.PUT("/:id/library-state", assetHandler.SetAssetLibraryState)
			assets.DELETE("/:id", assetHandler.DeleteAsset)
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

			// LLM admin（转发到 llm service）
			llm := admin.Group("/llm", edgemw.RequirePermission("llm:admin", iamCli))
			{
				llm.GET("/providers", llmProxy.ProxyLLM)
				llm.POST("/providers", llmProxy.ProxyLLM)
				llm.PUT("/providers/:id", llmProxy.ProxyLLM)
				llm.DELETE("/providers/:id", llmProxy.ProxyLLM)
				llm.PATCH("/providers/:id/api-key", llmProxy.ProxyLLM)
				llm.PATCH("/providers/:id/enabled", llmProxy.ProxyLLM)
				llm.POST("/providers/:id/test", llmProxy.ProxyLLM)

				llm.GET("/models", llmProxy.ProxyLLM)
				llm.POST("/models", llmProxy.ProxyLLM)
				llm.PUT("/models/:id", llmProxy.ProxyLLM)
				llm.DELETE("/models/:id", llmProxy.ProxyLLM)
				llm.PATCH("/models/:id/enabled", llmProxy.ProxyLLM)

				llm.POST("/generate", llmProxy.ProxyLLM)
				llm.POST("/stream", llmProxy.ProxyStream)
			}
		}
	}
}

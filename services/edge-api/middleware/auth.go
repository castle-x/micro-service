// Package middleware 提供 edge-api 的鉴权中间件。
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	pkgjwt "github.com/castlexu/micro-service/pkg/jwt"
	"github.com/castlexu/micro-service/pkg/logger"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	iambase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
	idpbase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
	iamgen "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam"
	iamclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam/iamservice"
	idpgen "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp"
	idpclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp/idpservice"
)

// ctxKey 用于在 Hertz context 中传递已解析的 claims。
const (
	ctxKeyUserID = "auth_user_id"
	ctxKeyRole   = "auth_role"
)

type apiResp struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

// Auth 解析并校验 Bearer token，将 user_id 和 role 注入 context。
// 所有需要登录的接口都应通过此中间件。
func Auth(jwtSecret []byte, idpCli idpclient.Client) app.HandlerFunc {
	verifier, _ := pkgjwt.NewHS256Verifier(jwtSecret)
	return func(c context.Context, ctx *app.RequestContext) {
		authHeader := string(ctx.GetHeader("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			ctx.JSON(http.StatusUnauthorized, apiResp{Code: errno.ErrUnauthorized.Code, Message: "missing or invalid Authorization header"})
			ctx.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 先尝试本地验签（快速路径）
		claims, err := verifier.Verify(token)
		if err != nil {
			// 降级：调 IDP VerifyToken（处理黑名单检查）
			resp, rpcErr := idpCli.VerifyToken(c, &idpgen.VerifyTokenReq{
				Base:  &idpbase.BaseReq{},
				Token: token,
			})
			if rpcErr != nil || (resp.GetBase() != nil && resp.Base.Code != 0) {
				ctx.JSON(http.StatusUnauthorized, apiResp{Code: errno.ErrUnauthorized.Code, Message: "invalid or expired token"})
				ctx.Abort()
				return
			}
			ctx.Set(ctxKeyUserID, resp.UserID)
			ctx.Set(ctxKeyRole, "")
			ctx.Next(c)
			return
		}

		// 本地验签成功：检查封禁标记（与 IDP 共享同一 Redis key）
		if claims.UserID != "" {
			if rdb := pkgredis.GetClient(); rdb != nil {
				if _, getErr := rdb.Get(c, pkgredis.Key("idp", "banned", claims.UserID)); getErr == nil {
					// key 存在 = 被封禁
					ctx.JSON(http.StatusUnauthorized, apiResp{Code: errno.ErrAccountLocked.Code, Message: "account is banned"})
					ctx.Abort()
					return
				}
			}
		}

		ctx.Set(ctxKeyUserID, claims.UserID)
		ctx.Set(ctxKeyRole, claims.Role)
		ctx.Next(c)
	}
}

// RequirePermission 检查当前用户是否拥有所需权限。
// super_admin 直接放行；其他角色查 IAM GetRolePermissions（有 Redis 缓存）。
func RequirePermission(permission string, iamCli iamclient.Client) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		role, _ := ctx.Get(ctxKeyRole)
		roleStr, _ := role.(string)

		// super_admin bypass
		if roleStr == "super_admin" {
			ctx.Next(c)
			return
		}

		if roleStr == "" {
			ctx.JSON(http.StatusForbidden, apiResp{Code: errno.ErrForbidden.Code, Message: "未登录或 Token 无效，无法访问此资源"})
			ctx.Abort()
			return
		}

		// 查角色权限（IAM 侧有 Redis 缓存 TTL=5min）
		resp, err := iamCli.GetRolePermissions(c, &iamgen.GetRolePermissionsReq{
			Base:     &iambase.BaseReq{},
			RoleName: roleStr,
		})
		if err != nil {
			logger.Ctx(c).Error("iam.GetRolePermissions failed", zap.Error(err))
			ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: "权限检查失败，请稍后重试"})
			ctx.Abort()
			return
		}
		if resp.GetBase() != nil && resp.Base.Code != 0 {
			logger.Ctx(c).Warn("GetRolePermissions returned error",
				zap.String("role", roleStr),
				zap.Int32("code", resp.Base.Code),
				zap.String("msg", resp.Base.Message),
			)
			ctx.JSON(http.StatusForbidden, apiResp{Code: errno.ErrForbidden.Code,
				Message: "角色 " + roleStr + " 不存在或权限查询失败：" + resp.Base.Message})
			ctx.Abort()
			return
		}

		for _, p := range resp.Permissions {
			if p == permission || p == "*" {
				ctx.Next(c)
				return
			}
		}

		ctx.JSON(http.StatusForbidden, apiResp{Code: errno.ErrPermissionDenied.Code,
			Message: "权限不足：角色 " + roleStr + " 缺少 " + permission + " 权限"})
		ctx.Abort()
	}
}

// GetUserID 从 Hertz context 中取出已认证的 user_id。
func GetUserID(ctx *app.RequestContext) string {
	v, _ := ctx.Get(ctxKeyUserID)
	s, _ := v.(string)
	return s
}

// GetRole 从 Hertz context 中取出已认证的 role。
func GetRole(ctx *app.RequestContext) string {
	v, _ := ctx.Get(ctxKeyRole)
	s, _ := v.(string)
	return s
}

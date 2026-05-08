// Package hertz 提供 Hertz HTTP 通用中间件。
//
// 三件套：
//   - Trace()    —— 从 X-Trace-ID header 取/生成 trace_id，注入 response header + metainfo + logger ctx
//   - Recovery() —— panic 兜底：打 stacktrace + 返回 500 + errno.ErrInternal
//   - Logging()  —— 记录 method / path / status / duration，不记录 body
//
// 使用示例：
//
//	h := server.Default()
//	h.Use(hertzmw.Recovery(), hertzmw.Trace(), hertzmw.Logging())
package hertz

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	"github.com/castlexu/micro-service/pkg/utils"
)

const (
	// HeaderTraceID 客户端/上游约定的 trace_id 请求头。
	HeaderTraceID = "X-Trace-ID"
	// HeaderUserID 可选：Kong 鉴权后注入的 user_id。
	HeaderUserID = "X-User-ID"
	// HeaderTenantID 可选：多租户场景下的租户标识。
	HeaderTenantID = "X-Tenant-ID"
)

// Trace 注入 trace_id：header 已带则复用，否则生成新的 uuid；
// 同时写入 response header（便于客户端排障）、metainfo（Kitex 出站透传用）、
// logger ctx key（logger.Ctx 直接读到）。
func Trace() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		traceID := string(ctx.GetHeader(HeaderTraceID))
		if traceID == "" {
			traceID = utils.NewID()
		}
		userID := string(ctx.GetHeader(HeaderUserID))
		tenantID := string(ctx.GetHeader(HeaderTenantID))

		c = mw.WithMeta(c, traceID, "edge-api", userID, tenantID)
		c = logger.WithTraceID(c, traceID)
		if userID != "" {
			c = logger.WithUserID(c, userID)
		}
		if tenantID != "" {
			c = logger.WithTenantID(c, tenantID)
		}
		ctx.Response.Header.Set(HeaderTraceID, traceID)
		ctx.Next(c)
	}
}

// Recovery 兜住 panic，写 500 + errno.ErrInternal。
func Recovery() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.Ctx(c).Error("hertz panic recovered",
					zap.Any("panic", r),
					zap.String("stack", stack),
				)
				e := errno.ErrInternal.WithMessagef("panic: %v", r)
				ctx.AbortWithStatusJSON(500, map[string]any{
					"code":    e.Code,
					"message": e.Message,
				})
			}
		}()
		ctx.Next(c)
	}
}

// Logging 打 method / path / status / duration。
func Logging() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		start := time.Now()
		ctx.Next(c)
		dur := time.Since(start)

		fields := []zap.Field{
			zap.ByteString("method", ctx.Method()),
			zap.ByteString("path", ctx.Path()),
			zap.Int("status", ctx.Response.StatusCode()),
			zap.Duration("duration", dur),
		}
		if ctx.Response.StatusCode() >= 500 {
			logger.Ctx(c).Error("hertz request error", fields...)
		} else {
			logger.Ctx(c).Info("hertz request", fields...)
		}
	}
}

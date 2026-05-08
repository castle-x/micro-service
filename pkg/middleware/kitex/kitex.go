// Package kitex 提供 Kitex 服务端/客户端通用中间件。
//
// 三件套：
//   - Trace()   —— 服务端：从 metainfo 取 trace_id，未携带则生成 uuid；客户端：将 ctx 中
//     logger 私有 ctx key 的 trace_id 透传到 metainfo，供下游服务消费。
//   - Recovery() —— panic 兜底：打 stacktrace + 返回 errno.ErrInternal。
//   - Logging() —— 记录 method / duration / code，不记录 req/resp body（防 PII 泄漏）。
//
// 使用示例（服务端）：
//
//	svr := xxxservice.NewServer(handler,
//	    server.WithMiddleware(mwkitex.Trace()),
//	    server.WithMiddleware(mwkitex.Recovery()),
//	    server.WithMiddleware(mwkitex.Logging()),
//	)
package kitex

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	"github.com/castlexu/micro-service/pkg/utils"
)

// Trace 返回一个 Kitex 中间件：确保 ctx 上携带 trace_id。
// 若 metainfo 中已有 trace_id 则复用；否则生成新的 uuid 并注入。
// 同时也将 logger 私有 ctx key 中的 trace_id 同步到 metainfo（用于出站调用透传）。
func Trace() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			traceID := mw.TraceIDFromContext(ctx)
			if traceID == "" {
				traceID = utils.NewID()
			}
			ctx = mw.WithMeta(ctx, traceID, "", "", "")
			ctx = logger.WithTraceID(ctx, traceID)
			return next(ctx, req, resp)
		}
	}
}

// Recovery 在下游 panic 时兜底：打日志 + 返回 ErrInternal。
func Recovery() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					logger.Ctx(ctx).Error("kitex panic recovered",
						zap.Any("panic", r),
						zap.String("stack", stack),
					)
					err = errno.ErrInternal.WithMessagef("panic: %v", r)
				}
			}()
			return next(ctx, req, resp)
		}
	}
}

// Logging 记录请求方法名、耗时、错误码。禁止记录 req/resp body（防 PII）。
func Logging() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			start := time.Now()
			err := next(ctx, req, resp)
			dur := time.Since(start)

			method := methodFromCtx(ctx)
			code := errnoCode(err)

			fields := []zap.Field{
				zap.String("method", method),
				zap.Duration("duration", dur),
				zap.Int32("code", code),
			}
			if err != nil {
				logger.Ctx(ctx).Error("kitex call failed", append(fields, zap.String("error", err.Error()))...)
			} else {
				logger.Ctx(ctx).Info("kitex call ok", fields...)
			}
			return err
		}
	}
}

// methodFromCtx 尝试从 Kitex rpcinfo 获取 method 名；失败返回 "unknown"。
func methodFromCtx(ctx context.Context) string {
	info := rpcinfo.GetRPCInfo(ctx)
	if info == nil || info.Invocation() == nil {
		return "unknown"
	}
	if m := info.Invocation().MethodName(); m != "" {
		return fmt.Sprintf("%s/%s", info.Invocation().ServiceName(), m)
	}
	return "unknown"
}

// errnoCode 从 err 中提取 errno.Code，不是 Errno 时返回 ErrInternal.Code 或 0。
func errnoCode(err error) int32 {
	if err == nil {
		return 0
	}
	var e errno.Errno
	if errors.As(err, &e) {
		return e.Code
	}
	return errno.ErrInternal.Code
}

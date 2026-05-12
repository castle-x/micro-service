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
	"strings"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
			ctx = otel.GetTextMapPropagator().Extract(ctx, metainfoCarrier{ctx: &ctx})

			traceID := mw.TraceIDFromContext(ctx)
			method := methodFromCtx(ctx)
			tracer := otel.Tracer("github.com/castlexu/micro-service/pkg/middleware/kitex")
			ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			if traceID == "" {
				traceID = spanTraceID(span)
			}
			if traceID == "" {
				traceID = utils.NewID()
			}
			ctx = mw.WithMeta(ctx, traceID, "", "", "")
			ctx = logger.WithTraceID(ctx, traceID)
			err := next(ctx, req, resp)
			span.SetAttributes(
				attribute.String("rpc.system", "kitex"),
				attribute.String("rpc.method", method),
			)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.Int64("error.code", int64(errnoCode(err))))
			}
			return err
		}
	}
}

// ClientTrace starts a Kitex client span and injects W3C trace context into metainfo.
func ClientTrace() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			method := methodFromCtx(ctx)
			tracer := otel.Tracer("github.com/castlexu/micro-service/pkg/middleware/kitex")
			ctx, span := tracer.Start(ctx, "RPC "+method, trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			traceID := mw.TraceIDFromContext(ctx)
			if traceID == "" {
				traceID = spanTraceID(span)
			}
			ctx = mw.WithMeta(ctx, traceID, "", "", "")
			ctx = logger.WithTraceID(ctx, traceID)
			otel.GetTextMapPropagator().Inject(ctx, metainfoCarrier{ctx: &ctx})
			ctx = metainfo.TransferForward(ctx)

			err := next(ctx, req, resp)
			span.SetAttributes(
				attribute.String("rpc.system", "kitex"),
				attribute.String("rpc.method", method),
			)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.Int64("error.code", int64(errnoCode(err))))
			}
			return err
		}
	}
}

type metainfoCarrier struct {
	ctx *context.Context
}

func (c metainfoCarrier) Get(key string) string {
	if c.ctx == nil || *c.ctx == nil {
		return ""
	}
	value, _ := metainfo.GetPersistentValue(*c.ctx, key)
	return value
}

func (c metainfoCarrier) Set(key, value string) {
	if c.ctx == nil || *c.ctx == nil || value == "" {
		return
	}
	*c.ctx = metainfo.WithPersistentValue(*c.ctx, key, value)
}

func (c metainfoCarrier) Keys() []string {
	return []string{"traceparent", "baggage"}
}

// Recovery 在下游 panic 时兜底：打日志 + 返回 ErrInternal。
func Recovery() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					err = errno.ErrInternal.WithMessagef("panic: %v", r)
					span := trace.SpanFromContext(ctx)
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.SetAttributes(attribute.Int64("error.code", int64(errnoCode(err))))
					logger.Ctx(ctx).Error("kitex panic recovered",
						zap.Any("panic", r),
						zap.String("stack", stack),
						zap.Int32("error_code", errnoCode(err)),
					)
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
				logger.Ctx(ctx).Error("kitex call failed", append(fields,
					zap.Int32("error_code", code),
					zap.String("error", err.Error()),
				)...)
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

func spanTraceID(span trace.Span) string {
	sc := span.SpanContext()
	if !sc.IsValid() || !sc.TraceID().IsValid() {
		return ""
	}
	traceID := sc.TraceID().String()
	if strings.Trim(traceID, "0") == "" {
		return ""
	}
	return traceID
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

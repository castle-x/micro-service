// Package middleware 提供 Kitex 与 Hertz 两套通用中间件。
//
// 子目录组织：
//   - pkg/middleware/kitex  —— Kitex 服务端/客户端中间件（trace / recovery / logging）
//   - pkg/middleware/hertz  —— Hertz HTTP 中间件（trace / recovery / logging）
//
// 共享常量与 logger.SetMetaInfoExtractor 注册逻辑定义在本文件，供两套子包复用。
package middleware

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/castlexu/micro-service/pkg/logger"
)

// Metainfo 持久化键名常量（由 Kitex/Hertz 在 RPC/HTTP 调用链中透传）。
//
// 说明：
//   - Kitex 使用 github.com/bytedance/gopkg/cloud/metainfo 的 Persistent KV 存储，
//     格式 "RPC_PERSIST_<Key>"，与 Kitex 默认的 transmeta 协议兼容；
//   - Hertz 接入时，在服务端 middleware 里将 X-Trace-ID 等 header 写入本包 logger
//     context key（logger.WithTraceID），同时注入到 metainfo，便于后续 Kitex 客户端透传。
const (
	MetaKeyTraceID  = "trace_id"
	MetaKeyCaller   = "caller"
	MetaKeyUserID   = "user_id"
	MetaKeyTenantID = "tenant_id"
)

// RegisterLoggerExtractor 将 logger.SetMetaInfoExtractor 注册为从 metainfo 读取链路
// 元数据。main.go 启动时显式调用一次即可让 logger.Ctx(ctx) 自动携带 trace_id 等。
//
// 为什么不在 init() 自动注册：
//   - 避免被测试代码隐式依赖；
//   - Hertz 纯 HTTP 场景可能不走 metainfo，留给业务层根据部署情况决定。
func RegisterLoggerExtractor() {
	logger.SetMetaInfoExtractor(func(ctx context.Context) (traceID, caller, userID, tenantID string) {
		if ctx == nil {
			return
		}
		traceID, _ = metainfo.GetPersistentValue(ctx, MetaKeyTraceID)
		caller, _ = metainfo.GetPersistentValue(ctx, MetaKeyCaller)
		userID, _ = metainfo.GetPersistentValue(ctx, MetaKeyUserID)
		tenantID, _ = metainfo.GetPersistentValue(ctx, MetaKeyTenantID)
		return
	})
}

// WithMeta 将四个元数据以 Persistent 形式写入 ctx，空值跳过。
// 由 kitex/hertz 子包在中间件中调用。
func WithMeta(ctx context.Context, traceID, caller, userID, tenantID string) context.Context {
	if traceID != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetaKeyTraceID, traceID)
	}
	if caller != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetaKeyCaller, caller)
	}
	if userID != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetaKeyUserID, userID)
	}
	if tenantID != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetaKeyTenantID, tenantID)
	}
	return ctx
}

// TraceIDFromContext 从 metainfo 或本包 logger ctx key 读取 trace_id。
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := metainfo.GetPersistentValue(ctx, MetaKeyTraceID); ok && v != "" {
		return v
	}
	return logger.TraceIDFromContext(ctx)
}

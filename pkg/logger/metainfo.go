package logger

import "context"

// MetaInfoExtractor 从 ctx 中提取链路元数据。
// Kitex / Hertz / OpenTelemetry 等链路框架可提供各自实现，
// 在 main.go 通过 SetMetaInfoExtractor 注入。未设置时仅从本包的
// WithTraceID / WithCaller 等 context key 中取值。
type MetaInfoExtractor func(ctx context.Context) (traceID, caller, userID, tenantID string)

var metaInfoExtractor MetaInfoExtractor

// SetMetaInfoExtractor 注入链路元数据提取器。一般在服务启动时调用一次。
// 传 nil 可恢复为仅读 context key 的默认行为。
func SetMetaInfoExtractor(fn MetaInfoExtractor) {
	metaInfoExtractor = fn
}

// extractMeta 优先使用外部注入的 extractor（可对接 Kitex metainfo 等），
// 再回退到 context.Value 读取本包定义的 key。两种来源以 extractor 优先，
// 为空则取 context key 的值作为兜底。
func extractMeta(ctx context.Context) (traceID, caller, userID, tenantID string) {
	if ctx == nil {
		return "", "", "", ""
	}
	if metaInfoExtractor != nil {
		traceID, caller, userID, tenantID = metaInfoExtractor(ctx)
	}
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}
	if caller == "" {
		caller = CallerFromContext(ctx)
	}
	if userID == "" {
		userID = UserIDFromContext(ctx)
	}
	if tenantID == "" {
		tenantID = TenantIDFromContext(ctx)
	}
	return
}

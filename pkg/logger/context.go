package logger

import "context"

// ctxKey 是 logger 在 context 中注入/读取元数据用的私有键类型。
// 业务代码不应直接构造 ctxKey，请使用 WithTraceID / WithCaller 等封装函数。
type ctxKey int

const (
	keyTraceID ctxKey = iota + 1
	keyCaller
	keyUserID
	keyTenantID
)

// WithTraceID 在 ctx 上附加 trace_id。返回新 ctx。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil || traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, keyTraceID, traceID)
}

// WithCaller 在 ctx 上附加调用方服务名（主调）。
func WithCaller(ctx context.Context, caller string) context.Context {
	if ctx == nil || caller == "" {
		return ctx
	}
	return context.WithValue(ctx, keyCaller, caller)
}

// WithUserID 在 ctx 上附加当前登录用户 ID。
func WithUserID(ctx context.Context, userID string) context.Context {
	if ctx == nil || userID == "" {
		return ctx
	}
	return context.WithValue(ctx, keyUserID, userID)
}

// WithTenantID 在 ctx 上附加租户 ID。
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if ctx == nil || tenantID == "" {
		return ctx
	}
	return context.WithValue(ctx, keyTenantID, tenantID)
}

// TraceIDFromContext 从 ctx 中取 trace_id；若不存在返回空串。
func TraceIDFromContext(ctx context.Context) string {
	return stringValue(ctx, keyTraceID)
}

// CallerFromContext 从 ctx 中取 caller。
func CallerFromContext(ctx context.Context) string {
	return stringValue(ctx, keyCaller)
}

// UserIDFromContext 从 ctx 中取 user_id。
func UserIDFromContext(ctx context.Context) string {
	return stringValue(ctx, keyUserID)
}

// TenantIDFromContext 从 ctx 中取 tenant_id。
func TenantIDFromContext(ctx context.Context) string {
	return stringValue(ctx, keyTenantID)
}

func stringValue(ctx context.Context, key ctxKey) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}

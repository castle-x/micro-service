// Package errno 定义项目统一的错误码体系。
//
// 区段分配（见 SPEC.md §5.1）：
//   - 系统级      : 10001 - 10999
//   - IDP         : 11001 - 11999
//   - IAM         : 12001 - 12999
//   - Billing     : 13001 - 13999
//   - Credits     : 14001 - 14999
//   - Notification: 15001 - 15999
//   - Model       : 16001 - 16999
//   - Model       : 16001 - 16999
//
// 使用规则：
//   - 业务层禁止 errors.New / 裸 fmt.Errorf，必须返回本包预定义的 Errno。
//   - Handler 层直接透传 biz 返回的 Errno，禁止二次包装。
//   - 错误消息允许携带业务上下文（order_id / user_id 等），但禁止包含密码 / Token。
package errno

// ---- 系统级 10001 - 10999 ----
var (
	ErrInternal           = New(10001, "internal server error")
	ErrInvalidParam       = New(10002, "invalid parameter")
	ErrUnauthorized       = New(10003, "unauthorized")
	ErrForbidden          = New(10004, "forbidden")
	ErrNotFound           = New(10005, "resource not found")
	ErrRateLimit          = New(10006, "rate limit exceeded")
	ErrServiceUnavailable = New(10007, "service unavailable")
	ErrCacheMiss          = New(10008, "cache miss")
	ErrNotImplemented     = New(10009, "not implemented")
	ErrDuplicateKey       = New(10010, "duplicate key")
)

// ---- IDP 11001 - 11999 ----
var (
	ErrInvalidCredentials = New(11001, "invalid credentials")
	ErrTokenExpired       = New(11002, "token expired")
	ErrTokenInvalid       = New(11003, "token invalid")
	ErrMFARequired        = New(11004, "mfa required")
	ErrAccountLocked      = New(11005, "account locked")
)

// ---- IAM 12001 - 12999 ----
var (
	ErrUserNotFound     = New(12001, "user not found")
	ErrRoleNotFound     = New(12002, "role not found")
	ErrPermissionDenied = New(12003, "permission denied")
)

// ---- Billing 13001 - 13999 ----
var (
	ErrOrderNotFound    = New(13001, "order not found")
	ErrOrderAlreadyPaid = New(13002, "order already paid")
	ErrPaymentFailed    = New(13003, "payment failed")
	ErrChannelError     = New(13004, "payment channel error")
	ErrWebhookInvalid   = New(13005, "webhook invalid")
)

// ---- Credits 14001 - 14999 ----
var (
	ErrInsufficientCredits = New(14001, "insufficient credits")
	ErrCreditsFrozen       = New(14002, "credits frozen")
	ErrDuplicateConsume    = New(14003, "duplicate consume")
)

// ---- Notification 15001 - 15999 ----
var (
	ErrTemplateNotFound  = New(15001, "template not found")
	ErrChannelSendFailed = New(15002, "channel send failed")
)

// ---- Model 16001 - 16999 ----
var (
	ErrProviderNotFound    = New(16001, "model provider not found")
	ErrProviderDisabled    = New(16002, "model provider disabled")
	ErrAdapterUnsupported  = New(16003, "model adapter unsupported")
	ErrUpstreamLLM         = New(16004, "upstream llm error")
)

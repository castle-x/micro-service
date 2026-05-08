package biz

// WebhookBiz 处理支付渠道回调的验签、幂等、状态流转。
type WebhookBiz struct{}

// NewWebhookBiz 构造 WebhookBiz。
func NewWebhookBiz() *WebhookBiz { return &WebhookBiz{} }

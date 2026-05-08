package biz

// PaymentBiz 处理渠道支付对接（微信 / 支付宝 / Stripe）。
type PaymentBiz struct{}

// NewPaymentBiz 构造 PaymentBiz。
func NewPaymentBiz() *PaymentBiz { return &PaymentBiz{} }

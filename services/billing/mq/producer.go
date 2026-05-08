// Package mq 封装 billing 的 NSQ 消息发送。
// 支付成功后发布 order.paid 事件，由 credits / notification 订阅。
package mq

// OrderEventProducer 发送订单相关事件。
type OrderEventProducer struct{}

// NewOrderEventProducer 构造 OrderEventProducer。
func NewOrderEventProducer() *OrderEventProducer { return &OrderEventProducer{} }

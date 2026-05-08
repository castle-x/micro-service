// Package mq 封装 credits 的 NSQ 订阅：消费 order.paid 触发积分赠送。
package mq

// OrderPaidConsumer 订阅 order.paid 事件。
type OrderPaidConsumer struct{}

// NewOrderPaidConsumer 构造 OrderPaidConsumer。
func NewOrderPaidConsumer() *OrderPaidConsumer { return &OrderPaidConsumer{} }

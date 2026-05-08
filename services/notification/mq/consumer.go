// Package mq 封装 notification 的 NSQ 订阅：消费业务事件触发通知发送。
package mq

// EventConsumer 订阅 order.paid / credit.changed 等事件。
type EventConsumer struct{}

// NewEventConsumer 构造 EventConsumer。
func NewEventConsumer() *EventConsumer { return &EventConsumer{} }

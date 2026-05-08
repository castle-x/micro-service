// Package mq 定义消息队列的抽象接口。
//
// 本阶段（Phase 02）为 L1 占位：仅暴露接口 + NotImplemented 实现，
// 真实 NSQ 接入推迟到 Phase 05（billing → credits 的事件驱动流水线）。
package mq

import (
	"context"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
)

// Message 消息载体。
type Message struct {
	Topic     string
	Payload   []byte
	Headers   map[string]string
	Timestamp time.Time
}

// HandlerFunc 消费者回调：返回 error 代表处理失败（由实现决定是否重投）。
type HandlerFunc func(ctx context.Context, msg *Message) error

// Producer 生产者。实现应保证并发安全。
type Producer interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Close() error
}

// Consumer 消费者。
type Consumer interface {
	Subscribe(ctx context.Context, topic string, handler HandlerFunc) error
	Close() error
}

// NotImplementedProducer 占位实现。
type NotImplementedProducer struct{}

func (NotImplementedProducer) Publish(context.Context, string, []byte) error {
	return errno.ErrNotImplemented.WithMessage("mq.Publish: not implemented (pkg/mq is L1 skeleton)")
}
func (NotImplementedProducer) Close() error { return nil }

// NotImplementedConsumer 占位实现。
type NotImplementedConsumer struct{}

func (NotImplementedConsumer) Subscribe(context.Context, string, HandlerFunc) error {
	return errno.ErrNotImplemented.WithMessage("mq.Subscribe: not implemented")
}
func (NotImplementedConsumer) Close() error { return nil }

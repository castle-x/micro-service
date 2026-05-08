// Package nsq 是 pkg/mq 的 NSQ 实现占位。
//
// 本阶段仅暴露构造函数与 NotImplemented 行为，不引入 go-nsq 依赖。
// Phase 05 接入时在此文件替换为真实实现，调用方 import path 不变。
package nsq

import (
	"context"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/mq"
)

// Config NSQ 配置（预留）。
type Config struct {
	NSQDAddrs       []string `mapstructure:"nsqd_addrs"`        // 生产者连接地址
	LookupdAddrs    []string `mapstructure:"lookupd_addrs"`     // 消费者服务发现地址
	MaxInFlight     int      `mapstructure:"max_in_flight"`     // 消费者并发
	ConsumerChannel string   `mapstructure:"consumer_channel"`  // 消费者 channel 名
}

// Producer NSQ 生产者（占位）。
type Producer struct {
	cfg Config
}

// NewProducer 构造 Producer。
func NewProducer(cfg Config) (mq.Producer, error) {
	return &Producer{cfg: cfg}, nil
}

// Publish 占位。
func (p *Producer) Publish(ctx context.Context, topic string, payload []byte) error {
	_ = ctx
	_ = topic
	_ = payload
	return errno.ErrNotImplemented.WithMessage("nsq.Publish: not implemented (Phase 02 skeleton)")
}

// Close 占位。
func (p *Producer) Close() error { return nil }

// Consumer NSQ 消费者（占位）。
type Consumer struct {
	cfg Config
}

// NewConsumer 构造 Consumer。
func NewConsumer(cfg Config) (mq.Consumer, error) {
	return &Consumer{cfg: cfg}, nil
}

// Subscribe 占位。
func (c *Consumer) Subscribe(ctx context.Context, topic string, handler mq.HandlerFunc) error {
	_ = ctx
	_ = topic
	_ = handler
	return errno.ErrNotImplemented.WithMessage("nsq.Subscribe: not implemented")
}

// Close 占位。
func (c *Consumer) Close() error { return nil }

// Package etcd 是 pkg/registry 的 etcd 实现占位。
//
// 本阶段（Phase 02）仅暴露 New* 构造函数与 NotImplemented 行为，
// 不引入 go.etcd.io/etcd/client/v3 依赖，避免 pkg/go.mod 体积膨胀。
//
// Phase 03/04 接入 etcd 时在此文件内完成真实实现，调用方 import path 不变。
package etcd

import (
	"context"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/registry"
)

// Config etcd 连接配置（预留）。
type Config struct {
	Endpoints []string `mapstructure:"endpoints"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
}

// Registry etcd 注册器（占位）。
type Registry struct {
	cfg Config
}

// NewRegistry 构造 etcd Registry。Phase 02 阶段返回占位实例。
func NewRegistry(cfg Config) (registry.Registry, error) {
	return &Registry{cfg: cfg}, nil
}

// Register 占位：返回 ErrNotImplemented。
func (r *Registry) Register(ctx context.Context, info registry.ServiceInfo) error {
	_ = ctx
	_ = info
	return errno.ErrNotImplemented.WithMessage("etcd.Registry.Register: not implemented (Phase 02 skeleton)")
}

// Deregister 占位。
func (r *Registry) Deregister(ctx context.Context, info registry.ServiceInfo) error {
	_ = ctx
	_ = info
	return errno.ErrNotImplemented.WithMessage("etcd.Registry.Deregister: not implemented")
}

// Close 占位。
func (r *Registry) Close() error { return nil }

// Resolver etcd 解析器（占位）。
type Resolver struct {
	cfg Config
}

// NewResolver 构造 etcd Resolver。
func NewResolver(cfg Config) (registry.Resolver, error) {
	return &Resolver{cfg: cfg}, nil
}

// Resolve 占位：返回 ErrNotImplemented。
func (r *Resolver) Resolve(ctx context.Context, name string) ([]registry.Endpoint, error) {
	_ = ctx
	_ = name
	return nil, errno.ErrNotImplemented.WithMessage("etcd.Resolver.Resolve: not implemented")
}

// Close 占位。
func (r *Resolver) Close() error { return nil }

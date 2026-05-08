// Package registry 定义服务注册与发现的抽象接口。
//
// 本阶段（Phase 02）为 L1 占位：仅暴露接口 + NotImplemented 实现，
// 真实 etcd 接入推迟到 Phase 03/04（业务服务需要跨进程调用时）。
//
// 设计取舍：
//   - 把接口与实现解耦到子目录（registry/etcd/），后续增加 nacos/consul 不影响调用方 import；
//   - 占位实现全部返回 errno.ErrNotImplemented，让未初始化时的调用显式失败（而非沉默）。
package registry

import (
	"context"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
)

// Endpoint 代表一个服务实例的网络位置。
type Endpoint struct {
	Addr     string            // host:port
	Weight   int               // 权重，负载均衡用
	Metadata map[string]string // 自由键值对（版本、可用区等）
}

// ServiceInfo 表示本服务实例的注册信息。
type ServiceInfo struct {
	Name     string            // 服务名，如 "idp"
	Addr     string            // 监听地址 host:port
	TTL      time.Duration     // 心跳过期时间
	Metadata map[string]string // 自由元数据
}

// Registry 服务端侧：注册/注销本服务实例。
type Registry interface {
	Register(ctx context.Context, info ServiceInfo) error
	Deregister(ctx context.Context, info ServiceInfo) error
	Close() error
}

// Resolver 客户端侧：按服务名发现可用实例。
type Resolver interface {
	Resolve(ctx context.Context, serviceName string) ([]Endpoint, error)
	Close() error
}

// NotImplementedRegistry 占位实现：所有方法返回 ErrNotImplemented。
// Phase 03/04 引入 etcd 之前，业务层可以先面向 Registry 接口开发。
type NotImplementedRegistry struct{}

func (NotImplementedRegistry) Register(context.Context, ServiceInfo) error {
	return errno.ErrNotImplemented.WithMessage("registry.Register: not implemented (pkg/registry is L1 skeleton)")
}
func (NotImplementedRegistry) Deregister(context.Context, ServiceInfo) error {
	return errno.ErrNotImplemented.WithMessage("registry.Deregister: not implemented")
}
func (NotImplementedRegistry) Close() error { return nil }

// NotImplementedResolver 占位实现。
type NotImplementedResolver struct{}

func (NotImplementedResolver) Resolve(context.Context, string) ([]Endpoint, error) {
	return nil, errno.ErrNotImplemented.WithMessage("registry.Resolve: not implemented")
}
func (NotImplementedResolver) Close() error { return nil }

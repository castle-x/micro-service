package biz

import "context"

const (
	// ServiceName 是 asset 在注册发现中的服务名。
	ServiceName = "asset"
	// HealthStatusOK 是 Health RPC 的静态健康状态。
	HealthStatusOK = "ok"
)

// HealthBiz 承载 AS-01 的最小探活逻辑。
type HealthBiz struct{}

// NewHealthBiz 构造 HealthBiz。
func NewHealthBiz() *HealthBiz {
	return &HealthBiz{}
}

// Check 返回服务静态健康信息。真实依赖探测留到生产化阶段。
func (b *HealthBiz) Check(ctx context.Context) (service string, status string, err error) {
	_ = ctx
	return ServiceName, HealthStatusOK, nil
}

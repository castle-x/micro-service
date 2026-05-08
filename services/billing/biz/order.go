// Package biz 是 billing 核心业务逻辑层。
package biz

// OrderBiz 处理订单创建 / 查询 / 对账。
type OrderBiz struct{}

// NewOrderBiz 构造 OrderBiz。
func NewOrderBiz() *OrderBiz { return &OrderBiz{} }

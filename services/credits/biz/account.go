// Package biz 是 credits 核心业务逻辑层。
package biz

// AccountBiz 处理积分账户业务。
type AccountBiz struct{}

// NewAccountBiz 构造 AccountBiz。
func NewAccountBiz() *AccountBiz { return &AccountBiz{} }

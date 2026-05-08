// Package biz 是 notification 核心业务逻辑层。
package biz

// TemplateBiz 管理通知模板 CRUD 与渲染。
type TemplateBiz struct{}

// NewTemplateBiz 构造 TemplateBiz。
func NewTemplateBiz() *TemplateBiz { return &TemplateBiz{} }

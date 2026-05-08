// Package mongo 封装 notification 的 MongoDB 访问。
package mongo

// TemplateMongo 封装 templates 集合的 CRUD。
type TemplateMongo struct{}

// NewTemplateMongo 构造 TemplateMongo。
func NewTemplateMongo() *TemplateMongo { return &TemplateMongo{} }

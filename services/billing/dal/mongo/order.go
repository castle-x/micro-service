// Package mongo 封装 billing 的 MongoDB 访问。
package mongo

// OrderMongo 封装 orders 集合的 CRUD。
type OrderMongo struct{}

// NewOrderMongo 构造 OrderMongo。
func NewOrderMongo() *OrderMongo { return &OrderMongo{} }

// Package mongo 封装 credits 的 MongoDB 访问。
package mongo

// AccountMongo 封装 accounts 集合的 CRUD。
type AccountMongo struct{}

// NewAccountMongo 构造 AccountMongo。
func NewAccountMongo() *AccountMongo { return &AccountMongo{} }

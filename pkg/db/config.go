package db

import (
	"time"

	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoConfig 描述 MongoDB 连接参数。
//
// 必填：URI；其他字段有合理默认值。
type MongoConfig struct {
	// URI 完整的 MongoDB 连接串，可选 mongodb:// 或 mongodb+srv://。
	URI string

	// DBName 默认数据库名。若为空则使用 URI 中 path 上的 database；都为空则退化为 "admin"。
	DBName string

	// PoolSize 连接池上限（MaxPoolSize），默认 100。
	PoolSize uint64

	// WMajority 为 true 时使用 {w:"majority", j:true} 写关注；false 使用 {w:1}。
	WMajority bool

	// ConnectTimeout 初始化连接与 Ping 的超时时间，默认 5s。
	ConnectTimeout time.Duration

	// ReadPreference 默认读偏好。为 nil 时使用 primary。
	ReadPreference *readpref.ReadPref

	// DisableAutoConnect 为 true 时 InitMongo 只构造 Client 对象，不实际拨号 / Ping，
	// 适合单元测试或按需连接场景。
	DisableAutoConnect bool
}

// withDefaults 补齐默认值，返回新副本，避免修改原始 cfg。
func (c MongoConfig) withDefaults() MongoConfig {
	if c.PoolSize == 0 {
		c.PoolSize = 100
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 5 * time.Second
	}
	if c.ReadPreference == nil {
		c.ReadPreference = readpref.Primary()
	}
	if c.DBName == "" {
		c.DBName = "admin"
	}
	return c
}

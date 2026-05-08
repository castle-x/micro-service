// Package db 封装 MongoDB 连接管理与通用仓储。
//
// 设计要点：
//   - 统一通过 InitMongo(cfg) 构造 *Client，内部持有 *mongo.Client 与默认 *mongo.Database。
//   - 业务层通过泛型 Repository[T BaseDocument] 使用 CRUD，自动注入 deleted_at 软删除过滤。
//   - 强类型 Options（FindOptions / UpdateOptions 等）替代旧的 bson.M + 类型断言。
//   - 索引管理集中在 index.go，提供 IndexOptions / CreateIndexesWithOptions / HasIndex / HasIndexesPrefixMatch。
//   - 事务通过 Client.Transaction 提供 session.WithTransaction 语义封装。
package db

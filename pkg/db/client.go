package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"

	"github.com/castlexu/micro-service/pkg/logger"
)

// Client 封装 MongoDB 客户端与默认 Database。
//
// 字段只读：不要在外部直接修改。通过 Raw() / Database() 获取底层实例。
type Client struct {
	raw    *mongo.Client
	db     *mongo.Database
	dbName string
}

// InitMongo 按 cfg 建立连接并返回 *Client。
//
//   - 解析 URI 中的 database 覆盖 cfg.DBName（遵循 URI 优先原则）；
//   - ConnectTimeout 内完成 connect + Ping；任一失败返回 wrap error。
//   - cfg.DisableAutoConnect = true 时跳过网络操作，仅返回未连接的 Client（适合测试）。
func InitMongo(cfg MongoConfig) (*Client, error) {
	if cfg.URI == "" {
		return nil, NewError("db: MongoConfig.URI is required", nil)
	}
	cfg = cfg.withDefaults()

	// URI 中的 database 路径优先
	if cs, err := connstring.Parse(cfg.URI); err == nil && cs.Database != "" {
		cfg.DBName = cs.Database
	} else if err != nil {
		return nil, errorf(err, "db: parse uri")
	}

	clientOpts := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(cfg.PoolSize).
		SetReadPreference(cfg.ReadPreference).
		SetRetryReads(true).
		SetRetryWrites(true)

	if cfg.WMajority {
		clientOpts.SetWriteConcern(writeconcern.New(writeconcern.WMajority(), writeconcern.J(true)))
	} else {
		clientOpts.SetWriteConcern(writeconcern.New(writeconcern.W(1)))
	}

	c := &Client{dbName: cfg.DBName}

	if cfg.DisableAutoConnect {
		logger.L().Info(fmt.Sprintf("mongo: init skipped auto-connect, db=%s", cfg.DBName))
		return c, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	mc, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, errorf(err, "db: connect mongo")
	}

	// Ping 必须复用带超时的 ctx，避免老代码的 context.TODO() 超时失效 bug。
	if err := mc.Ping(ctx, readpref.Primary()); err != nil {
		_ = mc.Disconnect(context.Background())
		return nil, errorf(err, "db: ping mongo")
	}

	c.raw = mc
	c.db = mc.Database(cfg.DBName)
	logger.L().Info(fmt.Sprintf("mongo: connected, db=%s poolSize=%d wMajority=%v",
		cfg.DBName, cfg.PoolSize, cfg.WMajority))
	return c, nil
}

// Raw 返回底层 *mongo.Client，用于极少数需要原生 API 的场景。
func (c *Client) Raw() *mongo.Client { return c.raw }

// Database 返回默认 Database。
func (c *Client) Database() *mongo.Database { return c.db }

// Collection 取默认 Database 下的集合。
func (c *Client) Collection(name string, opts ...*options.CollectionOptions) *mongo.Collection {
	return c.db.Collection(name, opts...)
}

// Ping 对外暴露的健康检查。
func (c *Client) Ping(ctx context.Context) error {
	if c.raw == nil {
		return NewError("db: client not connected", nil)
	}
	return c.raw.Ping(ctx, readpref.Primary())
}

// Close 关闭底层连接。安全地处理 nil。
func (c *Client) Close(ctx context.Context) error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Disconnect(ctx)
}

// Transaction 基于 session.WithTransaction 封装事务：
//   - fn 返回 error，回滚；返回 nil，提交。
//   - 由 driver 自动处理 TransientTransactionError / UnknownTransactionCommitResult 的重试。
//   - opts 允许传入 readConcern / writeConcern / readPref / maxCommitTime 等。
func (c *Client) Transaction(
	ctx context.Context,
	fn func(sc mongo.SessionContext) error,
	opts ...*options.TransactionOptions,
) error {
	if c == nil || c.raw == nil {
		return NewError("db: client not connected", nil)
	}
	session, err := c.raw.StartSession()
	if err != nil {
		return errorf(err, "db: start session")
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		return nil, fn(sc)
	}, opts...)
	if err != nil {
		return errorf(err, "db: transaction")
	}
	return nil
}

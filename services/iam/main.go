// Package main 是 iam 服务入口，基于 Kitex 框架。
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cloudwego/kitex/server"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
	iambiz "github.com/castlexu/micro-service/services/iam/biz"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
	"github.com/castlexu/micro-service/services/iam/kitex_gen/iam/iamservice"
)

// IAMConfig 是 iam 服务配置结构。
type IAMConfig struct {
	Mongo struct {
		URI string `mapstructure:"uri"`
		DB  string `mapstructure:"db"`
	} `mapstructure:"mongo"`
	Server struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"server"`
}

func main() {
	// 初始化日志
	_ = logger.Init(logger.Options{Service: "iam"})
	defer logger.Sync()

	// 注册 metainfo 提取器（trace_id 透传到日志）
	mw.RegisterLoggerExtractor()

	// 加载配置
	cfgPath := os.Getenv("IAM_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/iam.yaml"
	}
	var cfg IAMConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
	}

	// 初始化 MongoDB
	mongoURI := cfg.Mongo.URI
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	dbName := cfg.Mongo.DB
	if dbName == "" {
		dbName = "platform"
	}
	mongoClient, err := db.InitMongo(db.MongoConfig{URI: mongoURI, DBName: dbName})
	if err != nil {
		logger.L().Fatal("mongo init failed", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Close(ctx)
	}()

	// 建立索引
	userRepo := iammongo.NewUserRepo(mongoClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := userRepo.EnsureIndexes(ctx, mongoClient); err != nil {
		logger.L().Warn("ensure iam indexes failed", zap.Error(err))
	}

	// 注入依赖
	userBiz := iambiz.NewUserBiz(userRepo)
	handler := NewIAMImpl(userBiz)

	// 启动 Kitex server
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":8082"
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		logger.L().Fatal("invalid server addr", zap.String("addr", addr), zap.Error(err))
	}
	svr := iamservice.NewServer(handler,
		server.WithServiceAddr(tcpAddr),
		server.WithMiddleware(mwkitex.Trace()),
		server.WithMiddleware(mwkitex.Recovery()),
		server.WithMiddleware(mwkitex.Logging()),
	)
	logger.L().Info(fmt.Sprintf("iam server listening on %s", addr))
	if err := svr.Run(); err != nil {
		logger.L().Fatal("iam server stopped", zap.Error(err))
	}
}

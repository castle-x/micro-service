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

	"github.com/castlexu/micro-service/pkg/cloudwego"
	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/db"
	pkghealth "github.com/castlexu/micro-service/pkg/health"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
	pkgotel "github.com/castlexu/micro-service/pkg/otel"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	iambiz "github.com/castlexu/micro-service/services/iam/biz"
	iamcache "github.com/castlexu/micro-service/services/iam/cache"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
	"github.com/castlexu/micro-service/services/iam/kitex_gen/iam/iamservice"
)

// IAMConfig 是 iam 服务配置结构。
type IAMConfig struct {
	Mongo struct {
		URI string `mapstructure:"uri"`
		DB  string `mapstructure:"db"`
	} `mapstructure:"mongo"`
	Redis struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"redis"`
	Server struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"server"`
	Registry cloudwego.RegistryConfig `mapstructure:"registry"`
	OTel     pkgotel.Config           `mapstructure:"otel"`
}

func main() {
	_ = logger.Init(logger.Options{Service: "iam"})
	restoreStdLog := logger.IngestStdLog()
	defer restoreStdLog()
	defer logger.Sync()
	mw.RegisterLoggerExtractor()

	cfgPath := os.Getenv("IAM_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/iam.yaml"
	}
	var cfg IAMConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
	}
	otelShutdown, err := pkgotel.Init(context.Background(), "iam", cfg.OTel)
	if err != nil {
		logger.L().Fatal("otel init failed", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(ctx); err != nil {
			logger.L().Warn("otel shutdown failed", zap.Error(err))
		}
	}()

	// MongoDB
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

	// Redis
	redisAddr := cfg.Redis.Addr
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	if err := pkgredis.Init(pkgredis.Config{Addr: redisAddr}); err != nil {
		logger.L().Fatal("redis init failed", zap.Error(err))
	}
	defer func() { _ = pkgredis.Close() }()

	// 建立索引
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	userRepo := iammongo.NewUserRepo(mongoClient)
	roleRepo := iammongo.NewRoleRepo(mongoClient)
	permRepo := iammongo.NewPermissionRepo(mongoClient)

	for _, fn := range []func() error{
		func() error { return userRepo.EnsureIndexes(ctx, mongoClient) },
		func() error { return roleRepo.EnsureIndexes(ctx, mongoClient) },
		func() error { return permRepo.EnsureIndexes(ctx, mongoClient) },
	} {
		if err := fn(); err != nil {
			logger.L().Warn("ensure indexes failed", zap.Error(err))
		}
	}

	// 依赖组装
	roleCache := iamcache.NewRoleCache(pkgredis.GetClient())
	roleBiz := iambiz.NewRoleBiz(roleRepo, permRepo, roleCache)
	permBiz := iambiz.NewPermissionBiz(permRepo)
	userBiz := iambiz.NewUserBiz(userRepo, roleRepo)
	handler := NewIAMImpl(userBiz, roleBiz, permBiz)

	// Kitex server
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38082"
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		logger.L().Fatal("invalid server addr", zap.String("addr", addr), zap.Error(err))
	}
	opts := []server.Option{
		server.WithServiceAddr(tcpAddr),
		server.WithMiddleware(mwkitex.Trace()),
		server.WithMiddleware(mwkitex.Recovery()),
		server.WithMiddleware(mwkitex.Logging()),
	}
	registryOpts, err := cloudwego.KitexRegistryOptions(cfg.Registry)
	if err != nil {
		logger.L().Fatal("kitex registry init failed", zap.Error(err))
	}
	opts = append(opts, registryOpts...)
	svr := iamservice.NewServer(handler, opts...)
	adminHealth := pkghealth.NewServer(pkghealth.Config{Service: "iam", Addr: pkghealth.AdminAddr("iam", 48082)})
	adminHealth.Check("mongo", pkghealth.MongoCheck(mongoClient))
	adminHealth.Check("redis", pkghealth.RedisCheck(pkgredis.GetClient()))
	adminHealth.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := adminHealth.Shutdown(ctx); err != nil {
			logger.L().Warn("admin health shutdown failed", zap.Error(err))
		}
	}()
	logger.L().Info(fmt.Sprintf("iam server listening on %s", addr))
	if err := svr.Run(); err != nil {
		logger.L().Fatal("iam server stopped", zap.Error(err))
	}
}

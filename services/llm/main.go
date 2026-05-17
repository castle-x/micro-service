// Package main 是 llm service 入口，基于 Hertz 框架（端口 :38083）。
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/cloudwego"
	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/db"
	pkghealth "github.com/castlexu/micro-service/pkg/health"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	pkgotel "github.com/castlexu/micro-service/pkg/otel"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
)

const serviceName = "llm"

// LLMConfig 是 llm service 配置。
type LLMConfig struct {
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
	Encrypt struct {
		// Key 是至少 32 字节的加密主钥，优先读 LLM_ENCRYPT_KEY 环境变量。
		Key string `mapstructure:"key"`
	} `mapstructure:"encrypt"`
	Registry cloudwego.RegistryConfig `mapstructure:"registry"`
	OTel     pkgotel.Config           `mapstructure:"otel"`
}

// ServiceDeps 是业务 handler 组装所需的基础设施依赖。
type ServiceDeps struct {
	Mongo      *db.Client
	Redis      *pkgredis.Client
	EncryptKey []byte
}

type routeHandlerFactory func(context.Context, ServiceDeps) (ProviderHandler, ModelHandler, GenerateHandler, error)
type indexEnsurer func(context.Context, *db.Client) error

var buildRouteHandlers routeHandlerFactory = func(context.Context, ServiceDeps) (ProviderHandler, ModelHandler, GenerateHandler, error) {
	return nil, nil, nil, nil
}

var ensureServiceIndexes indexEnsurer = func(context.Context, *db.Client) error {
	return nil
}

func resolveEncryptKey(envKey, cfgKey string) ([]byte, error) {
	encKeyStr := envKey
	if encKeyStr == "" {
		encKeyStr = cfgKey
	}
	if len(encKeyStr) < 32 {
		return nil, fmt.Errorf("LLM_ENCRYPT_KEY must be at least 32 bytes, got %d", len(encKeyStr))
	}
	return []byte(encKeyStr)[:32], nil
}

func main() {
	_ = logger.Init(logger.Options{Service: serviceName})
	restoreStdLog := logger.IngestStdLog()
	defer restoreStdLog()
	defer logger.Sync()
	mw.RegisterLoggerExtractor()

	cfgPath := os.Getenv("LLM_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/llm.yaml"
	}
	var cfg LLMConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
	}
	otelShutdown, err := pkgotel.Init(context.Background(), serviceName, cfg.OTel)
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

	// 加密主密钥（32 字节），优先读环境变量。该值必须保持稳定，否则已入库的 provider api_key 无法解密。
	encryptKey, err := resolveEncryptKey(os.Getenv("LLM_ENCRYPT_KEY"), cfg.Encrypt.Key)
	if err != nil {
		logger.L().Fatal("resolve llm encrypt key failed", zap.Error(err))
	}

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

	redisAddr := cfg.Redis.Addr
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	if err := pkgredis.Init(pkgredis.Config{Addr: redisAddr}); err != nil {
		logger.L().Fatal("redis init failed", zap.Error(err))
	}
	defer func() { _ = pkgredis.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ensureServiceIndexes(ctx, mongoClient); err != nil {
		logger.L().Warn("ensure llm indexes failed", zap.Error(err))
	}

	providerHandler, modelHandler, generateHandler, err := buildRouteHandlers(ctx, ServiceDeps{
		Mongo:      mongoClient,
		Redis:      pkgredis.GetClient(),
		EncryptKey: encryptKey,
	})
	if err != nil {
		logger.L().Fatal("llm handler init failed", zap.Error(err))
	}

	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38083"
	}
	hertzOpts := []hertzconfig.Option{
		server.WithHostPorts(addr),
	}
	registryOpts, err := cloudwego.HertzServerOptions(cfg.Registry, addr)
	if err != nil {
		logger.L().Fatal("hertz registry init failed", zap.Error(err))
	}
	etcdClient, err := cloudwego.SharedEtcdClient(cfg.Registry.Endpoints)
	if err != nil {
		logger.L().Fatal("etcd health client init failed", zap.Error(err))
	}
	hertzOpts = append(hertzOpts, registryOpts...)
	h := server.Default(hertzOpts...)
	RegisterRoutes(h, providerHandler, modelHandler, generateHandler)

	adminHealth := pkghealth.NewServer(pkghealth.Config{Service: serviceName, Addr: pkghealth.AdminAddr(serviceName, 48083)})
	adminHealth.Check("mongo", pkghealth.MongoCheck(mongoClient))
	adminHealth.Check("redis", pkghealth.RedisCheck(pkgredis.GetClient()))
	adminHealth.Check("etcd", pkghealth.EtcdCheck(etcdClient))
	adminHealth.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := adminHealth.Shutdown(ctx); err != nil {
			logger.L().Warn("admin health shutdown failed", zap.Error(err))
		}
	}()

	logger.L().Info("llm service listening", zap.String("addr", addr))
	h.Spin()
}

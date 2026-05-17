// Package main 是 model service 入口，基于 Hertz 框架（端口 :38083）。
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
	mdlbiz "github.com/castlexu/micro-service/services/model/biz"
	mdlmongo "github.com/castlexu/micro-service/services/model/dal/mongo"
	mdlhandler "github.com/castlexu/micro-service/services/model/handler"
)

// ModelConfig 是 model service 配置。
type ModelConfig struct {
	Mongo struct {
		URI string `mapstructure:"uri"`
		DB  string `mapstructure:"db"`
	} `mapstructure:"mongo"`
	Server struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"server"`
	Encrypt struct {
		// Key 是至少 32 字节的加密主钥，优先读 MODEL_ENCRYPT_KEY 环境变量。
		Key string `mapstructure:"key"`
	} `mapstructure:"encrypt"`
	Registry cloudwego.RegistryConfig `mapstructure:"registry"`
	OTel     pkgotel.Config           `mapstructure:"otel"`
}

func resolveEncryptKey(envKey, cfgKey string) ([]byte, error) {
	encKeyStr := envKey
	if encKeyStr == "" {
		encKeyStr = cfgKey
	}
	if len(encKeyStr) < 32 {
		return nil, fmt.Errorf("MODEL_ENCRYPT_KEY must be at least 32 bytes, got %d", len(encKeyStr))
	}
	return []byte(encKeyStr)[:32], nil
}

func main() {
	_ = logger.Init(logger.Options{Service: "model"})
	restoreStdLog := logger.IngestStdLog()
	defer restoreStdLog()
	defer logger.Sync()
	mw.RegisterLoggerExtractor()

	cfgPath := os.Getenv("MODEL_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/model.yaml"
	}
	var cfg ModelConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
	}
	otelShutdown, err := pkgotel.Init(context.Background(), "model", cfg.OTel)
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
	encryptKey, err := resolveEncryptKey(os.Getenv("MODEL_ENCRYPT_KEY"), cfg.Encrypt.Key)
	if err != nil {
		logger.L().Fatal("resolve model encrypt key failed", zap.Error(err))
	}

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

	// 建立索引
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	providerRepo := mdlmongo.NewProviderRepo(mongoClient)
	if err := providerRepo.EnsureIndexes(ctx, mongoClient); err != nil {
		logger.L().Warn("ensure model_providers indexes failed", zap.Error(err))
	}

	// 依赖组装
	providerBiz := mdlbiz.NewProviderBiz(providerRepo, encryptKey)
	providerHandler := mdlhandler.NewProviderHandler(providerBiz)
	chatBiz := mdlbiz.NewChatBiz(providerBiz)
	chatHandler := mdlhandler.NewChatHandler(chatBiz)

	// Hertz server
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
	RegisterRoutes(h, providerHandler, chatHandler)

	adminHealth := pkghealth.NewServer(pkghealth.Config{Service: "model", Addr: pkghealth.AdminAddr("model", 48083)})
	adminHealth.Check("mongo", pkghealth.MongoCheck(mongoClient))
	adminHealth.Check("etcd", pkghealth.EtcdCheck(etcdClient))
	adminHealth.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := adminHealth.Shutdown(ctx); err != nil {
			logger.L().Warn("admin health shutdown failed", zap.Error(err))
		}
	}()

	logger.L().Info("model service listening", zap.String("addr", addr))
	h.Spin()
}

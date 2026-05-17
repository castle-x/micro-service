// Package main 是 edge-api 服务入口，基于 Hertz 框架。
package main

import (
	"context"
	"os"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/cloudwego"
	"github.com/castlexu/micro-service/pkg/config"
	pkghealth "github.com/castlexu/micro-service/pkg/health"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	pkgotel "github.com/castlexu/micro-service/pkg/otel"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	"github.com/castlexu/micro-service/services/edge-api/handler"
	assetclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/asset/assetservice"
	iamclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam/iamservice"
	idpclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp/idpservice"
)

// EdgeConfig 是 edge-api 服务配置。
type EdgeConfig struct {
	Server struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"server"`
	LLM struct {
		ServiceName string `mapstructure:"service_name"`
	} `mapstructure:"llm"`
	Asset struct {
		ServiceName string `mapstructure:"service_name"`
	} `mapstructure:"asset"`
	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
	Redis struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"redis"`
	Registry  cloudwego.RegistryConfig  `mapstructure:"registry"`
	Discovery cloudwego.DiscoveryConfig `mapstructure:"discovery"`
	OTel      pkgotel.Config            `mapstructure:"otel"`
}

func main() {
	_ = logger.Init(logger.Options{Service: "edge-api"})
	restoreStdLog := logger.IngestStdLog()
	defer restoreStdLog()
	defer logger.Sync()
	mw.RegisterLoggerExtractor()

	cfgPath := os.Getenv("EDGE_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/edge-api.yaml"
	}
	var cfg EdgeConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
	}
	otelShutdown, err := pkgotel.Init(context.Background(), "edge-api", cfg.OTel)
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

	// JWT secret（与 idp 服务共享同一个 secret）
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte(cfg.JWT.Secret)
	}
	if len(jwtSecret) < 32 {
		logger.L().Fatal("JWT_SECRET must be at least 32 bytes")
	}

	// FRONTEND_URL
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:35173"
	}

	// Redis（用于封禁标记检查）
	redisAddr := cfg.Redis.Addr
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	if err := pkgredis.Init(pkgredis.Config{Addr: redisAddr}); err != nil {
		logger.L().Fatal("redis init failed", zap.Error(err))
	}
	defer func() { _ = pkgredis.Close() }()

	// IDP Kitex 客户端
	idpClientOpts, err := cloudwego.KitexClientOptions(cfg.Discovery)
	if err != nil {
		logger.L().Fatal("idp resolver init failed", zap.Error(err))
	}
	idpCli, err := idpclient.NewClient("idp", idpClientOpts...)
	if err != nil {
		logger.L().Fatal("idp client init failed", zap.Error(err))
	}

	// IAM Kitex 客户端
	iamClientOpts, err := cloudwego.KitexClientOptions(cfg.Discovery)
	if err != nil {
		logger.L().Fatal("iam resolver init failed", zap.Error(err))
	}
	iamCli, err := iamclient.NewClient("iam", iamClientOpts...)
	if err != nil {
		logger.L().Fatal("iam client init failed", zap.Error(err))
	}

	// Asset Kitex 客户端
	assetServiceName := cfg.Asset.ServiceName
	if assetServiceName == "" {
		assetServiceName = "asset"
	}
	assetClientOpts, err := cloudwego.KitexClientOptions(cfg.Discovery)
	if err != nil {
		logger.L().Fatal("asset resolver init failed", zap.Error(err))
	}
	assetCli, err := assetclient.NewClient(assetServiceName, assetClientOpts...)
	if err != nil {
		logger.L().Fatal("asset client init failed", zap.Error(err))
	}

	// 注入 handler
	authHandler := handler.NewAuthHandler(idpCli, frontendURL)
	userHandler := handler.NewUserHandler(idpCli, iamCli)
	adminHandler := handler.NewAdminHandler(iamCli, idpCli)
	assetHandler := handler.NewAssetHandler(assetCli)

	// LLM service 代理
	llmServiceName := cfg.LLM.ServiceName
	if llmServiceName == "" {
		llmServiceName = "llm"
	}
	llmResolver, err := cloudwego.NewHertzServiceResolver(cfg.Discovery, llmServiceName)
	if err != nil {
		logger.L().Fatal("llm resolver init failed", zap.Error(err))
	}
	llmProxy := handler.NewLLMProxy(llmResolver)

	// Hertz server
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38080"
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
	RegisterRoutes(h, authHandler, userHandler, adminHandler, assetHandler, llmProxy, idpCli, iamCli, jwtSecret, frontendURL)

	adminHealth := pkghealth.NewServer(pkghealth.Config{Service: "edge-api", Addr: pkghealth.AdminAddr("edge-api", 48080)})
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

	logger.L().Info("edge-api listening", zap.String("addr", addr))
	h.Spin()
}

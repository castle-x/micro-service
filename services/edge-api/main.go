// Package main 是 edge-api 服务入口，基于 Hertz 框架。
package main

import (
	"os"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/kitex/client"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	"github.com/castlexu/micro-service/services/edge-api/handler"
	iamclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam/iamservice"
	idpclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp/idpservice"
)

// EdgeConfig 是 edge-api 服务配置。
type EdgeConfig struct {
	Server struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"server"`
	IDP struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"idp"`
	IAM struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"iam"`
	Model struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"model"`
	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
	Redis struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"redis"`
}

func main() {
	_ = logger.Init(logger.Options{Service: "edge-api"})
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
	idpAddr := cfg.IDP.Addr
	if idpAddr == "" {
		idpAddr = "127.0.0.1:38081"
	}
	idpCli, err := idpclient.NewClient("idp", client.WithHostPorts(idpAddr))
	if err != nil {
		logger.L().Fatal("idp client init failed", zap.Error(err))
	}

	// IAM Kitex 客户端
	iamAddr := cfg.IAM.Addr
	if iamAddr == "" {
		iamAddr = "127.0.0.1:38082"
	}
	iamCli, err := iamclient.NewClient("iam", client.WithHostPorts(iamAddr))
	if err != nil {
		logger.L().Fatal("iam client init failed", zap.Error(err))
	}

	// 注入 handler
	authHandler := handler.NewAuthHandler(idpCli, frontendURL)
	userHandler := handler.NewUserHandler(idpCli, iamCli)
	adminHandler := handler.NewAdminHandler(iamCli, idpCli)

	// Model service 代理
	modelAddr := cfg.Model.Addr
	if modelAddr == "" {
		modelAddr = "127.0.0.1:38083"
	}
	modelProxy := handler.NewModelProxy(modelAddr)

	// Hertz server
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38080"
	}
	h := server.Default(server.WithHostPorts(addr))
	RegisterRoutes(h, authHandler, userHandler, adminHandler, modelProxy, idpCli, iamCli, jwtSecret, frontendURL)

	logger.L().Info("edge-api listening", zap.String("addr", addr))
	h.Spin()
}

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

	// FRONTEND_URL — 用于 Google OAuth2 callback 重定向和 CORS
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:35173"
	}

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

	// Hertz server
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38080"
	}
	h := server.Default(server.WithHostPorts(addr))
	RegisterRoutes(h, authHandler, userHandler, frontendURL)

	logger.L().Info("edge-api listening", zap.String("addr", addr))
	h.Spin()
}

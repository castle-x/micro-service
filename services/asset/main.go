// Package main 是 asset 服务入口，基于 Kitex 框架。
package main

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/cloudwego/kitex/server"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/cloudwego"
	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
	pkgotel "github.com/castlexu/micro-service/pkg/otel"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	assetbiz "github.com/castlexu/micro-service/services/asset/biz"
	assetmongo "github.com/castlexu/micro-service/services/asset/dal/mongo"
	"github.com/castlexu/micro-service/services/asset/kitex_gen/asset/assetservice"
)

// AssetConfig 是 asset 配置结构。
type AssetConfig struct {
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
	_ = logger.Init(logger.Options{Service: assetbiz.ServiceName})
	defer logger.Sync()
	mw.RegisterLoggerExtractor()

	cfgPath := os.Getenv("ASSET_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/asset.yaml"
	}
	var cfg AssetConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
	}
	otelShutdown, err := pkgotel.Init(context.Background(), assetbiz.ServiceName, cfg.OTel)
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
	assetTypeRepo := assetmongo.NewAssetTypeRepo(mongoClient)
	assetRepo := assetmongo.NewAssetRepo(mongoClient)
	assetVersionRepo := assetmongo.NewAssetVersionRepo(mongoClient)
	mediaObjectRepo := assetmongo.NewMediaObjectRepo(mongoClient)
	assetCategoryRepo := assetmongo.NewAssetCategoryRepo(mongoClient)
	uploadSessionRepo := assetmongo.NewStorageUploadSessionRepo(mongoClient)
	for _, item := range []struct {
		name string
		fn   func(context.Context, *db.Client) error
	}{
		{name: "asset_types", fn: assetTypeRepo.EnsureIndexes},
		{name: "assets", fn: assetRepo.EnsureIndexes},
		{name: "asset_versions", fn: assetVersionRepo.EnsureIndexes},
		{name: "media_objects", fn: mediaObjectRepo.EnsureIndexes},
		{name: "asset_categories", fn: assetCategoryRepo.EnsureIndexes},
		{name: "storage_upload_sessions", fn: uploadSessionRepo.EnsureIndexes},
	} {
		if err := item.fn(ctx, mongoClient); err != nil {
			logger.L().Warn("ensure asset indexes failed", zap.String("collection", item.name), zap.Error(err))
		}
	}

	healthBiz := assetbiz.NewHealthBiz()
	handler := NewAssetImpl(
		healthBiz,
		assetbiz.NewAssetTypeBiz(assetTypeRepo, assetRepo),
		assetbiz.NewAssetCategoryBiz(assetCategoryRepo, assetRepo),
		assetbiz.NewAssetBiz(assetRepo, assetTypeRepo, assetCategoryRepo),
		assetbiz.NewAssetVersionBiz(assetVersionRepo, assetRepo, assetTypeRepo),
	)

	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38084"
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

	svr := assetservice.NewServer(handler, opts...)
	logger.L().Info("asset server listening", zap.String("addr", addr))
	if err := svr.Run(); err != nil {
		logger.L().Fatal("asset server stopped", zap.Error(err))
	}
}

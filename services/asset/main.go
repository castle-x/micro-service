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
	"github.com/castlexu/micro-service/pkg/errno"
	pkghealth "github.com/castlexu/micro-service/pkg/health"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
	pkgotel "github.com/castlexu/micro-service/pkg/otel"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	assetbiz "github.com/castlexu/micro-service/services/asset/biz"
	assetmongo "github.com/castlexu/micro-service/services/asset/dal/mongo"
	"github.com/castlexu/micro-service/services/asset/kitex_gen/asset/assetservice"
	assetstorage "github.com/castlexu/micro-service/services/asset/storage"
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
	Storage  struct {
		Provider              string                       `mapstructure:"provider"`
		ObjectKeyPrefix       string                       `mapstructure:"object_key_prefix"`
		UploadURLTTLSeconds   int64                        `mapstructure:"upload_url_ttl_seconds"`
		DownloadURLTTLSeconds int64                        `mapstructure:"download_url_ttl_seconds"`
		MaxUploadSizeBytes    int64                        `mapstructure:"max_upload_size_bytes"`
		AllowedContentTypes   []string                     `mapstructure:"allowed_content_types"`
		AliyunOSS             assetstorage.AliyunOSSConfig `mapstructure:"aliyun_oss"`
	} `mapstructure:"storage"`
}

func main() {
	_ = logger.Init(logger.Options{Service: assetbiz.ServiceName})
	restoreStdLog := logger.IngestStdLog()
	defer restoreStdLog()
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

	storageClient, err := newStorageClient(cfg)
	if err != nil {
		logger.L().Fatal("asset storage init failed", zap.Error(err))
	}
	healthBiz := assetbiz.NewHealthBiz()
	mediaBiz := assetbiz.NewMediaBiz(mediaObjectRepo, uploadSessionRepo, storageClient, assetbiz.MediaConfig{
		ObjectKeyPrefix:     cfg.Storage.ObjectKeyPrefix,
		Bucket:              storageClient.Bucket(),
		UploadURLTTL:        time.Duration(cfg.Storage.UploadURLTTLSeconds) * time.Second,
		DownloadURLTTL:      time.Duration(cfg.Storage.DownloadURLTTLSeconds) * time.Second,
		MaxUploadSizeBytes:  cfg.Storage.MaxUploadSizeBytes,
		AllowedContentTypes: cfg.Storage.AllowedContentTypes,
		PublicBaseURL:       cfg.Storage.AliyunOSS.PublicBaseURL,
		CDNBaseURL:          cfg.Storage.AliyunOSS.CDNBaseURL,
	})
	handler := NewAssetImpl(
		healthBiz,
		assetbiz.NewAssetTypeBiz(assetTypeRepo, assetRepo),
		assetbiz.NewAssetCategoryBiz(assetCategoryRepo, assetRepo),
		assetbiz.NewAssetBiz(assetRepo, assetTypeRepo, assetCategoryRepo, mediaObjectRepo),
		assetbiz.NewAssetVersionBiz(assetVersionRepo, assetRepo, assetTypeRepo, mediaObjectRepo),
		mediaBiz,
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
	adminHealth := pkghealth.NewServer(pkghealth.Config{Service: assetbiz.ServiceName, Addr: pkghealth.AdminAddr(assetbiz.ServiceName, 48084)})
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
	logger.L().Info("asset server listening", zap.String("addr", addr))
	if err := svr.Run(); err != nil {
		logger.L().Fatal("asset server stopped", zap.Error(err))
	}
}

func newStorageClient(cfg AssetConfig) (assetstorage.Client, error) {
	provider := cfg.Storage.Provider
	if provider == "" {
		provider = "aliyun_oss"
	}
	switch provider {
	case "aliyun_oss":
		return assetstorage.NewAliyunOSSClient(cfg.Storage.AliyunOSS)
	default:
		return nil, errno.ErrAssetStorageError.WithMessagef("asset: unsupported storage provider %q", provider)
	}
}

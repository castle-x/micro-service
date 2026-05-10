// Package main 是 idp 服务入口，基于 Kitex 框架。
package main

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/server"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/logger"
	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
	idpbiz "github.com/castlexu/micro-service/services/idp/biz"
	idpcache "github.com/castlexu/micro-service/services/idp/cache"
	idpmongo "github.com/castlexu/micro-service/services/idp/dal/mongo"
	"github.com/castlexu/micro-service/services/idp/kitex_gen/idp/idpservice"
	iamclient "github.com/castlexu/micro-service/services/idp/kitex_gen/iam/iamservice"
)

// IDPConfig 是 idp 服务配置结构。
type IDPConfig struct {
	Mongo struct {
		URI string `mapstructure:"uri"`
		DB  string `mapstructure:"db"`
	} `mapstructure:"mongo"`
	Redis struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"redis"`
	JWT struct {
		Secret string `mapstructure:"secret"` // 从环境变量 JWT_SECRET 注入
	} `mapstructure:"jwt"`
	Google struct {
		ClientID     string `mapstructure:"client_id"`      // GOOGLE_CLIENT_ID
		ClientSecret string `mapstructure:"client_secret"`  // GOOGLE_CLIENT_SECRET
		RedirectURL  string `mapstructure:"redirect_url"`
	} `mapstructure:"google"`
	Alipay struct {
		Env string `mapstructure:"env"` // "sandbox" | "production"

		// 正式环境
		AppID        string `mapstructure:"app_id"`
		PrivateKey   string `mapstructure:"private_key"`
		AlipayPubKey string `mapstructure:"alipay_pub_key"`
		RedirectURL  string `mapstructure:"redirect_url"`
		GatewayURL   string `mapstructure:"gateway_url"`
		AuthURL      string `mapstructure:"auth_url"`

		// 沙箱环境
		SandboxAppID        string `mapstructure:"sandbox_app_id"`
		SandboxPrivateKey   string `mapstructure:"sandbox_private_key"`
		SandboxAlipayPubKey string `mapstructure:"sandbox_alipay_pub_key"`
		SandboxRedirectURL  string `mapstructure:"sandbox_redirect_url"`
		SandboxGatewayURL   string `mapstructure:"sandbox_gateway_url"`
		SandboxAuthURL      string `mapstructure:"sandbox_auth_url"`
	} `mapstructure:"alipay"`
	Server struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"server"`
	IAM struct {
		Addr string `mapstructure:"addr"` // iam service addr e.g. "127.0.0.1:38082"
	} `mapstructure:"iam"`
}

func main() {
	_ = logger.Init(logger.Options{Service: "idp"})
	defer logger.Sync()
	mw.RegisterLoggerExtractor()

	cfgPath := os.Getenv("IDP_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/idp.yaml"
	}
	var cfg IDPConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		logger.L().Fatal("load config failed", zap.Error(err))
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
	identityRepo := idpmongo.NewIdentityRepo(mongoClient)
	stateRepo := idpmongo.NewOAuthStateRepo(mongoClient)
	passwordCredRepo := idpmongo.NewPasswordCredRepo(mongoClient)
	if err := identityRepo.EnsureIndexes(ctx, mongoClient); err != nil {
		logger.L().Warn("ensure idp identity indexes failed", zap.Error(err))
	}
	if err := stateRepo.EnsureIndexes(ctx); err != nil {
		logger.L().Warn("ensure idp oauth_states indexes failed", zap.Error(err))
	}
	if err := passwordCredRepo.EnsureIndexes(ctx, mongoClient); err != nil {
		logger.L().Warn("ensure idp password_credentials indexes failed", zap.Error(err))
	}

	// 依赖组装
	jwtSecret := []byte(cfg.JWT.Secret)
	if len(jwtSecret) < 32 {
		logger.L().Fatal("JWT_SECRET must be at least 32 bytes")
	}
	tokenCache := idpcache.NewTokenCache(pkgredis.GetClient())
	tokenBiz, err := idpbiz.NewTokenBiz(jwtSecret, tokenCache)
	if err != nil {
		logger.L().Fatal("token biz init failed", zap.Error(err))
	}

	iamAddr := cfg.IAM.Addr
	if iamAddr == "" {
		iamAddr = "127.0.0.1:38082"
	}
	iamCli, err := iamclient.NewClient("iam", client.WithHostPorts(iamAddr))
	if err != nil {
		logger.L().Fatal("iam client init failed", zap.Error(err))
	}

	oauthBiz := idpbiz.NewOAuthBiz(cfg.Google.ClientID, cfg.Google.ClientSecret, cfg.Google.RedirectURL, stateRepo)
	loginBiz := idpbiz.NewLoginBiz(oauthBiz, tokenBiz, identityRepo, iamCli)
	passwordLoginBiz := idpbiz.NewPasswordLoginBiz(tokenBiz, passwordCredRepo, iamCli)

	// 支付宝：直接读 ALIPAY_ENV 环境变量（不依赖 viper 嵌套解析）
	alipayEnv := os.Getenv("ALIPAY_ENV")
	if alipayEnv == "" {
		alipayEnv = cfg.Alipay.Env // fallback to yaml value
	}
	if alipayEnv == "" {
		alipayEnv = "sandbox" // 最终兜底：默认沙箱
	}
	var alipayCfg idpbiz.AlipayConfig
	if alipayEnv == "sandbox" {
		alipayCfg = idpbiz.AlipayConfig{
			AppID:        cfg.Alipay.SandboxAppID,
			PrivateKey:   cfg.Alipay.SandboxPrivateKey,
			AlipayPubKey: cfg.Alipay.SandboxAlipayPubKey,
			RedirectURL:  cfg.Alipay.SandboxRedirectURL,
			GatewayURL:   cfg.Alipay.SandboxGatewayURL,
			AuthURL:      cfg.Alipay.SandboxAuthURL,
		}
		logger.L().Info("alipay: using sandbox environment")
	} else {
		alipayCfg = idpbiz.AlipayConfig{
			AppID:        cfg.Alipay.AppID,
			PrivateKey:   cfg.Alipay.PrivateKey,
			AlipayPubKey: cfg.Alipay.AlipayPubKey,
			RedirectURL:  cfg.Alipay.RedirectURL,
			GatewayURL:   cfg.Alipay.GatewayURL,
			AuthURL:      cfg.Alipay.AuthURL,
		}
		logger.L().Info("alipay: using production environment")
	}
	alipayBiz := idpbiz.NewAlipayBiz(alipayCfg, stateRepo)
	alipayLoginBiz := idpbiz.NewAlipayLoginBiz(alipayBiz, tokenBiz, identityRepo, iamCli)

	handler := NewIDPImpl(loginBiz, alipayLoginBiz, passwordLoginBiz, tokenBiz, oauthBiz, alipayBiz)

	// Kitex server
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":38081"
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		logger.L().Fatal("invalid server addr", zap.String("addr", addr), zap.Error(err))
	}
	svr := idpservice.NewServer(handler,
		server.WithServiceAddr(tcpAddr),
		server.WithMiddleware(mwkitex.Trace()),
		server.WithMiddleware(mwkitex.Recovery()),
		server.WithMiddleware(mwkitex.Logging()),
	)
	logger.L().Info("idp server listening", zap.String("addr", addr))
	if err := svr.Run(); err != nil {
		logger.L().Fatal("idp server stopped", zap.Error(err))
	}
}

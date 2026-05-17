// Package main 是 IAM Bootstrap 脚本，用于系统首次部署时初始化超级管理员账号和内置角色/权限。
//
// 使用方式：
//
//	ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD="Str0ng!Pass" \
//	  IAM_CONFIG=deployments/config/iam.yaml \
//	  ./bin/iam-bootstrap
//
// 脚本是幂等的：数据已存在时直接跳过，重复执行无副作用。
// nolint:noprint -- bootstrap 是人工执行命令，stdout 进度输出是预期行为。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"github.com/castlexu/micro-service/pkg/config"
	"github.com/castlexu/micro-service/pkg/db"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
)

// BootstrapConfig 共享 iam.yaml 的 mongo 部分。
type BootstrapConfig struct {
	Mongo struct {
		URI string `mapstructure:"uri"`
		DB  string `mapstructure:"db"`
	} `mapstructure:"mongo"`
}

// ---- 内置权限 ----

var systemPermissions = []iammodel.Permission{
	{Code: "user:read", DisplayName: "查看用户", Description: "查看用户列表和详情", IsSystem: true},
	{Code: "user:write", DisplayName: "修改用户", Description: "修改用户资料", IsSystem: true},
	{Code: "user:role:assign", DisplayName: "分配角色", Description: "分配/变更用户角色", IsSystem: true},
	{Code: "user:status:update", DisplayName: "修改用户状态", Description: "启用/禁用/封禁用户", IsSystem: true},
	{Code: "role:read", DisplayName: "查看角色", Description: "查看角色列表", IsSystem: true},
	{Code: "role:write", DisplayName: "管理角色", Description: "创建/修改/删除角色", IsSystem: true},
	{Code: "permission:read", DisplayName: "查看权限", Description: "查看权限列表", IsSystem: true},
	{Code: "permission:write", DisplayName: "管理权限", Description: "创建自定义权限", IsSystem: true},
	{Code: "metrics:view", DisplayName: "查看指标", Description: "查看技术指标", IsSystem: true},
	{Code: "model:admin", DisplayName: "管理 AI 模型", Description: "配置 AI 模型供应商和调用 chat 接口", IsSystem: true},
}

// ---- 内置角色 ----

var systemRoles = []iammodel.Role{
	{
		Name:        "super_admin",
		DisplayName: "超级管理员",
		Permissions: []string{
			"user:read", "user:write", "user:role:assign", "user:status:update",
			"role:read", "role:write", "permission:read", "permission:write",
			"metrics:view", "model:admin",
		},
		IsSystem: true,
	},
	{
		Name:        "admin",
		DisplayName: "管理员",
		Permissions: []string{
			"user:read", "user:role:assign", "user:status:update",
			"role:read", "permission:read", "metrics:view", "model:admin",
		},
		IsSystem: true,
	},
	{
		Name:        "user",
		DisplayName: "普通用户",
		Permissions: []string{},
		IsSystem:    true,
	},
}

func main() {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminEmail == "" || adminPassword == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD environment variables are required")
	}

	cfgPath := os.Getenv("IAM_CONFIG")
	if cfgPath == "" {
		cfgPath = "deployments/config/iam.yaml"
	}
	var cfg BootstrapConfig
	if err := config.Load(cfgPath, &cfg); err != nil {
		log.Fatalf("load config: %v", err)
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
		log.Fatalf("mongo init: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Close(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	permRepo := iammongo.NewPermissionRepo(mongoClient)
	roleRepo := iammongo.NewRoleRepo(mongoClient)
	userRepo := iammongo.NewUserRepo(mongoClient)

	// 建立索引
	for _, fn := range []func() error{
		func() error { return permRepo.EnsureIndexes(ctx, mongoClient) },
		func() error { return roleRepo.EnsureIndexes(ctx, mongoClient) },
		func() error { return userRepo.EnsureIndexes(ctx, mongoClient) },
	} {
		if err := fn(); err != nil {
			log.Printf("warn: ensure indexes: %v", err)
		}
	}

	// 1. 写入内置权限（幂等）
	fmt.Println(">>> [1/3] Seeding permissions...")
	for _, p := range systemPermissions {
		p := p
		p.BaseDoc = db.BaseDoc{ID: primitive.NewObjectID()}
		if err := permRepo.Insert(ctx, &p); err != nil {
			if isAlreadyExists(err) {
				fmt.Printf("    skip (exists): %s\n", p.Code)
			} else {
				log.Fatalf("insert permission %s: %v", p.Code, err)
			}
		} else {
			fmt.Printf("    created: %s\n", p.Code)
		}
	}

	// 2. 写入内置角色（幂等）
	fmt.Println(">>> [2/3] Seeding roles...")
	for _, r := range systemRoles {
		r := r
		r.BaseDoc = db.BaseDoc{ID: primitive.NewObjectID()}
		if err := roleRepo.Insert(ctx, &r); err != nil {
			if isAlreadyExists(err) {
				fmt.Printf("    skip (exists): %s\n", r.Name)
			} else {
				log.Fatalf("insert role %s: %v", r.Name, err)
			}
		} else {
			fmt.Printf("    created: %s\n", r.Name)
		}
	}

	// 3. 创建 super_admin 用户（幂等）
	fmt.Println(">>> [3/3] Creating super_admin account...")
	existing, findErr := userRepo.FindByEmail(ctx, adminEmail)
	if findErr == nil && existing != nil {
		fmt.Printf("    skip: super_admin %s already exists (user_id=%s)\n", adminEmail, existing.ID.Hex())
		fmt.Println("\nBootstrap complete ✓ (already initialized)")
		return
	}

	userID := primitive.NewObjectID()
	adminUser := &iammodel.User{
		BaseDoc: db.BaseDoc{ID: userID},
		Email:   adminEmail,
		Name:    "Super Admin",
		Status:  iammodel.UserStatusActive,
		Role:    "super_admin",
		Source:  iammodel.UserSourceAdminCreated,
	}
	if _, insertErr := userRepo.Insert(ctx, adminUser); insertErr != nil {
		log.Fatalf("insert super_admin user: %v", insertErr)
	}
	fmt.Printf("    created iam user: %s (id=%s)\n", adminEmail, userID.Hex())

	// 写入 password_credentials（直接用 driver，避免跨 module 依赖）
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	credColl := mongoClient.Collection("password_credentials")
	now := time.Now().Unix()
	_, credErr := credColl.InsertOne(ctx, bson.D{
		{Key: "_id", Value: primitive.NewObjectID()},
		{Key: "user_id", Value: userID},
		{Key: "email", Value: adminEmail},
		{Key: "password_hash", Value: string(hash)},
		{Key: "created_at", Value: now},
		{Key: "updated_at", Value: now},
	})
	if credErr != nil {
		if mongo.IsDuplicateKeyError(credErr) {
			fmt.Printf("    skip: credential for %s already exists\n", adminEmail)
		} else {
			log.Fatalf("insert password credential: %v", credErr)
		}
	} else {
		fmt.Printf("    created credential for: %s\n", adminEmail)
	}

	fmt.Println()
	fmt.Println("Bootstrap complete ✓")
	fmt.Printf("  Super admin email:    %s\n", adminEmail)
	fmt.Printf("  IAM user_id:         %s\n", userID.Hex())
	fmt.Println()
	fmt.Println("  Next: start all services with 'make dev-start'")
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "10010") || strings.Contains(msg, "duplicate")
}

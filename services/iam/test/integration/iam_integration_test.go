//go:build integration

// Package integration 提供 iam 服务的集成测试样板。
//
// 运行：
//   cd services/iam && go test ./test/integration/... -count=1 -race -tags=integration -timeout=10m
//
// 或在仓库根：
//   make test-integration
//
// 前置：本机 Docker daemon 可用（testcontainers 会启动 ephemeral Mongo/Redis 容器）。
//
// 设计原则（详见 .axm/project/api-testing.md §3.2 §4）：
//   - 每个测试独立 DB name / Redis prefix，结束自动销毁
//   - 不共享 dev 数据库；不依赖外部网络
//   - 不用 time.Sleep；用 require.Eventually 轮询
//   - 不在测试间共享状态
package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startMongo 启动一个 ephemeral MongoDB 容器，返回连接 URI 与 cleanup。
//
// 调用方拿到 uri 后，应该用 fmt.Sprintf("itest_%s_%d", t.Name(), time.Now().UnixNano())
// 作为 db name，避免测试之间互相污染。
func startMongo(ctx context.Context, t *testing.T) (uri string, cleanup func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "mongo:7.0",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForListeningPort("27017/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start mongo container")

	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "27017")
	require.NoError(t, err)

	uri = fmt.Sprintf("mongodb://%s:%s", host, port.Port())
	cleanup = func() {
		_ = c.Terminate(context.Background())
	}
	return uri, cleanup
}

// startRedis 启动一个 ephemeral Redis 容器。
func startRedis(ctx context.Context, t *testing.T) (addr string, cleanup func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start redis container")

	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "6379")
	require.NoError(t, err)

	addr = fmt.Sprintf("%s:%s", host, port.Port())
	cleanup = func() {
		_ = c.Terminate(context.Background())
	}
	return addr, cleanup
}

// TestExample 是一个最小集成测试样板，新服务只需复制本文件并替换业务调用。
//
// 真实测试应该：
//  1. 启动 testcontainers（mongo/redis/nsq 按需）
//  2. 用真实 dal/biz 构造服务对象
//  3. 通过 RPC client 或直接调用 biz 入口发起 API 请求
//  4. 断言 DB 副作用 + RPC 响应
//  5. cleanup
func TestExample_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mongoURI, cleanupMongo := startMongo(ctx, t)
	defer cleanupMongo()

	redisAddr, cleanupRedis := startRedis(ctx, t)
	defer cleanupRedis()

	t.Logf("mongo: %s", mongoURI)
	t.Logf("redis: %s", redisAddr)

	// TODO: 用 mongoURI / redisAddr 构造真实的 iam dal/biz，跑完整 RPC 流程。
	// 例：
	//   dal := mongo.NewUserRepo(mongoURI, "itest_iam")
	//   svc := biz.NewUserService(dal, redis.NewClient(redisAddr))
	//   resp, err := svc.CreateUser(ctx, &iam.CreateUserReq{...})
	//   require.NoError(t, err)
	//   require.NotEmpty(t, resp.UserId)

	require.True(t, true, "replace this assertion with real flow")
}

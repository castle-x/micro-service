# services/iam 集成测试

> 详见 `.axm/project/api-testing.md §3.2`

## 运行

```bash
# 在仓库根
make test-integration

# 仅 iam
cd services/iam && go test ./test/integration/... -count=1 -race -tags=integration -timeout=10m
```

## 前置

- Docker daemon 可用（testcontainers 启动 Mongo/Redis 容器）
- 不依赖本机 `make infra-up`，自带 ephemeral 容器

## 文件约定

```
test/integration/
  iam_integration_test.go   # 入口（含 testcontainers 辅助）
  fixtures/                 # 静态 fixture（JSON）
  factory/                  # 动态构造（builder pattern）
```

## 编写新测试

1. 复制 `iam_integration_test.go` 作为模板
2. 用 `startMongo` / `startRedis` 拿到 ephemeral 连接
3. 用真实 `dal/` + `biz/` 跑完整 API 路径
4. 断言 DB 副作用 + RPC 响应
5. 不要用 `time.Sleep`，用 `require.Eventually`

## 禁忌

- ❌ 不依赖 dev DB
- ❌ 测试之间共享状态
- ❌ 测试里 `time.Sleep(5 * time.Second)`
- ❌ 只 mock 不验真（mock 一切等于测了寂寞）

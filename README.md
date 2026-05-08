# Platform Monorepo

> 基于 Kong + Hertz + Kitex + MongoDB + Redis + etcd + NSQ 的 Go 微服务平台骨架

## 架构概览

```
客户端 ──► Kong (edge-gateway) ──► Hertz (edge-api) ──► Kitex RPC ──► idp / iam / billing / credits / notification
                                                              │
                                                              ├─► MongoDB (主存储)
                                                              ├─► Redis    (缓存 / 锁 / 幂等)
                                                              ├─► etcd     (服务发现 / 配置)
                                                              └─► NSQ      (异步事件)
```

## 目录结构

```
.
├── go.work                  # Go Workspace
├── Makefile                 # 构建/生成/测试统一入口
├── AGENTS.md                # AI 协作入口与规范路由
├── SPEC.md                  # 历史入口（已降级，勿作为当前规范）
├── idl/                     # 全局 Thrift IDL（独立 module）
├── pkg/                     # 通用基础设施（独立 module）
├── services/                # 业务服务（每个独立 module）
│   ├── edge-api/            # Hertz HTTP/WebSocket 接入层
│   ├── idp/                 # 身份认证
│   ├── iam/                 # 用户与权限
│   ├── billing/             # 支付
│   ├── credits/             # 积分
│   └── notification/        # 通知
└── deployments/             # Docker / Kong / K8s 配置
```

## 快速开始

### 1. 环境要求

- Go 1.25.6+
- Docker & docker-compose
- CLI 工具：`kitex`、`hz`、`thriftgo`

### 2. 安装代码生成工具

```bash
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
go install github.com/cloudwego/hertz/cmd/hz@latest
go install github.com/cloudwego/thriftgo@latest
```

### 3. 拉取依赖

```bash
go work sync
```

### 4. 启动本地基础设施

```bash
make dev
```

### 5. 构建所有服务

```bash
make build
```

## 更多

- 当前 AI 协作入口与项目规范：[`AGENTS.md`](./AGENTS.md)
- 当前阶段进度与路线图：[`.phase/STATUS.md`](./.phase/STATUS.md)
- 旧 `SPEC.md` / `初步设计参考.md` 已降级为历史入口，立项初始设计参考见 [`.phase/phase-00-initial-design-reference.md`](./.phase/phase-00-initial-design-reference.md)。

<!-- axm-meta
status: active
last-reviewed: 2026-05-14
owner: castlexu
progress-type: spec
initiative: asset
related:
  - ../roadmap.md
  - ../../../project/architecture.md
  - ../../../project/coding.md
  - ../../../knowledge/services/overview.md
  - ./service-contract-and-domain-models.md
-->

# AS-02 · 资产库 CRUD

## 背景

AS-01 已建立 `asset` 的 Kitex 服务、共享 DTO、Mongo 文档模型、仓储骨架、错误码区段和启动配置。AS-02 在此基础上补齐第一版个人资产库闭环：用户可以管理自己的资产类型、组成部分 schema、分类、资产实例，并通过 `savedToLibrary` 区分资产库与历史产物。

AS-02 仍不处理资产版本内容、媒体对象、OSS 上传或 CDN 访问 URL；这些能力分别由 AS-03、AS-04 继续实现。

## 目标

- 扩展 `idl/asset/asset.thrift`，定义资产类型、分类、资产实例 CRUD RPC。
- 在 `services/asset` 中实现 Mongo repository、biz 校验、DTO 映射和 Kitex handler。
- 在 `services/edge-api` 中接入 asset Kitex client，并暴露 `/api/v1/assets/*` REST 门面。
- 使用个人资产库模型：`workspace_id = 当前登录 user_id`，客户端不允许传 `workspace_id`。
- 支持资产类型 `partSchemas` 始终可更新；AS-02 不维护 schema 历史版本。

## 范围

| 范围项 | AS-02 要做 |
|---|---|
| AssetType | 创建、更新名称/描述/schema、详情、列表、删除未使用类型 |
| AssetCategory | 创建、更新、列表、删除空分类 |
| Asset | 创建、更新名称/描述/分类/封面/source、详情、分页列表、更新 `savedToLibrary`、软删除 |
| edge-api | 登录态 REST 门面，透传当前 user_id 到 `Base.UserID` |
| 错误码 | 新增 `ErrAssetCategoryNotFound = 17009`；冲突场景复用 `ErrAssetConflict` |
| 测试 | 覆盖 biz、handler、edge-api 请求绑定和错误映射 |

## 非目标

- 不实现 `AssetVersion` 写入、parts 校验、版本查询、回滚或复制。
- 不实现媒体对象 CRUD、上传会话、OSS SDK、预签名 URL、生成结果入库。
- 不实现 CDN 刷新、预热、签名鉴权或生产化 URL 策略。
- 不实现团队资产库、协作权限、公开发布、市场、自动清理或智能分类。
- 不把 `edge-api` 变成资产数据访问层；它只做 HTTP 参数适配和 RPC 调用。

## 已确认开发细节

### Workspace 与登录态

- AS-02 采用个人资产库：`workspace_id` 与 `created_by` 都来自当前登录用户。
- asset RPC 从 `req.Base.UserID` 读取当前用户；为空返回 `ErrInvalidParam`。
- edge-api 从认证中间件注入的 user_id 构造 `base.BaseReq{UserID: &userID}`。
- 客户端请求体和 query 不包含 `workspace_id`；即使传入也忽略。

### IDL 契约

`idl/asset/asset.thrift` 继续使用 AS-01 的 DTO，并新增以下请求/响应：

| 能力 | RPC |
|---|---|
| 资产类型 | `CreateAssetType`、`UpdateAssetType`、`GetAssetType`、`ListAssetTypes`、`DeleteAssetType` |
| 分类 | `CreateAssetCategory`、`UpdateAssetCategory`、`ListAssetCategories`、`DeleteAssetCategory` |
| 资产 | `CreateAsset`、`UpdateAsset`、`GetAsset`、`ListAssets`、`SetAssetLibraryState`、`DeleteAsset` |

列表接口：

- 使用 `base.PageReq` / `base.PageResp`。
- `PageNum < 1` 归一到 `1`；`PageSize < 1` 或 `> 100` 归一到 `20`。
- 默认按 `updated_at desc` 返回。

字段规则：

- `AssetType.Code` 创建后不可变，更新接口不接受 `Code`。
- `AssetType.PartSchemas` 可始终替换更新，不因已有资产而阻塞。
- `Asset.CurrentVersion` 在 AS-02 固定为 `0`，版本能力留给 AS-03。
- `Asset.Source` 创建时可省略，默认 `UNKNOWN`。

### REST 契约

所有 REST 路由均需要登录，统一放在 `/api/v1/assets`：

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/types` | 创建资产类型 |
| `GET` | `/types` | 分页查询资产类型 |
| `GET` | `/types/:id` | 查询资产类型详情 |
| `PUT` | `/types/:id` | 更新资产类型名称、描述和 schema |
| `DELETE` | `/types/:id` | 删除未使用资产类型 |
| `POST` | `/categories` | 创建分类 |
| `GET` | `/categories` | 查询分类列表 |
| `PUT` | `/categories/:id` | 更新分类 |
| `DELETE` | `/categories/:id` | 删除空分类 |
| `POST` | `/` | 创建资产实例 |
| `GET` | `/` | 分页查询资产实例 |
| `GET` | `/:id` | 查询资产详情 |
| `PUT` | `/:id` | 更新资产基础信息 |
| `PUT` | `/:id/library-state` | 保存到资产库或移出资产库 |
| `DELETE` | `/:id` | 软删除资产 |

REST 响应继续使用 `apiResp{code,message,data}` 包装。

### 删除与冲突规则

- 删除资产类型前必须确认同一 workspace 下没有未删除资产引用该类型；否则返回 `ErrAssetConflict`。
- 删除分类前必须确认同一 workspace 下没有子分类、没有未删除资产引用该分类；否则返回 `ErrAssetConflict`。
- 删除资产使用软删除。
- 重复 `workspace_id + code`、非法引用、删除非空分类等冲突统一返回 `ErrAssetConflict`。

## 设计约束

- `asset` 继续作为 Kitex RPC 服务，不引入 Hertz。
- `services/edge-api` 只能调用 asset RPC，不直接 import asset 的 biz、dal 或 model 包。
- `asset` 不 import 其他业务服务内部 Go 包。
- 日志和 trace 沿用现有 middleware；AS-02 不新增观测基础设施。
- 业务错误统一使用 `pkg/errno`；参数错误优先复用 `ErrInvalidParam`。

## 验收标准

### AI 自动验收

实现 AS-02 时必须通过：

```bash
make gen
cd services/asset && go test ./... -count=1
cd services/edge-api && go test ./... -count=1
make build
node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service
```

可判定输出：

- `asset.thrift` 能生成 asset 和 edge-api 所需 Kitex 代码。
- asset 测试覆盖 CRUD、个人 workspace 隔离、删除冲突和错误码。
- edge-api 测试覆盖请求绑定、user_id 透传、分页默认值和错误码到 HTTP 状态映射。
- `make build` 能构建所有服务。
- axm validate 无 error。

### 人类验收

- 人类确认 AS-02 只实现个人资产库 CRUD，没有引入资产版本、媒体上传或 OSS 能力。
- 人类确认 `workspace_id = user_id` 的第一版策略符合当前产品阶段。
- 人类确认资产类型 schema 可以始终修改，历史 schema 漂移由 AS-03 后续处理。

<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: asset
related:
  - ../roadmap.md
  - ../../../project/architecture.md
  - ../../../project/coding.md
  - ../../../knowledge/services/overview.md
-->

# AS-01 · 服务契约与领域模型

## 实施状态

已完成并闭合。`idl/asset/asset.thrift`、`services/asset` Kitex 服务、Mongo 文档模型 / repository 骨架、asset 错误码区段、服务配置、本地启动与 etcd 注册发现均已落地；后续 AS-02 至 AS-04 已在此基础上完成。

闭合证据：

- 源码事实：`idl/asset/asset.thrift`、`services/asset/main.go`、`services/asset/handler.go`、`services/asset/dal/model/*`、`services/asset/dal/mongo/*`、`deployments/config/asset.yaml` 均已存在。
- 长期事实已同步：`../../../knowledge/services/overview.md` 已把 `asset` 记录为已运行 Kitex 服务，并列出 assets / versions / media / upload sessions 能力。
- 人类确认：2026-05-17 用户确认 asset 已开发完成。

## 背景

`asset` 是生成资产平台优先实现的基础服务。它负责资产类型、资产实例、版本、组成部分、媒体对象和对象存储会话的主数据，但第一阶段只建立可编译、可注册、边界清楚的服务地基。

AS-01 不做业务闭环。资产库 CRUD、版本写入、OSS 上传和生成结果入库分别进入 AS-02、AS-03、AS-04。

## 目标

- 新增 `idl/asset/asset.thrift`，定义 `asset` namespace、共享枚举、实体 DTO 和最小探活 RPC。
- 新增 Kitex 服务模块 `services/asset/`，遵循现有服务分层：`main.go`、`handler.go`、`biz/`、`dal/model/`、`dal/mongo/`。
- 新增 Mongo 文档模型与仓储骨架：`asset_types`、`assets`、`asset_versions`、`media_objects`、`asset_categories`、`storage_upload_sessions`。
- 新增 asset 错误码区段，错误统一通过 `pkg/errno` 返回。
- 新增服务配置与本地启动配置，服务可初始化 Mongo、Redis、OTel，并注册到 etcd。

## 范围

| 范围项 | AS-01 要做 |
|---|---|
| IDL | 定义共享枚举、实体 DTO、`Health` RPC；不定义完整 CRUD RPC |
| 服务模块 | 新建 `services/asset` 独立 Go module 并接入 go.work |
| 入口 | 复用现有 Kitex 服务启动方式，初始化 config/logger/middleware/otel/db/redis/registry |
| DAL Model | 每个集合有独立 Go model，内嵌 `db.BaseDoc`，时间字段使用 Unix 秒 |
| DAL Mongo | 每个集合有 repository 构造和 `EnsureIndexes` |
| Biz/Handler | 只实现 `Health` 所需的最小依赖检查或静态响应 |
| 部署配置 | 新增 `deployments/config/asset.yaml` 和本地默认端口 |
| Makefile | 将 `asset` 纳入生成、构建、测试命令覆盖范围 |

## 非目标

- 不实现资产类型创建、更新、删除、列表。
- 不实现资产实例 CRUD、保存到资产库、历史产物查询。
- 不实现资产版本写入、parts 校验、回滚或复制旧版本。
- 不实现 OSS SDK、预签名 URL、前端直传或生成结果入库。
- 不实现 CDN 刷新、预热、签名鉴权或生产化 URL 策略。
- 不引入复杂资产状态机、市场、权限协作、自动清理、智能分类或资产关系图。

## 已确认开发细节

### IDL 契约

`idl/asset/asset.thrift` 必须：

- 使用 `namespace go asset`。
- `include "../base.thrift"`。
- 请求结构统一携带 `base.BaseReq Base`。
- 响应结构统一携带 `base.BaseResp Base`。
- 枚举保留 `UNKNOWN = 0`。
- 时间字段使用 `i64` Unix 秒。

AS-01 定义这些共享枚举：

| 枚举 | 值 |
|---|---|
| `AssetValueKind` | `UNKNOWN`、`TEXT`、`MEDIA`、`JSON`、`MIXED` |
| `AssetSource` | `UNKNOWN`、`UPLOAD`、`WORKFLOW`、`GENERATION`、`IMPORT` |
| `StorageProvider` | `UNKNOWN`、`LOCAL`、`ALIYUN_OSS`、`S3`、`TENCENT_COS` |
| `URLVisibility` | `UNKNOWN`、`PRIVATE`、`PUBLIC`、`SIGNED` |
| `UploadSessionStatus` | `UNKNOWN`、`CREATED`、`FINALIZED`、`EXPIRED`、`CANCELLED` |

AS-01 定义这些实体 DTO，用于后续 RPC 复用：

| DTO | 字段方向 |
|---|---|
| `AssetPartSchemaDTO` | `Key`、`Name`、`Description`、`AllowedValueKinds`、`Multiple`、`Required`、`SortOrder` |
| `AssetTypeDTO` | `AssetTypeID`、`WorkspaceID`、`Name`、`Code`、`Description`、`PartSchemas`、`CreatedBy`、`CreatedAt`、`UpdatedAt` |
| `AssetDTO` | `AssetID`、`WorkspaceID`、`TypeID`、`Name`、`Description`、`SavedToLibrary`、`CategoryID`、`CurrentVersion`、`CoverMediaID`、`Source`、`Provenance`、`CreatedBy`、`CreatedAt`、`UpdatedAt` |
| `AssetPartValueDTO` | `ValueKind`、`Text`、`JSON`、`MediaIDs` |
| `AssetVersionDTO` | `VersionID`、`AssetID`、`Version`、`Parts`、`ChangeReason`、`Provenance`、`CreatedBy`、`CreatedAt` |
| `MediaVariantDTO` | `Kind`、`ObjectKey`、`CDNURL`、`Width`、`Height`、`Size` |
| `MediaObjectDTO` | `MediaID`、`WorkspaceID`、`Provider`、`Bucket`、`ObjectKey`、`CDNURL`、`URLVisibility`、`ContentType`、`Size`、`Width`、`Height`、`SHA256`、`Variants`、`Source`、`Provenance`、`CreatedBy`、`CreatedAt` |
| `AssetCategoryDTO` | `CategoryID`、`WorkspaceID`、`Name`、`ParentID`、`SortOrder`、`CreatedBy`、`CreatedAt`、`UpdatedAt` |
| `StorageUploadSessionDTO` | `SessionID`、`WorkspaceID`、`Provider`、`Bucket`、`ObjectKey`、`Status`、`ExpiresAt`、`CreatedBy`、`CreatedAt`、`FinalizedAt` |
| `ProvenanceDTO` | `WorkflowRunID`、`StepRunID`、`GenerationJobID`、`PromptID`、`Extra` |

AS-01 只新增一个最小 RPC：

```thrift
struct HealthReq {
    1: required base.BaseReq Base
}

struct HealthResp {
    1: required base.BaseResp Base
    2: required string Service
    3: required string Status
}

service AssetService {
    HealthResp Health(1: HealthReq req)
}
```

`Health` 只用于服务启动、Kitex 生成和注册发现验证，不承载资产业务逻辑。

### Mongo 文档模型

所有文档模型放在 `services/asset/dal/model/`，内嵌：

```go
db.BaseDoc `bson:",inline"`
```

集合与模型：

| 集合 | Go model | 关键字段 |
|---|---|---|
| `asset_types` | `AssetType` | `workspace_id`、`name`、`code`、`description`、`part_schemas`、`created_by` |
| `assets` | `Asset` | `workspace_id`、`type_id`、`name`、`description`、`saved_to_library`、`category_id`、`current_version`、`cover_media_id`、`source`、`provenance`、`created_by` |
| `asset_versions` | `AssetVersion` | `asset_id`、`version`、`parts`、`change_reason`、`provenance`、`created_by` |
| `media_objects` | `MediaObject` | `workspace_id`、`provider`、`bucket`、`object_key`、`cdn_url`、`url_visibility`、`content_type`、`size`、`width`、`height`、`sha256`、`variants`、`source`、`provenance`、`created_by` |
| `asset_categories` | `AssetCategory` | `workspace_id`、`name`、`parent_id`、`sort_order`、`created_by` |
| `storage_upload_sessions` | `StorageUploadSession` | `workspace_id`、`provider`、`bucket`、`object_key`、`status`、`expires_at`、`created_by`、`finalized_at` |

索引要求：

| 集合 | 必须索引 |
|---|---|
| `asset_types` | unique `workspace_id + code`；普通 `workspace_id + updated_at` |
| `assets` | `workspace_id + saved_to_library + updated_at`；`workspace_id + type_id + updated_at`；`workspace_id + category_id + updated_at` |
| `asset_versions` | unique `asset_id + version`；普通 `asset_id + created_at` |
| `media_objects` | unique `provider + bucket + object_key`；普通 `workspace_id + created_at`；普通 `sha256` |
| `asset_categories` | `workspace_id + parent_id + sort_order`；`workspace_id + name` |
| `storage_upload_sessions` | `workspace_id + status + expires_at`；unique `provider + bucket + object_key` |

### 仓储与服务分层

- Mongo 访问放 `services/asset/dal/mongo/`。
- 每个仓储基于 `pkg/db.Repository[T]` 封装，不在 biz 层直接操作 collection。
- `EnsureIndexes` 由 `main.go` 启动时统一调用；失败只记录 warn，是否 fatal 留到后续生产化决策。
- AS-01 的 biz 层只保留 `HealthBiz` 或等价最小服务对象，后续业务 biz 在 AS-02 起增加。
- Handler 使用 `pkg/errno` 转换业务错误到 `base.BaseResp`，沿用现有 `idp/iam` 风格。

### 配置与端口

新增 `AssetConfig`，字段包含：

| 字段 | 说明 |
|---|---|
| `mongo` | `uri`、`db` |
| `redis` | `addr` |
| `server` | `addr`，默认 `:38084` |
| `registry` | `pkg/cloudwego.RegistryConfig` |
| `otel` | `pkg/otel.Config` |

新增 `deployments/config/asset.yaml`，默认注册名为 `asset`。对象存储配置不在 AS-01 引入，留到 AS-04。

### 错误码

`pkg/errno` 新增 asset 区段 `17001 - 17999`：

| 错误 | 语义 |
|---|---|
| `ErrAssetTypeNotFound` | 资产类型不存在 |
| `ErrAssetNotFound` | 资产不存在 |
| `ErrAssetVersionNotFound` | 资产版本不存在 |
| `ErrMediaObjectNotFound` | 媒体对象不存在 |
| `ErrAssetUploadSessionNotFound` | 上传会话不存在 |
| `ErrAssetConflict` | code、版本号、对象 key 等唯一约束冲突 |
| `ErrAssetInvalidPart` | parts 与资产类型 schema 不匹配 |
| `ErrAssetStorageError` | 对象存储或 URL 生成错误 |

通用参数错误继续复用 `ErrInvalidParam`，不要为每个字段新增错误码。

## 设计约束

- `asset` 是 Kitex RPC 服务；不要使用 Hertz，SSE/HTTP 例外只属于 `services/model/`。
- `services/edge-api` 后续只做 HTTP 适配，不直接访问资产 Mongo 集合。
- `asset` 不 import 其他业务服务内部 Go 包；后续跨服务读取必须走 IDL + RPC 或 MQ。
- `pkg/` 不得反向依赖 `services/asset`。
- 任何日志通过 `pkg/logger.Ctx(ctx)`；trace/user/tenant 元数据由 middleware 透传。
- AS-01 不引入云厂商 SDK，避免在服务地基阶段绑定 OSS 实现。

## 验收标准

### AI 自动验收

AS-01 已完成。实现时必须通过：

```bash
make gen
make build
cd services/asset && go test ./... -count=1
node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service
```

可判定输出：

- `idl/asset/asset.thrift` 能生成 Kitex 代码。
- `services/asset` 能编译并通过本服务测试。
- `make build` 覆盖新增服务，无编译错误。
- axm validate 无 error。

### 人类验收

- 人类确认 AS-01 没有提前实现资产库 CRUD、版本写入或 OSS 上传。
- 人类确认 DTO 字段足够承载 AS-02、AS-03、AS-04，但没有引入复杂状态机。
- 人类确认 `asset` 默认端口、服务名、错误码区段与当前平台规划一致。

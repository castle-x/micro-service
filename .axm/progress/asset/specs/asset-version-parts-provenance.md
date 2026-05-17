<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: asset
workflow-state: closed
state-updated: 2026-05-17
related:
  - ../roadmap.md
  - ../../../project/architecture.md
  - ../../../project/coding.md
  - ../../../knowledge/services/overview.md
  - ./service-contract-and-domain-models.md
  - ./asset-library-crud.md
-->

# AS-03 · 资产版本、组成部分与溯源

## 实施状态

截至 2026-05-17，AS-03 已完成并闭合。实现范围限定为资产实例级版本快照、parts 写入与校验、版本查询、复制旧版本、当前版本切换和版本级 provenance 字段；未实现 workflow 或 generator job。

闭合证据：

- tester QA 已在 2026-05-14 判定 local/QA 测试通过。
- 源码事实：`services/asset/biz/asset_version.go`、`services/asset/biz/asset_version_test.go`、`services/asset/dal/mongo/asset_version.go`、`services/asset/asset_version_handler_test.go` 已存在。
- 人类确认：2026-05-17 用户确认 asset 已开发完成。
- AS-04 已补齐媒体对象和上传会话，AS-03 原“媒体对象存在性强校验”后续边界已由 AS-04 承接。

## 背景

AS-02 已补齐个人资产库 CRUD，但资产实例仍只有基础元信息，`Asset.CurrentVersion` 在创建时固定为 `0`。AS-03 负责把这个字段背后的版本快照层做出来：一个资产实例可以拥有多个不可变版本，每个版本保存该资产在某一时刻的完整 `parts` 内容。

已确认的建模决策：

- 资产版本属于某个 `Asset` 实例整体，不属于 `AssetType`，也不细分到单个 part。
- `AssetVersion.Parts` 保存完整快照，不保存增量 diff。
- AS-03 不实现 workflow；workflow 未来只会成为写入版本的来源之一。
- AS-03 不实现媒体上传或 OSS 入库；媒体 ID 只作为版本 part 中的引用值。

## 目标

- 扩展 `idl/asset/asset.thrift`，定义资产版本创建、查询、列表、复制和当前版本切换 RPC。
- 在 `services/asset` 中实现 `asset_versions` repository、biz 校验、DTO 映射和 Kitex handler。
- 在 `services/edge-api` 中暴露 `/api/v1/assets/:id/versions/*` REST 门面。
- 将 `Asset.CurrentVersion` 从占位字段变成当前版本指针。
- 创建版本时按当前 `AssetType.PartSchemas` 校验 `parts`。
- 保持个人资产库模型：`workspace_id = 当前登录 user_id`，客户端不传 `workspace_id`。

## 范围

| 范围项 | AS-03 要做 |
|---|---|
| AssetVersion | 创建不可变版本、复制旧版本为新版本、详情、分页列表、读取当前版本 |
| Asset.CurrentVersion | 创建新版本后推进当前版本；支持显式切换当前版本 |
| Parts | 按 `AssetPartSchema.Key` 写入 `TEXT`、`JSON`、`MEDIA`、`MIXED` 值 |
| Provenance | 在版本上保存可选来源信息，但不创建 workflow / generator 任务记录 |
| edge-api | 登录态 REST 门面，透传当前 user_id 到 `Base.UserID` |
| 错误码 | 复用 `ErrAssetVersionNotFound`、`ErrAssetInvalidPart`、`ErrAssetConflict` 和 `ErrInvalidParam` |
| 测试 | 覆盖版本创建、校验、复制、当前版本切换、分页、跨用户隔离和 REST 映射 |

## 非目标

- 不实现 workflow、workflow run、step run、generator job 或异步任务。
- 不实现 part 级独立版本、part 级回滚或 part 级组合发布。
- 不实现资产类型 schema 历史、schema 迁移或旧版本自动重写。
- 不实现媒体对象 CRUD、上传会话、OSS SDK、预签名 URL、CDN URL 或媒体文件存在性强校验。
- 不实现版本更新或版本删除；版本记录在 AS-03 中不可变。
- 不实现版本 diff、合并冲突、多人协作、发布审批或资产市场。

## 已确认开发细节

### 版本层级

版本层级固定为：

```text
AssetType
  定义 part schema

Asset
  表示某个具体资产实例
  currentVersion 指向当前激活版本号

AssetVersion
  表示某个 Asset 的一次完整内容快照
  parts 保存文本、JSON、媒体引用等叶子值
```

示例：

```text
AssetType: Character
Asset: 角色「林夜」
Asset.currentVersion = 3

AssetVersion v3:
  parts:
    appearance: TEXT / JSON
    personality: TEXT
    reference_images: MEDIA
    voice: MEDIA
```

### 版本不可变

- `AssetVersion` 创建后不可更新。
- AS-03 不提供 `UpdateAssetVersion`。
- AS-03 不提供 `DeleteAssetVersion`。
- 需要修改内容时创建新版本。
- 需要基于旧版本继续编辑时复制旧版本为新版本，再提交覆盖后的 `parts`。

### 版本号生成

- 版本号由服务端分配，客户端不传 `version`。
- 同一 `asset_id` 下版本号从 `1` 开始递增。
- `asset_versions` 必须保留唯一索引：`asset_id + version`。
- 并发创建时允许用“读取最大版本号 + 1 + 唯一索引冲突重试”的方式生成版本号。
- 创建新版本后更新 `Asset.currentVersion`，但普通创建路径不得把当前版本倒退；实现可使用 `$max: {current_version: newVersion}` 或等价逻辑。
- 显式切换当前版本是唯一允许把 `Asset.currentVersion` 改小的路径。

### Parts 校验

创建版本和复制版本时，最终写入的 `parts` 必须按当前 `AssetType.PartSchemas` 校验：

- `parts` 中的 key 必须存在于当前 asset type schema。
- `required = true` 的 part 必须出现。
- 每个 part 的 `ValueKind` 必须在 schema 的 `AllowedValueKinds` 中。
- `TEXT` 需要非空 `Text`。
- `JSON` 需要非空 `JSON` 字符串；AS-03 至少校验它是合法 JSON 文本。
- `MEDIA` 需要非空 `MediaIDs`，且每个 media id 必须是合法 ObjectID 字符串。
- `MIXED` 允许 `Text`、`JSON`、`MediaIDs` 任意组合，但至少包含一种非空值；如果包含 `JSON`，也必须是合法 JSON 文本。
- schema 未声明但传入的多余 part 返回 `ErrAssetInvalidPart`。

AS-03 不做媒体对象存在性强校验；AS-04 实现媒体对象和上传能力后再补 workspace 级引用校验。

### Schema 漂移

AS-02 允许资产类型 schema 始终可编辑，因此 AS-03 需要接受第一版 schema 漂移：

- 新版本创建按“当前 asset type schema”校验。
- 旧版本读取时按原样返回，不因当前 schema 变化而重算或隐藏字段。
- 切换当前版本只要求目标版本属于该资产，不重新按当前 schema 校验。
- AS-03 不保存 schema snapshot；如果未来需要强审计或旧 schema 渲染，再单独拆 schema history。

### 当前版本切换

- `SetCurrentAssetVersion` 只移动 `Asset.currentVersion` 指针，不创建新版本。
- 目标版本必须存在且属于当前用户 workspace 下的目标资产。
- `Asset.currentVersion = 0` 表示该资产还没有任何版本。
- `GetCurrentAssetVersion` 在 `currentVersion = 0` 时返回 `ErrAssetVersionNotFound`。

### 复制旧版本

复制旧版本用于“从历史版本继续编辑”：

- 请求指定 `sourceVersion`。
- 服务端读取旧版本完整 `parts`。
- 如果请求带 `PartOverrides`，按 key 覆盖旧版本中的 part 值。
- 覆盖后的完整 parts 按当前 schema 校验。
- 校验通过后创建一个新的递增版本，并把 `Asset.currentVersion` 推进到新版本。

## IDL 契约

`idl/asset/asset.thrift` 继续使用 AS-01 已定义的 `AssetVersionDTO` 和 `AssetPartValueDTO`，AS-03 新增以下 RPC：

| 能力 | RPC |
|---|---|
| 创建版本 | `CreateAssetVersion` |
| 复制旧版本 | `CopyAssetVersion` |
| 查询版本详情 | `GetAssetVersion` |
| 查询当前版本 | `GetCurrentAssetVersion` |
| 分页查询版本列表 | `ListAssetVersions` |
| 切换当前版本 | `SetCurrentAssetVersion` |

建议请求/响应结构：

```thrift
struct CreateAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required map<string, AssetPartValueDTO> Parts
    4: optional string ChangeReason
    5: optional ProvenanceDTO Provenance
}

struct CreateAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
    3: optional AssetVersionDTO Version
}

struct CopyAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required i32 SourceVersion
    4: optional map<string, AssetPartValueDTO> PartOverrides
    5: optional string ChangeReason
    6: optional ProvenanceDTO Provenance
}

struct CopyAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
    3: optional AssetVersionDTO Version
}

struct GetAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required i32 Version
}

struct GetAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetVersionDTO Version
}

struct GetCurrentAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
}

struct GetCurrentAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetVersionDTO Version
}

struct ListAssetVersionsReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required base.PageReq Page
}

struct ListAssetVersionsResp {
    1: required base.BaseResp Base
    2: required list<AssetVersionDTO> Versions
    3: required base.PageResp Page
}

struct SetCurrentAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required i32 Version
}

struct SetCurrentAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
    3: optional AssetVersionDTO Version
}
```

IDL 规则：

- 每个请求都必须带 `base.BaseReq Base`。
- asset 必须要求 `Base.UserID` 非空。
- 客户端不传 `workspace_id`。
- `ListAssetVersions` 使用 `base.PageReq` / `base.PageResp`；默认 page `1`，size `20`，max `100`。
- `Version <= 0` 返回 `ErrInvalidParam`。

## REST 契约

所有 REST 路由均需要登录，统一挂在 `/api/v1/assets` 下：

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/:id/versions` | 创建资产版本 |
| `GET` | `/:id/versions` | 分页查询资产版本 |
| `GET` | `/:id/versions/current` | 查询当前版本 |
| `PUT` | `/:id/versions/current` | 切换当前版本 |
| `GET` | `/:id/versions/:version` | 查询指定版本 |
| `POST` | `/:id/versions/:version/copy` | 复制指定版本为新版本 |

REST 响应继续使用 `apiResp{code,message,data}` 包装。

路由注册时应保证 `current` 路径优先于 `:version` 匹配，避免把 `current` 解析成版本号。

### REST 请求示例

创建版本：

```json
{
  "parts": {
    "appearance": {
      "valueKind": 1,
      "text": "黑发，短外套，冷色调"
    },
    "reference_images": {
      "valueKind": 2,
      "mediaIds": ["665000000000000000000001"]
    }
  },
  "changeReason": "初版角色设定",
  "provenance": {
    "generationJobId": "manual-draft-001"
  }
}
```

切换当前版本：

```json
{
  "version": 2
}
```

复制旧版本并覆盖部分 part：

```json
{
  "partOverrides": {
    "appearance": {
      "valueKind": 1,
      "text": "黑发，长外套，增加金属饰品"
    }
  },
  "changeReason": "基于 v2 调整外观"
}
```

## 实现约束

### asset

- 在 `dal/mongo` 中补齐 `AssetVersionRepo` 的 create、find、list、count、findLatestVersion 等方法。
- `AssetVersionRepo.EnsureIndexes` 继续保证 `asset_id + version` 唯一索引。
- 新增 `AssetVersionBiz`，负责：
  - 从 `Base.UserID` 派生 workspace。
  - 校验 asset 存在且属于当前 workspace。
  - 加载 asset type 的当前 part schemas。
  - 校验 parts。
  - 分配递增版本号并处理唯一索引冲突重试。
  - 创建版本后更新 `Asset.currentVersion`。
  - 当前版本切换和跨用户隔离。
- `AssetImpl` 注入 `AssetVersionBiz`，保持 `Health` 不变。
- DTO 映射中 `AssetPartValueDTO.MediaIDs` 与 Mongo `ObjectID` 互转。
- 参数和业务错误直接返回 `pkg/errno` 中的 Errno。

### edge-api

- 使用 AS-03 新生成的 asset Kitex client。
- 在现有 `AssetHandler` 中增加版本相关方法和路由。
- 从 auth context 构造 `Base.UserID`。
- Query 分页默认值与 AS-02 保持一致：page `1`、size `20`、max `100`。
- 错误到 HTTP 状态映射：
  - `ErrAssetNotFound`、`ErrAssetVersionNotFound` -> `404`
  - `ErrAssetInvalidPart`、`ErrInvalidParam` -> `400`
  - `ErrAssetConflict`、`ErrDuplicateKey` -> `409`
  - 其他业务错误保持现有默认映射

## 测试计划

### asset biz

- 创建第一个版本后返回 version `1`，并把 asset `currentVersion` 更新为 `1`。
- 创建第二个版本后返回 version `2`，并推进当前版本。
- 创建版本时拒绝未知 part key。
- 创建版本时拒绝缺少 required part。
- 创建版本时拒绝不在 `AllowedValueKinds` 内的 value kind。
- 创建版本时拒绝非法 JSON 文本。
- 创建版本时拒绝非法 media id。
- 查询指定版本、当前版本、版本列表均只返回当前用户 workspace 下的数据。
- `currentVersion = 0` 时查询当前版本返回 `ErrAssetVersionNotFound`。
- 切换当前版本到旧版本成功，且不创建新版本。
- 复制旧版本并覆盖部分 parts 后创建新版本，旧版本保持不变。
- schema 更新后，旧版本仍可读取；创建新版本按当前 schema 校验。

### asset handler

- 缺少 `Base.UserID` 返回 `ErrInvalidParam`。
- 非法 asset id 或 version 返回 `ErrInvalidParam`。
- asset 不存在返回 `ErrAssetNotFound`。
- version 不存在返回 `ErrAssetVersionNotFound`。
- DTO 映射覆盖 `TEXT`、`JSON`、`MEDIA`、`MIXED`。

### edge-api handler

- JSON body 能正确绑定 `parts`、`partOverrides`、`changeReason`、`provenance`。
- 登录用户 ID 能写入 RPC `Base.UserID`。
- 版本列表 query 分页应用默认值和上限。
- `ErrAssetInvalidPart` 映射到 HTTP `400`。
- `ErrAssetVersionNotFound` 映射到 HTTP `404`。
- `ErrAssetConflict` 映射到 HTTP `409`。

## 验收标准

### AI 自动验收

实现 AS-03 时必须通过：

```bash
make gen
cd services/asset && go test ./... -count=1
cd services/edge-api && go test ./... -count=1
make build
node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service
```

可判定输出：

- `asset.thrift` 能生成 asset 和 edge-api 所需 Kitex 代码。
- asset 测试覆盖版本创建、复制、读取、当前版本切换、parts 校验和个人 workspace 隔离。
- edge-api 测试覆盖 REST 请求绑定、user_id 透传、分页默认值和错误码到 HTTP 状态映射。
- `make build` 能构建所有服务。
- axm validate 无 error。

已记录的 tester QA 证据：

```bash
cd services/asset && go test ./biz -count=1
cd services/asset && go test . -count=1
cd services/asset && go test ./... -count=1
cd services/edge-api && go test ./handler -count=1
cd services/edge-api && GOCACHE=/private/tmp/codex-gocache-as03-tester go test ./... -count=1
```

说明：以上测试已由 tester QA 判定通过。文档更新 agent 只补充进度记录与 axm validate，不替代主线程最终 repo-wide build/validate。

### 人类验收

- 人类确认资产版本挂在 `Asset` 实例上，不挂在 `AssetType` 或单个 part 上。
- 人类确认 `AssetVersion.Parts` 是完整快照，不是增量 diff。
- 人类确认 AS-03 不实现 workflow、OSS 上传、媒体对象创建或 CDN URL。
- 人类确认旧版本可读取、可切为当前版本，即使当前 asset type schema 已变化。
- 人类确认版本不可变；修改内容必须创建新版本。

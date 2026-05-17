<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: roadmap
initiative: asset
related:
  - ../generation-platform/roadmap.md
  - ../../project/architecture.md
-->

# Asset Service 路线图

> 当前文档记录已经基本敲定的资产服务设计。第一版坚持必要、简单、可扩展，不提前设计复杂资产状态机。
>
> 最后更新：2026-05-17

## 背景与目标

`asset` 是通用生图平台的基础服务，负责保存用户产生和沉淀的数字资产。

它不仅保存已经确认的资产，也保存历史产物。区别只在于：

```text
savedToLibrary = true   已保存到资产库
savedToLibrary = false  历史产物 / 未分类
```

第一版目标：

- 支持用户自定义资产类型
- 支持资产类型定义多个组成部分
- 支持每个组成部分声明可存储的内容类型
- 支持资产实例的增删改查
- 支持资产版本号和生产溯源
- 支持图片等媒体文件接入 OSS 对象存储
- 为真实线上环境预留 CDN URL 能力

## 非目标

第一版暂不做：

- 复杂资产状态机
- 资产市场、公开发布、交易
- 复杂权限协作
- 自动清理策略
- 智能分类规则
- 任意复杂资产关系图

## 核心概念

### AssetType

用户自定义的资产类型，例如：

```text
角色资产
宠物资产
汽车资产
电商产品资产
写真风格资产
```

资产类型定义“这种资产由哪些部分组成”。

### AssetPartSchema

资产类型下的组成部分定义，例如角色资产包含：

```text
背景
DNA
脸
身体
风格
```

每个部分声明允许保存哪些内容形态：

```text
text        文本
media       图片/视频等媒体
json        结构化数据
mixed       文本 + 媒体 + JSON 的组合
```

### Asset

某个具体资产实例，例如：

```text
角色资产 / 角色 A
产品资产 / 连衣裙 X
汽车资产 / 车型 Y
```

资产可以保存到资产库，也可以只是历史产物。

### AssetVersion

资产版本。每次重要修改或工作流写入后，可以生成一个新版本。

版本中保存各个部分的实际内容。

### MediaObject

媒体文件记录，对接 OSS / CDN。

图片本体存在对象存储，Mongo 只保存对象存储 key、元数据、访问 URL 状态和衍生图信息。

## 建议 Mongo 集合

```text
asset_types
assets
asset_versions
media_objects
asset_categories
storage_upload_sessions
```

### asset_types

```ts
AssetType {
  id
  workspaceId
  name
  code
  description
  partSchemas: AssetPartSchema[]
  createdBy
  createdAt
  updatedAt
}
```

```ts
AssetPartSchema {
  key
  name
  description
  allowedValueKinds   // text, media, json, mixed
  multiple
  required
  sortOrder
}
```

### assets

```ts
Asset {
  id
  workspaceId
  typeId
  name
  description
  savedToLibrary
  categoryId
  currentVersion
  coverMediaId
  source
  provenance
  createdBy
  createdAt
  updatedAt
  deletedAt
}
```

说明：

- `savedToLibrary` 是第一版唯一核心资产库状态。
- `categoryId` 用于用户自定义分类。
- `source` 表示来源，例如 `upload`、`workflow`、`generation`、`import`。
- `provenance` 记录生产溯源，例如 `workflowRunId`、`stepRunId`、`generationJobId`。

### asset_versions

```ts
AssetVersion {
  id
  assetId
  version
  parts
  changeReason
  provenance
  createdBy
  createdAt
}
```

`parts` 按 `AssetPartSchema.key` 存储：

```ts
parts: {
  background: {
    valueKind: "mixed",
    text: "...",
    json: {...},
    mediaIds: []
  },
  face: {
    valueKind: "mixed",
    text: "...",
    json: {...},
    mediaIds: ["media_1", "media_2"]
  }
}
```

### media_objects

```ts
MediaObject {
  id
  workspaceId
  provider
  bucket
  objectKey
  cdnUrl
  urlVisibility
  contentType
  size
  width
  height
  sha256
  variants
  source
  provenance
  createdBy
  createdAt
}
```

说明：

- `provider` 支持阿里云 OSS、S3、腾讯云 COS、本地开发存储等实现。
- `cdnUrl` 为线上 CDN 预留；第一版可只保存和返回，不必实现完整 CDN 刷新。
- `variants` 可记录缩略图、预览图、原图等衍生资源。

### asset_categories

```ts
AssetCategory {
  id
  workspaceId
  name
  parentId
  sortOrder
  createdBy
  createdAt
  updatedAt
}
```

说明：

- 分类是用户资产库的管理方式，不等于资产类型。
- 资产类型回答“这是什么资产”；分类回答“用户想把它放在哪里”。

### storage_upload_sessions

```ts
StorageUploadSession {
  id
  workspaceId
  provider
  bucket
  objectKey
  status
  expiresAt
  createdBy
  createdAt
  finalizedAt
}
```

用于前端直传 OSS 的会话记录。

## 核心能力

### 资产类型管理

- 创建资产类型
- 更新资产类型基础信息
- 定义和调整资产组成部分
- 查询资产类型详情
- 删除未使用的资产类型

### 资产管理

- 创建资产实例
- 更新资产名称、描述、分类、封面
- 保存到资产库 / 移出资产库
- 查询资产详情和当前版本
- 查询历史产物
- 查询某个资产类型下的资产列表
- 软删除资产

### 资产版本管理

- 创建资产版本
- 查询版本列表
- 查询指定版本详情
- 回滚或复制旧版本作为新版本
- 记录版本生产溯源

### 媒体对象管理

- 创建上传会话
- 完成上传并登记媒体对象
- 从模型生成结果写入 OSS
- 查询媒体对象元数据
- 生成访问 URL
- 保存缩略图、预览图、原图等 variants

### 资产分类管理

- 创建分类
- 更新分类
- 删除空分类
- 移动资产到分类
- 按分类筛选资产库

## OSS / CDN 设计方向

第一版需要打通对象存储通用能力：

```text
Client → edge-api → asset: create upload session
asset → OSS: presigned URL / policy
Client → OSS: upload
Client → edge-api → asset: finalize upload
asset → Mongo: media_objects + optional asset
```

模型生成结果入库：

```text
generator-service → asset: ingest generated media
asset → OSS: save original / preview / thumbnail
asset → Mongo: media_objects + history asset
```

CDN 第一版只做抽象：

- `MediaObject.cdnUrl`
- `urlVisibility`
- 访问 URL 生成接口

真实刷新、预热、鉴权策略等生产细节后续再拆 spec。

## 阶段路线图

| Phase | 主题 | 状态 | 产物 |
|---|---|---|---|
| AS-01 | 服务契约与领域模型 | 已完成 | [`service-contract-and-domain-models.md`](specs/service-contract-and-domain-models.md)：`idl/asset/asset.thrift`、`services/asset` 骨架、Mongo 模型与仓储、配置、错误码、服务启动验证 |
| AS-02 | 资产库 CRUD | 已完成 | [`asset-library-crud.md`](specs/asset-library-crud.md)：自定义资产类型、组成部分 schema、分类、资产实例、资产库保存状态、edge-api REST 适配 |
| AS-03 | 资产版本、组成部分与溯源 | 已完成 | [`asset-version-parts-provenance.md`](specs/asset-version-parts-provenance.md)：资产实例级版本快照、parts 写入、版本查询、复制旧版本、当前版本切换、生产溯源字段；不包含 workflow 或 generator job |
| AS-04 | 媒体存储、上传与生成入库 | 已完成第一版 | [`media-storage-upload-and-ingest.md`](specs/media-storage-upload-and-ingest.md)：OSS 抽象、上传会话、前端直传、媒体对象、访问 URL、media 引用校验；模型生成结果入库留给后续 `generator` 集成 |

## 阶段依赖

```text
AS-01 已完成
  → AS-02 已完成
  → AS-03 已完成
  → AS-04 已完成第一版媒体上传与访问 URL
```

说明：

- AS-01 至 AS-04 已闭合为 asset 第一版能力：资产类型、资产实例、分类、版本、媒体对象、上传会话和访问 URL。
- AS-04 合并原 A4、A5、A6：第一版已完成媒体存储与上传闭环；CDN 只保留字段与访问 URL 抽象，不拆成独立阶段。
- 模型生成结果入库不再归入 asset 当前阶段，后续由 `generator` 在调用 asset 媒体与版本接口时闭合。

## 与其他服务的关系

| 调用方 | 关系 |
|---|---|
| `edge-api` | 暴露资产类型、资产库、上传会话等 HTTP API；不直接访问 Mongo |
| `workflow` | 将节点产物写入某个资产部分，或创建历史产物 |
| `generator` | 将模型生成图片交给 asset 入库 |
| `agent` / `llm` | 读取资产上下文、调用模型或工具；不直接管理资产主存储 |
| 旧 `model` | 不直接管理资产；后续由 `llm` / `generator` 新路线替代 |

## 下一步

asset initiative 当前已闭合。后续工作不在本路线内继续追加：

- `generator -> asset` 生成结果入库协议放到 `generation-platform` 的 `GP-05 generator` spec 中拆。
- 真实 OSS smoke test 依赖用户提供 bucket 与密钥，作为环境验收或后续回归项，不阻塞 asset 第一版闭合。
- 缩略图、对象生命周期、multipart 上传、大文件和清理策略均按需另拆新 spec。

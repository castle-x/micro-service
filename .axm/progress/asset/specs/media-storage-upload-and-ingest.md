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
  - ../../../project/observability.md
  - ../../../knowledge/services/overview.md
  - ./service-contract-and-domain-models.md
  - ./asset-library-crud.md
  - ./asset-version-parts-provenance.md
-->

# AS-04 · 媒体存储、上传会话与 OSS 接入

## 实施状态

截至 2026-05-17，AS-04 已完成并闭合第一版范围：支持用户/前端上传图片到对象存储，并把上传完成后的对象登记为 `MediaObject`；支持媒体对象查询、访问 URL 和 media 引用校验。模型生成结果入库不作为本阶段目标，后续由 `generator` 服务在调用 asset 接口时闭合。

闭合证据：

- 源码事实：`services/asset/biz/media.go`、`services/asset/biz/media_test.go`、`services/asset/storage/{client,aliyun_oss}.go`、`services/asset/dal/mongo/{media_object,storage_upload_session}.go`、`services/asset/handler_media_test.go` 已存在。
- edge-api 已暴露资产媒体路由，长期事实见 `../../../knowledge/services/overview.md`。
- 人类确认：2026-05-17 用户确认 asset 已开发完成。
- 真实 OSS smoke test 依赖用户提供 bucket 与密钥，作为环境验收 / 回归项 deferred，不阻塞 AS-04 第一版闭合。

## 背景

AS-01 已建立 `media_objects` 与 `storage_upload_sessions` 的 Mongo 模型、DTO 和索引骨架；AS-03 已允许 `AssetVersion.Parts` 保存 `mediaIds`，但当时只校验 media id 是合法 ObjectID 字符串，不验证媒体对象是否真实存在。

AS-04 要把这条链路闭合：

```text
前端申请上传会话
  -> asset 服务生成对象 key 与 OSS 预签名上传 URL
  -> 前端直传图片到 OSS
  -> 前端 finalize 上传会话
  -> asset 服务校验对象元数据并创建 MediaObject
  -> Asset.coverMediaId / AssetVersion.parts.mediaIds 可引用真实 MediaObject
```

第一版选择“前端直传 OSS”，避免 asset 服务转发大文件流量，也方便用真实 OSS bucket 做端到端测试。

## 目标

- 扩展 `idl/asset/asset.thrift`，新增上传会话、媒体对象查询和访问 URL RPC。
- 在 `services/asset` 新增对象存储抽象，第一版实现阿里云 OSS provider。
- 支持前端通过短期预签名 `PUT` URL 上传图片。
- 上传完成后通过 finalize 创建 `media_objects` 记录，并把 `storage_upload_sessions` 状态置为 `FINALIZED`。
- 支持按 `media_id` 查询媒体对象，并获取短期访问 URL。
- 在 AS-03 parts 校验基础上补齐 media id 存在性与 workspace 隔离校验。
- 在 `services/edge-api` 暴露登录态 REST 门面，保持 edge-api 只做参数适配和 RPC 调用。
- 补齐对象存储外部 API 的日志和 OTel span 设计，禁止记录 access key、签名 URL query、Authorization 等敏感信息。

## 范围

| 范围项 | AS-04 要做 |
|---|---|
| Storage adapter | 新增对象存储接口；实现阿里云 OSS 的 presign put、presign get、head object |
| Upload session | 创建上传会话、生成 object key、保存 session、返回预签名上传信息 |
| Finalize | 校验 session、校验对象已上传、创建或返回 MediaObject、更新 session 状态 |
| MediaObject | 创建、详情、分页列表、按 object key 去重或幂等返回 |
| Access URL | 对私有对象生成短期签名访问 URL；CDN 字段只返回和预留 |
| Media reference validation | `coverMediaId` 和 `AssetVersion.parts.mediaIds` 必须存在且属于当前 workspace |
| edge-api | `/api/v1/assets/media/*` REST 门面 |
| 配置 | asset 服务增加 storage/aliyun_oss 配置；密钥只能来自环境变量或密钥系统 |
| 测试 | fake storage 单测 + handler 绑定测试 + 可选真实 OSS smoke test |

## 非目标

- 不实现服务端代理上传文件内容。
- 不实现 multipart 分片上传、断点续传或大文件上传。
- 不实现模型生图结果下载/转存/入库；该能力后续跟随 `generator` 服务拆分。
- 不实现 OSS 回调验签、异步扫描、病毒检测或内容安全审核。
- 不实现缩略图/预览图自动生成。
- 不实现 CDN 刷新、预热、签名鉴权或域名切换策略。
- 不实现媒体对象删除、对象生命周期清理或孤儿文件回收。
- 不实现视频、音频、任意二进制文件的产品化支持；第一版按图片上传设计。
- 不把 OSS SDK 或 provider 代码放入 `pkg/`，除非未来多个服务复用同一抽象。

## 已确认开发细节

### 第一版上传策略

第一版采用短期预签名 URL：

1. 客户端调用 asset 创建上传会话，提交 `contentType`、`size`、可选 `filename`、可选 `sha256`。
2. asset 根据当前用户派生 `workspace_id = user_id`，生成不可猜测的 `objectKey`。
3. asset 持久化 `storage_upload_sessions`，状态为 `CREATED`。
4. asset 使用 OSS adapter 生成预签名 `PUT` URL，并返回 `uploadUrl`、`uploadMethod`、`uploadHeaders`、`expiresAt`。
5. 客户端按返回的 method/header 直传图片到 OSS。
6. 客户端调用 finalize；asset 调用 OSS `HeadObject` 校验对象存在、大小和 content type。
7. asset 创建 `media_objects`，并把 session 状态置为 `FINALIZED`。

选择直传的原因：

- 图片字节流不经过 edge-api/asset，减少服务带宽和内存压力。
- 对象 key、bucket、过期时间和 content type 仍由服务端控制。
- 后续要接 CDN、multipart 或生成结果转存时，可以复用 `MediaObject` 主数据。

### 对象 key 规则

对象 key 由服务端生成，不使用用户原始文件名作为路径主体。

建议格式：

```text
<object_key_prefix>/<workspace_id>/uploads/<yyyy>/<mm>/<dd>/<session_id>/original.<ext>
```

示例：

```text
assets/665000000000000000000001/uploads/2026/05/14/665111111111111111111111/original.png
```

规则：

- `workspace_id` 只来自 `Base.UserID`，客户端不能传。
- `session_id` 使用 Mongo ObjectID，避免 key 冲突。
- 扩展名优先由 `contentType` 映射得到：`image/jpeg -> jpg`、`image/png -> png`、`image/webp -> webp`、`image/gif -> gif`。
- `filename` 只用于扩展名兜底或未来展示字段；第一版不把原始文件名写入 object key，避免泄露用户输入。
- `object_key_prefix` 默认 `assets`，可通过配置覆盖。

### 允许的文件类型和大小

第一版按图片上传设计，默认允许：

```text
image/jpeg
image/png
image/webp
image/gif
```

默认最大文件大小建议为 `20 MiB`。实现时通过配置控制：

```yaml
storage:
  max_upload_size_bytes: ${ASSET_STORAGE_MAX_UPLOAD_SIZE_BYTES:20971520}
  allowed_content_types:
    - image/jpeg
    - image/png
    - image/webp
    - image/gif
```

校验规则：

- `contentType` 不能为空，且必须在 allowlist 内。
- `size` 必须大于 `0` 且不超过 `max_upload_size_bytes`。
- 如果客户端传 `sha256`，只保存为期望值；第一版不强制服务端读取对象内容计算 sha256。
- 如果客户端在 finalize 传 `width` / `height`，asset 保存到 `MediaObject`；第一版不强制服务端解析图片尺寸。

### 配置

`deployments/config/asset.yaml` 增加 storage 配置。敏感值只允许通过环境变量注入，不能提交真实密钥。

建议结构：

```yaml
storage:
  provider: "${ASSET_STORAGE_PROVIDER:aliyun_oss}"
  object_key_prefix: "${ASSET_STORAGE_OBJECT_KEY_PREFIX:assets}"
  upload_url_ttl_seconds: ${ASSET_UPLOAD_URL_TTL_SECONDS:900}
  download_url_ttl_seconds: ${ASSET_DOWNLOAD_URL_TTL_SECONDS:900}
  max_upload_size_bytes: ${ASSET_STORAGE_MAX_UPLOAD_SIZE_BYTES:20971520}
  allowed_content_types:
    - image/jpeg
    - image/png
    - image/webp
    - image/gif
  aliyun_oss:
    region: "${ALIYUN_OSS_REGION}"
    endpoint: "${ALIYUN_OSS_ENDPOINT}"
    bucket: "${ALIYUN_OSS_BUCKET}"
    access_key_id: "${ALIYUN_OSS_ACCESS_KEY_ID}"
    access_key_secret: "${ALIYUN_OSS_ACCESS_KEY_SECRET}"
    security_token: "${ALIYUN_OSS_SECURITY_TOKEN}"
    public_base_url: "${ALIYUN_OSS_PUBLIC_BASE_URL}"
    cdn_base_url: "${ALIYUN_OSS_CDN_BASE_URL}"
```

说明：

- `security_token` 可选，用于 STS 临时凭证。
- `public_base_url` 可选；bucket 公开读或绑定公开域名时可返回公开 URL。
- `cdn_base_url` 只用于拼出 `cdnUrl` 字段，不实现 CDN 刷新或签名。
- 如果 provider 是 `aliyun_oss`，`region`、`endpoint`、`bucket`、`access_key_id`、`access_key_secret` 必填。
- 本地单测使用 fake storage adapter，不依赖真实 OSS。

### 对象存储接口

在 `services/asset` 内部新增 storage 包，不放入 `pkg`：

```go
type ObjectSpec struct {
    Bucket      string
    ObjectKey   string
    ContentType string
    Size        int64
}

type PresignedRequest struct {
    Method    string
    URL       string
    Headers   map[string]string
    ExpiresAt int64
}

type ObjectMeta struct {
    Bucket      string
    ObjectKey   string
    ContentType string
    Size        int64
    ETag        string
}

type Client interface {
    Provider() assetmodel.StorageProvider
    Bucket() string
    PresignPut(ctx context.Context, spec ObjectSpec, ttl time.Duration) (*PresignedRequest, error)
    PresignGet(ctx context.Context, bucket, objectKey string, ttl time.Duration) (*PresignedRequest, error)
    HeadObject(ctx context.Context, bucket, objectKey string) (*ObjectMeta, error)
}
```

阿里云 OSS adapter 负责把 SDK 错误映射为业务错误：

- SDK 配置或签名失败 -> `ErrAssetStorageError`
- `HeadObject` 发现对象不存在 -> 交给 biz 转成 `ErrAssetConflict`，表示上传尚未完成或 object key 不匹配
- 网络/权限/服务端错误 -> `ErrAssetStorageError`

### Mongo 模型调整

当前 `MediaObject` 已有主要字段，AS-04 只需补强 repository 方法。

`StorageUploadSession` 需要增加上传校验和幂等字段：

```go
type StorageUploadSession struct {
    db.BaseDoc `bson:",inline"`

    WorkspaceID string
    Provider    StorageProvider
    Bucket      string
    ObjectKey   string
    Status      UploadSessionStatus
    ExpiresAt   int64
    CreatedBy   primitive.ObjectID
    FinalizedAt int64

    ContentType string
    Size        int64
    SHA256      string
    MediaID     primitive.ObjectID
}
```

说明：

- `MediaID` 只在 `FINALIZED` 后写入，用于 finalize 幂等返回。
- `SHA256` 是客户端声明值；第一版不保证强校验。
- 不持久化 presigned URL，避免把带签名的 URL 长期落库。

### Repository 能力

`MediaObjectRepo` 需要补齐：

- `CreateMediaObject(ctx, doc)`
- `FindMediaObjectByID(ctx, workspaceID, id)`
- `FindMediaObjectByObjectKey(ctx, provider, bucket, objectKey)`
- `ListMediaObjects(ctx, workspaceID, pageNum, pageSize, source, contentType)`

`StorageUploadSessionRepo` 需要补齐：

- `CreateStorageUploadSession(ctx, doc)`
- `FindStorageUploadSessionByID(ctx, workspaceID, id)`
- `UpdateStorageUploadSession(ctx, doc)`
- `SetUploadSessionFinalized(ctx, workspaceID, sessionID, mediaID, finalizedAt)`
- `SetUploadSessionExpired(ctx, workspaceID, sessionID)`

唯一索引继续沿用：

- `media_objects`: unique `provider + bucket + object_key`
- `storage_upload_sessions`: unique `provider + bucket + object_key`

### Biz 分层

新增 `MediaBiz` 或等价业务对象，负责：

- 从 `Base.UserID` 派生 workspace。
- 校验上传输入：content type、size、ttl。
- 生成 object key。
- 创建上传会话。
- 调用 storage adapter 生成预签名上传 URL。
- finalize 时校验 session 状态、过期时间、OSS object 元信息。
- 创建 `MediaObject`，处理重复 finalize 的幂等返回。
- 查询媒体对象和生成访问 URL。

`AssetBiz` 与 `AssetVersionBiz` 需要接入 media 引用校验：

- `Asset.Create` / `Asset.Update` 中的 `CoverMediaID` 若非空，必须存在于当前 workspace。
- `AssetVersion.Create` / `AssetVersion.Copy` 对 `MEDIA` 和 `MIXED` parts 中的每个 media id 进行存在性校验。
- part 中 media id 不存在或属于其他 workspace 时返回 `ErrAssetInvalidPart`，避免泄露跨 workspace 资源存在性。
- 顶层 `GetMediaObject` 找不到媒体对象时返回 `ErrMediaObjectNotFound`。

### IDL 契约

`idl/asset/asset.thrift` 继续使用已有 `MediaObjectDTO`、`StorageUploadSessionDTO`、`StorageProvider`、`URLVisibility`、`UploadSessionStatus`。

建议新增通用预签名 DTO：

```thrift
struct StoragePresignedURLDTO {
    1: required string Method
    2: required string URL
    3: optional map<string, string> Headers
    4: required i64 ExpiresAt
}
```

建议扩展 `StorageUploadSessionDTO`：

```thrift
struct StorageUploadSessionDTO {
    1: required string SessionID
    2: required string WorkspaceID
    3: required StorageProvider Provider
    4: required string Bucket
    5: required string ObjectKey
    6: required UploadSessionStatus Status
    7: required i64 ExpiresAt
    8: required string CreatedBy
    9: required i64 CreatedAt
    10: optional i64 FinalizedAt
    11: optional string ContentType
    12: optional i64 Size
    13: optional string SHA256
    14: optional string MediaID
}
```

新增 RPC：

```thrift
struct CreateStorageUploadSessionReq {
    1: required base.BaseReq Base
    2: required string ContentType
    3: required i64 Size
    4: optional string Filename
    5: optional string SHA256
}

struct CreateStorageUploadSessionResp {
    1: required base.BaseResp Base
    2: optional StorageUploadSessionDTO Session
    3: optional StoragePresignedURLDTO Upload
}

struct FinalizeStorageUploadSessionReq {
    1: required base.BaseReq Base
    2: required string SessionID
    3: optional string SHA256
    4: optional i32 Width
    5: optional i32 Height
}

struct FinalizeStorageUploadSessionResp {
    1: required base.BaseResp Base
    2: optional StorageUploadSessionDTO Session
    3: optional MediaObjectDTO Media
}

struct GetMediaObjectReq {
    1: required base.BaseReq Base
    2: required string MediaID
}

struct GetMediaObjectResp {
    1: required base.BaseResp Base
    2: optional MediaObjectDTO Media
}

struct ListMediaObjectsReq {
    1: required base.BaseReq Base
    2: required base.PageReq Page
    3: optional AssetSource Source
    4: optional string ContentType
}

struct ListMediaObjectsResp {
    1: required base.BaseResp Base
    2: required list<MediaObjectDTO> Media
    3: required base.PageResp Page
}

struct GetMediaObjectAccessURLReq {
    1: required base.BaseReq Base
    2: required string MediaID
    3: optional i32 ExpiresInSeconds
}

struct GetMediaObjectAccessURLResp {
    1: required base.BaseResp Base
    2: optional MediaObjectDTO Media
    3: optional StoragePresignedURLDTO Access
}
```

`AssetService` 新增：

```thrift
CreateStorageUploadSessionResp CreateStorageUploadSession(1: CreateStorageUploadSessionReq req)
FinalizeStorageUploadSessionResp FinalizeStorageUploadSession(1: FinalizeStorageUploadSessionReq req)
GetMediaObjectResp GetMediaObject(1: GetMediaObjectReq req)
ListMediaObjectsResp ListMediaObjects(1: ListMediaObjectsReq req)
GetMediaObjectAccessURLResp GetMediaObjectAccessURL(1: GetMediaObjectAccessURLReq req)
```

IDL 规则：

- 所有请求必须携带 `base.BaseReq Base`。
- asset 必须要求 `Base.UserID` 非空。
- 客户端不传 `workspace_id`、`bucket`、`provider`、`objectKey`。
- `ExpiresInSeconds` 归一化：小于等于 0 使用默认下载 TTL；超过配置上限则截断到上限。

### REST 契约

所有 REST 路由均需要登录，统一挂在 `/api/v1/assets` 下。媒体相关路由必须注册在 `/:id` 资产详情路由之前，避免被动态路由吞掉。

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/media/upload-sessions` | 创建上传会话并返回预签名上传 URL |
| `POST` | `/media/upload-sessions/:session_id/finalize` | finalize 上传会话，创建 MediaObject |
| `GET` | `/media` | 分页查询当前用户媒体对象 |
| `GET` | `/media/:id` | 查询媒体对象详情 |
| `GET` | `/media/:id/access-url` | 获取媒体对象访问 URL |

REST 响应继续使用 `apiResp{code,message,data}` 包装。

创建上传会话请求：

```json
{
  "content_type": "image/png",
  "size": 1048576,
  "filename": "avatar.png",
  "sha256": "optional-client-declared-sha256"
}
```

创建上传会话响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "session": {
      "sessionID": "665111111111111111111111",
      "provider": 2,
      "bucket": "example-bucket",
      "objectKey": "assets/665.../uploads/2026/05/14/665.../original.png",
      "status": 1,
      "expiresAt": 1778755200
    },
    "upload": {
      "method": "PUT",
      "url": "https://...",
      "headers": {
        "Content-Type": "image/png"
      },
      "expiresAt": 1778755200
    }
  }
}
```

前端直传要求：

- 使用响应中的 `method`、`url`、`headers` 原样上传。
- 如果返回 `Content-Type` header，上传时必须带同样值。
- 浏览器环境需要 OSS bucket CORS 允许目标前端域名执行 `PUT`、`GET`、`HEAD`，并允许 `Content-Type` 等签名相关 header。

Finalize 请求：

```json
{
  "sha256": "optional-client-declared-sha256",
  "width": 1024,
  "height": 1024
}
```

Finalize 响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "session": {},
    "media": {
      "mediaID": "665222222222222222222222",
      "provider": 2,
      "bucket": "example-bucket",
      "objectKey": "assets/...",
      "urlVisibility": 3,
      "contentType": "image/png",
      "size": 1048576,
      "width": 1024,
      "height": 1024,
      "source": 1
    }
  }
}
```

### 状态和幂等规则

`storage_upload_sessions.status` 使用已有枚举：

| 状态 | 含义 |
|---|---|
| `CREATED` | 会话已创建，等待客户端上传并 finalize |
| `FINALIZED` | 已创建 MediaObject，后续 finalize 幂等返回同一个媒体对象 |
| `EXPIRED` | 超过 `expiresAt` 后不再允许 finalize |
| `CANCELLED` | 第一版不提供取消接口，保留状态 |

Finalize 规则：

- 找不到 session 或 session 不属于当前 workspace -> `ErrAssetUploadSessionNotFound`。
- session 已过期 -> 更新状态为 `EXPIRED`，返回 `ErrAssetConflict`。
- session 已 `FINALIZED` 且 `MediaID` 可查到 -> 返回已有 session 和 media。
- OSS object 不存在或 head 元信息还不可见 -> `ErrAssetConflict`，客户端可稍后重试 finalize。
- OSS object size 与 session size 不一致 -> `ErrAssetConflict`。
- OSS object content type 与 session content type 明显不一致 -> `ErrAssetConflict`。
- 创建 `MediaObject` 时遇到唯一键冲突 -> 按 `provider + bucket + objectKey` 查询已存在对象并返回，实现幂等。

### URL 可见性策略

第一版默认按私有对象处理：

- `MediaObject.URLVisibility = SIGNED`
- `MediaObject.CDNURL` 仅在配置了 `cdn_base_url` 时填入，不保证可直接访问。
- `GetMediaObjectAccessURL` 对 `SIGNED` 对象生成短期 `GET` URL。

如果配置了 `public_base_url`，可选择：

- `URLVisibility = PUBLIC`
- 直接返回 `<public_base_url>/<objectKey>`

但第一版推荐仍使用私有 bucket + signed URL，避免过早暴露公开读策略。

### 与 Asset/AssetVersion 的集成

AS-04 完成后，媒体引用从“弱引用”升级为“workspace 内强校验引用”：

- `Asset.CoverMediaID` 必须指向当前 workspace 下已存在的 `MediaObject`。
- `AssetVersion.Parts` 中 `MEDIA` 和 `MIXED` 的每个 media id 必须指向当前 workspace 下已存在的 `MediaObject`。
- 旧版本读取不回查媒体对象，仍按原样返回，避免历史数据因后续删除策略变化而不可读。
- 创建新版本、复制旧版本并覆盖 parts 时，都使用当前 media 校验规则。

示例完整使用路径：

1. 上传图片并 finalize 得到 `mediaID`。
2. 创建资产时把该 `mediaID` 作为 `cover_media_id`。
3. 创建资产版本时，在 part 中写入：

```json
{
  "reference_images": {
    "value_kind": 2,
    "media_ids": ["665222222222222222222222"]
  }
}
```

### 错误码与 HTTP 映射

复用已有 asset 错误码：

| 错误 | 场景 | REST 状态 |
|---|---|---|
| `ErrInvalidParam` | content type、size、ttl、id 格式非法 | `400` |
| `ErrAssetInvalidPart` | part 中 media id 不存在或跨 workspace | `400` |
| `ErrMediaObjectNotFound` | 查询媒体对象不存在 | `404` |
| `ErrAssetUploadSessionNotFound` | 查询或 finalize session 不存在 | `404` |
| `ErrAssetConflict` | session 过期、对象未上传、size/content type 不匹配 | `409` |
| `ErrDuplicateKey` | 唯一键冲突且无法幂等恢复 | `409` |
| `ErrAssetStorageError` | OSS 签名、HeadObject、网络、权限等存储错误 | `502` |

需要更新 `services/edge-api/handler/auth.go` 的 `bizCodeToHTTP`：

- `ErrMediaObjectNotFound`、`ErrAssetUploadSessionNotFound` -> `404`
- `ErrAssetStorageError` -> `502 Bad Gateway`

### 可观测性

新增对象存储外部 API 链路，必须遵守 `.axm/project/observability.md`。

建议 span：

| 操作 | span name | 关键属性 |
|---|---|---|
| 生成上传 URL | `OSS presign_put` | `storage.provider`、`storage.bucket`、`storage.operation=presign_put` |
| 生成访问 URL | `OSS presign_get` | `storage.provider`、`storage.bucket`、`storage.operation=presign_get` |
| 校验对象 | `OSS head_object` | `storage.provider`、`storage.bucket`、`storage.operation=head_object` |

禁止写入 span/log：

- access key、secret、security token
- presigned URL 完整 query
- Authorization、Cookie、签名 header
- 用户原始文件名中可能包含的敏感信息

允许写入：

- provider、bucket、operation、content type、size、错误码
- object key 可在 debug 场景使用，但默认日志避免高基数批量输出

### 阿里云 OSS 测试准备

真实 OSS smoke test 需要用户提供：

```text
ALIYUN_OSS_REGION
ALIYUN_OSS_ENDPOINT
ALIYUN_OSS_BUCKET
ALIYUN_OSS_ACCESS_KEY_ID
ALIYUN_OSS_ACCESS_KEY_SECRET
```

如果使用浏览器直传，还需要在 OSS bucket 配置 CORS：

| 项 | 建议 |
|---|---|
| Allowed Origins | 本地前端域名，例如 `http://localhost:3000` / `http://localhost:5173` |
| Allowed Methods | `PUT`、`GET`、`HEAD` |
| Allowed Headers | `Content-Type`、`x-oss-*` |
| Expose Headers | `ETag`、`Content-Length`、`Content-Type` |

AI 自动测试不依赖真实 OSS；真实 OSS smoke test 只在用户提供配置后执行。

## 实现约束

### asset

- `asset` 仍是 Kitex RPC 服务，不引入 Hertz。
- 对象存储 SDK 只能在 `services/asset` 内部使用，不能放入 `pkg`。
- `MediaBiz` 不直接依赖 edge-api 或 model service。
- 所有业务错误使用 `pkg/errno`。
- 所有日志使用 `logger.Ctx(ctx)`。
- 上传 URL 和访问 URL 只作为响应返回，不落库。
- 密钥只能从配置环境变量读取，不能进入日志、错误 message、测试快照或 `.axm` 示例值。

### edge-api

- edge-api 只负责 HTTP 参数绑定、登录态 user id 透传、RPC 调用和错误映射。
- edge-api 不直接调用 OSS SDK，不访问 asset Mongo 集合。
- `/media/*` 路由必须先于 `/:id` 注册。
- REST 响应继续保持 `apiResp{code,message,data}`。

### 依赖管理

- 实现阿里云 OSS 时，优先使用阿里云官方 Go SDK v2。
- 新增依赖后在 `services/asset` 执行 `go mod tidy`。
- 如果 SDK 初始化需要 region/endpoint 差异处理，配置结构要显式表达，不在代码里硬编码某个地域。

## 测试计划

### asset biz 单测

- 创建上传会话成功：生成 session、object key、预签名 PUT 信息。
- 创建上传会话拒绝空 content type。
- 创建上传会话拒绝不在 allowlist 的 content type。
- 创建上传会话拒绝 size <= 0 或超过上限。
- object key 不包含原始 filename 主体。
- finalize 成功：fake storage `HeadObject` 返回匹配元信息后创建 `MediaObject`。
- finalize 幂等：重复 finalize 返回同一个 `MediaObject`。
- finalize 拒绝过期 session，并把状态置为 `EXPIRED`。
- finalize 拒绝 OSS object 不存在或未上传完成。
- finalize 拒绝 size/content type 不匹配。
- 查询媒体对象只能访问当前 workspace。
- 获取访问 URL 对私有对象返回 signed GET。
- `Asset.CoverMediaID` 引用不存在媒体对象时失败。
- `AssetVersion.parts.mediaIds` 引用不存在或跨 workspace 媒体对象时返回 `ErrAssetInvalidPart`。

### asset handler 单测

- `CreateStorageUploadSession` 从 `Base.UserID` 派生 workspace。
- `FinalizeStorageUploadSession` 正确绑定 `sessionID`、`width`、`height`。
- `GetMediaObject`、`ListMediaObjects`、`GetMediaObjectAccessURL` 正确返回 DTO。
- nil request、缺 user id、非法参数返回正确 BaseResp。

### edge-api handler 单测

- `POST /api/v1/assets/media/upload-sessions` 正确绑定 JSON 并透传 user id。
- `POST /api/v1/assets/media/upload-sessions/:session_id/finalize` 正确绑定路径参数和 body。
- `GET /api/v1/assets/media` 正确归一化分页。
- `GET /api/v1/assets/media/:id/access-url` 正确绑定 `expires_in_seconds`。
- 错误映射覆盖 `ErrMediaObjectNotFound`、`ErrAssetUploadSessionNotFound`、`ErrAssetStorageError`。
- 确认 `/media/*` 路由不会被 `/:id` 资产详情路由吞掉。

### 可选真实 OSS smoke test

用户提供 OSS 配置后，可增加默认跳过的 integration test：

```bash
ASSET_OSS_INTEGRATION=1 \
ALIYUN_OSS_REGION=... \
ALIYUN_OSS_ENDPOINT=... \
ALIYUN_OSS_BUCKET=... \
ALIYUN_OSS_ACCESS_KEY_ID=... \
ALIYUN_OSS_ACCESS_KEY_SECRET=... \
cd services/asset && go test ./... -run TestAliyunOSSPresignUploadSmoke -count=1
```

该测试建议验证：

1. 创建 presigned PUT。
2. 用标准 HTTP client PUT 一个小 PNG/JPEG fixture。
3. `HeadObject` 能看到对象。
4. 创建 presigned GET。
5. HTTP GET 能取回对象内容。

测试必须默认 skip，避免 CI 依赖外部云资源和真实密钥。

## 验收标准

### AI 自动验收

AS-04 已完成。实现时必须通过：

```bash
make gen
cd services/asset && go test ./... -count=1
cd services/edge-api && go test ./... -count=1
make build
node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service
```

可判定输出：

- `asset.thrift` 能生成 asset 和 edge-api 所需 Kitex 代码。
- asset 单测覆盖上传会话、finalize、media object、访问 URL、media 引用校验。
- edge-api 单测覆盖媒体 REST 门面、分页、错误映射和路由顺序。
- `make build` 能构建所有服务。
- axm validate 无 error。

如果用户提供真实 OSS 配置，还应通过一次 smoke test：

```bash
ASSET_OSS_INTEGRATION=1 cd services/asset && go test ./... -run TestAliyunOSSPresignUploadSmoke -count=1
```

### 人类验收

- 人类确认能通过 REST 创建上传会话，拿到预签名上传 URL。
- 人类使用 curl 或浏览器把一张小图片 PUT 到 OSS。
- 人类调用 finalize 后能拿到 `mediaID`。
- 人类调用 access-url 后能打开或下载该图片。
- 人类确认 `mediaID` 可作为 `cover_media_id` 或 `AssetVersion.parts.media_ids` 使用。
- 人类确认未提供 OSS 密钥时服务失败方式清晰，不泄露敏感信息。
- 人类确认 AS-04 没有提前实现模型生图入库、multipart、缩略图流水线或 CDN 刷新。

## 后续阶段预留

AS-04 完成后，后续可以单独拆分：

- AS-05：模型生成结果下载/转存/入库，复用 `MediaObject` 与 storage adapter。
- AS-06：缩略图、预览图、图片尺寸服务端解析和 variants 写入。
- AS-07：对象生命周期、孤儿文件清理、删除媒体对象和引用保护。
- AS-08：multipart 分片上传、断点续传和大文件支持。

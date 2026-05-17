<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: service-contract-and-domain-models.md
    title: AS-01 服务契约与领域模型
    when-to-read: 理解已完成 asset IDL、服务骨架、Mongo 模型、仓储、配置和错误码时
  - path: asset-library-crud.md
    title: AS-02 资产库 CRUD
    when-to-read: 理解已完成 asset 资产类型、分类、资产实例 CRUD 和 edge-api REST 门面时
  - path: asset-version-parts-provenance.md
    title: AS-03 资产版本、组成部分与溯源
    when-to-read: 理解已完成 asset 资产版本快照、parts 写入、版本查询、复制旧版本和当前版本切换时
  - path: media-storage-upload-and-ingest.md
    title: AS-04 媒体存储、上传会话与 OSS 接入
    when-to-read: 理解已完成 asset OSS 抽象、前端直传、上传会话、媒体对象、访问 URL 和 media 引用强校验时
-->
# specs/ — Asset Service 阶段 Spec

承载 `asset` 的阶段开发 spec。metadata `entries` 只登记已经写好的 spec 文件；尚未写详细内容的后续阶段仅在正文中预留名称，避免索引断链。

## 已写 spec

| Spec | 文件 | 内容 |
|---|---|---|
| AS-01 | `service-contract-and-domain-models.md` | 服务契约、领域模型、Mongo 集合、仓储、配置、错误码、服务启动骨架 |
| AS-02 | `asset-library-crud.md` | 资产类型、组成部分 schema、分类、资产实例、资产库状态、edge-api REST 适配 |
| AS-03 | `asset-version-parts-provenance.md` | 已完成；资产版本快照、parts 写入、版本查询、复制旧版本、当前版本切换、生产溯源字段 |
| AS-04 | `media-storage-upload-and-ingest.md` | 已完成第一版；OSS 抽象、前端直传上传会话、媒体对象、访问 URL、media 引用强校验 |

## 后续 spec 预留

| Spec | 计划文件名 | 状态 | 内容 |
|---|---|---|---|
| GP-05 关联项 | `../../generation-platform/roadmap.md` | 后续在 `generator` spec 中拆 | 模型生图结果下载/转存/入库，复用 MediaObject 与 storage adapter |

<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: overview.md
    title: 服务拓扑速查
    when-to-read: 理解或修改 services/edge-api、idp、iam、asset、model、billing、credits、notification 时
  - path: auth-sequence.md
    title: 认证与权限交互时序
    when-to-read: 排查或修改注册、登录、OAuth、Token 刷新、鉴权、封禁、RBAC 流程时
-->


# services/ — 服务拓扑索引

`services/` 包含 Hertz 接入层、Hertz model HTTP/SSE 服务和多个 Kitex 业务服务（含 asset 数字资产服务）。先读 `overview.md` 理解边界和调用方向。

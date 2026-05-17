<!-- axm-meta
status: active
last-reviewed: 2026-05-14
owner: castlexu
entries:
  - path: process-lifecycle.md
    title: DEV-01 进程生命周期
    when-to-read: 实施 PID 文件、优雅停、状态查询、就绪等待时
  - path: health-endpoints.md
    title: DEV-02 健康检查接口
    when-to-read: 实施 /healthz /readyz /version、admin 端口、依赖探测时
  - path: log-unification.md
    title: DEV-03 日志统一与查询
    when-to-read: 统一日志格式、接管第三方库日志、新增 logs 查询脚本时
  - path: env-split.md
    title: DEV-04 .env 拆分与校验
    when-to-read: 拆分环境变量分组、新增 check-env 脚本时
-->
# specs/ — 本地开发运维改造 specs

每份 spec 必须包含 AI 自动验收和人类验收。

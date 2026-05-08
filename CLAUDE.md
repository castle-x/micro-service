# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## AI 协作入口

本项目的规范、架构说明与协作指南统一维护在 [`AGENTS.md`](./AGENTS.md)，请直接阅读该文件。

## 常用命令

```bash
make build         # 构建所有服务到 ./bin
make test          # go vet + go test（pkg + 所有服务）
make dev           # 启动本地基础设施（Docker）
make gen           # 从 Thrift IDL 生成 Kitex 代码
make lint          # go vet（+ golangci-lint，若已安装）
make fmt           # gofmt -w
```

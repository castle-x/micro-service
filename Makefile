.PHONY: gen build dev dev-start dev-stop dev-status dev-restart dev-check-env dev-logs logs-query infra-up infra-down infra-ps konga-bootstrap obs-up obs-down obs-ps obs-trace obs-logs obs-metrics obs-errors test test-pkg test-services test-unit test-integration test-contract test-e2e test-all idl-compat openapi-validate lint lint-noprint fmt clean help model-start model-stop model-restart asset-start asset-stop asset-restart

MODULE := github.com/castlexu/micro-service
SERVICES := idp iam billing credits notification asset
ALL_SERVICES := edge-api model $(SERVICES)
BIN_DIR := bin
OBS_COMPOSE_FILE := deployments/docker-compose.observability.yml
OBS_QUERY := node scripts/observability/openobserve-query.mjs
DEV_OTEL_ENV := OTEL_ENABLED=true OTEL_ENDPOINT=localhost:4317 OTEL_PROTOCOL=grpc OTEL_ENVIRONMENT=local OTEL_INSECURE=true OTEL_STRICT=false
DEV_LOG_SERVICES ?= edge-api idp iam asset model
DEV_LOG_FILES := $(addprefix bin/log/,$(addsuffix .log,$(DEV_LOG_SERVICES)))

# 自动检测 docker compose 命令（新版插件 vs 旧版独立命令）
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; fi)

help:
	@echo "Platform Monorepo - Available targets:"
	@echo ""
	@echo "  [DEV] ----------------------------------------------------------------"
	@echo "  make dev-start     [DEV] 一键启动：infra + observability + 编译 + 后端服务"
	@echo "  make dev-restart   [DEV] 重编译并按正确顺序重启所有后端服务（默认启用 OTel）"
	@echo "  make dev-stop      [DEV] 优雅停止后端服务进程（不停 Docker infra/obs）"
	@echo "  make dev-status    [DEV] 输出后端服务 pid / port / ready 状态 JSON"
	@echo "  make dev-check-env [DEV] 校验本地 env 文件缺失、占位符和重复 key"
	@echo "  make dev-logs      [DEV] tail 本地后端服务日志"
	@echo "  make logs-query    [DEV] 查询本地 JSON 日志：ARGS=\"--service=iam --since=15m\""
	@echo "  make model-start   [MODEL] 单独编译并启动 model service :38083"
	@echo "  make model-stop    [MODEL] 停止 model service"
	@echo "  make model-restart [MODEL] 重编译并重启 model service"
	@echo "  make asset-start   [ASSET] 单独编译并启动 asset service :38084"
	@echo "  make asset-stop    [ASSET] 停止 asset service"
	@echo "  make asset-restart [ASSET] 重编译并重启 asset service"
	@echo "  make infra-up      Start local dev dependencies (MongoDB + Redis + etcd + NSQ + Kong) via Docker"
	@echo "  make infra-down    Stop local dev dependencies"
	@echo "  make infra-ps      Show status of local dev containers"
	@echo "  make konga-bootstrap Initialize/activate Konga's local Kong Admin connection"
	@echo "  make obs-up        Start OpenObserve + OpenTelemetry Collector"
	@echo "  make obs-down      Stop local observability containers"
	@echo "  make obs-ps        Show status of local observability containers"
	@echo "  make obs-trace     Query trace summary: TRACE_ID=<id> [SINCE=15m]"
	@echo "  make obs-logs      Query logs by trace id: TRACE_ID=<id> [SINCE=15m]"
	@echo "  make obs-metrics   Query service metrics: SERVICE=edge-api [SINCE=15m]"
	@echo "  make obs-errors    Query recent error spans: [SINCE=15m]"
	@echo "  make dev           Alias for dev-start"
	@echo ""
	@echo "  [BUILD] --------------------------------------------------------------"
	@echo "  make gen           Generate Kitex code for all services from thrift IDL"
	@echo "  make build         Build all services into ./bin"
	@echo "  make fmt           Run gofmt -w on the whole repo"
	@echo "  make clean         Remove build artifacts"
	@echo ""
	@echo "  [TEST] ---------------------------------------------------------------"
	@echo "  make test          Run go vet + go test across pkg and all services"
	@echo "  make test-pkg      Run go test for pkg/ only"
	@echo "  make test-services Run go test for services/* only"
	@echo "  make test-unit         [TEST L1] Unit tests with -race -short (no DB)"
	@echo "  make test-integration  [TEST L2] Integration tests with -tags=integration (testcontainers)"
	@echo "  make test-contract     [TEST L3] IDL compat + OpenAPI validation"
	@echo "  make test-e2e          [TEST L4] End-to-end shell scripts"
	@echo "  make test-all          Run unit + integration + contract + e2e"
	@echo "  make idl-compat        Check Thrift IDL backward compatibility (vs origin/develop)"
	@echo "  make openapi-validate  Validate OpenAPI spec & route coverage"
	@echo "  make lint          Run go vet (+ golangci-lint if installed)"

# Start local dev infra. Services require etcd discovery by default.
infra-up:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then \
		echo "Error: docker or docker-compose not found. Install Docker Desktop first."; \
		echo "  https://www.docker.com/products/docker-desktop/"; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml up -d
	@echo ">>> Waiting for services to be healthy..."
	@sleep 3
	@bash scripts/dev/bootstrap-konga.sh
	@echo ">>> MongoDB : localhost:27017"
	@echo ">>> Redis   : localhost:6379"
	@echo ">>> etcd    : localhost:2379"
	@echo ">>> Run 'make infra-ps' to check status."

# Stop local dev infra
infra-down:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml down
	@echo ">>> Dev infra stopped."

# Show container status
infra-ps:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml ps

konga-bootstrap:
	@bash scripts/dev/bootstrap-konga.sh

obs-up:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then \
		echo "Error: docker or docker-compose not found. Install Docker Desktop first."; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) -f $(OBS_COMPOSE_FILE) up -d
	@echo ">>> OpenObserve UI : http://localhost:5080"
	@echo ">>> OTel Collector : localhost:4317 (grpc), localhost:4318 (http)"
	@echo ">>> dev-start/dev-restart enable OTel by default via localhost:4317"

obs-down:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f $(OBS_COMPOSE_FILE) down
	@echo ">>> Observability stack stopped."

obs-ps:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f $(OBS_COMPOSE_FILE) ps

obs-trace:
	@TRACE_ID="$(TRACE_ID)" SINCE="$(SINCE)" $(OBS_QUERY) trace

obs-logs:
	@TRACE_ID="$(TRACE_ID)" SINCE="$(SINCE)" $(OBS_QUERY) logs

obs-metrics:
	@SERVICE="$(SERVICE)" SINCE="$(SINCE)" $(OBS_QUERY) metrics

obs-errors:
	@SINCE="$(SINCE)" $(OBS_QUERY) errors

# Alias
dev: dev-start

# ----------------------------------------------------------------
# [DEV] 一键启动本地后端服务（infra + observability + 编译 + readyz 等待）
# 前置：复制 deployments/env/*.env.example 为 *.env 并填写真实凭据
# 日志：bin/log/iam.log / bin/log/idp.log / bin/log/asset.log / bin/log/edge-api.log / bin/log/model.log
# ----------------------------------------------------------------
dev-check-env:
	@bash scripts/dev/check-env.sh

dev-start: dev-check-env infra-up obs-up build
	@bash scripts/dev/start.sh
	@echo ""
	@echo "  Services started:"
	@echo "     Backend → http://localhost:38080"
	@echo "     Observe → http://localhost:5080/web/"
	@echo ""
	@echo "  Status: make dev-status"
	@echo "  Logs:   make dev-logs"
	@echo "  Stop:   make dev-stop"

# [DEV] 停止所有服务进程（不停 Docker infra）
dev-stop:
	@bash scripts/dev/stop.sh
	@echo ">>> [DEV] Services stopped. Docker infra still running (make infra-down to stop)."

dev-status:
	@bash scripts/dev/status.sh

# [DEV] 重启所有后端服务（停止 → 重新编译 → 启动），顺序来自 scripts/dev/services.json
dev-restart: dev-check-env infra-up obs-up build
	@bash scripts/dev/restart.sh
	@echo ">>> [DEV] Services restarted. Status: make dev-status"

dev-logs:
	@mkdir -p bin/log
	@touch $(DEV_LOG_FILES)
	@tail -F $(DEV_LOG_FILES)

logs-query:
	@bash scripts/dev/logs-query.sh $(ARGS)

# ----------------------------------------------------------------
# [MODEL] 单独启动 / 停止 / 重启 model service（:38083）
# 前置：.env 存在（包含 MODEL_ENCRYPT_KEY 等），infra 已启动
# 日志：bin/log/model.log
# ----------------------------------------------------------------
model-start: dev-check-env infra-up obs-up build
	@bash scripts/dev/start.sh model
	@echo ">>> [MODEL] model service started. Log: bin/log/model.log"

model-stop:
	@bash scripts/dev/stop.sh model
	@echo ">>> [MODEL] Stopped."

model-restart: dev-check-env infra-up obs-up build
	@bash scripts/dev/restart.sh model
	@echo ">>> [MODEL] model service restarted. Log: bin/log/model.log"

# ----------------------------------------------------------------
# [ASSET] 单独启动 / 停止 / 重启 asset service（:38084）
# 前置：.env 存在（包含 ASSET/OSS 配置），infra 已启动
# 日志：bin/log/asset.log
# ----------------------------------------------------------------
asset-start: dev-check-env infra-up obs-up build
	@bash scripts/dev/start.sh asset
	@echo ">>> [ASSET] asset service started. Log: bin/log/asset.log"

asset-stop:
	@bash scripts/dev/stop.sh asset
	@echo ">>> [ASSET] Stopped."

asset-restart: dev-check-env infra-up obs-up build
	@bash scripts/dev/restart.sh asset
	@echo ">>> [ASSET] asset service restarted. Log: bin/log/asset.log"


# Generate Kitex code for all Kitex-based services (待 IDL 补齐后可用)
gen:
	@for svc in $(SERVICES); do \
		echo ">>> Generating $$svc..."; \
		idl_svc=$$svc; \
		(cd services/$$svc && \
			kitex -module $(MODULE)/services/$$svc ../../idl/$$idl_svc/$$idl_svc.thrift); \
	done
	@echo ">>> Generating edge-api asset client..."
	@(cd services/edge-api && kitex -module $(MODULE)/services/edge-api ../../idl/asset/asset.thrift)

# Build all services
build:
	@mkdir -p $(BIN_DIR)
	@for svc in $(ALL_SERVICES); do \
		echo ">>> Building $$svc..."; \
		(cd services/$$svc && go build -o ../../$(BIN_DIR)/$$svc .); \
	done
	@echo ">>> Building iam-bootstrap..."
	@(cd services/iam && go build -o ../../$(BIN_DIR)/iam-bootstrap ./cmd/bootstrap/)

# ---- lint / test 真实命令（Phase 02 补齐）----

# lint: 跑 go vet，检测到 golangci-lint 时追加运行
lint: lint-noprint
	@echo ">>> go vet ./pkg/..."
	@(cd pkg && go vet ./...)
	@for svc in $(ALL_SERVICES); do \
		echo ">>> go vet ./services/$$svc/..."; \
		(cd services/$$svc && go vet ./...); \
	done
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo ">>> golangci-lint run (workspace)"; \
		golangci-lint run ./... || exit 1; \
	else \
		echo ">>> golangci-lint not installed, skipped (install: https://golangci-lint.run)"; \
	fi

lint-noprint:
	@bad="$$(grep -RIn --include='*.go' --exclude='*_test.go' -E '(^|[^a-zA-Z])(fmt\.Print|fmt\.Println|log\.Print)' services pkg | while IFS= read -r line; do file="$${line%%:*}"; if grep -q 'nolint:noprint' "$$file"; then continue; fi; echo "$$line"; done || true)"; \
	if [ -n "$$bad" ]; then \
		echo "$$bad"; \
		echo "direct fmt/log prints are blocked; use pkg/logger or add // nolint:noprint for intentional stdout"; \
		exit 1; \
	fi

test: test-pkg test-services

test-pkg:
	@echo ">>> go test ./pkg/..."
	@(cd pkg && go test ./... -count=1)

test-services:
	@for svc in $(ALL_SERVICES); do \
		echo ">>> go test ./services/$$svc/..."; \
		(cd services/$$svc && go test ./... -count=1); \
	done

# ---- API 测试体系（详见 .axm/project/api-testing.md） ----

# L1: 单元测试 —— 无外部依赖，开 -race，标记 -short 跳过集成
test-unit:
	@echo ">>> [L1 unit] go test ./pkg/..."
	@(cd pkg && go test ./... -count=1 -race -short)
	@for svc in $(ALL_SERVICES); do \
		echo ">>> [L1 unit] go test ./services/$$svc/..."; \
		(cd services/$$svc && go test ./... -count=1 -race -short); \
	done

# L2: 集成测试 —— 用 testcontainers 起真实 Mongo/Redis/NSQ，build tag=integration
test-integration:
	@echo ">>> [L2 integration] go test -tags=integration ./..."
	@for svc in $(ALL_SERVICES); do \
		if [ -d services/$$svc/test/integration ]; then \
			echo ">>> [L2] services/$$svc"; \
			(cd services/$$svc && go test ./test/integration/... -count=1 -race -tags=integration -timeout=10m); \
		fi; \
	done

# L3: 契约测试 —— IDL 兼容 + OpenAPI schema 校验
test-contract: idl-compat openapi-validate

idl-compat:
	@bash scripts/idl-compat.sh

openapi-validate:
	@bash scripts/openapi-validate.sh

# L4: E2E 端到端 —— shell + curl，跑全栈关键链路
test-e2e:
	@bash scripts/e2e-all.sh

# 全量测试 —— 本地阶段交付前 / merge to develop 触发
test-all: test-unit test-integration test-contract test-e2e

fmt:
	gofmt -w .

clean:
	rm -rf $(BIN_DIR)

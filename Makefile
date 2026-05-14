.PHONY: gen build dev dev-start dev-stop dev-restart infra-up infra-down infra-ps obs-up obs-down obs-ps obs-trace obs-logs obs-metrics obs-errors test test-pkg test-services lint fmt clean help model-start model-stop model-restart asset-start asset-stop asset-restart

MODULE := github.com/castlexu/micro-service
SERVICES := idp iam billing credits notification asset
ALL_SERVICES := edge-api model $(SERVICES)
BIN_DIR := bin
OBS_COMPOSE_FILE := deployments/docker-compose.observability.yml
OBS_QUERY := node scripts/observability/openobserve-query.mjs
ENV_FILE_VARS = $$(grep -v '^\#' .env | grep -v '^$$$$' | xargs)
DEV_OTEL_ENV := OTEL_ENABLED=true OTEL_ENDPOINT=localhost:4317 OTEL_PROTOCOL=grpc OTEL_ENVIRONMENT=local OTEL_INSECURE=true OTEL_STRICT=false

# 自动检测 docker compose 命令（新版插件 vs 旧版独立命令）
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; fi)

help:
	@echo "Platform Monorepo - Available targets:"
	@echo ""
	@echo "  [DEV] ----------------------------------------------------------------"
	@echo "  make dev-start     [DEV] 一键启动：infra + observability + 编译 + 后端 + 前端"
	@echo "  make dev-restart   [DEV] 重编译并按正确顺序重启所有后端服务（默认启用 OTel）"
	@echo "  make dev-stop      [DEV] 一键停止后端和前端进程（不停 Docker infra/obs）"
	@echo "  make model-start   [MODEL] 单独编译并启动 model service :38083"
	@echo "  make model-stop    [MODEL] 停止 model service"
	@echo "  make model-restart [MODEL] 重编译并重启 model service"
	@echo "  make asset-start   [ASSET] 单独编译并启动 asset service :38084"
	@echo "  make asset-stop    [ASSET] 停止 asset service"
	@echo "  make asset-restart [ASSET] 重编译并重启 asset service"
	@echo "  make infra-up      Start local dev dependencies (MongoDB + Redis + etcd + NSQ + Kong) via Docker"
	@echo "  make infra-down    Stop local dev dependencies"
	@echo "  make infra-ps      Show status of local dev containers"
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
# [DEV] 一键启动所有服务（infra + observability + 后端 + 前端）
# 前置：复制 .env.example 为 .env 并填写真实凭据
# 日志：bin/log/iam.log / bin/log/idp.log / bin/log/asset.log / bin/log/edge-api.log / bin/log/web.log
# ----------------------------------------------------------------
dev-start: infra-up obs-up build
	@mkdir -p bin/log
	@echo ">>> [DEV] Loading .env..."
	@if [ ! -f .env ]; then \
		echo "Error: .env not found. Copy .env.example and fill in credentials:"; \
		echo "  cp .env.example .env"; \
		exit 1; \
	fi
	@echo ">>> [DEV] OpenTelemetry enabled by default: localhost:4317"
	@echo ">>> [DEV] Starting iam :38082)..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/iam > bin/log/iam.log 2>&1 &
	@echo ">>> [DEV] Starting idp :38081)..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/idp > bin/log/idp.log 2>&1 &
	@echo ">>> [DEV] Starting asset :38084)..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/asset > bin/log/asset.log 2>&1 &
	@echo ">>> [DEV] Starting edge-api :38080)..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/edge-api > bin/log/edge-api.log 2>&1 &
	@echo ">>> [DEV] Starting model :38083)..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/model > bin/log/model.log 2>&1 &
	@sleep 2
	@echo ">>> [DEV] Starting web dev server :35173)..."
	@cd web && npm run dev > ../bin/log/web.log 2>&1 &
	@sleep 2
	@echo ""
	@echo "  ✅  All services started:"
	@echo "     Backend  → http://localhost:38080"
	@echo "     Frontend → http://localhost:35173"
	@echo "     Observe  → http://localhost:5080/web/"
	@echo ""
	@echo "  Logs: bin/log/{iam,idp,asset,edge-api,web}.log"
	@echo "  Stop: make dev-stop"

# [DEV] 停止所有服务进程（不停 Docker infra）
dev-stop:
	@echo ">>> [DEV] Stopping services..."
	@lsof -ti tcp:38080 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:38081 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:38082 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:38083 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:38084 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:35173 | xargs kill -9 2>/dev/null || true
	@echo ">>> [DEV] Services stopped. Docker infra still running (make infra-down to stop)."

# [DEV] 重启所有服务（停止 → 重新编译 → 启动），正确顺序：iam → idp → edge-api → model → web
dev-restart: infra-up obs-up dev-stop build
	@mkdir -p bin/log
	@echo ">>> [DEV] Loading .env..."
	@if [ ! -f .env ]; then echo "Error: .env not found"; exit 1; fi
	@echo ">>> [DEV] OpenTelemetry enabled by default: localhost:4317"
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/iam > bin/log/iam.log 2>&1 &
	@sleep 1
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/idp > bin/log/idp.log 2>&1 &
	@sleep 1
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/asset > bin/log/asset.log 2>&1 &
	@sleep 1
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/edge-api > bin/log/edge-api.log 2>&1 &
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/model > bin/log/model.log 2>&1 &
	@sleep 2
	@lsof -ti tcp:35173 | xargs kill -9 2>/dev/null; true
	@cd web && npm run dev > ../bin/log/web.log 2>&1 &
	@sleep 2
	@echo ""
	@echo "  ✅  Services restarted:"
	@echo "     Backend  → http://localhost:38080"
	@echo "     Frontend → http://localhost:35173"
	@echo "     Observe  → http://localhost:5080/web/"
	@echo ""
	@echo "  Logs: bin/log/{iam,idp,asset,edge-api,web}.log"

# ----------------------------------------------------------------
# [MODEL] 单独启动 / 停止 / 重启 model service（:38083）
# 前置：.env 存在（包含 MODEL_ENCRYPT_KEY 等），infra 已启动
# 日志：bin/log/model.log
# ----------------------------------------------------------------
model-start: infra-up obs-up build
	@mkdir -p bin/log
	@if [ ! -f .env ]; then echo "Error: .env not found"; exit 1; fi
	@lsof -ti tcp:38083 | xargs kill -9 2>/dev/null || true
	@echo ">>> [MODEL] Starting model service :38083..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/model > bin/log/model.log 2>&1 &
	@sleep 1
	@echo ">>> [MODEL] model service started. Log: bin/log/model.log"

model-stop:
	@echo ">>> [MODEL] Stopping model service..."
	@lsof -ti tcp:38083 | xargs kill -9 2>/dev/null || true
	@echo ">>> [MODEL] Stopped."

model-restart: infra-up obs-up model-stop build
	@mkdir -p bin/log
	@if [ ! -f .env ]; then echo "Error: .env not found"; exit 1; fi
	@echo ">>> [MODEL] Starting model service :38083..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/model > bin/log/model.log 2>&1 &
	@sleep 1
	@echo ">>> [MODEL] model service restarted. Log: bin/log/model.log"

# ----------------------------------------------------------------
# [ASSET] 单独启动 / 停止 / 重启 asset service（:38084）
# 前置：.env 存在（包含 ASSET/OSS 配置），infra 已启动
# 日志：bin/log/asset.log
# ----------------------------------------------------------------
asset-start: infra-up obs-up build
	@mkdir -p bin/log
	@if [ ! -f .env ]; then echo "Error: .env not found"; exit 1; fi
	@lsof -ti tcp:38084 | xargs kill -9 2>/dev/null || true
	@echo ">>> [ASSET] Starting asset service :38084..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/asset > bin/log/asset.log 2>&1 &
	@sleep 1
	@echo ">>> [ASSET] asset service started. Log: bin/log/asset.log"

asset-stop:
	@echo ">>> [ASSET] Stopping asset service..."
	@lsof -ti tcp:38084 | xargs kill -9 2>/dev/null || true
	@echo ">>> [ASSET] Stopped."

asset-restart: infra-up obs-up asset-stop build
	@mkdir -p bin/log
	@if [ ! -f .env ]; then echo "Error: .env not found"; exit 1; fi
	@echo ">>> [ASSET] Starting asset service :38084..."
	@nohup env $(ENV_FILE_VARS) $(DEV_OTEL_ENV) ./bin/asset > bin/log/asset.log 2>&1 &
	@sleep 1
	@echo ">>> [ASSET] asset service restarted. Log: bin/log/asset.log"


# Generate Kitex code for all Kitex-based services (待 IDL 补齐后可用)
gen:
	@for svc in $(SERVICES); do \
		echo ">>> Generating $$svc..."; \
		idl_svc=$$svc; \
		cd services/$$svc && \
			kitex -module $(MODULE)/services/$$svc ../../idl/$$idl_svc/$$idl_svc.thrift && \
			cd ../..; \
	done
	@echo ">>> Generating edge-api asset client..."
	@cd services/edge-api && kitex -module $(MODULE)/services/edge-api ../../idl/asset/asset.thrift && cd ../..

# Build all services
build:
	@mkdir -p $(BIN_DIR)
	@for svc in $(ALL_SERVICES); do \
		echo ">>> Building $$svc..."; \
		cd services/$$svc && go build -o ../../$(BIN_DIR)/$$svc . && cd ../..; \
	done
	@echo ">>> Building iam-bootstrap..."
	@cd services/iam && go build -o ../../$(BIN_DIR)/iam-bootstrap ./cmd/bootstrap/ && cd ../..

# ---- lint / test 真实命令（Phase 02 补齐）----

# lint: 跑 go vet，检测到 golangci-lint 时追加运行
lint:
	@echo ">>> go vet ./pkg/..."
	@cd pkg && go vet ./...
	@for svc in $(ALL_SERVICES); do \
		echo ">>> go vet ./services/$$svc/..."; \
		cd services/$$svc && go vet ./... && cd ../..; \
	done
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo ">>> golangci-lint run (workspace)"; \
		golangci-lint run ./... || exit 1; \
	else \
		echo ">>> golangci-lint not installed, skipped (install: https://golangci-lint.run)"; \
	fi

test: test-pkg test-services

test-pkg:
	@echo ">>> go test ./pkg/..."
	@cd pkg && go test ./... -count=1

test-services:
	@for svc in $(ALL_SERVICES); do \
		echo ">>> go test ./services/$$svc/..."; \
		cd services/$$svc && go test ./... -count=1 && cd ../..; \
	done

fmt:
	gofmt -w .

clean:
	rm -rf $(BIN_DIR)

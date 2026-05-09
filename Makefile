.PHONY: gen build dev dev-start dev-stop dev-restart infra-up infra-down infra-ps test test-pkg test-services lint fmt clean help

MODULE := github.com/castlexu/micro-service
SERVICES := idp iam billing credits notification
ALL_SERVICES := edge-api $(SERVICES)
BIN_DIR := bin

# 自动检测 docker compose 命令（新版插件 vs 旧版独立命令）
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; fi)

help:
	@echo "Platform Monorepo - Available targets:"
	@echo ""
	@echo "  [DEV] ----------------------------------------------------------------"
	@echo "  make dev-start     [DEV] 一键启动：infra + 编译 + 后端三服务 + 前端"
	@echo "  make dev-restart   [DEV] 重编译并按正确顺序重启所有后端服务"
	@echo "  make dev-stop      [DEV] 一键停止所有服务进程（不停 Docker）"
	@echo "  make infra-up      Start local dev dependencies (MongoDB + Redis) via Docker"
	@echo "  make infra-down    Stop local dev dependencies"
	@echo "  make infra-ps      Show status of local dev containers"
	@echo "  make dev           Alias for infra-up"
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

# Start minimal local dev infra (MongoDB + Redis)
infra-up:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then \
		echo "Error: docker or docker-compose not found. Install Docker Desktop first."; \
		echo "  https://www.docker.com/products/docker-desktop/"; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.dev.yml up -d
	@echo ">>> Waiting for services to be healthy..."
	@sleep 3
	@echo ">>> MongoDB : localhost:27017"
	@echo ">>> Redis   : localhost:6379"
	@echo ">>> Run 'make infra-ps' to check status."

# Stop local dev infra
infra-down:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.dev.yml down
	@echo ">>> Dev infra stopped."

# Show container status
infra-ps:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.dev.yml ps

# Alias
dev: infra-up

# ----------------------------------------------------------------
# [DEV] 一键启动所有服务（infra + 后端 + 前端）
# 前置：复制 .env.example 为 .env 并填写真实凭据
# 日志：/tmp/iam.log / /tmp/idp.log / /tmp/edge-api.log / /tmp/web.log
# ----------------------------------------------------------------
dev-start: infra-up build
	@echo ">>> [DEV] Loading .env..."
	@if [ ! -f .env ]; then \
		echo "Error: .env not found. Copy .env.example and fill in credentials:"; \
		echo "  cp .env.example .env"; \
		exit 1; \
	fi
	@echo ">>> [DEV] Starting iam :38082)..."
	@env $$(cat .env | grep -v '^#' | xargs) ./bin/iam > /tmp/iam.log 2>&1 &
	@echo ">>> [DEV] Starting idp :38081)..."
	@env $$(cat .env | grep -v '^#' | xargs) ./bin/idp > /tmp/idp.log 2>&1 &
	@echo ">>> [DEV] Starting edge-api :38080)..."
	@env $$(cat .env | grep -v '^#' | xargs) ./bin/edge-api > /tmp/edge-api.log 2>&1 &
	@sleep 2
	@echo ">>> [DEV] Starting web dev server :35173)..."
	@cd web && npm run dev > /tmp/web.log 2>&1 &
	@sleep 2
	@echo ""
	@echo "  ✅  All services started:"
	@echo "     Backend  → http://localhost:38080"
	@echo "     Frontend → http://localhost:35173"
	@echo ""
	@echo "  Logs: /tmp/{iam,idp,edge-api,web}.log"
	@echo "  Stop: make dev-stop"

# [DEV] 停止所有服务进程（不停 Docker infra）
dev-stop:
	@echo ">>> [DEV] Stopping services..."
	@lsof -ti tcp:38080 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:38081 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:38082 | xargs kill -9 2>/dev/null || true
	@lsof -ti tcp:35173 | xargs kill -9 2>/dev/null || true
	@echo ">>> [DEV] Services stopped. Docker infra still running (make infra-down to stop)."

# [DEV] 重启所有服务（停止 → 重新编译 → 启动），正确顺序：iam → idp → edge-api → web
dev-restart: dev-stop build
	@echo ">>> [DEV] Loading .env..."
	@if [ ! -f .env ]; then echo "Error: .env not found"; exit 1; fi
	@env $$(cat .env | grep -v '^#' | grep -v '^$$' | xargs) ./bin/iam > /tmp/iam.log 2>&1 &
	@sleep 1
	@env $$(cat .env | grep -v '^#' | grep -v '^$$' | xargs) ./bin/idp > /tmp/idp.log 2>&1 &
	@sleep 1
	@env $$(cat .env | grep -v '^#' | grep -v '^$$' | xargs) ./bin/edge-api > /tmp/edge-api.log 2>&1 &
	@sleep 2
	@lsof -ti tcp:35173 | xargs kill -9 2>/dev/null; true
	@cd web && npm run dev > /tmp/web.log 2>&1 &
	@sleep 2
	@echo ""
	@echo "  ✅  Services restarted:"
	@echo "     Backend  → http://localhost:38080"
	@echo "     Frontend → http://localhost:35173"
	@echo ""
	@echo "  Logs: /tmp/{iam,idp,edge-api,web}.log"


# Generate Kitex code for all Kitex-based services (待 IDL 补齐后可用)
gen:
	@for svc in $(SERVICES); do \
		echo ">>> Generating $$svc..."; \
		cd services/$$svc && \
			kitex -module $(MODULE)/services/$$svc ../../idl/$$svc/$$svc.thrift && \
			cd ../..; \
	done

# Build all services
build:
	@mkdir -p $(BIN_DIR)
	@for svc in $(ALL_SERVICES); do \
		echo ">>> Building $$svc..."; \
		cd services/$$svc && go build -o ../../$(BIN_DIR)/$$svc . && cd ../..; \
	done

# Start full infrastructure (all services including etcd/NSQ/Kong)
dev-full:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then echo "docker compose not found"; exit 1; fi
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml up -d
	@echo ">>> Full infrastructure started."

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

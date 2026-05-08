.PHONY: gen build dev infra-up infra-down infra-ps test test-pkg test-services lint fmt clean help

MODULE := github.com/castlexu/micro-service
SERVICES := idp iam billing credits notification
ALL_SERVICES := edge-api $(SERVICES)
BIN_DIR := bin

# 自动检测 docker compose 命令（新版插件 vs 旧版独立命令）
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; fi)

help:
	@echo "Platform Monorepo - Available targets:"
	@echo "  make infra-up      Start local dev dependencies (MongoDB + Redis) via Docker"
	@echo "  make infra-down    Stop local dev dependencies"
	@echo "  make infra-ps      Show status of local dev containers"
	@echo "  make dev           Alias for infra-up"
	@echo "  make gen           Generate Kitex code for all services from thrift IDL"
	@echo "  make build         Build all services into ./bin"
	@echo "  make test          Run go vet + go test across pkg and all services"
	@echo "  make test-pkg      Run go test for pkg/ only"
	@echo "  make test-services Run go test for services/* only"
	@echo "  make lint          Run go vet (+ golangci-lint if installed)"
	@echo "  make fmt           Run gofmt -w on the whole repo"
	@echo "  make clean         Remove build artifacts"

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

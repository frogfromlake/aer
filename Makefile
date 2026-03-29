.PHONY: up down stop restart
.PHONY: infra-clean infra-clean-postgres infra-clean-minio infra-clean-clickhouse
.PHONY: services-up services-down services-restart services-clean
.PHONY: ingestion-up ingestion-down ingestion-restart
.PHONY: worker-up worker-down worker-restart
.PHONY: bff-up bff-down bff-restart
.PHONY: logs tidy codegen test test-go test-python lint build-services

SHELL := /bin/bash

# ==========================================
# GLOBLAL COLORS & SYMBOLS (For Makefile Echoes)
# ==========================================
BOLD          := \033[1m
RESET         := \033[0m
GREEN         := \033[38;5;76m
CYAN          := \033[38;5;39m
GOLD          := \033[38;5;214m
GRAY          := \033[38;5;245m

SYMBOL_SUCCESS := $(GREEN)✔$(RESET)
SYMBOL_STOP    := $(GOLD)■$(RESET)
SYMBOL_INFO    := $(CYAN)ℹ$(RESET)

# ==========================================
# 0. GLOBAL STACK COMMANDS
# ==========================================

up: infra-up
	@echo -e "$(BOLD)$(CYAN)Waiting a moment for infrastructure to settle...$(RESET)"
	@sleep 3
	@$(MAKE) services-up
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) The entire AĒR stack is up and running!$(RESET)"

down: services-down infra-down
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) The entire AĒR stack has been shut down.$(RESET)"

stop: down

restart: down up

# ==========================================
# 1. INFRASTRUCTURE & OBSERVABILITY
# ==========================================

infra-up:
	@echo -e "$(BOLD)$(GRAY)--- STARTING INFRASTRUCTURE ---$(RESET)"
	@docker compose up -d nats minio postgres clickhouse minio-init otel-collector tempo prometheus grafana docs
	@echo -e "$(SYMBOL_SUCCESS) MinIO:      $(CYAN)http://localhost:9001$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) Postgres:   $(CYAN)localhost:5432$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) ClickHouse: $(CYAN)http://localhost:8123/play$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) NATS:       $(CYAN)http://localhost:8222$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) Grafana:    $(CYAN)http://localhost:3000$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) Docs:       $(CYAN)http://localhost:8000$(RESET)"

infra-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING INFRASTRUCTURE ---$(RESET)"
	@docker compose stop nats minio postgres clickhouse minio-init otel-collector tempo prometheus grafana docs
	@echo -e "$(SYMBOL_STOP) $(GRAY)Infrastructure stopped.$(RESET)"

infra-restart: infra-down infra-up

infra-clean:
	@./scripts/clean_infra.sh all

infra-clean-postgres:
	@./scripts/clean_infra.sh postgres

infra-clean-minio:
	@./scripts/clean_infra.sh minio

infra-clean-clickhouse:
	@./scripts/clean_infra.sh clickhouse

# ==========================================
# 2. APPLICATION SERVICES (INDIVIDUAL)
# ==========================================

ingestion-up:
	@./scripts/start.sh ingestion

ingestion-down:
	@./scripts/stop.sh ingestion

ingestion-restart: ingestion-down ingestion-up

worker-up:
	@./scripts/start.sh worker

worker-down:
	@./scripts/stop.sh worker

worker-restart: worker-down worker-up

bff-up:
	@./scripts/start.sh bff

bff-down:
	@./scripts/stop.sh bff

bff-restart: bff-down bff-up

# ==========================================
# 3. APPLICATION SERVICES (ALL TOGETHER)
# ==========================================

services-up: ingestion-up worker-up bff-up
	@echo ""
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) All AĒR services are running in the background!$(RESET)"
	@echo -e "$(GRAY)Use 'make logs' to view the live output.$(RESET)"

services-down: ingestion-down worker-down bff-down
	@echo -e "$(BOLD)$(GOLD)$(SYMBOL_STOP) All AĒR services stopped.$(RESET)"

services-restart: services-down services-up

services-clean: services-down
	@./scripts/clean.sh

# ==========================================
# 4. UTILITIES
# ==========================================

logs:
	@echo -e "$(BOLD)$(CYAN)Showing live logs for all services (Ctrl+C to exit)...$(RESET)"
	@mkdir -p .pids
	@touch .pids/ingestion.log .pids/worker.log .pids/bff.log
	@tail -f .pids/*.log

tidy:
	@cd services/ingestion-api && go mod tidy
	@cd services/bff-api && go mod tidy
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)Go modules tidied up.$(RESET)"

codegen:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running oapi-codegen for BFF API...$(RESET)"
	@cd services/bff-api && oapi-codegen -config api/codegen.yaml api/openapi.yaml
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)API contracts generated successfully.$(RESET)"

build-services:
	@echo -e "$(BOLD)$(CYAN)Compiling AĒR binaries...$(RESET)"
	@mkdir -p bin
	@go build -o bin/ingestion-api ./services/ingestion-api/cmd/api
	@go build -o bin/bff-api ./services/bff-api/cmd/server
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Build complete. Binaries in ./bin/$(RESET)"

# ==========================================
# 5. TESTING & LINTING
# ==========================================

test: test-go test-python
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)All test suites passed successfully!$(RESET)"

test-go:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Go Integration Tests (Testcontainers)...$(RESET)"
	@cd services/ingestion-api && go test -v ./...
	@cd services/bff-api && go test -v ./...

test-python:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Python Unit Tests...$(RESET)"
	@cd services/analysis-worker && ./venv/bin/python -m pytest tests/ -v

lint:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Linters...$(RESET)"
	@cd services/analysis-worker && ./venv/bin/python -m ruff check . && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Python lint passed!$(RESET)"
	@cd services/ingestion-api && golangci-lint run && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (Ingestion API) lint passed!$(RESET)"
	@cd services/bff-api && golangci-lint run && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (BFF API) lint passed!$(RESET)"
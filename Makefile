.PHONY: help up down stop restart
.PHONY: infra-clean infra-clean-postgres infra-clean-minio infra-clean-clickhouse
.PHONY: services-up services-down services-restart services-clean
.PHONY: ingestion-up ingestion-down ingestion-restart
.PHONY: worker-up worker-down worker-restart
.PHONY: bff-up bff-down bff-restart
.PHONY: debug-up debug-down
.PHONY: logs tidy codegen test test-go test-go-pkg test-go-crawlers test-python test-e2e lint lint-go-pkg audit audit-go audit-python build-services crawl setup

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

up: infra-up debug-up
	@echo -e "$(BOLD)$(CYAN)Waiting a moment for infrastructure to settle...$(RESET)"
	@sleep 3
	@$(MAKE) services-up
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) The entire AĒR stack is up and running!$(RESET)"

down: services-down debug-down infra-down
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) The entire AĒR stack has been shut down.$(RESET)"

stop: down

restart: down up

# ==========================================
# 1. INFRASTRUCTURE & OBSERVABILITY
# ==========================================

infra-up:
	@echo -e "$(BOLD)$(GRAY)--- STARTING INFRASTRUCTURE ---$(RESET)"
	@docker compose up -d traefik nats nats-init minio postgres clickhouse minio-init clickhouse-init otel-collector tempo prometheus grafana docs
	@echo -e "$(SYMBOL_SUCCESS) Docs:       $(CYAN)http://localhost:8000$(RESET)"
	@echo -e "$(GRAY)  Backend services (PostgreSQL, ClickHouse, NATS, MinIO, OTel, Grafana) are internal only.$(RESET)"
	@echo -e "$(GRAY)  Grafana and MinIO Console are routed through Traefik (HTTPS).$(RESET)"
	@echo -e "$(GRAY)  Run '$(BOLD)make debug-up$(RESET)$(GRAY)' to expose all ports to the host for debugging.$(RESET)"

infra-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING INFRASTRUCTURE ---$(RESET)"
	@docker compose stop traefik nats minio postgres clickhouse minio-init clickhouse-init otel-collector tempo prometheus grafana docs
	@echo -e "$(SYMBOL_STOP) $(GRAY)Infrastructure stopped.$(RESET)"

infra-restart: infra-down infra-up

debug-up:
	@if [ -f .env ]; then \
		APP_ENV=$$(grep -E '^APP_ENV=' .env | cut -d'=' -f2); \
		if [ "$$APP_ENV" = "production" ]; then \
			echo -e "\033[1m\033[38;5;196mERROR:\033[0m debug-up is forbidden when APP_ENV=production. Exposing internal ports in production is a security risk."; \
			exit 1; \
		fi; \
	fi
	@echo -e "$(BOLD)$(GRAY)--- STARTING DEBUG PORT FORWARDER ---$(RESET)"
	@docker compose --profile debug up -d debug-ports
	@echo -e "$(SYMBOL_SUCCESS) PostgreSQL: $(CYAN)localhost:5432$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) ClickHouse: $(CYAN)http://localhost:8123/play$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) NATS:       $(CYAN)localhost:4222$(RESET)  Monitor: $(CYAN)http://localhost:8222$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) MinIO API:  $(CYAN)http://localhost:9000$(RESET)  Console: $(CYAN)http://localhost:9001$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) OTel:       $(CYAN)localhost:4317$(RESET) (gRPC)  $(CYAN)localhost:4318$(RESET) (HTTP)"
	@echo -e "$(SYMBOL_SUCCESS) Ingestion:  $(CYAN)http://localhost:8081$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) Grafana:    $(CYAN)http://localhost:3000$(RESET)"

debug-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING DEBUG PORT FORWARDER ---$(RESET)"
	@docker compose --profile debug stop debug-ports
	@echo -e "$(SYMBOL_STOP) $(GRAY)Debug ports closed. Backend services still running internally.$(RESET)"

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
	@cd crawlers/rss-crawler && go mod tidy
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
	@go build -o bin/rss-crawler ./crawlers/rss-crawler
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Build complete. Binaries in ./bin/$(RESET)"

crawl:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Building RSS crawler...$(RESET)"
	@go build -o bin/rss-crawler ./crawlers/rss-crawler
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running RSS crawler (feeds: crawlers/rss-crawler/feeds.yaml)...$(RESET)"
	@if [ ! -f .env ]; then \
		echo -e "\033[1m\033[38;5;196mERROR:\033[0m .env file not found. Copy .env.example to .env and set INGESTION_API_KEY before running make crawl."; \
		exit 1; \
	fi
	@set -a; source .env; set +a; ./bin/rss-crawler -config crawlers/rss-crawler/feeds.yaml -state crawlers/rss-crawler/.rss-crawler-state.json
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Crawl complete.$(RESET)"

# ==========================================
# 5. TESTING & LINTING
# ==========================================

test: test-go test-go-pkg test-go-crawlers test-python
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)All test suites passed successfully!$(RESET)"

test-e2e:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running End-to-End Smoke Test (full Docker Compose stack)...$(RESET)"
	@./scripts/e2e_smoke_test.sh

test-go:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Go Integration Tests (Testcontainers)...$(RESET)"
	@cd services/ingestion-api && go test -v ./...
	@cd services/bff-api && go test -v ./...

test-go-pkg:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Go Tests (pkg/)...$(RESET)"
	@cd pkg && go test -v ./...
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (pkg/) tests passed!$(RESET)"

test-go-crawlers:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Go Crawler Tests...$(RESET)"
	@cd crawlers/rss-crawler && go test -v ./...
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (crawlers) tests passed!$(RESET)"

test-python:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Python Unit Tests...$(RESET)"
	@cd services/analysis-worker && \
		if [ -f ./.venv/bin/python ]; then \
			./.venv/bin/python -m pytest tests/ -v; \
		else \
			python -m pytest tests/ -v; \
		fi

lint:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running Linters...$(RESET)"
	@cd services/analysis-worker && \
		if [ -f ./.venv/bin/python ]; then \
			./.venv/bin/python -m ruff check . && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Python lint passed!$(RESET)"; \
		else \
			python -m ruff check . && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Python lint passed!$(RESET)"; \
		fi
	@cd services/ingestion-api && golangci-lint run && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (Ingestion API) lint passed!$(RESET)"
	@cd services/bff-api && golangci-lint run && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (BFF API) lint passed!$(RESET)"
	@cd pkg && golangci-lint run && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (pkg/) lint passed!$(RESET)"
	@cd crawlers/rss-crawler && golangci-lint run && echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (RSS Crawler) lint passed!$(RESET)"

lint-go-pkg:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running golangci-lint for pkg/...$(RESET)"
	@cd pkg && golangci-lint run
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Go (pkg/) lint passed!$(RESET)"

# ==========================================
# 6. DEPENDENCY AUDITING
# ==========================================

audit: audit-go audit-python
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)All dependency audits passed!$(RESET)"

audit-go:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running govulncheck (Go vulnerability scanner)...$(RESET)"
	@cd services/ingestion-api && govulncheck ./... && echo -e "$(SYMBOL_SUCCESS) $(GREEN)govulncheck (Ingestion API) passed!$(RESET)"
	@cd services/bff-api && govulncheck ./... && echo -e "$(SYMBOL_SUCCESS) $(GREEN)govulncheck (BFF API) passed!$(RESET)"
	@cd crawlers/rss-crawler && govulncheck ./... && echo -e "$(SYMBOL_SUCCESS) $(GREEN)govulncheck (RSS Crawler) passed!$(RESET)"
	@cd pkg && govulncheck ./... && echo -e "$(SYMBOL_SUCCESS) $(GREEN)govulncheck (pkg/) passed!$(RESET)"

audit-python:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running pip-audit (Python vulnerability scanner)...$(RESET)"
	@cd services/analysis-worker && \
		if [ -f ./.venv/bin/pip-audit ]; then \
			./.venv/bin/pip-audit -r requirements.txt && echo -e "$(SYMBOL_SUCCESS) $(GREEN)pip-audit (Analysis Worker) passed!$(RESET)"; \
		else \
			pip-audit -r requirements.txt && echo -e "$(SYMBOL_SUCCESS) $(GREEN)pip-audit (Analysis Worker) passed!$(RESET)"; \
		fi

# ==========================================
# 7. DEVELOPER SETUP
# ==========================================

setup:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Installing developer tooling (versions from .github/workflows/ci.yml)...$(RESET)"
	@# golangci-lint
	@GOLANGCI_VERSION=$$(grep -oP 'golangci-lint/cmd/golangci-lint@\K[v0-9.]+' .github/workflows/ci.yml | head -1) && \
		if command -v golangci-lint >/dev/null 2>&1 && golangci-lint --version 2>&1 | grep -q "$$GOLANGCI_VERSION"; then \
			echo -e "$(SYMBOL_SUCCESS) $(GREEN)golangci-lint $$GOLANGCI_VERSION already installed$(RESET)"; \
		else \
			echo -e "$(SYMBOL_INFO) Installing golangci-lint@$$GOLANGCI_VERSION..." && \
			go install github.com/golangci/golangci-lint/cmd/golangci-lint@$$GOLANGCI_VERSION; \
		fi
	@# oapi-codegen
	@OAPIGEN_VERSION=$$(grep -oP 'oapi-codegen/v2/cmd/oapi-codegen@\K[v0-9.]+' .github/workflows/ci.yml | head -1) && \
		if command -v oapi-codegen >/dev/null 2>&1 && oapi-codegen --version 2>&1 | grep -q "$$OAPIGEN_VERSION"; then \
			echo -e "$(SYMBOL_SUCCESS) $(GREEN)oapi-codegen $$OAPIGEN_VERSION already installed$(RESET)"; \
		else \
			echo -e "$(SYMBOL_INFO) Installing oapi-codegen@$$OAPIGEN_VERSION..." && \
			go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$$OAPIGEN_VERSION; \
		fi
	@# govulncheck
	@GOVULN_VERSION=$$(grep -oP 'govulncheck@\K[v0-9.]+' .github/workflows/ci.yml | head -1) && \
		if command -v govulncheck >/dev/null 2>&1 && govulncheck -version 2>&1 | grep -q "$$GOVULN_VERSION"; then \
			echo -e "$(SYMBOL_SUCCESS) $(GREEN)govulncheck $$GOVULN_VERSION already installed$(RESET)"; \
		else \
			echo -e "$(SYMBOL_INFO) Installing govulncheck@$$GOVULN_VERSION..." && \
			go install golang.org/x/vuln/cmd/govulncheck@$$GOVULN_VERSION; \
		fi
	@# pip-audit
	@PIPAUDIT_VERSION=$$(grep -oP 'pip-audit==\K[0-9.]+' .github/workflows/ci.yml | head -1) && \
		if [ -f services/analysis-worker/.venv/bin/pip-audit ] && services/analysis-worker/.venv/bin/pip-audit --version 2>&1 | grep -q "$$PIPAUDIT_VERSION"; then \
			echo -e "$(SYMBOL_SUCCESS) $(GREEN)pip-audit $$PIPAUDIT_VERSION already installed$(RESET)"; \
		else \
			echo -e "$(SYMBOL_INFO) Installing pip-audit==$$PIPAUDIT_VERSION into worker venv..." && \
			services/analysis-worker/.venv/bin/pip install pip-audit==$$PIPAUDIT_VERSION; \
		fi
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)All developer tools installed!$(RESET)"

# ==========================================
# HELP MENU
# ==========================================
help:
	@echo -e "$(BOLD)$(CYAN)AĒR Stack - Makefile Commands$(RESET)"
	@echo -e "$(GRAY)================================================================================$(RESET)"
	@echo -e "$(BOLD)Global Commands:$(RESET)"
	@echo -e "  $(GREEN)up$(RESET)              $(GRAY)Start the entire stack (infrastructure + services)$(RESET)"
	@echo -e "  $(GOLD)down$(RESET)            $(GRAY)Stop the entire stack$(RESET)"
	@echo -e "  $(CYAN)restart$(RESET)         $(GRAY)Restart the entire stack$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Infrastructure:$(RESET)"
	@echo -e "  $(GREEN)infra-up$(RESET)        $(GRAY)Start backend infra (DBs, Queues, Observability)$(RESET)"
	@echo -e "  $(GOLD)infra-down$(RESET)      $(GRAY)Stop backend infra$(RESET)"
	@echo -e "  $(CYAN)debug-up$(RESET)        $(GRAY)Expose internal infra ports to host for debugging$(RESET)"
	@echo -e "  $(CYAN)infra-clean$(RESET)     $(GRAY)Wipe all infra data (append -postgres, -minio, etc. for specific)$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Services:$(RESET)"
	@echo -e "  $(GREEN)services-up$(RESET)     $(GRAY)Start ingestion, worker, and bff services$(RESET)"
	@echo -e "  $(GOLD)services-down$(RESET)   $(GRAY)Stop all application services$(RESET)"
	@echo -e "  $(CYAN)<svc>-up/down$(RESET)   $(GRAY)Manage individual services (e.g., worker-up, bff-down)$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Development & Utils:$(RESET)"
	@echo -e "  $(CYAN)logs$(RESET)            $(GRAY)Tail live logs for all application services$(RESET)"
	@echo -e "  $(GREEN)crawl$(RESET)           $(GRAY)Build and run the RSS crawler (requires stack + debug-up)$(RESET)"
	@echo -e "  $(CYAN)build-services$(RESET)  $(GRAY)Compile Go API binaries into ./bin/$(RESET)"
	@echo -e "  $(CYAN)codegen$(RESET)         $(GRAY)Generate Go code from OpenAPI contracts$(RESET)"
	@echo -e "  $(CYAN)tidy$(RESET)            $(GRAY)Run 'go mod tidy' across all modules$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Testing & Linting:$(RESET)"
	@echo -e "  $(GREEN)test$(RESET)            $(GRAY)Run all unit/integration tests (Go & Python)$(RESET)"
	@echo -e "  $(GREEN)test-e2e$(RESET)        $(GRAY)Run Docker Compose end-to-end smoke test$(RESET)"
	@echo -e "  $(CYAN)lint$(RESET)            $(GRAY)Run linters across all Go and Python code$(RESET)"
	@echo -e "  $(GREEN)audit$(RESET)           $(GRAY)Run dependency vulnerability scanners (govulncheck + pip-audit)$(RESET)"
	@echo -e "  $(GREEN)setup$(RESET)           $(GRAY)Install all required developer tools (pinned to CI versions)$(RESET)"
	@echo -e "$(GRAY)================================================================================$(RESET)"
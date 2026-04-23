.PHONY: help up down stop restart
.PHONY: infra-clean infra-clean-postgres infra-clean-minio infra-clean-clickhouse
.PHONY: services-up services-down services-restart services-clean
.PHONY: ingestion-up ingestion-down ingestion-restart
.PHONY: worker-up worker-down worker-restart
.PHONY: bff-up bff-down bff-restart bff-image-build
.PHONY: debug-up debug-down
.PHONY: swagger-up swagger-down
.PHONY: logs tidy codegen openapi-bundle openapi-lint test test-go test-go-pkg test-go-crawlers test-python test-e2e lint lint-go-pkg audit audit-go audit-python build-services crawl crawl-reset setup deps-refresh
.PHONY: fe-install fe-dev fe-preview fe-lint fe-lint-fix fe-format fe-typecheck fe-test fe-test-e2e fe-test-e2e-update fe-build fe-bundle-size fe-codegen fe-check codegen-ts
.PHONY: fe-image-build fe-image-size frontend-up frontend-down frontend-restart backend-up backend-down backend-restart

SHELL := /bin/bash

include .tool-versions

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
#
# Orchestration model:
#   - Everything is a container. `compose.yaml` is the single source of
#     truth for what runs and how it's wired.
#   - `make up` brings up infra + backend services + dashboard + debug
#     port forwarder, gated by Compose healthchecks (`--wait`).
#   - `make backend-up` is the same minus the dashboard container — used
#     in the frontend iteration loop together with `make fe-dev`, which
#     serves the SvelteKit dev server on :5173 and proxies /api through
#     Traefik.
#
# The `dashboard` profile (declared on the dashboard service in
# compose.yaml) is the mechanism: it must be activated explicitly so
# `backend-up` can omit it without a separate compose file.

# Compose profile that gates the static dashboard container.
COMPOSE_DASHBOARD_PROFILE := --profile dashboard
# Always-on debug port forwarder for host-side tooling (psql, mc, curl).
COMPOSE_DEBUG_PROFILE := --profile debug

up:
	@echo -e "$(BOLD)$(GRAY)--- STARTING FULL STACK (containerized) ---$(RESET)"
	@docker compose $(COMPOSE_DEBUG_PROFILE) $(COMPOSE_DASHBOARD_PROFILE) up -d --wait
	@echo ""
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) AĒR stack is healthy.$(RESET)"
	@echo -e "$(GRAY)  Dashboard:  $(CYAN)https://localhost/$(RESET)"
	@echo -e "$(GRAY)  BFF API:    $(CYAN)https://localhost/api/v1/healthz$(RESET) (X-API-Key injected by Traefik)"
	@echo -e "$(GRAY)  Docs:       $(CYAN)http://localhost:8000$(RESET)"
	@echo -e "$(GRAY)  Use '$(BOLD)make logs$(RESET)$(GRAY)' to tail container logs.$(RESET)"

down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING FULL STACK ---$(RESET)"
	@docker compose $(COMPOSE_DEBUG_PROFILE) $(COMPOSE_DASHBOARD_PROFILE) down --remove-orphans
	@echo -e "$(SYMBOL_STOP) $(GRAY)AĒR stack stopped.$(RESET)"

stop: down

restart: down up

# Backend-only flow for the frontend iteration loop:
#   make backend-up && make fe-dev
# Skips the dashboard container so Vite owns the browser-facing surface
# on :5173 with a /api proxy → https://localhost (Traefik).
backend-up:
	@echo -e "$(BOLD)$(GRAY)--- STARTING BACKEND (no dashboard container) ---$(RESET)"
	@docker compose $(COMPOSE_DEBUG_PROFILE) up -d --wait
	@echo ""
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) Backend is healthy.$(RESET)"
	@echo -e "$(GRAY)  Run '$(BOLD)make fe-dev$(RESET)$(GRAY)' to start the SvelteKit dev server on :5173.$(RESET)"

backend-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING BACKEND ---$(RESET)"
	@docker compose $(COMPOSE_DEBUG_PROFILE) down --remove-orphans
	@echo -e "$(SYMBOL_STOP) $(GRAY)Backend stopped.$(RESET)"

backend-restart: backend-down backend-up

# ==========================================
# 1. INFRASTRUCTURE & OBSERVABILITY
# ==========================================

infra-up:
	@echo -e "$(BOLD)$(GRAY)--- STARTING INFRASTRUCTURE ONLY ---$(RESET)"
	@docker compose up -d --wait \
		traefik nats nats-init minio minio-init postgres clickhouse clickhouse-init \
		otel-collector tempo prometheus grafana docs
	@echo -e "$(SYMBOL_SUCCESS) Docs:       $(CYAN)http://localhost:8000$(RESET)"
	@echo -e "$(GRAY)  Backend application services are NOT started by this target — use 'make up' or 'make backend-up'.$(RESET)"

infra-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING INFRASTRUCTURE ---$(RESET)"
	@docker compose stop traefik nats minio postgres clickhouse otel-collector tempo prometheus grafana docs 2>/dev/null || true
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
	@docker compose $(COMPOSE_DEBUG_PROFILE) up -d debug-ports
	@echo -e "$(SYMBOL_SUCCESS) PostgreSQL: $(CYAN)localhost:5432$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) ClickHouse: $(CYAN)http://localhost:8123/play$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) NATS:       $(CYAN)localhost:4222$(RESET)  Monitor: $(CYAN)http://localhost:8222$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) MinIO API:  $(CYAN)http://localhost:9000$(RESET)  Console: $(CYAN)http://localhost:9001$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) OTel:       $(CYAN)localhost:4317$(RESET) (gRPC)  $(CYAN)localhost:4318$(RESET) (HTTP)"
	@echo -e "$(SYMBOL_SUCCESS) Ingestion:  $(CYAN)http://localhost:8081$(RESET)"
	@echo -e "$(SYMBOL_SUCCESS) Grafana:    $(CYAN)http://localhost:3000$(RESET)"

debug-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING DEBUG PORT FORWARDER ---$(RESET)"
	@docker compose $(COMPOSE_DEBUG_PROFILE) rm --stop --force debug-ports 2>/dev/null || true
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
#
# Every application service runs in a container. Individual targets are
# thin wrappers around `docker compose` so a single rebuild flow works:
#
#   make ingestion-restart  # rebuild image + recreate container
#
# Use these for tightening the loop on one service without bouncing the
# whole stack. The compose `depends_on` graph still applies.

ingestion-up:
	@docker compose up -d --wait ingestion-api

ingestion-down:
	@docker compose stop ingestion-api 2>/dev/null || true

ingestion-restart:
	@docker compose up -d --build --force-recreate --wait ingestion-api

worker-up:
	@docker compose up -d --wait analysis-worker

worker-down:
	@docker compose stop analysis-worker 2>/dev/null || true

worker-restart:
	@docker compose up -d --build --force-recreate --wait analysis-worker

bff-up:
	@docker compose up -d --wait bff-api

bff-image-build:
	@docker compose build bff-api

bff-down:
	@docker compose stop bff-api 2>/dev/null || true

bff-restart:
	@docker compose up -d --build --force-recreate --wait bff-api

# ==========================================
# 3. APPLICATION SERVICES (ALL TOGETHER)
# ==========================================

services-up: ingestion-up worker-up bff-up
	@echo ""
	@echo -e "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) All AĒR services are running.$(RESET)"
	@echo -e "$(GRAY)Use 'make logs' to view the live output.$(RESET)"

services-down: bff-down worker-down ingestion-down
	@echo -e "$(BOLD)$(GOLD)$(SYMBOL_STOP) All AĒR services stopped.$(RESET)"

services-restart: services-down services-up

services-clean: services-down
	@./scripts/clean.sh

# ==========================================
# 4. UTILITIES
# ==========================================

logs:
	@echo -e "$(BOLD)$(CYAN)Tailing container logs (Ctrl+C to exit)...$(RESET)"
	@docker compose $(COMPOSE_DEBUG_PROFILE) $(COMPOSE_DASHBOARD_PROFILE) logs -f --tail=100

tidy:
	@cd services/ingestion-api && go mod tidy
	@cd services/bff-api && go mod tidy
	@cd crawlers/rss-crawler && go mod tidy
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)Go modules tidied up.$(RESET)"

openapi-bundle:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Bundling modular OpenAPI specs (scripts/openapi_bundle.py)...$(RESET)"
	@for svc in services/bff-api services/ingestion-api; do \
		if [ -f $$svc/api/openapi.yaml ]; then \
			echo -e "$(SYMBOL_INFO) $(GRAY)→ $$svc/api/openapi.yaml$(RESET)"; \
			python3 scripts/openapi_bundle.py $$svc/api/openapi.yaml $$svc/api/openapi.bundle.yaml; \
		fi; \
	done
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)OpenAPI bundles produced.$(RESET)"

openapi-lint:
	@./scripts/openapi_ref_style_check.sh

swagger-up:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Bundling OpenAPI specs for Swagger UI...$(RESET)"
	@$(MAKE) --no-print-directory openapi-bundle
	@echo -e "$(SYMBOL_INFO) $(CYAN)Starting Swagger UI (dev profile)...$(RESET)"
	@docker compose --profile dev up -d swagger-ui
	@echo -e "$(SYMBOL_SUCCESS) Swagger UI: $(CYAN)http://localhost:8089$(RESET)"

swagger-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING SWAGGER UI ---$(RESET)"
	@docker compose --profile dev rm --stop --force swagger-ui 2>/dev/null || true
	@echo -e "$(SYMBOL_STOP) $(GRAY)Swagger UI stopped.$(RESET)"

codegen:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running oapi-codegen for BFF API...$(RESET)"
	@cd services/bff-api && oapi-codegen -config api/codegen.yaml api/openapi.yaml
	@if [ -f services/ingestion-api/api/codegen.yaml ]; then \
		echo -e "$(SYMBOL_INFO) $(CYAN)Running oapi-codegen for Ingestion API...$(RESET)"; \
		cd services/ingestion-api && oapi-codegen -config api/codegen.yaml api/openapi.yaml; \
	fi
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)API contracts generated successfully.$(RESET)"

build-services:
	@echo -e "$(BOLD)$(CYAN)Compiling AĒR binaries...$(RESET)"
	@mkdir -p bin
	@go build -o bin/ingestion-api ./services/ingestion-api/cmd/api
	@go build -o bin/bff-api ./services/bff-api/cmd/server
	@go build -o bin/rss-crawler ./crawlers/rss-crawler
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Build complete. Binaries in ./bin/$(RESET)"

crawl:
	@if [ ! -f .env ]; then \
		echo -e "\033[1m\033[38;5;196mERROR:\033[0m .env file not found. Copy .env.example to .env and set INGESTION_API_KEY before running make crawl."; \
		exit 1; \
	fi
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running RSS crawler (containerized, on aer-backend network)...$(RESET)"
	@docker compose --profile crawlers run --rm --build rss-crawler
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Crawl complete.$(RESET)"

# Clears the crawler's dedup state volume so the next `make crawl` re-
# processes every feed item. Useful after a volume wipe has emptied
# bronze/gold but the host-side state still thinks everything's seen.
crawl-reset:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Removing rss-crawler dedup state volume...$(RESET)"
	@docker volume rm aer_rss_crawler_state 2>/dev/null || true
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)State cleared. Next `make crawl` will re-ingest every feed item.$(RESET)"

# ==========================================
# 5. TESTING & LINTING
# ==========================================

test: test-go test-go-pkg test-go-crawlers test-python fe-test
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
	@$(MAKE) --no-print-directory openapi-lint
	@$(MAKE) --no-print-directory fe-lint

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
# 7. DEPENDENCY REFRESH (supply-chain baseline)
# ==========================================

# One-shot maintainer entrypoint that advances every externally-pinned
# dependency the stack ships: base image digests across all three service
# Dockerfiles, the analysis-worker pip lockfile, and the SentiWS lexicon
# hash. Delegates to scripts/deps_refresh.sh — see that script's header
# and docs/operations_playbook.md "Dependency refresh" for the runbook.
#
# Flags are forwarded verbatim, e.g.:
#   make deps-refresh ARGS="--dry-run"
#   make deps-refresh ARGS="--skip-e2e"
#   make deps-refresh ARGS="--skip-build"
deps-refresh:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running dependency refresh (see docs/operations_playbook.md)...$(RESET)"
	@./scripts/deps_refresh.sh $(ARGS)
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)deps-refresh done — review 'git diff' before committing.$(RESET)"

# ==========================================
# 7b. FRONTEND (services/dashboard/)
# ==========================================
#
# Phase 97 scaffolding. The dashboard is a SvelteKit static bundle — no Node
# runtime in production (ADR-020). Developer toolchain is Node 22 + pnpm via
# Corepack, both pinned in .tool-versions (SSoT).
#
# These targets assume pnpm is already activated:
#   corepack enable && corepack prepare pnpm@$(PNPM_VERSION) --activate
# `make fe-install` is the one-shot bootstrap.

FE_DIR := services/dashboard

fe-install:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Installing frontend dependencies (pnpm $(PNPM_VERSION))...$(RESET)"
	@cd $(FE_DIR) && pnpm install --frozen-lockfile
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend dependencies installed.$(RESET)"

fe-dev:
	@if ! docker compose ps --status=running --services | grep -qx bff-api; then \
		echo -e "\033[1m\033[38;5;214m!  Backend is not running.\033[0m Run '$(BOLD)make backend-up$(RESET)' first."; \
		exit 1; \
	fi
	@echo -e "$(SYMBOL_INFO) $(CYAN)Starting SvelteKit dev server on http://localhost:5173 (proxying /api to Traefik)...$(RESET)"
	@cd $(FE_DIR) && pnpm run dev

fe-preview: fe-build
	@echo -e "$(SYMBOL_INFO) $(CYAN)Starting preview server for the production build...$(RESET)"
	@cd $(FE_DIR) && pnpm run preview

fe-format:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Formatting frontend sources with Prettier...$(RESET)"
	@cd $(FE_DIR) && pnpm run format
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend sources formatted.$(RESET)"

fe-lint:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running frontend linters (ESLint + Prettier + svelte-check)...$(RESET)"
	@cd $(FE_DIR) && pnpm run lint
	@cd $(FE_DIR) && pnpm run check
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend lint passed!$(RESET)"

fe-lint-fix:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Auto-fixing frontend linters (Prettier + ESLint)...$(RESET)"
	@cd $(FE_DIR) && pnpm exec prettier --write .
	@cd $(FE_DIR) && pnpm exec eslint --fix .
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend auto-fix completed!$(RESET)"

fe-typecheck:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running frontend TypeScript strict typecheck...$(RESET)"
	@cd $(FE_DIR) && pnpm run check
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend typecheck passed!$(RESET)"

fe-test:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running frontend unit tests (Vitest)...$(RESET)"
	@cd $(FE_DIR) && pnpm run test:unit
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend unit tests passed!$(RESET)"

# Playwright (visual + a11y) runs inside the pinned image from compose.yaml
# so baselines match CI byte-for-byte. Browser font rendering is OS-sensitive,
# so host-local runs are not trusted for snapshot comparison.
PLAYWRIGHT_IMAGE := $(shell awk '/^  playwright-runner:/{f=1} f && /image:/{print $$2; exit}' compose.yaml)

fe-test-e2e:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running frontend Playwright gate (visual + a11y) inside $(PLAYWRIGHT_IMAGE)...$(RESET)"
	@docker run --rm --ipc=host \
		-v $(PWD):/repo -w /repo/$(FE_DIR) \
		$(PLAYWRIGHT_IMAGE) \
		bash -lc 'set +e; corepack enable >/dev/null 2>&1; CI=1 pnpm playwright test; status=$$?; chown -R $(shell id -u):$(shell id -g) .svelte-kit build test-results playwright-report 2>/dev/null; exit $$status'
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend Playwright gate passed!$(RESET)"

# Regenerate committed baselines. Runs inside the same pinned image so the
# snapshots match what CI will compare against. Commit the diff afterwards.
fe-test-e2e-update:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Updating Playwright baselines inside $(PLAYWRIGHT_IMAGE)...$(RESET)"
	@docker run --rm --ipc=host \
		-v $(PWD):/repo -w /repo/$(FE_DIR) \
		$(PLAYWRIGHT_IMAGE) \
		bash -lc 'set +e; corepack enable >/dev/null 2>&1; CI=1 pnpm playwright test --update-snapshots; status=$$?; chown -R $(shell id -u):$(shell id -g) .svelte-kit build tests test-results playwright-report 2>/dev/null; exit $$status'
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Playwright baselines updated under $(FE_DIR)/tests/e2e/__snapshots__/$(RESET)"

fe-build:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Building frontend (static adapter)...$(RESET)"
	@cd $(FE_DIR) && pnpm run build
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Frontend build complete: $(FE_DIR)/build/$(RESET)"

fe-bundle-size: fe-build
	@echo -e "$(SYMBOL_INFO) $(CYAN)Enforcing initial-bundle size budget (80 kB gzipped)...$(RESET)"
	@cd $(FE_DIR) && pnpm run bundle-size
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Bundle-size gate passed!$(RESET)"

# TypeScript codegen from the BFF OpenAPI spec. Peer of `make codegen` (Go).
# Requires the OpenAPI bundle to exist (see openapi-bundle target).
fe-codegen: openapi-bundle
	@echo -e "$(SYMBOL_INFO) $(CYAN)Running openapi-typescript for BFF API → dashboard...$(RESET)"
	@cd $(FE_DIR) && pnpm run codegen
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)TypeScript API types generated.$(RESET)"

# Root-level alias mirroring the ROADMAP's naming (`make codegen-ts`).
codegen-ts: fe-codegen

# Composite gate mirroring Go's `make lint && make test` ergonomics.
fe-check: fe-lint fe-typecheck fe-test fe-build fe-bundle-size
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Frontend check suite passed!$(RESET)"

# Image budget — ROADMAP Phase 97 requires the dashboard image to stay
# under 50 MB. Enforced post-build; Docker itself cannot gate on size.
FE_IMAGE_MAX_MB := 50

fe-image-build:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Building dashboard container image (aer-dashboard:local)...$(RESET)"
	@docker compose build dashboard
	@$(MAKE) --no-print-directory fe-image-size

fe-image-size:
	@size_mb=$$(docker image inspect aer-dashboard:local --format '{{.Size}}' | awk '{printf "%d", $$1/1048576}'); \
	if [ -z "$$size_mb" ]; then \
		echo -e "\033[1m\033[38;5;196mERROR:\033[0m aer-dashboard:local not built. Run \`make fe-image-build\`."; \
		exit 1; \
	fi; \
	echo -e "$(SYMBOL_INFO) $(GRAY)Dashboard image size: $$size_mb MB  (budget: $(FE_IMAGE_MAX_MB) MB)$(RESET)"; \
	if [ $$size_mb -gt $(FE_IMAGE_MAX_MB) ]; then \
		echo -e "\033[1m\033[38;5;196mERROR:\033[0m dashboard image exceeds $(FE_IMAGE_MAX_MB) MB budget."; \
		exit 1; \
	fi; \
	echo -e "$(SYMBOL_SUCCESS) $(GREEN)Dashboard image under budget.$(RESET)"

frontend-up:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Starting dashboard container...$(RESET)"
	@docker compose $(COMPOSE_DASHBOARD_PROFILE) up -d --wait dashboard
	@echo -e "$(SYMBOL_SUCCESS) $(GREEN)Dashboard routed via Traefik (https://localhost/).$(RESET)"

frontend-down:
	@echo -e "$(BOLD)$(GRAY)--- STOPPING DASHBOARD ---$(RESET)"
	@docker compose $(COMPOSE_DASHBOARD_PROFILE) stop dashboard 2>/dev/null || true
	@docker compose $(COMPOSE_DASHBOARD_PROFILE) rm --force dashboard 2>/dev/null || true
	@echo -e "$(SYMBOL_STOP) $(GRAY)Dashboard stopped.$(RESET)"

frontend-restart: frontend-down frontend-up

# ==========================================
# 8. DEVELOPER SETUP
# ==========================================

setup:
	@echo -e "$(SYMBOL_INFO) $(CYAN)Installing developer tooling (versions from .tool-versions)...$(RESET)"
	@# golangci-lint
	@if command -v golangci-lint >/dev/null 2>&1 && golangci-lint --version 2>&1 | grep -q "$(GOLANGCI_LINT_VERSION)"; then \
		echo -e "$(SYMBOL_SUCCESS) $(GREEN)golangci-lint $(GOLANGCI_LINT_VERSION) already installed$(RESET)"; \
	else \
		echo -e "$(SYMBOL_INFO) Installing golangci-lint@$(GOLANGCI_LINT_VERSION)..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@# oapi-codegen
	@if command -v oapi-codegen >/dev/null 2>&1 && oapi-codegen --version 2>&1 | grep -q "$(OAPI_CODEGEN_VERSION)"; then \
		echo -e "$(SYMBOL_SUCCESS) $(GREEN)oapi-codegen $(OAPI_CODEGEN_VERSION) already installed$(RESET)"; \
	else \
		echo -e "$(SYMBOL_INFO) Installing oapi-codegen@$(OAPI_CODEGEN_VERSION)..." && \
		go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION); \
	fi
	@# govulncheck
	@if command -v govulncheck >/dev/null 2>&1 && govulncheck -version 2>&1 | grep -q "$(GOVULNCHECK_VERSION)"; then \
		echo -e "$(SYMBOL_SUCCESS) $(GREEN)govulncheck $(GOVULNCHECK_VERSION) already installed$(RESET)"; \
	else \
		echo -e "$(SYMBOL_INFO) Installing govulncheck@$(GOVULNCHECK_VERSION)..." && \
		go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
	fi
	@# pip-audit
	@if [ -f services/analysis-worker/.venv/bin/pip-audit ] && services/analysis-worker/.venv/bin/pip-audit --version 2>&1 | grep -q "$(PIP_AUDIT_VERSION)"; then \
		echo -e "$(SYMBOL_SUCCESS) $(GREEN)pip-audit $(PIP_AUDIT_VERSION) already installed$(RESET)"; \
	else \
		echo -e "$(SYMBOL_INFO) Installing pip-audit==$(PIP_AUDIT_VERSION) into worker venv..." && \
		services/analysis-worker/.venv/bin/pip install pip-audit==$(PIP_AUDIT_VERSION); \
	fi
	@echo -e "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)All developer tools installed!$(RESET)"

# ==========================================
# HELP MENU
# ==========================================
help:
	@echo -e "$(BOLD)$(CYAN)AĒR Stack - Makefile Commands$(RESET)"
	@echo -e "$(GRAY)================================================================================$(RESET)"
	@echo -e "$(BOLD)Global Commands:$(RESET)"
	@echo -e "  $(GREEN)up$(RESET)                  $(GRAY)Start the entire stack (infrastructure + services)$(RESET)"
	@echo -e "  $(GOLD)down$(RESET)                $(GRAY)Stop the entire stack (alias: stop)$(RESET)"
	@echo -e "  $(CYAN)restart$(RESET)             $(GRAY)Restart the entire stack$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Infrastructure:$(RESET)"
	@echo -e "  $(GREEN)infra-up$(RESET)            $(GRAY)Start backend infra (DBs, Queues, Observability)$(RESET)"
	@echo -e "  $(GOLD)infra-down$(RESET)          $(GRAY)Stop backend infra$(RESET)"
	@echo -e "  $(CYAN)infra-restart$(RESET)       $(GRAY)Restart backend infra$(RESET)"
	@echo -e "  $(CYAN)debug-up$(RESET)            $(GRAY)Expose internal infra ports to host for debugging$(RESET)"
	@echo -e "  $(GOLD)debug-down$(RESET)          $(GRAY)Close debug port forwarder (services keep running)$(RESET)"
	@echo -e "  $(GOLD)infra-clean$(RESET)         $(GRAY)Wipe ALL infra volumes (interactive); append -postgres/-minio/-clickhouse for specific$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Services:$(RESET)"
	@echo -e "  $(GREEN)services-up$(RESET)         $(GRAY)Start ingestion, worker, and bff services$(RESET)"
	@echo -e "  $(GOLD)services-down$(RESET)       $(GRAY)Stop all application services$(RESET)"
	@echo -e "  $(CYAN)services-restart$(RESET)    $(GRAY)Restart all application services$(RESET)"
	@echo -e "  $(CYAN)<svc>-up/down/restart$(RESET) $(GRAY)Manage individual services: ingestion, worker, bff$(RESET)"
	@echo -e "  $(GOLD)services-clean$(RESET)      $(GRAY)Stop services and wipe their state (./scripts/clean.sh)$(RESET)"
	@echo -e "  $(CYAN)bff-image-build$(RESET)     $(GRAY)Build the bff-api container image$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Development & Utils:$(RESET)"
	@echo -e "  $(CYAN)logs$(RESET)                $(GRAY)Tail live logs for all application services$(RESET)"
	@echo -e "  $(GREEN)crawl$(RESET)               $(GRAY)Run the RSS crawler as a one-shot container on aer-backend$(RESET)"
	@echo -e "  $(GOLD)crawl-reset$(RESET)         $(GRAY)Wipe crawler dedup state volume so next crawl re-ingests everything$(RESET)"
	@echo -e "  $(CYAN)build-services$(RESET)      $(GRAY)Compile Go API binaries into ./bin/$(RESET)"
	@echo -e "  $(CYAN)codegen$(RESET)             $(GRAY)Generate Go types/stubs from OpenAPI contracts$(RESET)"
	@echo -e "  $(CYAN)openapi-bundle$(RESET)      $(GRAY)Bundle modular OpenAPI specs into single-file artifacts$(RESET)"
	@echo -e "  $(CYAN)openapi-lint$(RESET)        $(GRAY)Enforce two-style \$$ref convention across all OpenAPI files$(RESET)"
	@echo -e "  $(GREEN)swagger-up$(RESET)          $(GRAY)Bundle OpenAPI specs and start Swagger UI (dev, http://localhost:8089)$(RESET)"
	@echo -e "  $(GOLD)swagger-down$(RESET)        $(GRAY)Stop the Swagger UI dev container$(RESET)"
	@echo -e "  $(CYAN)tidy$(RESET)                $(GRAY)Run 'go mod tidy' across all modules$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Testing & Linting:$(RESET)"
	@echo -e "  $(GREEN)test$(RESET)                $(GRAY)Full suite: Go integration + pkg + crawlers + Python unit$(RESET)"
	@echo -e "  $(GREEN)test-go$(RESET)             $(GRAY)Go integration tests — ingestion-api + bff-api (Testcontainers)$(RESET)"
	@echo -e "  $(GREEN)test-go-pkg$(RESET)         $(GRAY)Go tests for shared pkg/ module$(RESET)"
	@echo -e "  $(GREEN)test-go-crawlers$(RESET)    $(GRAY)Go tests for rss-crawler$(RESET)"
	@echo -e "  $(GREEN)test-python$(RESET)         $(GRAY)Python unit tests (pytest, analysis-worker)$(RESET)"
	@echo -e "  $(GREEN)test-e2e$(RESET)            $(GRAY)Docker Compose end-to-end smoke test$(RESET)"
	@echo -e "  $(CYAN)lint$(RESET)                $(GRAY)All linters: ruff (Python) + golangci-lint (all Go modules) + openapi-lint$(RESET)"
	@echo -e "  $(CYAN)lint-go-pkg$(RESET)         $(GRAY)golangci-lint for shared pkg/ only$(RESET)"
	@echo -e "  $(GREEN)audit$(RESET)               $(GRAY)Dependency vulnerability scanners: govulncheck + pip-audit$(RESET)"
	@echo -e "  $(CYAN)audit-go$(RESET)            $(GRAY)govulncheck across all Go modules$(RESET)"
	@echo -e "  $(CYAN)audit-python$(RESET)        $(GRAY)pip-audit for analysis-worker$(RESET)"
	@echo -e "  $(GREEN)deps-refresh$(RESET)        $(GRAY)Rotate base image digests, pip lock, SentiWS hash (see playbook)$(RESET)"
	@echo -e "  $(GREEN)setup$(RESET)               $(GRAY)Install all developer tools pinned to .tool-versions$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Frontend (services/dashboard/):$(RESET)"
	@echo -e "  $(GREEN)fe-install$(RESET)          $(GRAY)Install frontend dependencies (pnpm, frozen lockfile)$(RESET)"
	@echo -e "  $(CYAN)fe-dev$(RESET)              $(GRAY)Start SvelteKit dev server (hot reload, localhost:5173)$(RESET)"
	@echo -e "  $(CYAN)fe-preview$(RESET)          $(GRAY)Build and serve the production bundle locally (localhost:4173)$(RESET)"
	@echo -e "  $(CYAN)fe-format$(RESET)           $(GRAY)Auto-format frontend sources with Prettier$(RESET)"
	@echo -e "  $(CYAN)fe-lint$(RESET)             $(GRAY)ESLint + Prettier check + svelte-check$(RESET)"
	@echo -e "  $(CYAN)fe-lint-fix$(RESET)         $(GRAY)Auto-fix ESLint + Prettier issues (no svelte-check auto-fix)$(RESET)"
	@echo -e "  $(CYAN)fe-typecheck$(RESET)        $(GRAY)TypeScript strict typecheck (svelte-check)$(RESET)"
	@echo -e "  $(GREEN)fe-test$(RESET)             $(GRAY)Vitest unit tests$(RESET)"
	@echo -e "  $(GREEN)fe-test-e2e$(RESET)         $(GRAY)Playwright visual + axe a11y gate (pinned Docker image)$(RESET)"
	@echo -e "  $(GREEN)fe-test-e2e-update$(RESET)  $(GRAY)Regenerate committed Playwright snapshots (pinned Docker image)$(RESET)"
	@echo -e "  $(CYAN)fe-build$(RESET)            $(GRAY)Production static build (SvelteKit static adapter)$(RESET)"
	@echo -e "  $(CYAN)fe-bundle-size$(RESET)      $(GRAY)Enforce the 80 kB initial-bundle budget (Design Brief §7)$(RESET)"
	@echo -e "  $(CYAN)fe-codegen$(RESET)          $(GRAY)Generate TypeScript API types from bff-api/openapi.yaml$(RESET)"
	@echo -e "  $(CYAN)codegen-ts$(RESET)          $(GRAY)Alias of fe-codegen (peer of \`make codegen\` for Go)$(RESET)"
	@echo -e "  $(GREEN)fe-check$(RESET)            $(GRAY)Composite: fe-lint + fe-typecheck + fe-test + fe-build + fe-bundle-size$(RESET)"
	@echo -e "  $(CYAN)fe-image-build$(RESET)      $(GRAY)Build dashboard container image (enforces 50 MB budget)$(RESET)"
	@echo -e "  $(CYAN)fe-image-size$(RESET)       $(GRAY)Check existing dashboard image against the 50 MB budget$(RESET)"
	@echo -e "  $(GREEN)frontend-up$(RESET)         $(GRAY)Start dashboard container (served via Traefik)$(RESET)"
	@echo -e "  $(GOLD)frontend-down$(RESET)       $(GRAY)Stop and remove the dashboard container$(RESET)"
	@echo -e "  $(CYAN)frontend-restart$(RESET)    $(GRAY)Restart the dashboard container$(RESET)"
	@echo -e ""
	@echo -e "$(BOLD)Frontend iteration loop:$(RESET)"
	@echo -e "  $(GREEN)backend-up$(RESET)          $(GRAY)Containerized backend stack (no dashboard container) — pair with make fe-dev$(RESET)"
	@echo -e "  $(GOLD)backend-down$(RESET)        $(GRAY)Stop backend stack$(RESET)"
	@echo -e "  $(CYAN)backend-restart$(RESET)     $(GRAY)Restart backend stack$(RESET)"
	@echo -e "$(GRAY)================================================================================$(RESET)"
.PHONY: up down restart docs docs-down docs-restart infra infra-down infra-restart tidy codegen run-ingestion run-bff run-analysis-worker build-services

# Terminal Colors & Styles (Modern Palette)
BOLD          := \033[1m
RESET         := \033[0m
GREEN         := \033[38;5;76m
CYAN          := \033[38;5;39m
MAGENTA       := \033[38;5;170m
GOLD          := \033[38;5;214m
GRAY          := \033[38;5;245m

# Symbols (Distinctly Colored)
SYMBOL_SERVICE := $(MAGENTA)◆$(RESET)
SYMBOL_SUCCESS := $(GREEN)✔$(RESET)
SYMBOL_STOP    := $(GOLD)■$(RESET)
SYMBOL_RESTART := $(CYAN)↻$(RESET)
SYMBOL_INFO    := $(CYAN)ℹ$(RESET)

# ==========================================
# GLOBAL STACK CONTROLS
# ==========================================

SERVICES = run-ingestion run-analysis-worker run-bff

# Starts everything sequentially, then runs services in parallel
up: 
	@echo ""
	@echo "$(BOLD)$(GRAY)--- INFRASTRUCTURE ---$(RESET)"
	@$(MAKE) --no-print-directory infra
	@echo ""
	@echo "$(BOLD)$(GRAY)--- DOCUMENTATION ---$(RESET)"
	@$(MAKE) --no-print-directory docs
	@echo ""
	@echo "$(BOLD)$(GRAY)--- SERVICES ---$(RESET)"
	@echo "$(BOLD)$(CYAN)Initializing AĒR orchestration...$(RESET)"
	@$(MAKE) --no-print-directory -j $(shell echo $(SERVICES) | wc -w) $(SERVICES)
	@echo ""
	@echo "$(BOLD)$(GREEN)$(SYMBOL_SUCCESS) AĒR Stack is fully operational!$(RESET)"
	@echo ""

# Stops everything (Containers)
down: 
	@echo ""
	@echo "$(BOLD)$(GRAY)--- SHUTDOWN ---$(RESET)"
	@$(MAKE) --no-print-directory infra-down
	@$(MAKE) --no-print-directory docs-down
	@echo "$(BOLD)$(GOLD)$(SYMBOL_STOP) Entire stack stopped and cleaned up.$(RESET)"
	@echo ""

# Restarts the entire stack
restart: 
	@echo "$(BOLD)$(CYAN)$(SYMBOL_RESTART) Restarting entire stack...$(RESET)"
	@$(MAKE) --no-print-directory down
	@$(MAKE) --no-print-directory up

# Cleans up Go modules in the entire workspace
tidy:
	@cd services/ingestion-api && go mod tidy
	@cd services/bff-api && go mod tidy
	@echo "$(SYMBOL_SUCCESS) $(BOLD)Go modules tidied up.$(RESET)"
	@echo "$(GRAY)Cleaning Python caches...$(RESET)"
	@find . -type d -name "__pycache__" -exec rm -rf {} +
	@echo "$(SYMBOL_SUCCESS) $(BOLD)Python environment cleaned.$(RESET)"

# ==========================================
# CODE GENERATION
# ==========================================

codegen:
	@echo "$(SYMBOL_INFO) $(CYAN)Running oapi-codegen for BFF API...$(RESET)"
	@cd services/bff-api && oapi-codegen -config api/codegen.yaml api/openapi.yaml
	@echo "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)API contracts generated successfully.$(RESET)"

# ==========================================
# INFRASTRUCTURE STACK (DATA LAKE, METADATA, ANALYTICS & OBSERVABILITY)
# ==========================================

infra:
	@docker compose up nats minio postgres clickhouse minio-init otel-collector tempo prometheus grafana -d > /dev/null 2>&1
	@echo "$(SYMBOL_SUCCESS) MinIO Data Lake:      $(CYAN)http://localhost:9001$(RESET) $(GRAY)(Credentials in .env)$(RESET)"
	@echo "$(SYMBOL_SUCCESS) PostgreSQL Database:  $(CYAN)http://localhost:5432$(RESET) $(GRAY)(DB: aer_metadata)$(RESET)"
	@echo "$(SYMBOL_SUCCESS) ClickHouse Analytics: $(CYAN)http://localhost:8123/play$(RESET) $(GRAY)(DB: aer_gold)$(RESET)"
	@echo "$(SYMBOL_SUCCESS) NATS Message Broker:  $(CYAN)http://localhost:8222$(RESET) $(GRAY)(Monitoring UI)$(RESET)"
	@echo "$(SYMBOL_SUCCESS) Grafana Dashboards:   $(CYAN)http://localhost:3000$(RESET) $(GRAY)(Credentials in .env)$(RESET)"

infra-down:
	@docker compose rm -f -s -v nats minio postgres clickhouse minio-init otel-collector tempo prometheus grafana > /dev/null 2>&1
	@echo "$(SYMBOL_STOP) $(GRAY)Infrastructure & Observability services terminated.$(RESET)"

infra-restart: infra-down infra

# ==========================================
# DOCUMENTATION STACK
# ==========================================

docs:
	@docker compose up docs -d > /dev/null 2>&1
	@echo "$(SYMBOL_SUCCESS) Documentation server: $(CYAN)http://localhost:8000$(RESET)"

docs-down:
	@docker compose rm -f -s -v docs > /dev/null 2>&1
	@echo "$(SYMBOL_STOP) $(GRAY)Documentation server offline.$(RESET)"

# ==========================================
# MICROSERVICE CONTROLS (Go)
# ==========================================

run-ingestion:
	@echo "$(SYMBOL_SERVICE) $(MAGENTA)Starting Ingestion API...$(RESET) $(GRAY)(Internal Background Service)$(RESET)"
	@go run ./services/ingestion-api/cmd/api/main.go

run-bff:
	@echo "$(SYMBOL_SERVICE) $(MAGENTA)Starting BFF API...$(RESET) $(CYAN)http://localhost:8080/api/v1/metrics$(RESET)"
	@go run ./services/bff-api/cmd/api/main.go

build-services:
	@echo "$(BOLD)$(CYAN)Compiling AĒR binaries...$(RESET)"
	@mkdir -p bin
	@go build -o bin/ingestion-api ./services/ingestion-api/cmd/api
	@go build -o bin/bff-api ./services/bff-api/cmd/api
	@echo "$(SYMBOL_SUCCESS) $(BOLD)$(GREEN)Build complete.$(RESET) $(GRAY)Binaries located in ./bin/$(RESET)"

# ==========================================
# MICROSERVICE CONTROLS (Python)
# ==========================================
run-analysis-worker:
	@echo "$(SYMBOL_SERVICE) $(MAGENTA)Starting Analysis Worker (Python)...$(RESET) $(GRAY)(Internal NATS Consumer)$(RESET)"
	@cd services/analysis-worker && \
	if [ ! -f "venv/bin/python" ]; then \
		echo "$(GRAY)Creating virtual environment...$(RESET)"; \
		rm -rf venv; \
		python3 -m venv venv; \
	fi && \
	echo "$(GRAY)Checking dependencies...$(RESET)" && \
	./venv/bin/python -m pip install -r requirements.txt -q && \
	./venv/bin/python main.py
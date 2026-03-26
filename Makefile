.PHONY: up down restart docs docs-down docs-restart infra infra-down infra-restart tidy

# ==========================================
# GLOBAL STACK CONTROLS
# ==========================================

up: infra docs
	@echo "Entire stack is up and running!"

down: infra-down docs-down
	@echo "Entire stack stopped and cleaned up."

restart: down up
	@echo "Entire stack restarted."

# Cleans up Go modules in the entire workspace
tidy:
	cd services/ingestion-api && go mod tidy
	cd services/bff-api && go mod tidy
	@echo "Go modules tidied up."

# ==========================================
# DOCUMENTATION STACK
# ==========================================

docs:
	docker compose up docs -d
	@echo "Documentation running at http://localhost:8000"

docs-down:
	docker compose rm -f -s -v docs
	@echo "Documentation stopped."

docs-restart: docs-down docs
	@echo "Documentation restarted."

# ==========================================
# INFRASTRUCTURE STACK (DATA LAKE & METADATA)
# ==========================================

infra:
	docker compose up minio postgres -d
	@echo "Data Lake is running!"
	@echo "MinIO UI: http://localhost:9001 (User: aer_admin, Pass: aer_password_123)"
	@echo "Postgres: localhost:5432 (DB: aer_metadata)"

infra-down:
	docker compose rm -f -s -v minio postgres
	@echo "Infrastructure stopped."

infra-restart: infra-down infra
	@echo "Infrastructure restarted."
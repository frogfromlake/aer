.PHONY: up down restart docs docs-down docs-restart infra infra-down infra-restart

# ==========================================
# GLOBAL STACK CONTROLS
# ==========================================

# Starts the entire stack (Infrastructure + Documentation)
up: infra docs
	@echo "Entire stack is up and running!"

# Stops and removes the entire stack
down: infra-down docs-down
	@echo "Entire stack stopped and cleaned up."

# Restarts the entire stack
restart: down up
	@echo "Entire stack restarted."


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
#!/bin/bash
TARGET=$1

# Colors
GOLD='\033[38;5;214m'
GREEN='\033[38;5;76m'
CYAN='\033[38;5;39m'
RESET='\033[0m'

# Default prefix for Docker Compose volumes (usually the folder name)
PROJECT_NAME="aer"

confirm_deletion() {
    local db_name=$1
    echo -e "${GOLD}⚠️  WARNING: This will permanently delete all data for ${db_name}!${RESET}"
    read -p "Are you sure? (y/N): " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        echo -e "${CYAN}ℹ Cancelled. No data deleted.${RESET}"
        exit 0
    fi
}

if [ "$TARGET" == "all" ]; then
    confirm_deletion "ALL DATABASES (Postgres, MinIO, ClickHouse)"
    echo "Stopping infrastructure and deleting volumes..."
    docker compose down -v
    echo -e "${GREEN}✔ All infrastructure data deleted.${RESET}"

elif [ "$TARGET" == "postgres" ]; then
    confirm_deletion "PostgreSQL"
    docker compose stop postgres
    docker compose rm -f postgres
    docker volume rm ${PROJECT_NAME}_postgres_data 2>/dev/null || true
    echo -e "${GREEN}✔ PostgreSQL data deleted.${RESET}"

elif [ "$TARGET" == "minio" ]; then
    confirm_deletion "MinIO (Data Lake)"
    docker compose stop minio minio-init
    docker compose rm -f minio minio-init
    docker volume rm ${PROJECT_NAME}_minio_data 2>/dev/null || true
    echo -e "${GREEN}✔ MinIO data deleted.${RESET}"

elif [ "$TARGET" == "clickhouse" ]; then
    confirm_deletion "ClickHouse"
    docker compose stop clickhouse
    docker compose rm -f clickhouse
    docker volume rm ${PROJECT_NAME}_clickhouse_data 2>/dev/null || true
    echo -e "${GREEN}✔ ClickHouse data deleted.${RESET}"

else
    echo "Usage: $0 {all|postgres|minio|clickhouse}"
    exit 1
fi

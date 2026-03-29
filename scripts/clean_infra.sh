#!/bin/bash
TARGET=$1

# Farben
GOLD='\033[38;5;214m'
GREEN='\033[38;5;76m'
CYAN='\033[38;5;39m'
RESET='\033[0m'

# Der Standard-Präfix für Docker Compose Volumes (meist der Ordnername)
PROJECT_NAME="aer"

confirm_deletion() {
    local db_name=$1
    echo -e "${GOLD}⚠️  ACHTUNG: Dies löscht die Daten von ${db_name} unwiderruflich!${RESET}"
    read -p "Bist du sicher? (y/N): " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        echo -e "${CYAN}ℹ Abbruch. Keine Daten gelöscht.${RESET}"
        exit 0
    fi
}

if [ "$TARGET" == "all" ]; then
    confirm_deletion "ALLEN DATENBANKEN (Postgres, MinIO, ClickHouse)"
    echo "Stoppe Infrastruktur und lösche Volumes..."
    docker compose down -v
    echo -e "${GREEN}✔ Alle Infrastruktur-Daten gelöscht.${RESET}"

elif [ "$TARGET" == "postgres" ]; then
    confirm_deletion "PostgreSQL"
    docker compose stop postgres
    docker compose rm -f postgres
    docker volume rm ${PROJECT_NAME}_postgres_data 2>/dev/null || true
    echo -e "${GREEN}✔ PostgreSQL Daten gelöscht.${RESET}"

elif [ "$TARGET" == "minio" ]; then
    confirm_deletion "MinIO (Data Lake)"
    docker compose stop minio minio-init
    docker compose rm -f minio minio-init
    docker volume rm ${PROJECT_NAME}_minio_data 2>/dev/null || true
    echo -e "${GREEN}✔ MinIO Daten gelöscht.${RESET}"

elif [ "$TARGET" == "clickhouse" ]; then
    confirm_deletion "ClickHouse"
    docker compose stop clickhouse
    docker compose rm -f clickhouse
    docker volume rm ${PROJECT_NAME}_clickhouse_data 2>/dev/null || true
    echo -e "${GREEN}✔ ClickHouse Daten gelöscht.${RESET}"

else
    echo "Usage: $0 {all|postgres|minio|clickhouse}"
    exit 1
fi
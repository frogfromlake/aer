#!/bin/bash
SERVICE=$1

# Colors
GOLD='\033[38;5;214m'
RESET='\033[0m'

if [ "$SERVICE" == "ingestion" ] || [ "$SERVICE" == "worker" ] || [ "$SERVICE" == "bff" ]; then
    if [ -f .pids/${SERVICE}.pid ]; then
        kill $(cat .pids/${SERVICE}.pid) 2>/dev/null || true
        rm -f .pids/${SERVICE}.pid
        echo -e "${GOLD}■ ${SERVICE} stopped.${RESET}"
    fi
else
    echo "Usage: $0 {ingestion|worker|bff}"
    exit 1
fi

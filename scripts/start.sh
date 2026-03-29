#!/bin/bash
SERVICE=$1
mkdir -p .pids

# Farben definieren
MAGENTA='\033[38;5;170m'
GREEN='\033[38;5;76m'
GRAY='\033[38;5;245m'
CYAN='\033[38;5;39m'
RESET='\033[0m'

if [ "$SERVICE" == "ingestion" ]; then
    if [ -f .pids/ingestion.pid ] && kill -0 $(cat .pids/ingestion.pid) 2>/dev/null; then
        echo -e "${CYAN}ℹ Ingestion API is already running.${RESET}"
    else
        echo -e "${MAGENTA}◆ Starting Ingestion API...${RESET}"
        go run ./services/ingestion-api/cmd/api/main.go > .pids/ingestion.log 2>&1 &
        echo $! > .pids/ingestion.pid
        echo -e "${GREEN}✔ Ingestion API running in background (PID: $(cat .pids/ingestion.pid))${RESET}"
    fi

elif [ "$SERVICE" == "worker" ]; then
    if [ -f .pids/worker.pid ] && kill -0 $(cat .pids/worker.pid) 2>/dev/null; then
        echo -e "${CYAN}ℹ Analysis Worker is already running.${RESET}"
    else
        echo -e "${MAGENTA}◆ Starting Analysis Worker...${RESET}"
        cd services/analysis-worker || exit
        if [ ! -d "venv" ]; then
            echo -e "${GRAY}Creating virtual environment...${RESET}"
            python3 -m venv venv
        fi
        ./venv/bin/python -m pip install -r requirements.txt -q
        ./venv/bin/python main.py > ../../.pids/worker.log 2>&1 &
        echo $! > ../../.pids/worker.pid
        echo -e "${GREEN}✔ Analysis Worker running in background (PID: $(cat ../../.pids/worker.pid))${RESET}"
    fi

elif [ "$SERVICE" == "bff" ]; then
    if [ -f .pids/bff.pid ] && kill -0 $(cat .pids/bff.pid) 2>/dev/null; then
        echo -e "${CYAN}ℹ BFF API is already running.${RESET}"
    else
        echo -e "${MAGENTA}◆ Starting BFF API...${RESET}"
        go run ./services/bff-api/cmd/server/main.go > .pids/bff.log 2>&1 &
        echo $! > .pids/bff.pid
        echo -e "${GREEN}✔ BFF API running in background (PID: $(cat .pids/bff.pid))${RESET}"
    fi
else
    echo "Usage: $0 {ingestion|worker|bff}"
    exit 1
fi
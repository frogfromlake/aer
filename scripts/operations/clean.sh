#!/bin/bash
CYAN='\033[38;5;39m'
GREEN='\033[38;5;76m'
RESET='\033[0m'

echo -e "${CYAN}ℹ Cleaning up service states...${RESET}"
rm -rf .pids
find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null
echo -e "${GREEN}✔ Cleaned.${RESET}"
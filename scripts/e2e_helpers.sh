#!/bin/bash
# Shared helper functions and color definitions for AĒR E2E test scripts.
# Source this file from e2e_smoke_test.sh — do not execute directly.

# --- Colors ---
GREEN='\033[38;5;76m'
RED='\033[38;5;196m'
CYAN='\033[38;5;39m'
GOLD='\033[38;5;214m'
RESET='\033[0m'

# --- Helpers ---
log_info()  { echo -e "${CYAN}[INFO]${RESET}  $*"; }
log_ok()    { echo -e "${GREEN}[PASS]${RESET}  $*"; PASS=$((PASS + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${RESET}  $*"; FAIL=$((FAIL + 1)); }
log_step()  { echo -e "\n${GOLD}══ $* ${RESET}"; }

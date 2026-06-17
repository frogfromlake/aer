#!/usr/bin/env bash
#
# go_coverage.sh — per-module Go line-coverage gate (Phase 142 / ADR-041).
#
# Runs the module's test suite with a merged coverage profile, then computes
# the total over the *floored* denominator: ADR-041 excludes thin `cmd/` mains
# and generated code (`generated.go`) — testing main()/wiring carries low
# signal. Fails (exit 1) when the total is below the given threshold.
#
# Usage: go_coverage.sh <module-dir> <threshold-percent>
#   e.g. go_coverage.sh services/bff-api 80
#
# Note: coverage is per-package self-coverage (Go's default), aggregated across
# the module — the same shape `go test -cover ./...` reports, so the number
# matches the measured baseline.
set -euo pipefail

module_dir="${1:?usage: go_coverage.sh <module-dir> <threshold>}"
threshold="${2:?usage: go_coverage.sh <module-dir> <threshold>}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
raw="$(mktemp)"
filtered="$(mktemp)"
trap 'rm -f "$raw" "$filtered"' EXIT

cd "$repo_root/$module_dir"

# Run all tests with a single merged profile (Testcontainer suites included).
go test -covermode=set -coverprofile="$raw" ./...

# ADR-041 denominator: drop cmd/ mains and generated code; keep the `mode:`
# header line (it never matches the exclusions).
grep -vE '/cmd/|generated\.go' "$raw" > "$filtered"

total="$(go tool cover -func="$filtered" | awk '/^total:/ { gsub("%","",$3); print $3 }')"

awk -v t="$total" -v thr="$threshold" -v m="$module_dir" 'BEGIN {
  if (t+0 < thr+0) {
    printf "FAIL  %-26s coverage %.1f%% < floor %s%%\n", m, t, thr
    exit 1
  }
  printf "ok    %-26s coverage %.1f%% >= floor %s%%\n", m, t, thr
}'

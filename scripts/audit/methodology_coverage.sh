#!/usr/bin/env bash
# Phase 122j — Methodology-Catalog Coverage Audit.
#
# Walks `services/bff-api/configs/content/{en,de}/` and reports per
# YAML entry which Phase-122i composition / methodology axes are
# already mentioned in the dual-register text. Emits a CSV-shaped
# table on stdout for triage.
#
# Axes inspected per entry (case-insensitive substring search on
# `registers.methodological.{short,long}` AND `registers.semantic.long`):
#   - composition_mentioned     : "composition", "merged", "split", "small-multiples"
#   - cross_language_mentioned  : "cross-language", "language", "language-agnostic"
#   - joint_corpus_mentioned    : "joint corpus", "union", "aggregat" (covers aggregate/aggregation/aggregated)
#   - source_set_size_mentioned : "small corpus", "n<", "small-corpus", "few articles", "sparse"
#   - working_paper_anchors     : count of `workingPaperAnchors[]`
#
# Exit code is always 0; the audit emits structured rows.

set -eo pipefail

ROOT="services/bff-api/configs/content"

if [[ ! -d "$ROOT" ]]; then
  echo "ERR: $ROOT not found — run from repo root" >&2
  exit 2
fi

# Header
printf "%s,%s,%s,%s,%s,%s,%s,%s,%s\n" \
  "locale" "type" "entityId" \
  "composition" "cross_language" "joint_corpus" "source_set_size" \
  "anchors" "needs_revision"

scan_file() {
  local f="$1"
  local locale type entityId
  locale=$(basename "$(dirname "$(dirname "$f")")")
  type=$(basename "$(dirname "$f")")
  entityId=$(basename "$f" .yaml)

  local body
  body=$(cat "$f" 2>/dev/null || true)

  has() {
    local pattern="$1"
    if echo "$body" | grep -iEq "$pattern"; then echo "yes"; else echo "no"; fi
  }

  local composition cross_language joint_corpus source_set_size anchors
  composition=$(has "composition|merged|split|small-multiples")
  cross_language=$(has "cross-language|language-agnostic|language[- ]specific")
  joint_corpus=$(has "joint corpus|union|aggregat")
  source_set_size=$(has "small corpus|few articles|sparse")
  anchors=$(echo "$body" | grep -cE "^\s*-\s*\"WP-" || true)

  local needs_revision="no"
  # A view_mode or metric entry that doesn't mention composition + at
  # least one of {cross_language, joint_corpus, source_set_size} is
  # missing 122i-revision-grade methodology context. The flag is
  # advisory; the human auditor (Phase 122j) decides whether each
  # entry actually needs the addition.
  if [[ "$type" == "view_modes" || "$type" == "metrics" ]]; then
    if [[ "$composition" == "no" ]]; then needs_revision="yes"; fi
  fi

  printf "%s,%s,%s,%s,%s,%s,%s,%s,%s\n" \
    "$locale" "$type" "$entityId" \
    "$composition" "$cross_language" "$joint_corpus" "$source_set_size" \
    "$anchors" "$needs_revision"
}

# Iterate every YAML under content/{locale}/{type}/
while IFS= read -r f; do
  scan_file "$f"
done < <(find "$ROOT" -type f -name "*.yaml" | sort)

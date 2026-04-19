#!/usr/bin/env bash
# openapi_ref_style_check.sh — enforces the AĒR OpenAPI ref-style convention.
#
# Convention (documented in ADR-021 and docs/arc42/08_concepts.md §8.19):
#   • Path-level refs to top-level components MAY use `#/components/…` — these
#     produce named Go/TS types via kin-openapi's path-item flattening.
#   • ALL other refs (schema→schema, response→schema, inside external files
#     other than `paths/`) MUST be external file refs (e.g. `../schemas/X.yaml`).
#     `#` means "this file" per JSON Reference, so in-document refs in such
#     files would not resolve without bundling.
#
# This script fails CI when an external file under `schemas/`, `parameters/`,
# or `responses/` contains a `#/…` ref, which is the only style drift that
# breaks isolated validation / bundling.

set -euo pipefail

violations=0

for svc in services/bff-api services/ingestion-api; do
  api_dir="$svc/api"
  [ -d "$api_dir" ] || continue
  for sub in schemas parameters responses; do
    dir="$api_dir/$sub"
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' file; do
      if grep -Hn -E "\\\$ref:\s*['\"]#/" "$file" >/dev/null 2>&1; then
        echo "ERROR: in-document \$ref found in external file: $file" >&2
        grep -Hn -E "\\\$ref:\s*['\"]#/" "$file" >&2 || true
        violations=$((violations + 1))
      fi
    done < <(find "$dir" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0)
  done
done

if [ "$violations" -gt 0 ]; then
  echo "openapi-lint: $violations file(s) violate the ref-style convention" >&2
  echo "Hint: inside schemas/parameters/responses, use external file refs (e.g. '../schemas/Error.yaml')." >&2
  exit 1
fi

echo "openapi-lint: ref-style convention OK"

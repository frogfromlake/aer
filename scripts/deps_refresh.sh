#!/usr/bin/env bash
# deps_refresh.sh — Phase 88 maintainer entrypoint for refreshing every
# externally-pinned dependency the AĒR stack ships in its container images.
#
# This is the single operation a maintainer runs to advance the
# supply-chain baseline set up in Phase 84:
#
#   1. Re-pull every base image referenced by a `FROM image:tag@sha256:…`
#      line across all three service Dockerfiles and rewrite the digest
#      in place. Tags stay put; only digests move.
#   2. Regenerate services/analysis-worker/requirements.lock.txt inside
#      the Python image the worker actually builds from, so the hash set
#      is byte-compatible with `pip install --require-hashes`.
#   3. Recompute SENTIWS_SHA256 by downloading the URL declared in the
#      worker Dockerfile and hashing it locally.
#   4. Rebuild all images (`docker compose build --no-cache`) and run the
#      full end-to-end smoke test so the new baseline is proven green
#      before it lands in a commit.
#
# Idempotency: running on an already-current baseline produces a clean
# `git diff`. Failure at any step aborts early with set -euo pipefail,
# leaving partial diffs the user can inspect and either keep or `git
# restore`.
#
# Flags:
#   --skip-e2e     Skip `make test-e2e` (build-only validation).
#   --skip-build   Skip both the no-cache rebuild and the e2e test.
#   --dry-run      Print every rewrite/pull/hash decision without touching
#                  the working tree. Still reaches out to the network.
#   --help         Show this help and exit.
#
# Exit codes:
#   0  all steps completed (or dry-run reported what it would do)
#   1  pre-flight failed (missing command, wrong cwd, docker daemon, etc.)
#   2  digest refresh failed
#   3  requirements.lock.txt regeneration failed
#   4  SentiWS hash recomputation failed
#   5  build or e2e validation failed
#
# See docs/operations_playbook.md "Dependency refresh" for the full runbook.

set -euo pipefail

# ---------------------------------------------------------------------------
# Formatting helpers — match the Makefile palette so the output blends in.
# ---------------------------------------------------------------------------
BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[38;5;76m'; CYAN=$'\033[38;5;39m'
GOLD=$'\033[38;5;214m'; GRAY=$'\033[38;5;245m'
RED=$'\033[38;5;196m'

log_step() { printf '\n%s%s--- %s ---%s\n' "$BOLD" "$GRAY" "$1" "$RESET"; }
log_info() { printf '%s%sℹ%s %s\n' "$BOLD" "$CYAN" "$RESET" "$1"; }
log_ok()   { printf '%s%s✔%s %s\n' "$BOLD" "$GREEN" "$RESET" "$1"; }
log_warn() { printf '%s%s!%s %s\n' "$BOLD" "$GOLD" "$RESET" "$1"; }
log_err()  { printf '%s%s✗%s %s\n' "$BOLD" "$RED" "$RESET" "$1" >&2; }

# ---------------------------------------------------------------------------
# Flag parsing
# ---------------------------------------------------------------------------
SKIP_E2E=0
SKIP_BUILD=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-e2e)   SKIP_E2E=1 ;;
        --skip-build) SKIP_BUILD=1 ;;
        --dry-run)    DRY_RUN=1 ;;
        --help|-h)
            # Print the header comment block (lines 2..40) with the leading
            # "# " stripped. Keep this range in sync with the header size.
            sed -n '2,40p' "$0" | sed -E 's/^#( |$)//'
            exit 0
            ;;
        *)
            log_err "unknown flag: $1 (see --help)"
            exit 1
            ;;
    esac
    shift
done

# ---------------------------------------------------------------------------
# Pre-flight — fail fast with actionable messages.
# ---------------------------------------------------------------------------
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

if [[ ! -f compose.yaml ]] || [[ ! -f .tool-versions ]]; then
    log_err "must run from the aer repo root (missing compose.yaml or .tool-versions)"
    exit 1
fi

for cmd in docker curl sha256sum sed awk grep; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        log_err "required command not found: $cmd"
        exit 1
    fi
done

if ! docker info >/dev/null 2>&1; then
    log_err "docker daemon is not reachable — start Docker Desktop or the engine"
    exit 1
fi

# Load SSoT versions (.tool-versions is plain KEY=value lines).
# shellcheck disable=SC1091
source .tool-versions
: "${PIP_TOOLS_VERSION:?PIP_TOOLS_VERSION missing from .tool-versions}"

DOCKERFILES=(
    "services/analysis-worker/Dockerfile"
    "services/ingestion-api/Dockerfile"
    "services/bff-api/Dockerfile"
)

for df in "${DOCKERFILES[@]}"; do
    if [[ ! -f "$df" ]]; then
        log_err "expected Dockerfile not found: $df"
        exit 1
    fi
done

if [[ "$DRY_RUN" == 1 ]]; then
    log_warn "dry-run mode: no files will be modified"
fi

# Rewrite a single file if not in dry-run mode, otherwise log the intent.
# Arguments: <file> <sed-expression>
apply_sed() {
    local file="$1" expr="$2"
    if [[ "$DRY_RUN" == 1 ]]; then
        log_info "would: sed -i '$expr' $file"
    else
        sed -i "$expr" "$file"
    fi
}

# ---------------------------------------------------------------------------
# Step 1 — Base image digest refresh.
#
# Strategy: scan every Dockerfile for `FROM image:tag@sha256:…`, deduplicate
# by (image:tag), pull each unique ref once, resolve its fresh digest, then
# substitute globally across all three Dockerfiles. That keeps shared base
# images (alpine, python, golang) in lockstep even when only one Dockerfile
# references a given tag.
# ---------------------------------------------------------------------------
log_step "STEP 1/4 — Refreshing base image digests"

# Collect unique image:tag references into an array.
mapfile -t PINNED_LINES < <(
    grep -hE '^FROM[[:space:]]+[^@[:space:]]+@sha256:[a-f0-9]+' "${DOCKERFILES[@]}" \
        | awk '{print $2}' \
        | sort -u
)

if [[ ${#PINNED_LINES[@]} -eq 0 ]]; then
    log_err "no pinned FROM ...@sha256:… lines found — Dockerfiles drifted from Phase 84 baseline?"
    exit 2
fi

declare -A CHANGED_DIGESTS=()

for ref in "${PINNED_LINES[@]}"; do
    image_tag="${ref%@*}"           # e.g. python:3.14.3-slim-bookworm
    old_digest="${ref#*@}"          # e.g. sha256:abcd…
    log_info "pulling $image_tag"
    if ! docker pull --quiet "$image_tag" >/dev/null; then
        log_err "docker pull failed for $image_tag"
        exit 2
    fi
    # Resolve the repo digest that matches the pulled tag. We ask docker
    # for the full list and grep by the bare image name (everything left
    # of the colon) so variants don't collide.
    bare_name="${image_tag%:*}"
    new_digest="$(
        docker image inspect --format='{{range .RepoDigests}}{{println .}}{{end}}' "$image_tag" \
            | grep -F "${bare_name}@" \
            | head -n1 \
            | awk -F'@' '{print $2}' \
            || true
    )"
    if [[ -z "$new_digest" ]]; then
        log_err "could not resolve repo digest for $image_tag"
        exit 2
    fi
    if [[ "$old_digest" == "$new_digest" ]]; then
        log_ok "$image_tag already at $old_digest"
        continue
    fi
    log_info "$image_tag: $old_digest -> $new_digest"
    CHANGED_DIGESTS["$image_tag"]="$old_digest|$new_digest"
done

if [[ ${#CHANGED_DIGESTS[@]} -eq 0 ]]; then
    log_ok "all base image digests already current"
else
    for image_tag in "${!CHANGED_DIGESTS[@]}"; do
        pair="${CHANGED_DIGESTS[$image_tag]}"
        old_digest="${pair%|*}"
        new_digest="${pair#*|}"
        # Escape regex metacharacters in image_tag for the sed pattern.
        escaped_tag="$(printf '%s' "$image_tag" | sed -e 's/[][\/.^$*]/\\&/g')"
        expr="s|${escaped_tag}@${old_digest}|${image_tag}@${new_digest}|g"
        for df in "${DOCKERFILES[@]}"; do
            if grep -q "${image_tag}@${old_digest}" "$df"; then
                apply_sed "$df" "$expr"
            fi
        done
    done
    log_ok "digest rewrite complete for ${#CHANGED_DIGESTS[@]} image(s)"
fi

# ---------------------------------------------------------------------------
# Step 2 — Regenerate services/analysis-worker/requirements.lock.txt.
#
# Must run inside the exact Python image the worker builds from so
# resolver output (platform tags, metadata, hash set) stays compatible
# with `pip install --require-hashes` at build time. We read that image
# reference back out of the freshly-updated Dockerfile.
# ---------------------------------------------------------------------------
log_step "STEP 2/4 — Regenerating analysis-worker requirements.lock.txt"

WORKER_DF="services/analysis-worker/Dockerfile"
PYTHON_REF="$(
    grep -m1 -E '^FROM[[:space:]]+python:' "$WORKER_DF" | awk '{print $2}'
)"
if [[ -z "$PYTHON_REF" ]]; then
    log_err "could not locate python base image in $WORKER_DF"
    exit 3
fi
log_info "using python image: $PYTHON_REF"
log_info "using pip-tools==$PIP_TOOLS_VERSION (from .tool-versions)"

if [[ "$DRY_RUN" == 1 ]]; then
    log_info "would: pip-compile --generate-hashes --allow-unsafe inside $PYTHON_REF"
else
    docker run --rm \
        -v "${REPO_ROOT}/services/analysis-worker:/work" \
        -w /work \
        "$PYTHON_REF" \
        bash -c "
            set -euo pipefail
            pip install --quiet --disable-pip-version-check 'pip-tools==${PIP_TOOLS_VERSION}'
            pip-compile \
                --quiet \
                --generate-hashes \
                --allow-unsafe \
                --output-file=requirements.lock.txt \
                requirements.txt
        " || { log_err "pip-compile failed"; exit 3; }
    log_ok "requirements.lock.txt regenerated"
fi

# ---------------------------------------------------------------------------
# Step 3 — Recompute SENTIWS_SHA256.
#
# The SentiWS URL is parsed back out of the Dockerfile so that when it
# changes upstream, the maintainer only needs to edit one place (the
# Dockerfile) and rerun this script.
# ---------------------------------------------------------------------------
log_step "STEP 3/4 — Recomputing SentiWS lexicon hash"

SENTIWS_URL="$(
    grep -m1 -oE 'https://downloads\.wortschatz-leipzig\.de/etc/SentiWS/SentiWS[^ ]+\.zip' "$WORKER_DF" \
        || true
)"
if [[ -z "$SENTIWS_URL" ]]; then
    log_err "SentiWS URL not found in $WORKER_DF (expected wortschatz-leipzig download)"
    exit 4
fi
log_info "downloading $SENTIWS_URL"

TMP_ZIP="$(mktemp --suffix=.zip)"
trap 'rm -f "$TMP_ZIP"' EXIT
if ! curl -fsSL -o "$TMP_ZIP" "$SENTIWS_URL"; then
    log_err "curl failed for $SENTIWS_URL"
    exit 4
fi

NEW_SENTIWS_HASH="$(sha256sum "$TMP_ZIP" | awk '{print $1}')"
OLD_SENTIWS_HASH="$(
    grep -m1 -E '^ARG[[:space:]]+SENTIWS_SHA256=' "$WORKER_DF" \
        | sed -E 's/^ARG[[:space:]]+SENTIWS_SHA256=//'
)"
log_info "sha256: $NEW_SENTIWS_HASH"

if [[ "$OLD_SENTIWS_HASH" == "$NEW_SENTIWS_HASH" ]]; then
    log_ok "SENTIWS_SHA256 already current"
else
    log_info "SENTIWS_SHA256: $OLD_SENTIWS_HASH -> $NEW_SENTIWS_HASH"
    apply_sed "$WORKER_DF" \
        "s|^ARG[[:space:]]\\+SENTIWS_SHA256=.*|ARG SENTIWS_SHA256=${NEW_SENTIWS_HASH}|"
    log_ok "SENTIWS_SHA256 rewritten"
fi

# ---------------------------------------------------------------------------
# Step 4 — Rebuild all images (no cache) and run the end-to-end smoke test.
# ---------------------------------------------------------------------------
log_step "STEP 4/4 — Rebuilding images and validating"

if [[ "$DRY_RUN" == 1 ]]; then
    log_warn "dry-run: skipping rebuild and validation"
    log_ok "dry-run finished — review the log above to see what would change"
    exit 0
fi

if [[ "$SKIP_BUILD" == 1 ]]; then
    log_warn "--skip-build: skipping docker compose build and make test-e2e"
    log_ok "dependency refresh finished (build skipped)"
    exit 0
fi

log_info "docker compose build --no-cache (all services)"
if ! docker compose build --no-cache; then
    log_err "docker compose build failed"
    exit 5
fi
log_ok "no-cache rebuild succeeded"

if [[ "$SKIP_E2E" == 1 ]]; then
    log_warn "--skip-e2e: skipping make test-e2e (CI will catch regressions on push)"
    log_ok "dependency refresh finished (e2e skipped)"
    exit 0
fi

log_info "running make test-e2e"
if ! make test-e2e; then
    log_err "e2e smoke test failed — investigate before committing the diff"
    exit 5
fi

log_ok "dependency refresh complete — commit the diff to land the new baseline"

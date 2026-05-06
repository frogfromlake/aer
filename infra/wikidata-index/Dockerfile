# =============================================================================
# AĒR — Wikidata Alias Index Distribution Image (Phase 118)
# =============================================================================
# Single-purpose image: bakes the SQLite alias DB built by
# scripts/build/build_wikidata_index.py and ships it on /index/. The image is NOT a
# service — the wikidata-index-init compose container copies the file into
# the persistent `wikidata_data` volume that the analysis-worker mounts
# read-only. This pattern mirrors nats-init / minio-init / clickhouse-init
# per Hard Rule 5 (IaC-only provisioning).
#
# The `data/` build context is staged by the GitHub Actions workflow
# (.github/workflows/wikidata_index_rebuild.yml) before `docker buildx`
# runs; locally, populate `infra/wikidata-index/data/` with the built
# `wikidata_aliases.db` + `.sha256` sidecar from the build script.
# =============================================================================

# Digest-pinned alpine for the runtime layer (Hard Rule 5 / Phase 84
# container hardening). Single-stage — no compilation needed; this image
# is pure data plus the BusyBox shell already present in alpine.
FROM alpine:3.22@sha256:beefdbd8a1da6d2915566fde36db9db0b524eb737fc57cd1367effd16dc0d06d

# Non-root user (Hard Rule 5 / Phase 84). The index file is world-readable
# at 0644 so the read-only bind mount in the analysis-worker can read it
# regardless of the worker's user id.
RUN addgroup -g 10001 -S aer \
    && adduser -u 10001 -S -G aer aer

# OCI labels (sbom + provenance are emitted by the workflow build step).
# `org.aer.wikidata.build-method` records which build pipeline produced the
# baked SQLite. Phase 118b made this explicit (`dump-stream`); the Phase
# 118 SPARQL build path is superseded.
LABEL org.opencontainers.image.title="AĒR Wikidata Alias Index" \
      org.opencontainers.image.description="SQLite alias index for Phase 118 entity linking. Built quarterly via .github/workflows/wikidata_index_rebuild.yml." \
      org.opencontainers.image.source="https://github.com/frogfromlake/aer" \
      org.opencontainers.image.licenses="CC0-1.0" \
      org.aer.wikidata.build-method="dump-stream"

# The DB and its sha256 sidecar are baked at /index/. The wikidata-index-init
# compose service copies them into the named volume mounted at /data/.
WORKDIR /index
COPY --chown=aer:aer data/wikidata_aliases.db .
COPY --chown=aer:aer data/wikidata_aliases.db.sha256 .

USER aer

# Image is data-only — no long-running process, no port. The compose init
# service overrides ENTRYPOINT/CMD with the copy script. We declare a
# default just so `docker run` does not crash with an empty CMD; in
# production the init container's command takes precedence.
ENTRYPOINT ["sh", "-c"]
CMD ["sha256sum -c /index/wikidata_aliases.db.sha256 && echo OK"]

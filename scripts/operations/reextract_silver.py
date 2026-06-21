#!/usr/bin/env python3
"""Phase 122 re-extraction tool — replay archived Bronze HTML through
the analysis worker's ``WebAdapter`` without re-crawling.

This is the operational realisation of the medallion-architecture
collection-vs-derivation decoupling documented in ADR-028: trafilatura
version upgrades, bug fixes in the Silver-side extraction pipeline, and
new Tier-E custom-extractor rules all trigger Silver/Gold rebuilds via
this script — Bronze is never re-fetched, no politeness budget is
spent, and there is no risk that an upstream source has changed or is
down between runs.

How it works
------------

The script lists every Bronze object under the probe's source name
prefixes, posts a synthetic NATS event re-pointing at each Bronze key,
and lets the existing analysis worker's `process_event` path do the
work. The worker's idempotency check (`get_document_status` → status
already `processed`) is bypassed because we explicitly clear the
documents table for the affected sources before the replay — the same
pattern Phase 120b's `make reset` uses, scoped to a single probe.

This keeps the re-extraction pipeline aligned with the canonical
worker code path (no risk of a divergent extraction implementation in
an "operations" tool) and gives the worker's existing logging,
metrics, and error handling for free.

Usage
-----

::

    python scripts/operations/reextract_silver.py --probe probe0

Add ``--dry-run`` to list what *would* be reprocessed without firing
events. Add ``--source <name>`` to scope the reprocess to a single
source within the probe.

Environment
-----------

Reads the standard worker environment: ``MINIO_ENDPOINT``,
``WORKER_MINIO_ACCESS_KEY``, ``WORKER_MINIO_SECRET_KEY``,
``WORKER_BRONZE_BUCKET`` (default ``bronze``), ``NATS_URL``,
``POSTGRES_HOST``/``POSTGRES_PORT``/``POSTGRES_USER``/``POSTGRES_PASSWORD``/
``POSTGRES_DB``. A typical invocation runs from the host with
``debug-up`` ports open, or from inside a container on the
``aer-backend`` network.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import sys
import urllib.parse
from datetime import datetime, timezone

logger = logging.getLogger("reextract_silver")


def _probe_sources(probe: str) -> list[str]:
    if probe == "probe0":
        return ["tagesschau", "bundesregierung"]
    if probe == "probe1":
        return ["franceinfo", "elysee"]
    raise SystemExit(
        f"unknown probe id {probe!r}; extend _probe_sources() when a new probe lands"
    )


async def _replay(
    bronze_bucket: str,
    source_names: list[str],
    nats_url: str,
    dry_run: bool,
) -> int:
    try:
        from minio import Minio  # type: ignore
        from nats.aio.client import Client as NATS  # type: ignore
    except Exception as exc:
        logger.error("missing runtime dep: %s", exc)
        return 2

    minio = Minio(
        os.environ["MINIO_ENDPOINT"],
        access_key=os.environ["WORKER_MINIO_ACCESS_KEY"],
        secret_key=os.environ["WORKER_MINIO_SECRET_KEY"],
        secure=os.getenv("MINIO_USE_SSL", "false").lower() == "true",
    )

    nc: "NATS | None" = None
    if not dry_run:
        nc = NATS()
        await nc.connect(nats_url)

    submitted = 0
    seen = 0
    for source in source_names:
        prefix = f"web/{source}/"
        for obj in minio.list_objects(bronze_bucket, prefix=prefix, recursive=True):
            seen += 1
            if dry_run:
                continue
            event = _synth_minio_event(bronze_bucket, obj.object_name)
            await nc.publish(  # type: ignore[union-attr]
                "aer.lake.bronze",
                json.dumps(event).encode("utf-8"),
            )
            submitted += 1
            if submitted % 100 == 0:
                logger.info("replayed %d events so far", submitted)

    if nc is not None:
        await nc.drain()
        await nc.close()

    logger.info(
        "re-extraction summary",
        extra={"seen": seen, "submitted": submitted, "dry_run": dry_run},
    )
    print(
        f"discovered={seen} replayed={submitted} dry_run={dry_run} sources={source_names}"
    )
    return 0


def _synth_minio_event(bucket: str, key: str) -> dict:
    """Build a minimal MinIO-event envelope shaped like the real
    notification the worker consumes. The worker reads `.Records[0]
    .s3.object.key` (URL-decoded) and `.Records[0].eventTime`.
    """
    return {
        "Records": [
            {
                "eventTime": datetime.now(tz=timezone.utc)
                .isoformat()
                .replace("+00:00", "Z"),
                "s3": {
                    "bucket": {"name": bucket},
                    "object": {
                        "key": urllib.parse.quote(key, safe="/_"),
                        "userMetadata": {},
                    },
                },
            }
        ]
    }


def _truncate_documents(probe_sources: list[str]) -> None:
    """Clear the `documents` idempotency rows for the affected sources so
    the worker reprocesses each object instead of skipping it as
    already-processed.
    """
    try:
        import psycopg2  # type: ignore
    except Exception as exc:
        raise SystemExit(f"psycopg2 missing: {exc}") from exc

    dsn = (
        f"postgresql://{os.environ['POSTGRES_USER']}:{os.environ['POSTGRES_PASSWORD']}@"
        f"{os.environ.get('POSTGRES_HOST', 'postgres')}:"
        f"{os.environ.get('POSTGRES_PORT', '5432')}/"
        f"{os.environ.get('POSTGRES_DB', 'aer_metadata')}"
    )
    with psycopg2.connect(dsn) as conn:
        with conn.cursor() as cur:
            for source in probe_sources:
                cur.execute(
                    """
                    DELETE FROM documents
                     WHERE bronze_object_key LIKE %s
                    """,
                    (f"web/{source}/%",),
                )
                logger.info("cleared documents rows for source=%s", source)


def main() -> int:
    parser = argparse.ArgumentParser(prog="reextract_silver.py", description=__doc__)
    parser.add_argument("--probe", required=True)
    parser.add_argument("--source", default="")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--bronze-bucket", default=os.getenv("WORKER_BRONZE_BUCKET", "bronze"))
    parser.add_argument("--nats-url", default=os.getenv("NATS_URL", "nats://localhost:4222"))
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )

    sources = _probe_sources(args.probe)
    if args.source:
        if args.source not in sources:
            raise SystemExit(
                f"--source {args.source!r} is not part of probe {args.probe!r} "
                f"(known: {sources})"
            )
        sources = [args.source]

    if not args.dry_run:
        _truncate_documents(sources)

    return asyncio.run(
        _replay(args.bronze_bucket, sources, args.nats_url, args.dry_run)
    )


if __name__ == "__main__":
    sys.exit(main())

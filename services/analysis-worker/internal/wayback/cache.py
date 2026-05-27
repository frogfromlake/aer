"""Postgres cache for Wayback CDX lookups — Phase 122d.0.

The CDX API is the cross-source bottleneck of the silent-edit
observability surface: every harmonised web article queries it. Caching
hits in Postgres lets us amortise the per-URL HTTP cost across NATS
redeliveries, re-processings, and adapter restarts, and keeps us well
inside the public CDX usage envelope.

The cache row shape mirrors the operational footprint:

    canonical_url    TEXT PRIMARY KEY      -- the lookup key
    fetched_at       TIMESTAMPTZ NOT NULL  -- when this row was written
    status           TEXT NOT NULL         -- one of CDXResult statuses
    revisions_jsonb  JSONB NOT NULL        -- ordered list of WaybackRevision dicts

Freshness is enforced at read time: a row older than the configured TTL
is treated as a miss. We do not run a separate retention sweep — at
~one row per canonical URL the table is naturally bounded by the
worker's source set, and stale rows are overwritten on the next lookup.
"""

from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from typing import Optional

import structlog
from psycopg2.extras import Json
from psycopg2.pool import ThreadedConnectionPool

from .client import CDXResult, WaybackRevision

logger = structlog.get_logger()


def _cache_ttl_seconds() -> int:
    """Resolve the cache TTL from the environment.

    Default 24 h matches the operational envelope: silent edits are a
    slow-moving signal and an hourly re-poll across the whole corpus
    would dominate the CDX rate budget without changing the analytical
    picture.
    """
    raw = os.getenv("WAYBACK_CDX_CACHE_TTL_HOURS", "24").strip()
    try:
        hours = int(raw)
    except (TypeError, ValueError):
        return 24 * 3600
    if hours <= 0:
        return 24 * 3600
    return hours * 3600


class WaybackCDXCache:
    """Thin Postgres-backed cache over the `wayback_cdx_cache` table."""

    def __init__(self, pg_pool: ThreadedConnectionPool, ttl_seconds: Optional[int] = None) -> None:
        self._pg_pool = pg_pool
        self._ttl_seconds = ttl_seconds if ttl_seconds is not None else _cache_ttl_seconds()

    def get(self, canonical_url: str) -> Optional[CDXResult]:
        """Return the cached `CDXResult` if fresh, else None.

        A stale row is treated as a miss (the caller will re-fetch and
        overwrite). On any database error the function returns None and
        logs at INFO — the lookup degrades to a network call, never to
        a hard failure.

        Connection hygiene: psycopg2 starts an implicit transaction on
        the first statement and ``ThreadedConnectionPool.putconn`` does
        NOT auto-rollback before returning the connection to the pool.
        Without the explicit ``conn.rollback()`` in ``finally`` the
        connection would return as ``idle in transaction``, pinning
        the xmin horizon and blocking VACUUM on ``wayback_cdx_cache``.
        Under sustained load that table would bloat indefinitely.
        """
        if not canonical_url:
            return None
        try:
            conn = self._pg_pool.getconn()
        except Exception as exc:
            logger.info("Wayback cache: pool acquire failed; falling back to network.", error=str(exc))
            return None
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT fetched_at, status, revisions_jsonb
                      FROM wayback_cdx_cache
                     WHERE canonical_url = %s
                     LIMIT 1
                    """,
                    (canonical_url,),
                )
                row = cur.fetchone()
            if row is None:
                return None
            fetched_at, status, revisions_jsonb = row
            if not isinstance(fetched_at, datetime):
                return None
            if fetched_at.tzinfo is None:
                fetched_at = fetched_at.replace(tzinfo=timezone.utc)
            age = (datetime.now(tz=timezone.utc) - fetched_at).total_seconds()
            if age > self._ttl_seconds:
                return None
            return CDXResult(
                status=str(status),
                revisions=_decode_revisions(revisions_jsonb),
            )
        except Exception as exc:
            logger.info("Wayback cache read failed; falling back to network.", canonical_url=canonical_url, error=str(exc))
            return None
        finally:
            # End the implicit transaction the SELECT opened so the
            # connection returns to the pool in `idle`, not
            # `idle in transaction`. A read-only path has nothing to
            # commit; rollback is the cheapest no-op-on-success
            # equivalent.
            try:
                conn.rollback()
            except Exception as rollback_err:
                logger.info(
                    "Wayback cache: rollback after SELECT failed; pool returns may be tainted.",
                    error=str(rollback_err),
                )
            self._pg_pool.putconn(conn)

    def put(self, canonical_url: str, result: CDXResult) -> None:
        """Upsert the lookup result.

        Idempotent via `ON CONFLICT`: a fresh row overwrites stale ones
        without leaving duplicate entries.
        """
        if not canonical_url:
            return
        payload = [r.to_dict() for r in result.revisions]
        try:
            conn = self._pg_pool.getconn()
        except Exception as exc:
            logger.info("Wayback cache: pool acquire failed; skipping cache write.", error=str(exc))
            return
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO wayback_cdx_cache (canonical_url, fetched_at, status, revisions_jsonb)
                    VALUES (%s, %s, %s, %s)
                    ON CONFLICT (canonical_url) DO UPDATE
                       SET fetched_at      = EXCLUDED.fetched_at,
                           status          = EXCLUDED.status,
                           revisions_jsonb = EXCLUDED.revisions_jsonb
                    """,
                    (
                        canonical_url,
                        datetime.now(tz=timezone.utc),
                        result.status,
                        Json(payload),
                    ),
                )
            conn.commit()
        except Exception as exc:
            try:
                conn.rollback()
            except Exception:
                pass
            logger.info("Wayback cache write failed; continuing.", canonical_url=canonical_url, error=str(exc))
        finally:
            self._pg_pool.putconn(conn)


def _decode_revisions(raw: object) -> list[WaybackRevision]:
    """Decode the jsonb payload back into typed `WaybackRevision` objects.

    psycopg2 returns jsonb as already-decoded Python objects (dicts /
    lists). The defensive `isinstance` checks handle the corner case
    where an out-of-band writer (e.g. a migration backfill) inserts a
    raw JSON string.
    """
    if isinstance(raw, str):
        try:
            raw = json.loads(raw)
        except (TypeError, ValueError):
            return []
    if not isinstance(raw, list):
        return []
    out: list[WaybackRevision] = []
    for entry in raw:
        if not isinstance(entry, dict):
            continue
        snapshot_at_raw = entry.get("snapshot_at")
        content_hash = entry.get("content_hash") or ""
        archive_url = entry.get("archive_url") or ""
        snapshot_at = _parse_iso(snapshot_at_raw)
        if snapshot_at is None or not content_hash:
            continue
        out.append(
            WaybackRevision(
                snapshot_at=snapshot_at,
                content_hash=str(content_hash),
                archive_url=str(archive_url),
            )
        )
    return out


def _parse_iso(value: object) -> Optional[datetime]:
    if not isinstance(value, str):
        return None
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed

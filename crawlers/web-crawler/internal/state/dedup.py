"""PostgreSQL-backed dedup + conditional-GET state (Phase 122).

Replaces the legacy Go RSS crawler's local JSON file. Backing table is
``crawler_state`` (schema in
``infra/postgres/migrations/000017_create_crawler_state.up.sql``).

The state is queried at three points in the crawl loop:

* Discovery — :meth:`CrawlerState.has_seen` filters out URLs whose
  canonical form already has a row, unless the discovered
  ``sitemap_lastmod`` is strictly newer than the stored value (in which
  case the article likely had a substantive update worth re-fetching).
* Fetch — :meth:`CrawlerState.conditional_headers` returns
  ``If-None-Match`` / ``If-Modified-Since`` headers for an in-flight
  request.
* Post-fetch — :meth:`CrawlerState.record` upserts the row with the
  observed ``etag`` / ``Last-Modified`` / ``content_hash`` and the
  fetch timestamp.

The class is thread-safe under multiple Scrapy worker threads via a
single ``ThreadedConnectionPool``.
"""

from __future__ import annotations

import hashlib
import logging
from datetime import datetime, timedelta, timezone
from typing import Optional

import psycopg2
from psycopg2.pool import ThreadedConnectionPool

logger = logging.getLogger(__name__)


def _now_utc() -> datetime:
    return datetime.now(tz=timezone.utc)


def content_hash(body: bytes | str) -> str:
    """Hex SHA-256 of a fetched body (str is UTF-8 encoded) — the change-detection
    fingerprint stored in crawler_state."""
    if isinstance(body, str):
        body = body.encode("utf-8", errors="ignore")
    return hashlib.sha256(body).hexdigest()


class CrawlerState:
    def __init__(self, dsn: str, minconn: int = 1, maxconn: int = 4):
        self._pool = ThreadedConnectionPool(minconn, maxconn, dsn=dsn)

    # ------------------------------------------------------------------
    # Reads
    # ------------------------------------------------------------------
    def has_seen(
        self,
        source_id: int,
        canonical_url: str,
        sitemap_lastmod: Optional[datetime] = None,
        refetch_stale_after: Optional[timedelta] = None,
    ) -> bool:
        """Return True when the URL should be skipped at discovery.

        A recorded URL is skipped unless one of two re-fetch triggers fires:

        * the supplied ``sitemap_lastmod`` is strictly newer than the stored
          value (the publisher signalled a substantive update); or
        * ``refetch_stale_after`` is set and the last fetch is older than that
          window — a freshness safety net for dateless discovery channels
          (HTML sitemaps, archive indexes, undated RSS) whose ``lastmod`` is
          always None/static, so the first trigger never re-fires. The
          re-fetched request carries conditional-GET headers, so an unchanged
          article short-circuits cheaply at a 304 (no re-submit to Bronze).
        """
        row = self._fetch_row(source_id, canonical_url)
        if row is None:
            return False
        stored_lastmod: Optional[datetime] = row.get("sitemap_lastmod")
        if sitemap_lastmod is not None and stored_lastmod is not None:
            if sitemap_lastmod > stored_lastmod:
                return False
        if refetch_stale_after is not None:
            last_fetched: Optional[datetime] = row.get("last_fetched")
            if last_fetched is not None and _now_utc() - last_fetched > refetch_stale_after:
                return False
        return True

    def conditional_headers(
        self, source_id: int, canonical_url: str
    ) -> dict[str, str]:
        """Return ``If-None-Match`` / ``If-Modified-Since`` headers when the
        prior fetch recorded an etag / Last-Modified value.
        """
        row = self._fetch_row(source_id, canonical_url)
        if row is None:
            return {}
        headers: dict[str, str] = {}
        etag = row.get("etag") or ""
        if etag:
            headers["If-None-Match"] = etag
        http_lm: Optional[datetime] = row.get("http_last_modified")
        if http_lm is not None:
            headers["If-Modified-Since"] = http_lm.strftime("%a, %d %b %Y %H:%M:%S GMT")
        return headers

    # ------------------------------------------------------------------
    # Writes
    # ------------------------------------------------------------------
    def record(
        self,
        source_id: int,
        canonical_url: str,
        etag: Optional[str],
        http_last_modified: Optional[datetime],
        content_sha256: Optional[str],
        sitemap_lastmod: Optional[datetime] = None,
    ) -> None:
        conn = self._pool.getconn()
        try:
            with conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO crawler_state
                            (source_id, canonical_url, last_fetched, etag,
                             http_last_modified, content_hash, sitemap_lastmod)
                        VALUES (%s, %s, %s, %s, %s, %s, %s)
                        ON CONFLICT (source_id, canonical_url) DO UPDATE SET
                            last_fetched       = EXCLUDED.last_fetched,
                            etag               = EXCLUDED.etag,
                            http_last_modified = EXCLUDED.http_last_modified,
                            content_hash       = EXCLUDED.content_hash,
                            sitemap_lastmod    = COALESCE(EXCLUDED.sitemap_lastmod,
                                                          crawler_state.sitemap_lastmod);
                        """,
                        (
                            source_id,
                            canonical_url,
                            _now_utc(),
                            etag,
                            http_last_modified,
                            content_sha256,
                            sitemap_lastmod,
                        ),
                    )
        except psycopg2.Error as exc:
            # Symmetric with _fetch_row: a transient write hiccup degrades
            # gracefully rather than propagating out of parse_article. The
            # Bronze key is deterministic, so a lost crawler_state row only
            # causes a re-fetch next run (idempotent, no duplication). (SEC-085)
            logger.warning(
                "crawler_state write failed for source=%s url=%s: %s",
                source_id,
                canonical_url,
                exc,
            )
        finally:
            self._pool.putconn(conn)

    # ------------------------------------------------------------------
    # Plumbing
    # ------------------------------------------------------------------
    def _fetch_row(
        self, source_id: int, canonical_url: str
    ) -> Optional[dict]:
        conn = self._pool.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT etag, http_last_modified, content_hash,
                           sitemap_lastmod, last_fetched
                      FROM crawler_state
                     WHERE source_id = %s AND canonical_url = %s
                    """,
                    (source_id, canonical_url),
                )
                row = cur.fetchone()
            if row is None:
                return None
            return {
                "etag": row[0],
                "http_last_modified": row[1],
                "content_hash": row[2],
                "sitemap_lastmod": row[3],
                "last_fetched": row[4],
            }
        except psycopg2.Error as exc:
            logger.warning("crawler_state read failed for source=%s url=%s: %s",
                           source_id, canonical_url, exc)
            return None
        finally:
            self._pool.putconn(conn)

    def close(self) -> None:
        try:
            self._pool.closeall()
        except Exception:
            pass

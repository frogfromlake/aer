"""Wayback archived-HTML fetcher — Phase 122d.1.

Sibling of `internal.wayback.client` (the CDX lookup). Where the CDX
client asks "*which* snapshots exist", this module asks "*what is the
HTML of this specific snapshot*". Both run against IA infrastructure
and share the polite-rate-limit posture, but the snapshot fetcher
uses a SEPARATE token bucket: full-HTML fetches are heavier than CDX
queries and IA's operators publicly recommend a lower rate for them.

Operator-managed via:

* ``WAYBACK_SNAPSHOT_RATE_LIMIT_PER_SECOND`` (default ``2.0``)
* ``WAYBACK_SNAPSHOT_TIMEOUT_SECONDS`` (default ``15.0`` — full HTML
  is larger than a CDX JSON, the timeout reflects that)
* Re-uses ``WEB_CRAWLER_USER_AGENT`` for HTTP identity (one polite UA
  for everything AĒR fetches; lets IA operators ban us coherently if
  needed).

Fail-silent invariant (Phase 122d.x family): a missing snapshot, a
timeout, a non-2xx HTTP, or a rate-limit denial NEVER produces a DLQ
event. The diff sweep loop simply skips the affected pair on this
tick; the next tick re-attempts.
"""

from __future__ import annotations

import threading
import time
from dataclasses import dataclass
from typing import Optional

import requests
import structlog

logger = structlog.get_logger()


# Allowed `SnapshotFetchResult.status` values. The literal strings are
# part of the worker contract — tests pin them and the sweep loop reads
# them as a coverage signal.
FETCH_OK: str = "ok"
"""Snapshot returned a non-empty HTML body."""

FETCH_EMPTY: str = "empty"
"""IA returned 200 but the body was empty / unparseable."""

FETCH_FAILED: str = "failed"
"""Timeout, network error, non-2xx HTTP, or rate-limit denial."""

FETCH_SKIPPED: str = "skipped"
"""Caller declined to fetch (e.g. empty archive_url)."""

FETCH_DISABLED: str = "disabled"
"""Snapshot integration is off in this deployment."""


@dataclass(frozen=True)
class SnapshotFetchResult:
    """Outcome of one snapshot HTML fetch. Never raises; always typed."""

    status: str
    html: str = ""


class _TokenBucket:
    """Simple thread-safe token bucket.

    Mirrors the implementation in ``internal.wayback.client._TokenBucket``;
    duplicated rather than imported because the snapshot fetcher needs
    a SEPARATE bucket (different rate limit). The two buckets bound IA
    usage independently — a busy CDX-lookup phase does not starve the
    snapshot-fetch budget and vice versa.
    """

    def __init__(self, rate_per_second: float) -> None:
        self._rate = max(0.1, float(rate_per_second))
        self._capacity = max(1.0, float(rate_per_second))
        self._tokens = self._capacity
        self._last_refill = time.monotonic()
        self._lock = threading.Lock()

    def acquire(self) -> bool:
        with self._lock:
            now = time.monotonic()
            elapsed = now - self._last_refill
            if elapsed > 0:
                self._tokens = min(self._capacity, self._tokens + elapsed * self._rate)
                self._last_refill = now
            if self._tokens >= 1.0:
                self._tokens -= 1.0
                return True
            return False

    def wait_for_token(self, max_wait_seconds: float = 5.0) -> bool:
        """Block (sleep) up to ``max_wait_seconds`` for a token.

        Unlike :meth:`acquire`, this blocks. The diff sweep loop runs
        in a background thread (via ``asyncio.to_thread``) and prefers
        a short bounded wait over the rate-limit-denial-as-failure
        path that the CDX inline-lookup uses — at sweep time we are
        not on the per-document critical path, so a brief wait is
        always preferable to losing the snapshot to a `failed` status.
        Returns True on token acquired, False on timeout.
        """
        deadline = time.monotonic() + max_wait_seconds
        while time.monotonic() < deadline:
            if self.acquire():
                return True
            time.sleep(0.1)
        return False


class WaybackSnapshotFetcher:
    """Polite sync client for full-HTML Wayback snapshot fetches.

    One instance per worker process. The diff sweep loop reuses it
    across many ``(article_id, revision_index)`` pairs per tick.
    """

    def __init__(
        self,
        *,
        enabled: bool,
        timeout_seconds: float,
        rate_limit_per_second: float,
        user_agent: str,
        session: Optional[requests.Session] = None,
    ) -> None:
        self._enabled = enabled
        self._timeout_seconds = max(1.0, float(timeout_seconds))
        self._bucket = _TokenBucket(rate_limit_per_second)
        self._user_agent = user_agent
        # Reuse the connection pool; archive.org snapshot fetches all
        # land on the same host so the TLS handshake amortises well.
        self._session = session or requests.Session()
        self._session.headers.update({"User-Agent": user_agent})

    def fetch(self, archive_url: str) -> SnapshotFetchResult:
        """Return the snapshot's raw HTML, or a typed status.

        Order of operations:
          1. Disabled deployment → ``FETCH_DISABLED`` immediately.
          2. Missing URL → ``FETCH_SKIPPED``.
          3. Wait briefly for a rate-limit token (≤ 5 s).
          4. HTTP GET with the configured timeout.
          5. Empty body → ``FETCH_EMPTY``; otherwise ``FETCH_OK``.

        Never raises. Every exception path is logged at INFO (an IA
        outage is expected; the corpus must keep flowing) and
        collapses to ``FETCH_FAILED``.
        """
        if not self._enabled:
            return SnapshotFetchResult(status=FETCH_DISABLED)
        if not archive_url:
            return SnapshotFetchResult(status=FETCH_SKIPPED)

        if not self._bucket.wait_for_token():
            logger.info(
                "Wayback snapshot rate-limited; collapsing to failed.",
                archive_url=archive_url,
            )
            return SnapshotFetchResult(status=FETCH_FAILED)

        try:
            resp = self._session.get(archive_url, timeout=self._timeout_seconds)
            resp.raise_for_status()
            body = resp.text or ""
            if not body.strip():
                return SnapshotFetchResult(status=FETCH_EMPTY)
            return SnapshotFetchResult(status=FETCH_OK, html=body)
        except Exception as exc:
            logger.info(
                "Wayback snapshot fetch failed; continuing.",
                archive_url=archive_url,
                error=str(exc),
                error_type=type(exc).__name__,
            )
            return SnapshotFetchResult(status=FETCH_FAILED)

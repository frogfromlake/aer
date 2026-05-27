"""Wayback Machine CDX client — Phase 122d.0.

Polite, sync-`requests`-based CDX client that runs in the worker's
asyncio executor pool (the existing harmoniser is already sync; adding
an async HTTP stack would buy nothing). The client enforces a
host-scoped token-bucket rate limit and a hard per-call timeout, and
never raises: every failure mode collapses to a typed `CDXResult`
status so the harmoniser can mark the article and move on.

The cache (`internal.wayback.cache.WaybackCDXCache`) is consulted FIRST;
a fresh hit short-circuits the network call entirely. The cache TTL is
the operator-managed `WAYBACK_CDX_CACHE_TTL_HOURS` (default 24 h) —
silent edits are a slow-moving signal, so a one-day cache window keeps
us well within the CDX API's polite-use envelope without losing
analytically-meaningful resolution.
"""

from __future__ import annotations

import hashlib
import threading
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Optional

import requests
import structlog

if TYPE_CHECKING:
    from .cache import WaybackCDXCache

logger = structlog.get_logger()


# Allowed `wayback_lookup_status` values. The literal strings are part of
# the WebMeta contract surface: tests pin them and the BFF reads them as
# a queryable coverage signal.
STATUS_OK: str = "ok"
"""CDX returned ≥ 1 snapshot for the canonical URL."""

STATUS_NO_SNAPSHOTS: str = "no_snapshots"
"""CDX returned 0 snapshots — the URL is not yet archived."""

STATUS_FAILED: str = "failed"
"""CDX call raised (timeout, network error, non-2xx HTTP, malformed body)."""

STATUS_SKIPPED: str = "skipped"
"""Caller declined to call CDX (e.g. missing canonical_url) — distinct from `disabled`."""

STATUS_DISABLED: str = "disabled"
"""CDX integration is off in this deployment (`WAYBACK_CDX_ENABLED=false`)."""


@dataclass(frozen=True)
class WaybackRevision:
    """One archived snapshot returned by the CDX API.

    `content_hash` is the body digest CDX records as the SHA-1 of the raw
    HTML at archive time; we keep it verbatim. `snapshot_at` is the
    archive timestamp in UTC. `archive_url` is the playback URL the
    L5EvidenceReader links to in Phase 122d.1; we record it here so the
    BFF does not need to reconstruct it.
    """

    snapshot_at: datetime
    content_hash: str
    archive_url: str

    def to_dict(self) -> dict:
        """Plain-dict serialisation for the Postgres jsonb cache + WebMeta dump."""
        return {
            "snapshot_at": self.snapshot_at.isoformat(),
            "content_hash": self.content_hash,
            "archive_url": self.archive_url,
        }


@dataclass(frozen=True)
class CDXResult:
    """Outcome of a single CDX lookup. Never raises; always typed."""

    status: str
    revisions: list[WaybackRevision] = field(default_factory=list)


class _TokenBucket:
    """Simple thread-safe token bucket.

    `rate_per_second` tokens accrue per second up to a burst of
    `max(1, rate_per_second)`. `acquire()` returns True iff a token was
    available; callers that hit `False` collapse to `STATUS_FAILED` —
    the rate limit is operator-tunable (`WAYBACK_CDX_RATE_LIMIT_PER_SECOND`),
    so a denial means the operator has chosen to bound CDX usage and the
    excess article is correctly recorded as "we did not look".
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


class WaybackCDXClient:
    """Polite sync client over the Internet Archive CDX API.

    A typical call:

        client.lookup("https://www.tagesschau.de/inland/...")

    returns a `CDXResult` whose `status` distinguishes the four
    operationally-meaningful outcomes (see module-level constants). The
    client owns the token bucket and the optional cache; the worker
    instantiates ONE `WaybackCDXClient` and reuses it across every
    harmonization call.
    """

    def __init__(
        self,
        *,
        enabled: bool,
        base_url: str,
        timeout_seconds: float,
        rate_limit_per_second: float,
        user_agent: str,
        cache: "Optional[WaybackCDXCache]" = None,
        session: Optional[requests.Session] = None,
    ) -> None:
        self._enabled = enabled
        self._base_url = base_url.rstrip("/")
        self._timeout_seconds = max(0.5, float(timeout_seconds))
        self._bucket = _TokenBucket(rate_limit_per_second)
        self._user_agent = user_agent
        self._cache = cache
        # Reuse the connection pool; CDX is one host so this halves TLS
        # handshakes on warm workers.
        self._session = session or requests.Session()
        self._session.headers.update({"User-Agent": user_agent})

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------
    def lookup(self, canonical_url: str) -> CDXResult:
        """Return the CDX snapshot list for `canonical_url`.

        Order of operations:
          1. Disabled deployment → `STATUS_DISABLED` immediately.
          2. Missing URL → `STATUS_SKIPPED`.
          3. Cache hit within TTL → return cached result (no network).
          4. Rate-limit denial → `STATUS_FAILED`.
          5. Network call → parse → optionally write cache → return.

        Never raises. Every exception path is logged and collapsed to
        `STATUS_FAILED`.
        """
        if not self._enabled:
            return CDXResult(status=STATUS_DISABLED)
        if not canonical_url:
            return CDXResult(status=STATUS_SKIPPED)

        if self._cache is not None:
            cached = self._cache.get(canonical_url)
            if cached is not None:
                return cached

        if not self._bucket.acquire():
            logger.info(
                "Wayback CDX rate-limited; collapsing to failed status.",
                canonical_url=canonical_url,
            )
            return CDXResult(status=STATUS_FAILED)

        try:
            result = self._fetch_remote(canonical_url)
        except Exception as exc:
            # Any exception inside `_fetch_remote` (timeout, DNS, malformed
            # JSON, etc.) must collapse to `STATUS_FAILED` — Phase 122d.0
            # fail-silent invariant. Log at INFO, not ERROR: an IA outage is
            # expected and the corpus must keep flowing.
            logger.info(
                "Wayback CDX lookup failed; continuing without revisions.",
                canonical_url=canonical_url,
                error=str(exc),
                error_type=type(exc).__name__,
            )
            result = CDXResult(status=STATUS_FAILED)

        # Only positive outcomes (ok / no_snapshots) are cached. A
        # `failed` row would otherwise persist a transient IA outage
        # for `WAYBACK_CDX_CACHE_TTL_HOURS` — during which every
        # short-circuit hit returns the cached failure without
        # retrying. The Postgres point-cache is meant to amortise
        # *successful* CDX answers across NATS redeliveries, not to
        # memoise outages. `skipped` is also not cached (it depends
        # only on the per-call argument, not on remote state) and
        # `disabled` cannot be reached here (the early-return at the
        # top of `lookup` skips the cache write path entirely).
        if self._cache is not None and result.status in {STATUS_OK, STATUS_NO_SNAPSHOTS}:
            try:
                self._cache.put(canonical_url, result)
            except Exception as exc:
                # Cache-write failure must not propagate — the lookup
                # itself succeeded, the next call will simply re-fetch.
                logger.warning(
                    "Wayback CDX cache write failed; continuing.",
                    canonical_url=canonical_url,
                    error=str(exc),
                )
        return result

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------
    def _fetch_remote(self, canonical_url: str) -> CDXResult:
        """Issue the CDX HTTP request and parse the response.

        Uses the CDX `output=json` shape: the first row is a header row
        of column names; subsequent rows are values aligned to that
        header. The `fl=timestamp,digest,original` projection keeps the
        payload small and stable.
        """
        params = {
            "url": canonical_url,
            "output": "json",
            "fl": "timestamp,digest,original",
            # `filter=statuscode:200` discards 3xx redirects + 4xx/5xx
            # archive errors — they are not real revisions of the page.
            "filter": "statuscode:200",
            # `collapse=digest` deduplicates consecutive snapshots whose
            # body digest is identical. A "revision" in Phase 122d is a
            # content change, not a re-archive; without this we would
            # over-count unchanged re-fetches.
            "collapse": "digest",
        }
        resp = self._session.get(
            self._base_url,
            params=params,
            timeout=self._timeout_seconds,
        )
        resp.raise_for_status()
        body = resp.json()
        if not isinstance(body, list) or len(body) <= 1:
            return CDXResult(status=STATUS_NO_SNAPSHOTS)

        header = body[0]
        try:
            ts_idx = header.index("timestamp")
            digest_idx = header.index("digest")
            url_idx = header.index("original")
        except ValueError:
            # Header missing an expected column — treat as failure rather
            # than risk silent misalignment.
            return CDXResult(status=STATUS_FAILED)

        revisions: list[WaybackRevision] = []
        for row in body[1:]:
            if not isinstance(row, list) or len(row) <= max(ts_idx, digest_idx, url_idx):
                continue
            ts_raw = row[ts_idx]
            digest = row[digest_idx]
            original = row[url_idx]
            snapshot_at = _parse_cdx_timestamp(ts_raw)
            if snapshot_at is None or not digest:
                continue
            archive_url = (
                f"https://web.archive.org/web/{ts_raw}/{original}"
                if isinstance(ts_raw, str) and isinstance(original, str)
                else ""
            )
            revisions.append(
                WaybackRevision(
                    snapshot_at=snapshot_at,
                    content_hash=str(digest),
                    archive_url=archive_url,
                )
            )

        if not revisions:
            return CDXResult(status=STATUS_NO_SNAPSHOTS)
        return CDXResult(status=STATUS_OK, revisions=revisions)


def _parse_cdx_timestamp(raw: object) -> Optional[datetime]:
    """Parse a CDX 14-digit timestamp (YYYYMMDDhhmmss) into UTC datetime.

    Returns None when the value is missing, mis-shaped, or unparseable —
    callers drop the row rather than substitute a fake timestamp.
    """
    if not isinstance(raw, str) or len(raw) != 14 or not raw.isdigit():
        return None
    try:
        return datetime.strptime(raw, "%Y%m%d%H%M%S").replace(tzinfo=timezone.utc)
    except ValueError:
        return None


def synthesise_republication_hash(article_id: str, sitemap_lastmod_iso: str) -> str:
    """Deterministic 40-char digest identifying one republication event.

    Republication-trigger rows (Phase 131a artefact reconciled in
    122d.0) are NOT CDX snapshots — they are publisher-side re-list
    events detected from the sitemap-lastmod / published_date delta. We
    still want a content hash so the row participates in the
    `(article_id, snapshot_at, content_hash)` ORDER BY tuple under
    `ReplacingMergeTree(ingestion_version)`.

    The hash MUST be stable across Bronze→Silver replays (ADR-028).
    Using the cleaned-text SHA-1 would silently break that invariant:
    every trafilatura / readability upgrade drifts the cleaned text
    by whitespace / boilerplate-trim differences, the hash changes,
    and the ReplacingMergeTree key tuple no longer matches the prior
    row — leaving BOTH rows for one re-list event. Permanent double-
    counting after every parser upgrade.

    Hashing the trigger identity instead — the article_id and the
    sitemap_lastmod that drove the detection — produces the same
    digest across replays and never collides with a real CDX SHA-1
    (which hashes raw HTML at archive time, not a derived identity
    string). The ``republication:`` prefix is purely cosmetic / makes
    the input self-describing for anyone who recomputes the hash
    manually.
    """
    payload = f"republication:{article_id}:{sitemap_lastmod_iso}"
    return hashlib.sha1(payload.encode("utf-8"), usedforsecurity=False).hexdigest()

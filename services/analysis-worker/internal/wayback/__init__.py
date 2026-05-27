"""Internet Archive CDX integration — Phase 122d Silent-Edit Observability.

The Wayback Machine CDX API is the independent third-party witness that
turns silent edits (publishers revising articles without notice) into an
observable signal (WP-003 §5). This package bundles the polite,
fail-silent CDX client and its Postgres cache; the analytical
projection that turns CDX results into `aer_gold.article_revisions`
rows lives in `internal.article_revisions`.

Fail-silent invariant: a CDX timeout, HTTP error, or rate-limiter denial
NEVER produces a DLQ event and NEVER aborts harmonization. The lookup
status (`ok` / `no_snapshots` / `failed` / `skipped` / `disabled`) is
captured on `WebMeta.wayback_lookup_status` so the BFF can render the
coverage signal honestly.
"""

from .client import CDXResult, WaybackCDXClient, WaybackRevision
from .cache import WaybackCDXCache
from .snapshot_fetcher import SnapshotFetchResult, WaybackSnapshotFetcher

__all__ = [
    "CDXResult",
    "WaybackCDXClient",
    "WaybackCDXCache",
    "WaybackRevision",
    "SnapshotFetchResult",
    "WaybackSnapshotFetcher",
]

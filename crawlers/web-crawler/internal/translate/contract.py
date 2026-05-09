"""Bronze ingestion contract for the Phase 122 web crawler.

Builds the ``IngestRequest.documents[*]`` payload sent to the
ingestion API. **Bronze is verbatim source-of-truth: no trafilatura,
no extruct, no extraction logic here.** The ingestion API stores the
``data`` field byte-for-byte in MinIO; the analysis worker's
``WebAdapter`` runs the extraction pipeline at the Silver boundary.

The Bronze key pattern is::

    web/<source>/<sha256(canonical_url)[:16]>.json

The date is intentionally absent — ``published_date`` is not yet known at
crawl time (it is extracted at the Silver boundary).
"""

from __future__ import annotations

import hashlib
import re
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Optional


def canonical_url_or(url: str) -> str:
    """Return courlan-canonicalised URL, or the input verbatim if courlan
    is unavailable. Mirrors the worker-side helper so dedup state and
    Silver canonical URLs agree.
    """
    if not url:
        return ""
    try:
        import courlan  # type: ignore

        normalised = courlan.normalize_url(url)
        if isinstance(normalised, tuple):
            normalised = normalised[0]
        return normalised or url
    except Exception:
        return url


def build_object_key(source: str, canonical_url: str) -> str:
    digest = hashlib.sha256(canonical_url.encode("utf-8")).hexdigest()[:16]
    safe_source = re.sub(r"[^a-z0-9_-]", "", source.lower())
    return f"web/{safe_source}/{digest}.json"


def _isoformat(dt: Optional[datetime]) -> Optional[str]:
    if dt is None:
        return None
    if dt.tzinfo is None:
        dt = dt.astimezone()
    return dt.isoformat().replace("+00:00", "Z")


def filter_response_headers(headers: dict[str, str]) -> dict[str, str]:
    """Strip tracking cookies and noisy headers, keep what the WebAdapter
    actually consumes. Returned dict is JSON-serialisable.
    """
    keep_lower = {
        "etag",
        "last-modified",
        "content-type",
        "content-language",
        "content-encoding",
        "cache-control",
        "vary",
    }
    out: dict[str, str] = {}
    for key, value in (headers or {}).items():
        if key.lower() in keep_lower:
            out[key.lower()] = value
    return out


@dataclass
class FetchEnvelope:
    """Per-fetch context passed by the spider to :func:`build_payload`."""

    source: str
    original_url: str
    canonical_url: str
    fetch_at: datetime
    http_status: int
    response_headers: dict[str, str]
    sitemap_lastmod: Optional[datetime] = None
    sitemap_section: Optional[str] = None
    custom_extractors: Optional[dict[str, Any]] = None


def build_payload(html: str, envelope: FetchEnvelope) -> tuple[str, dict[str, Any]]:
    """Return ``(object_key, bronze_payload_dict)``.

    The dict is the value of ``IngestDocument.data`` — the ingestion API
    stores it verbatim under the returned key. Field names mirror what
    the analysis worker's ``WebAdapter`` consumes.
    """
    if not envelope.canonical_url:
        raise ValueError("envelope.canonical_url is required")
    if not envelope.original_url:
        raise ValueError("envelope.original_url is required")
    if not envelope.source:
        raise ValueError("envelope.source is required")
    if not isinstance(html, str) or not html.strip():
        raise ValueError("html must be a non-empty string")

    headers = filter_response_headers(envelope.response_headers or {})
    http_last_modified = headers.get("last-modified")

    payload: dict[str, Any] = {
        "source": envelope.source,
        "source_type": "web",
        "raw_html": html,
        "original_url": envelope.original_url,
        "canonical_url": envelope.canonical_url,
        "url": envelope.canonical_url,
        "fetch_at": _isoformat(envelope.fetch_at),
        "http_status": envelope.http_status,
        "headers": headers,
        "etag": headers.get("etag", ""),
        "http_last_modified": http_last_modified,
        "sitemap_lastmod": _isoformat(envelope.sitemap_lastmod),
        "sitemap_section": envelope.sitemap_section or "",
    }
    if envelope.custom_extractors:
        # The worker's WebAdapter applies these XPath/CSS rules over the
        # archived raw HTML at extraction time; we forward them through
        # Bronze so a re-extraction on archived data uses the same rules
        # the original crawl was configured with.
        payload["custom_extractors"] = envelope.custom_extractors

    return build_object_key(envelope.source, envelope.canonical_url), payload

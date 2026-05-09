"""Ingestion-API client for the web-crawler.

Resolves a source name to its numeric ``source_id`` and submits batches
of Bronze documents. Single-document submission is the default for this
crawler — the analysis worker processes per-document NATS events, and a
Bronze key is deterministic on ``canonical_url``, so retries are
idempotent at the storage layer.
"""

from __future__ import annotations

import json
import logging
from typing import Any

import requests

logger = logging.getLogger(__name__)


class IngestionClient:
    def __init__(
        self,
        *,
        ingest_url: str,
        sources_url: str,
        api_key: str,
        timeout_seconds: float = 30.0,
    ) -> None:
        self._ingest_url = ingest_url
        self._sources_url = sources_url
        self._api_key = api_key
        self._timeout = timeout_seconds
        self._session = requests.Session()
        if api_key:
            self._session.headers["X-API-Key"] = api_key

    # ------------------------------------------------------------------
    def resolve_source_id(self, name: str) -> int:
        resp = self._session.get(
            self._sources_url,
            params={"name": name},
            timeout=self._timeout,
        )
        resp.raise_for_status()
        data = resp.json()
        source_id = int(data.get("id") or 0)
        if source_id <= 0:
            raise ValueError(
                f"sources API returned invalid id={data.get('id')!r} for name={name!r}"
            )
        return source_id

    # ------------------------------------------------------------------
    def submit(self, source_id: int, object_key: str, payload: dict[str, Any]) -> None:
        body = {
            "source_id": source_id,
            "documents": [{"key": object_key, "data": payload}],
        }
        resp = self._session.post(
            self._ingest_url,
            data=json.dumps(body),
            headers={"Content-Type": "application/json"},
            timeout=self._timeout,
        )
        if resp.status_code not in (200, 207):
            raise RuntimeError(
                f"ingestion API returned status={resp.status_code} body={resp.text[:512]!r}"
            )

    # ------------------------------------------------------------------
    def close(self) -> None:
        try:
            self._session.close()
        except Exception:
            pass

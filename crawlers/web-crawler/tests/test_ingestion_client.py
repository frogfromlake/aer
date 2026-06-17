"""Unit tests for the web-crawler's ingestion-API client.

`requests` is faked by the conftest, so each constructed client gets a MagicMock
session; tests configure the mock's get/post return values directly and assert
the request shape + the status/id error handling.
"""

from __future__ import annotations

import json
from unittest.mock import MagicMock

import pytest

from internal.ingestion.client import IngestionClient

INGEST_URL = "http://ingest/api/v1/ingest"
SOURCES_URL = "http://ingest/api/v1/sources"


def _client(api_key: str = "k") -> IngestionClient:
    c = IngestionClient(ingest_url=INGEST_URL, sources_url=SOURCES_URL, api_key=api_key)
    c._session = MagicMock(name="session")
    return c


def test_constructor_runs_both_api_key_branches():
    # api_key truthy → the X-API-Key header line runs; "" → it is skipped.
    IngestionClient(ingest_url="u", sources_url="s", api_key="secret")
    IngestionClient(ingest_url="u", sources_url="s", api_key="")


def test_resolve_source_id_returns_id_and_passes_name_param():
    c = _client()
    resp = MagicMock()
    resp.json.return_value = {"id": 42}
    resp.raise_for_status.return_value = None
    c._session.get.return_value = resp

    assert c.resolve_source_id("franceinfo") == 42
    _, kwargs = c._session.get.call_args
    assert kwargs["params"] == {"name": "franceinfo"}


def test_resolve_source_id_rejects_nonpositive_id():
    c = _client()
    resp = MagicMock()
    resp.json.return_value = {"id": 0}
    resp.raise_for_status.return_value = None
    c._session.get.return_value = resp
    with pytest.raises(ValueError):
        c.resolve_source_id("unknown")


def test_resolve_source_id_rejects_missing_id():
    c = _client()
    resp = MagicMock()
    resp.json.return_value = {}
    resp.raise_for_status.return_value = None
    c._session.get.return_value = resp
    with pytest.raises(ValueError):
        c.resolve_source_id("unknown")


@pytest.mark.parametrize("status", [200, 207])
def test_submit_accepts_success_statuses(status):
    c = _client()
    resp = MagicMock()
    resp.status_code = status
    c._session.post.return_value = resp

    c.submit(3, "bronze/key", {"url": "https://x/a"})

    args, kwargs = c._session.post.call_args
    assert args[0] == INGEST_URL
    body = json.loads(kwargs["data"])
    assert body["source_id"] == 3
    assert body["documents"][0]["key"] == "bronze/key"
    assert body["documents"][0]["data"] == {"url": "https://x/a"}


def test_submit_raises_on_error_status():
    c = _client()
    resp = MagicMock()
    resp.status_code = 500
    resp.text = "internal error"
    c._session.post.return_value = resp
    with pytest.raises(RuntimeError):
        c.submit(3, "k", {})


def test_close_closes_session():
    c = _client()
    c.close()
    c._session.close.assert_called_once()


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(pytest.main([__file__, "-v"]))

"""Unit tests for the PostgreSQL-backed dedup / conditional-GET state.

The pg connection pool is fully faked (mirrors tests/test_discovery_runs.py) —
pure unit tests, no real Postgres. The ``except psycopg2.Error`` degraded read
and write branches (the read path and, since SEC-085, the symmetric write path)
are intentionally not exercised: under the conftest's mocked psycopg2 the
sentinel is not a real exception class, so the happy paths are asserted here and
those branches are left to integration coverage.
"""

from __future__ import annotations

from datetime import datetime, timezone
from unittest.mock import MagicMock

import pytest

from internal.state.dedup import CrawlerState, content_hash


def _fake_pool(fetchone=None) -> tuple[MagicMock, MagicMock]:
    """Pool whose getconn() yields a conn usable as both a `with conn:` context
    (record) and a `with conn.cursor() as cur:` context (_fetch_row). Returns
    (pool, cursor) so tests can seed fetchone and introspect execute calls."""
    pool = MagicMock(name="pool")
    conn = MagicMock(name="conn")
    cursor = MagicMock(name="cursor")
    pool.getconn.return_value = conn
    conn.__enter__.return_value = conn
    conn.__exit__.return_value = False
    conn.cursor.return_value.__enter__.return_value = cursor
    conn.cursor.return_value.__exit__.return_value = False
    cursor.fetchone.return_value = fetchone
    return pool, cursor


def _state(pool: MagicMock) -> CrawlerState:
    """Construct a CrawlerState without touching a DB (bypass __init__)."""
    state = CrawlerState.__new__(CrawlerState)
    state._pool = pool
    return state


def _row(etag=None, http_lm=None, chash=None, sitemap_lm=None, fetched=None):
    # _fetch_row column order: etag, http_last_modified, content_hash,
    # sitemap_lastmod, last_fetched.
    return (etag, http_lm, chash, sitemap_lm, fetched)


# --- content_hash ------------------------------------------------------------


def test_content_hash_str_and_bytes_agree():
    assert content_hash("hello") == content_hash(b"hello")


def test_content_hash_is_sha256_hex():
    h = content_hash("x")
    assert len(h) == 64
    assert all(c in "0123456789abcdef" for c in h)


def test_content_hash_distinguishes_content():
    assert content_hash("a") != content_hash("b")


# --- has_seen ----------------------------------------------------------------


def test_has_seen_false_when_no_row():
    pool, _ = _fake_pool(fetchone=None)
    assert _state(pool).has_seen(1, "https://x/a") is False


def test_has_seen_true_when_row_exists_and_no_lastmod():
    pool, _ = _fake_pool(fetchone=_row(etag="e"))
    assert _state(pool).has_seen(1, "https://x/a") is True


def test_has_seen_false_when_sitemap_lastmod_strictly_newer():
    stored = datetime(2026, 1, 1, tzinfo=timezone.utc)
    newer = datetime(2026, 6, 1, tzinfo=timezone.utc)
    pool, _ = _fake_pool(fetchone=_row(sitemap_lm=stored))
    assert _state(pool).has_seen(1, "https://x/a", sitemap_lastmod=newer) is False


def test_has_seen_true_when_sitemap_lastmod_not_newer():
    stored = datetime(2026, 6, 1, tzinfo=timezone.utc)
    older = datetime(2026, 1, 1, tzinfo=timezone.utc)
    pool, _ = _fake_pool(fetchone=_row(sitemap_lm=stored))
    assert _state(pool).has_seen(1, "https://x/a", sitemap_lastmod=older) is True


def test_has_seen_true_when_stored_lastmod_missing():
    newer = datetime(2026, 6, 1, tzinfo=timezone.utc)
    pool, _ = _fake_pool(fetchone=_row(sitemap_lm=None))
    assert _state(pool).has_seen(1, "https://x/a", sitemap_lastmod=newer) is True


# --- conditional_headers -----------------------------------------------------


def test_conditional_headers_empty_when_no_row():
    pool, _ = _fake_pool(fetchone=None)
    assert _state(pool).conditional_headers(1, "https://x/a") == {}


def test_conditional_headers_etag_only():
    pool, _ = _fake_pool(fetchone=_row(etag='W/"abc"'))
    assert _state(pool).conditional_headers(1, "https://x/a") == {"If-None-Match": 'W/"abc"'}


def test_conditional_headers_last_modified_formatted():
    lm = datetime(2026, 5, 12, 8, 30, 0, tzinfo=timezone.utc)
    pool, _ = _fake_pool(fetchone=_row(http_lm=lm))
    headers = _state(pool).conditional_headers(1, "https://x/a")
    assert headers["If-Modified-Since"].endswith("12 May 2026 08:30:00 GMT")


def test_conditional_headers_both_validators():
    lm = datetime(2026, 5, 12, 8, 30, 0, tzinfo=timezone.utc)
    pool, _ = _fake_pool(fetchone=_row(etag="xyz", http_lm=lm))
    headers = _state(pool).conditional_headers(1, "https://x/a")
    assert headers["If-None-Match"] == "xyz"
    assert "If-Modified-Since" in headers


def test_conditional_headers_empty_when_no_validators():
    pool, _ = _fake_pool(fetchone=_row(etag="", http_lm=None))
    assert _state(pool).conditional_headers(1, "https://x/a") == {}


# --- record ------------------------------------------------------------------


def test_record_upserts_row_and_returns_connection():
    pool, cursor = _fake_pool()
    fetched_lm = datetime(2026, 5, 12, tzinfo=timezone.utc)
    _state(pool).record(
        7,
        "https://x/a",
        etag="E",
        http_last_modified=fetched_lm,
        content_sha256="HASH",
        sitemap_lastmod=None,
    )
    assert cursor.execute.call_count == 1
    sql, params = cursor.execute.call_args[0]
    assert "INSERT INTO crawler_state" in sql
    assert params[0] == 7
    assert params[1] == "https://x/a"
    assert params[3] == "E"
    assert params[4] == fetched_lm
    assert params[5] == "HASH"
    pool.putconn.assert_called_once()


# --- close -------------------------------------------------------------------


def test_close_calls_pool_closeall():
    pool, _ = _fake_pool()
    _state(pool).close()
    pool.closeall.assert_called_once()


def test_close_swallows_pool_errors():
    pool, _ = _fake_pool()
    pool.closeall.side_effect = RuntimeError("already closed")
    _state(pool).close()  # must not raise


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(pytest.main([__file__, "-v"]))

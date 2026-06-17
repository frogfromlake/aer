"""Unit tests for the revision-diff I/O helpers (Phase 122d.1 / Phase 133).

The ClickHouse pool + MinIO client are fully faked — pure unit tests, no real
infrastructure. `ch_pool` exposes getconn()/putconn()/insert(); the connection
exposes query(sql, parameters) returning an object with `.result_rows`.
"""

from __future__ import annotations

import json
from types import SimpleNamespace
from unittest.mock import MagicMock

from internal.corpus_revision_io import (
    RevisionDiffConfig,
    _silver_text_to_html,
    fetch_silver_body_for_article,
    fetch_undiffed_pairs,
    write_editorial_revision_counts,
)


def _fake_ch_pool(query_result_rows: list[list]) -> tuple[MagicMock, MagicMock]:
    """Pool whose connection.query() yields each supplied row-list in turn."""
    pool = MagicMock(name="ch_pool")
    client = MagicMock(name="ch_client")
    pool.getconn.return_value = client
    client.query.side_effect = [SimpleNamespace(result_rows=rows) for rows in query_result_rows]
    return pool, client


# --- _silver_text_to_html (pure) ---------------------------------------------


def test_silver_text_to_html_wraps_paragraphs():
    out = _silver_text_to_html("Para one.\n\nPara two.")
    assert out == "<!DOCTYPE html><html><body><p>Para one.</p><p>Para two.</p></body></html>"


def test_silver_text_to_html_drops_blank_paragraphs():
    out = _silver_text_to_html("A\n\n\n\n   \n\nB")
    assert out == "<!DOCTYPE html><html><body><p>A</p><p>B</p></body></html>"


def test_silver_text_to_html_empty_body():
    assert _silver_text_to_html("") == "<!DOCTYPE html><html><body></body></html>"


# --- RevisionDiffConfig (env-driven defaults) --------------------------------

_CFG_ENV = (
    "REVISION_DIFF_EXTRACTION_ENABLED",
    "REVISION_DIFF_EXTRACTION_INTERVAL_SECONDS",
    "REVISION_DIFF_EXTRACTION_INITIAL_DELAY_SECONDS",
    "REVISION_DIFF_MAX_PAIRS_PER_TICK",
)


def test_revision_diff_config_defaults(monkeypatch):
    for key in _CFG_ENV:
        monkeypatch.delenv(key, raising=False)
    cfg = RevisionDiffConfig()
    assert cfg.enabled is False
    assert cfg.interval_seconds == 3600.0
    assert cfg.initial_delay_seconds == 180.0
    assert cfg.max_pairs_per_tick == 200


def test_revision_diff_config_env_overrides(monkeypatch):
    monkeypatch.setenv("REVISION_DIFF_EXTRACTION_ENABLED", "true")
    monkeypatch.setenv("REVISION_DIFF_MAX_PAIRS_PER_TICK", "50")
    cfg = RevisionDiffConfig()
    assert cfg.enabled is True
    assert cfg.max_pairs_per_tick == 50


# --- fetch_undiffed_pairs ----------------------------------------------------

# row column order matches the mid/head SELECT projections (13 columns).
def _pair_row(article_id, idx, curr_url, last_url, lang):
    return (article_id, "src", "report", "2026-05-12", "h", "ph", idx, 1.0,
            "cdx_snapshot", 99, curr_url, last_url, lang)


def test_fetch_undiffed_pairs_mid_chain():
    pool, _ = _fake_ch_pool([[_pair_row("a1", 2, "curr_url", "prev_url", "de")], []])
    pairs = fetch_undiffed_pairs(pool, limit=10)
    assert len(pairs) == 1
    assert pairs[0]["kind"] == "mid_chain"
    assert pairs[0]["article_id"] == "a1"
    assert pairs[0]["prev_archive_url"] == "prev_url"
    pool.putconn.assert_called()


def test_fetch_undiffed_pairs_limit_reached_skips_head_query():
    mid = [_pair_row(f"a{i}", 2, "c", "p", "de") for i in range(3)]
    pool, client = _fake_ch_pool([mid])  # only the mid query is provided
    pairs = fetch_undiffed_pairs(pool, limit=3)
    assert len(pairs) == 3
    assert client.query.call_count == 1  # head query skipped (remaining == 0)


def test_fetch_undiffed_pairs_chain_head():
    pool, _ = _fake_ch_pool([[], [_pair_row("a2", 0, "own_url", "newest_url", "fr")]])
    pairs = fetch_undiffed_pairs(pool, limit=10)
    assert len(pairs) == 1
    assert pairs[0]["kind"] == "chain_head"
    assert pairs[0]["compare_archive_url"] == "newest_url"
    assert pairs[0]["prev_archive_url"] == ""


# --- fetch_silver_body_for_article -------------------------------------------


def test_fetch_silver_body_returns_cleaned_text():
    pool, _ = _fake_ch_pool([[("bronze/key",)]])
    minio = MagicMock()
    resp = MagicMock()
    resp.read.return_value = json.dumps({"core": {"cleaned_text": "Body text"}}).encode("utf-8")
    minio.get_object.return_value = resp
    assert fetch_silver_body_for_article(pool, minio, "silver", "a1") == "Body text"
    resp.close.assert_called_once()


def test_fetch_silver_body_empty_when_no_silver_row():
    pool, _ = _fake_ch_pool([[]])
    assert fetch_silver_body_for_article(pool, MagicMock(), "silver", "a1") == ""


def test_fetch_silver_body_empty_on_minio_error():
    pool, _ = _fake_ch_pool([[("bronze/key",)]])
    minio = MagicMock()
    minio.get_object.side_effect = RuntimeError("minio down")
    assert fetch_silver_body_for_article(pool, minio, "silver", "a1") == ""


# --- write_editorial_revision_counts -----------------------------------------


def test_write_editorial_revision_counts_empty_is_noop():
    pool = MagicMock()
    assert write_editorial_revision_counts(pool, []) == 0
    pool.getconn.assert_not_called()


def test_write_editorial_revision_counts_inserts_metric_rows():
    # reconcile-query column order: article_id, source, discourse_function, edits, ts
    recon_row = ("a1", "src", "report", 3.0, "2026-05-12")
    pool, _ = _fake_ch_pool([[recon_row]])
    written = write_editorial_revision_counts(pool, ["a1"])
    assert written == 1
    pool.insert.assert_called_once()
    args, kwargs = pool.insert.call_args
    assert args[0] == "aer_gold.metrics"
    assert kwargs["column_names"][3] == "metric_name"

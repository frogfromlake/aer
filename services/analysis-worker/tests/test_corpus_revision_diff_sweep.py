"""Unit tests for ``run_revision_diff_sweep`` — the synchronous per-tick
revision-diff orchestration (Phase 122d.1/.3, Phase 133).

The sweep is the real business logic (which pairs to diff, how to resolve the
two HTMLs per pair-kind, fail-silent skips, the optional discourse-delta pass,
and the editorial-count reconcile). The surrounding async background loop is an
entrypoint-style ``while``/``asyncio`` wrapper covered by the loop's own guard
tests below. The Wayback/MinIO/ClickHouse IO is faked; ``compute_diff`` /
``extract_paragraphs`` run for real against the supplied HTML.
"""

from __future__ import annotations

import asyncio
from unittest.mock import MagicMock

import pytest

import internal.corpus_revision_diff as crd
from internal.corpus_revision_io import RevisionDiffConfig
from internal.wayback.snapshot_fetcher import FETCH_OK, SnapshotFetchResult

PREV_HTML = "<html><head><title>Old headline</title></head><body><h1>Old headline</h1><p>First paragraph stays.</p><p>Second paragraph original.</p></body></html>"
CURR_HTML = "<html><head><title>New headline</title></head><body><h1>New headline</h1><p>First paragraph stays.</p><p>Second paragraph EDITED substantially.</p></body></html>"


class _Fetcher:
    """Fake snapshot fetcher: maps archive URL → SnapshotFetchResult."""

    def __init__(self, by_url):
        self._by_url = by_url

    def fetch(self, url):
        return self._by_url.get(url, SnapshotFetchResult(status="error", html=""))


def _mid_chain_pair(**over):
    pair = {
        "kind": "mid_chain",
        "article_id": "a1",
        "source": "tagesschau",
        "discourse_function": "epistemic_authority",
        "snapshot_at": "2025-01-06T09:00:00Z",
        "content_hash": "h1",
        "prev_content_hash": "h0",
        "revision_index": 1,
        "time_since_prev_hours": 25.0,
        "revision_trigger": "cdx_snapshot",
        "ingestion_version": 1,
        "curr_archive_url": "https://web.archive.org/curr",
        "prev_archive_url": "https://web.archive.org/prev",
        "language": "de",
    }
    pair.update(over)
    return pair


def _chain_head_pair(**over):
    pair = {
        "kind": "chain_head",
        "article_id": "a2",
        "source": "tagesschau",
        "discourse_function": "epistemic_authority",
        "snapshot_at": "2025-01-06T09:00:00Z",
        "content_hash": "h0",
        "prev_content_hash": "",
        "revision_index": 0,
        "time_since_prev_hours": 0.0,
        "revision_trigger": "cdx_snapshot",
        "ingestion_version": 1,
        "curr_archive_url": "https://web.archive.org/head",
        "compare_archive_url": "https://web.archive.org/newest",
        "language": "de",
    }
    pair.update(over)
    return pair


@pytest.fixture
def patched_io(monkeypatch):
    """Patch the corpus-revision IO helpers the sweep calls at module scope."""
    write_counts = MagicMock(return_value=2)
    monkeypatch.setattr(crd, "write_editorial_revision_counts", write_counts)
    return {"write_counts": write_counts}


def test_no_pairs_returns_zero(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [])
    ch_pool = MagicMock()
    n = crd.run_revision_diff_sweep(ch_pool, _Fetcher({}), MagicMock(), "silver", 10)
    assert n == 0
    ch_pool.insert.assert_not_called()
    patched_io["write_counts"].assert_not_called()


def test_mid_chain_writes_row_and_reconciles(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair()])
    fetcher = _Fetcher({
        "https://web.archive.org/curr": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
        "https://web.archive.org/prev": SnapshotFetchResult(status=FETCH_OK, html=PREV_HTML),
    })
    ch_pool = MagicMock()

    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)

    assert n == 1
    ch_pool.insert.assert_called_once()
    table = ch_pool.insert.call_args.args[0]
    rows = ch_pool.insert.call_args.args[1]
    assert table == "aer_gold.article_revisions"
    assert len(rows) == 1
    # The headline change must be detected and surfaced on the row.
    row = rows[0]
    assert row[0] == "a1"  # article_id
    assert row[12] is True  # headline_changed
    # Editorial-count reconcile runs for the touched article.
    patched_io["write_counts"].assert_called_once()
    assert patched_io["write_counts"].call_args.args[1] == ["a1"]


def test_mid_chain_skips_when_snapshot_fetch_fails(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair()])
    # curr OK, prev fails → the whole pair is skipped, nothing written.
    fetcher = _Fetcher({
        "https://web.archive.org/curr": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
        "https://web.archive.org/prev": SnapshotFetchResult(status="error", html=""),
    })
    ch_pool = MagicMock()
    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)
    assert n == 0
    ch_pool.insert.assert_not_called()


def test_mid_chain_skips_when_curr_fetch_fails(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair()])
    # curr itself fails first → skip before prev is even fetched.
    fetcher = _Fetcher({
        "https://web.archive.org/curr": SnapshotFetchResult(status="error", html=""),
    })
    ch_pool = MagicMock()
    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)
    assert n == 0
    ch_pool.insert.assert_not_called()


def test_chain_head_skips_when_compare_fetch_fails(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_chain_head_pair()])
    fetcher = _Fetcher({
        "https://web.archive.org/newest": SnapshotFetchResult(status="error", html=""),
    })
    ch_pool = MagicMock()
    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)
    assert n == 0
    ch_pool.insert.assert_not_called()


def test_chain_head_diffs_against_current_silver(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_chain_head_pair()])
    monkeypatch.setattr(crd, "fetch_silver_body_for_article", lambda *a, **k: "First paragraph stays.\n\nSecond paragraph original.")
    fetcher = _Fetcher({
        "https://web.archive.org/newest": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
    })
    ch_pool = MagicMock()

    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)

    assert n == 1
    rows = ch_pool.insert.call_args.args[1]
    assert rows[0][0] == "a2"
    # The head row keeps its OWN archive_url (identity), not the compare URL.
    assert rows[0][10] == "https://web.archive.org/head"


def test_chain_head_skips_when_no_silver_body(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_chain_head_pair()])
    monkeypatch.setattr(crd, "fetch_silver_body_for_article", lambda *a, **k: "")
    fetcher = _Fetcher({
        "https://web.archive.org/newest": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
    })
    ch_pool = MagicMock()
    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)
    assert n == 0
    ch_pool.insert.assert_not_called()


def test_unknown_pair_kind_is_skipped(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair(kind="weird")])
    ch_pool = MagicMock()
    n = crd.run_revision_diff_sweep(ch_pool, _Fetcher({}), MagicMock(), "silver", 10)
    assert n == 0


def test_delta_path_computes_deltas_for_real_edits(monkeypatch, patched_io):
    from internal.extractors import revision_deltas

    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair()])
    fetcher = _Fetcher({
        "https://web.archive.org/curr": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
        "https://web.archive.org/prev": SnapshotFetchResult(status=FETCH_OK, html=PREV_HTML),
    })
    captured = revision_deltas.DeltaResult(
        sentiment_delta=-0.2,
        entities_added=["Q1"],
        entities_removed=[],
        topic_shift_score=0.4,
        deltas_computed=True,
    )
    monkeypatch.setattr(revision_deltas, "compute_deltas", lambda *a, **k: captured)
    ch_pool = MagicMock()

    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10, delta_tools=object())

    assert n == 1
    row = ch_pool.insert.call_args.args[1][0]
    assert row[15] == -0.2  # sentiment_delta
    assert row[19] is True  # deltas_computed


def test_delta_path_fail_silent_on_model_error(monkeypatch, patched_io):
    from internal.extractors import revision_deltas

    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair()])
    fetcher = _Fetcher({
        "https://web.archive.org/curr": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
        "https://web.archive.org/prev": SnapshotFetchResult(status=FETCH_OK, html=PREV_HTML),
    })

    def _boom(*a, **k):
        raise RuntimeError("model exploded")

    monkeypatch.setattr(revision_deltas, "compute_deltas", _boom)
    ch_pool = MagicMock()

    # The diff is still written; only the deltas fall back to uncomputed.
    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10, delta_tools=object())

    assert n == 1
    row = ch_pool.insert.call_args.args[1][0]
    assert row[19] is False  # deltas_computed fell back to default


def test_reconcile_failure_does_not_lose_diffs(monkeypatch, patched_io):
    monkeypatch.setattr(crd, "fetch_undiffed_pairs", lambda pool, n: [_mid_chain_pair()])
    patched_io["write_counts"].side_effect = RuntimeError("count write failed")
    fetcher = _Fetcher({
        "https://web.archive.org/curr": SnapshotFetchResult(status=FETCH_OK, html=CURR_HTML),
        "https://web.archive.org/prev": SnapshotFetchResult(status=FETCH_OK, html=PREV_HTML),
    })
    ch_pool = MagicMock()
    # Reconcile raises, but the diffs are already inserted → still returns 1.
    n = crd.run_revision_diff_sweep(ch_pool, fetcher, MagicMock(), "silver", 10)
    assert n == 1
    ch_pool.insert.assert_called_once()


# --- the async background loop (entrypoint-style guards) ---


def test_loop_disabled_returns_immediately():
    cfg = RevisionDiffConfig(enabled=False)
    stop = asyncio.Event()
    asyncio.run(
        crd.revision_diff_extraction_loop(MagicMock(), MagicMock(), MagicMock(), "silver", cfg, stop)
    )


def test_loop_without_snapshot_fetcher_returns_immediately():
    cfg = RevisionDiffConfig(enabled=True)
    stop = asyncio.Event()
    asyncio.run(
        crd.revision_diff_extraction_loop(MagicMock(), None, MagicMock(), "silver", cfg, stop)
    )

"""Phase 148d (WP-007 §5) — per-source crawl-funnel writer + builder.

The Scrapy spider cannot be imported in the unit environment, so the
funnel row-construction is a pure helper (`build_funnel_record`) covered
here, and the PG write path is exercised against a fake connection pool
(mirrors test_discovery_runs.py).
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from types import SimpleNamespace
from unittest.mock import MagicMock

from internal.state.funnel_runs import (
    FunnelRunRecord,
    FunnelRunsWriter,
    build_funnel_record,
    funnel_counters_from,
)

_STARTED = datetime(2026, 6, 22, 10, 0, tzinfo=timezone.utc)
_DONE = _STARTED + timedelta(seconds=42)


def _fake_pool():
    cursor = MagicMock()
    conn = MagicMock()
    conn.cursor.return_value.__enter__.return_value = cursor
    conn.__enter__.return_value = conn
    pool = MagicMock()
    pool.getconn.return_value = conn
    return pool, conn, cursor


def test_build_funnel_record_maps_all_stages() -> None:
    counters = {
        "discovered": 100,
        "url_filtered": 5,
        "already_collected": 30,
        "fetched": 65,
        "not_modified": 10,
        "content_dropped": 3,
        "thin_content_dropped": 7,
        "submitted": 44,
        "errored": 1,
    }
    rec = build_funnel_record(
        source_id=7,
        counters=counters,
        run_started_at=_STARTED,
        run_completed_at=_DONE,
    )
    assert rec.source_id == 7
    assert rec.discovered == 100
    assert rec.url_filtered == 5
    assert rec.already_collected == 30
    assert rec.fetched == 65
    assert rec.not_modified == 10
    assert rec.content_dropped == 3
    assert rec.thin_content_dropped == 7
    assert rec.submitted == 44
    assert rec.errored == 1


def test_build_funnel_record_missing_keys_default_to_zero() -> None:
    """A spider that never reached a stage records a true zero, not a crash."""
    rec = build_funnel_record(
        source_id=1,
        counters={"discovered": 10, "submitted": 8},
        run_started_at=_STARTED,
        run_completed_at=_DONE,
    )
    assert rec.discovered == 10
    assert rec.submitted == 8
    assert rec.url_filtered == 0
    assert rec.thin_content_dropped == 0
    assert rec.errored == 0


def test_funnel_counters_from_reads_spider_attributes() -> None:
    """The duck-typed extractor pulls the funnel counters off a spider-like
    object; a missing attribute reads as 0 (the closed-handler shim)."""
    spider = SimpleNamespace(
        discovered=50,
        url_filtered=4,
        already_collected=10,
        fetched=36,
        not_modified=2,
        content_dropped=1,
        thin_content_dropped=5,
        submitted=28,
        # `errored` intentionally omitted → defaults to 0
    )
    counters = funnel_counters_from(spider)
    assert counters["discovered"] == 50
    assert counters["thin_content_dropped"] == 5
    assert counters["errored"] == 0
    assert set(counters) == {
        "discovered",
        "url_filtered",
        "already_collected",
        "fetched",
        "not_modified",
        "content_dropped",
        "thin_content_dropped",
        "submitted",
        "errored",
    }


def test_record_run_inserts_one_row() -> None:
    pool, _conn, cursor = _fake_pool()
    writer = FunnelRunsWriter(pool)
    rec = FunnelRunRecord(
        source_id=3,
        discovered=283,
        url_filtered=2,
        already_collected=0,
        fetched=281,
        not_modified=0,
        content_dropped=1,
        thin_content_dropped=3,
        submitted=277,
        errored=0,
        run_started_at=_STARTED,
        run_completed_at=_DONE,
    )
    run_id = writer.record_run(rec)
    assert run_id is not None
    assert cursor.execute.call_count == 1
    sql, params = cursor.execute.call_args[0]
    assert "INSERT INTO crawler_funnel_runs" in sql
    # run_id at [0]; source_id + the nine stage counts follow in order.
    assert params[1:11] == (3, 283, 2, 0, 281, 0, 1, 3, 277, 0)
    assert params[11:13] == (_STARTED, _DONE)

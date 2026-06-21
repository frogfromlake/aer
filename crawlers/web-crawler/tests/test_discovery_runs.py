"""Tests for the Phase-122g discovery-telemetry writer.

Validates:
  * single-row insert + batch insert into ``crawler_discovery_runs``
  * the two-consecutive-runs underflow gate (single below-floor run
    yields a `pending` event; second yields `alerted`; recovery yields
    `recovered`)
  * absence of `expected_floor` is a no-op (sources without a declared
    floor never fire an underflow alert)
  * pool-exception path returns ``None`` cleanly (telemetry failure
    never propagates and degrades the crawl)

The pg connection pool is fully faked — these are pure unit tests, no
real Postgres required.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from unittest.mock import MagicMock

from internal.state.discovery_runs import (
    DiscoveryRunRecord,
    DiscoveryRunsWriter,
)


def _fake_pool() -> tuple[MagicMock, MagicMock, MagicMock]:
    """Build a pool whose getconn returns a connection whose cursor()
    context manager exposes a MagicMock cursor. Returns
    (pool, conn, cursor) so tests can introspect call arguments."""
    pool = MagicMock(name="pool")
    conn = MagicMock(name="conn")
    cursor = MagicMock(name="cursor")
    pool.getconn.return_value = conn
    conn.__enter__.return_value = conn
    conn.__exit__.return_value = False
    conn.cursor.return_value.__enter__.return_value = cursor
    conn.cursor.return_value.__exit__.return_value = False
    return pool, conn, cursor


# --- record_run --------------------------------------------------------------


def test_record_run_inserts_one_row() -> None:
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    started = datetime(2026, 5, 12, 10, 0, tzinfo=timezone.utc)
    rec = DiscoveryRunRecord(
        source_id=1,
        channel="rss",
        urls_discovered=70,
        urls_after_dedup=68,
        run_started_at=started,
        run_completed_at=started + timedelta(seconds=3),
    )
    run_id = writer.record_run(rec)
    assert run_id is not None
    # One INSERT issued.
    assert cursor.execute.call_count == 1
    sql, params = cursor.execute.call_args[0]
    assert "INSERT INTO crawler_discovery_runs" in sql
    assert params[1:7] == (1, "rss", 70, 68, started, started + timedelta(seconds=3))


def test_record_run_batch_groups_per_channel() -> None:
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    started = datetime(2026, 5, 12, 10, 0, tzinfo=timezone.utc)
    records = [
        DiscoveryRunRecord(
            source_id=1,
            channel=ch,
            urls_discovered=n,
            urls_after_dedup=n,
            run_started_at=started,
            run_completed_at=started + timedelta(seconds=1),
        )
        for ch, n in [("sitemap", 0), ("rss", 70), ("html_sitemap", 64), ("archive_index", 250)]
    ]
    ids = writer.record_run_batch(records)
    assert len(ids) == 4
    # executemany single round-trip for all four channels.
    assert cursor.executemany.call_count == 1
    rows_arg = cursor.executemany.call_args[0][1]
    channels = [row[2] for row in rows_arg]
    assert channels == ["sitemap", "rss", "html_sitemap", "archive_index"]


def test_record_run_batch_empty_is_noop() -> None:
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    ids = writer.record_run_batch([])
    assert ids == []
    assert cursor.execute.call_count == 0
    assert cursor.executemany.call_count == 0


# --- evaluate_alerts ---------------------------------------------------------


def test_evaluate_alerts_no_op_when_floor_is_none() -> None:
    """A source without a declared `expected_floor` is not eligible
    for underflow alerting."""
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    event = writer.evaluate_alerts(
        source_id=1,
        expected_floor=None,
        urls_after_dedup_this_run=0,
        run_started_at=datetime(2026, 5, 12, tzinfo=timezone.utc),
    )
    assert event is None
    # No DB traffic at all.
    assert cursor.execute.call_count == 0


def test_evaluate_alerts_first_below_floor_is_pending() -> None:
    """First below-floor run records a `pending` marker without
    firing the alert. The two-consecutive-runs gate prevents
    transient-hiccup false positives."""
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    cursor.fetchone.return_value = None  # no prior alert / pending row

    event = writer.evaluate_alerts(
        source_id=1,
        expected_floor=50,
        urls_after_dedup_this_run=10,
        run_started_at=datetime(2026, 5, 12, tzinfo=timezone.utc),
    )
    assert event == "pending"
    # SELECT then INSERT/UPSERT (2 calls).
    assert cursor.execute.call_count == 2
    upsert_sql = cursor.execute.call_args_list[1][0][0]
    assert "underflow_pending" in upsert_sql


def test_evaluate_alerts_second_consecutive_below_floor_fires() -> None:
    """A second consecutive below-floor run promotes the pending row
    into a fired `underflow` alert."""
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    first_seen = datetime(2026, 5, 10, tzinfo=timezone.utc)
    # SELECT now returns (consecutive_runs, alert_type, first_observed_at).
    cursor.fetchone.return_value = (1, "underflow_pending", first_seen)

    event = writer.evaluate_alerts(
        source_id=1,
        expected_floor=50,
        urls_after_dedup_this_run=8,
        run_started_at=datetime(2026, 5, 12, tzinfo=timezone.utc),
    )
    assert event == "alerted"
    # 3 calls: SELECT, INSERT-or-update alert, DELETE pending.
    assert cursor.execute.call_count == 3
    upsert_sql, upsert_params = cursor.execute.call_args_list[1][0]
    assert "'underflow'" in upsert_sql
    # SEC-080 regression guard: first_observed_at must be bound to the pending
    # row's real timestamp, never the integer run-count (binding the int into
    # the TIMESTAMPTZ column raised a datatype error that the broad except
    # swallowed, silently disabling the underflow alert for every source).
    assert upsert_params[1] == first_seen
    assert isinstance(upsert_params[1], datetime)
    # Third call is the cleanup DELETE of the pending row.
    delete_sql = cursor.execute.call_args_list[2][0][0]
    assert "DELETE" in delete_sql
    assert "underflow_pending" in delete_sql


def test_evaluate_alerts_recovery_clears_prior_alert() -> None:
    """First above-floor run after an alert state clears both the
    underflow row and any lingering pending row."""
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    cursor.fetchone.return_value = (2, "underflow", datetime(2026, 5, 10, tzinfo=timezone.utc))

    event = writer.evaluate_alerts(
        source_id=1,
        expected_floor=50,
        urls_after_dedup_this_run=200,
        run_started_at=datetime(2026, 5, 12, tzinfo=timezone.utc),
    )
    assert event == "recovered"
    # 2 calls: SELECT, DELETE.
    assert cursor.execute.call_count == 2
    delete_sql = cursor.execute.call_args_list[1][0][0]
    assert "DELETE" in delete_sql


def test_evaluate_alerts_above_floor_with_no_prior_is_noop() -> None:
    pool, _conn, cursor = _fake_pool()
    writer = DiscoveryRunsWriter(pool)
    cursor.fetchone.return_value = None

    event = writer.evaluate_alerts(
        source_id=1,
        expected_floor=50,
        urls_after_dedup_this_run=200,
        run_started_at=datetime(2026, 5, 12, tzinfo=timezone.utc),
    )
    assert event is None
    # Only the SELECT — no further writes.
    assert cursor.execute.call_count == 1


def test_evaluate_alerts_failure_returns_none_cleanly() -> None:
    """A Postgres failure (connection error, query timeout, etc.) is
    caught — the alert evaluator never propagates an exception that
    could degrade the crawl path."""
    pool = MagicMock()
    conn = MagicMock()
    pool.getconn.return_value = conn
    conn.__enter__.side_effect = Exception("simulated pg outage")

    writer = DiscoveryRunsWriter(pool)
    event = writer.evaluate_alerts(
        source_id=1,
        expected_floor=50,
        urls_after_dedup_this_run=8,
        run_started_at=datetime(2026, 5, 12, tzinfo=timezone.utc),
    )
    assert event is None
    # Connection was returned to the pool.
    pool.putconn.assert_called_once_with(conn)

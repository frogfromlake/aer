"""Unit tests for the corpus co-occurrence I/O helpers (source list, entity
fetch, co-occurrence insert). ClickHouse + Postgres pools are fully faked.
"""

from __future__ import annotations

from datetime import datetime, timezone
from types import SimpleNamespace
from unittest.mock import MagicMock

from internal.corpus import (
    COOCCURRENCE_COLUMNS,
    CoOccurrenceRow,
    EntityRecord,
    TimeWindow,
    fetch_entities_for_window,
    insert_cooccurrence_rows,
    list_active_sources,
)

UTC = timezone.utc
WINDOW = TimeWindow(start=datetime(2026, 1, 1, tzinfo=UTC), end=datetime(2026, 2, 1, tzinfo=UTC))


def _fake_pg_pool(fetchall_rows: list) -> tuple[MagicMock, MagicMock]:
    pool, conn, cur = MagicMock(name="pg_pool"), MagicMock(), MagicMock()
    pool.getconn.return_value = conn
    conn.cursor.return_value.__enter__.return_value = cur
    conn.cursor.return_value.__exit__.return_value = False
    cur.fetchall.return_value = fetchall_rows
    return pool, cur


def _fake_ch_pool(result_rows: list) -> MagicMock:
    pool, client = MagicMock(name="ch_pool"), MagicMock()
    pool.getconn.return_value = client
    client.query.return_value = SimpleNamespace(result_rows=result_rows)
    return pool


def _cooc_row(window_start: datetime) -> CoOccurrenceRow:
    return CoOccurrenceRow(
        window_start=window_start,
        window_end=datetime(2026, 2, 1, tzinfo=UTC),
        source="src",
        article_id="a1",
        entity_a_text="A",
        entity_a_label="PER",
        entity_b_text="B",
        entity_b_label="LOC",
        cooccurrence_count=2,
    )


# --- list_active_sources -----------------------------------------------------


def test_list_active_sources_returns_names():
    pool, _ = _fake_pg_pool([("franceinfo",), ("tagesschau",)])
    assert list_active_sources(pool) == ["franceinfo", "tagesschau"]
    pool.putconn.assert_called_once()


def test_list_active_sources_empty():
    pool, _ = _fake_pg_pool([])
    assert list_active_sources(pool) == []


# --- fetch_entities_for_window -----------------------------------------------


def test_fetch_entities_builds_records():
    pool = _fake_ch_pool(
        [
            ("a1", "Merkel", "PER", datetime(2026, 5, 12, tzinfo=UTC)),
            ("a1", "Berlin", "LOC", datetime(2026, 5, 12, tzinfo=UTC)),
        ]
    )
    out = fetch_entities_for_window(pool, "src", WINDOW)
    assert [e.entity_text for e in out] == ["Merkel", "Berlin"]
    assert isinstance(out[0], EntityRecord)
    pool.putconn.assert_called_once()


def test_fetch_entities_empty():
    assert fetch_entities_for_window(_fake_ch_pool([]), "src", WINDOW) == []


# --- insert_cooccurrence_rows ------------------------------------------------


def test_insert_cooccurrence_rows_empty_is_noop():
    pool = MagicMock()
    insert_cooccurrence_rows(pool, [], 1)
    pool.insert.assert_not_called()


def test_insert_cooccurrence_rows_keys_token_on_sweep_window():
    pool = MagicMock()
    sweep = TimeWindow(start=datetime(2026, 1, 1, tzinfo=UTC), end=datetime(2026, 2, 1, tzinfo=UTC))
    insert_cooccurrence_rows(pool, [_cooc_row(datetime(2026, 3, 15, tzinfo=UTC))], 999, sweep_window=sweep)
    pool.insert.assert_called_once()
    args, kwargs = pool.insert.call_args
    assert args[0] == "aer_gold.entity_cooccurrences"
    assert kwargs["column_names"] == COOCCURRENCE_COLUMNS
    token = kwargs["deduplication_token"]
    assert "999" in token and "2026-01-01" in token  # sweep window, not the row's


def test_insert_cooccurrence_rows_falls_back_to_row_window():
    pool = MagicMock()
    insert_cooccurrence_rows(pool, [_cooc_row(datetime(2026, 3, 15, tzinfo=UTC))], 5)
    _, kwargs = pool.insert.call_args
    assert "2026-03-15" in kwargs["deduplication_token"]

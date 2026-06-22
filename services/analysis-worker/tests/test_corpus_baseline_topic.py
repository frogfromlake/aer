"""Unit tests for the topic-sweep I/O helpers (window fetch + assignment insert).

ClickHouse pool + MinIO client are fully faked — no real infrastructure.
"""

from __future__ import annotations

import asyncio
import json
from datetime import datetime, timezone
from types import SimpleNamespace
from unittest.mock import MagicMock

from internal.corpus import BaselineConfig
from internal.corpus_baseline_topic import (
    TOPIC_ASSIGNMENT_COLUMNS,
    DocumentRecord,
    TimeWindow,
    TopicAssignmentRow,
    baseline_extraction_loop,
    fetch_silver_documents_for_window,
    insert_topic_assignment_rows,
)

UTC = timezone.utc
WINDOW = TimeWindow(start=datetime(2026, 1, 1, tzinfo=UTC), end=datetime(2026, 2, 1, tzinfo=UTC))


def _fake_ch_pool(index_rows: list) -> MagicMock:
    pool = MagicMock(name="ch_pool")
    client = MagicMock(name="ch_client")
    pool.getconn.return_value = client
    client.query.return_value = SimpleNamespace(result_rows=index_rows)
    return pool


def _minio_returning(text_by_key: dict[str, str]) -> MagicMock:
    minio = MagicMock()

    def get_object(_bucket, key):
        resp = MagicMock()
        resp.read.return_value = json.dumps({"core": {"cleaned_text": text_by_key[key]}}).encode("utf-8")
        return resp

    minio.get_object.side_effect = get_object
    return minio


def test_fetch_silver_documents_builds_records():
    pool = _fake_ch_pool([("a1", "src", "de", "key1"), ("a2", "src", "fr", "key2")])
    minio = _minio_returning({"key1": "Text one", "key2": "Text two"})
    docs = fetch_silver_documents_for_window(pool, minio, "silver", WINDOW)
    assert [d.article_id for d in docs] == ["a1", "a2"]
    assert docs[0].cleaned_text == "Text one"
    assert isinstance(docs[0], DocumentRecord)


def test_fetch_silver_documents_defaults_language_to_und():
    pool = _fake_ch_pool([("a1", "src", "", "key1")])
    minio = _minio_returning({"key1": "Body"})
    docs = fetch_silver_documents_for_window(pool, minio, "silver", WINDOW)
    assert docs[0].language == "und"


def test_fetch_silver_documents_skips_minio_failures():
    pool = _fake_ch_pool([("a1", "src", "de", "key1")])
    minio = MagicMock()
    minio.get_object.side_effect = RuntimeError("minio down")
    assert fetch_silver_documents_for_window(pool, minio, "silver", WINDOW) == []


def test_fetch_silver_documents_skips_empty_text():
    pool = _fake_ch_pool([("a1", "src", "de", "key1")])
    minio = _minio_returning({"key1": ""})
    assert fetch_silver_documents_for_window(pool, minio, "silver", WINDOW) == []


def test_insert_topic_assignment_rows_empty_is_noop():
    pool = MagicMock()
    insert_topic_assignment_rows(pool, [], 123)
    pool.insert.assert_not_called()


def test_insert_topic_assignment_rows_inserts_with_dedup_token():
    pool = MagicMock()
    row = TopicAssignmentRow(
        window_start=datetime(2026, 1, 1, tzinfo=UTC),
        window_end=datetime(2026, 2, 1, tzinfo=UTC),
        source="src",
        article_id="a1",
        language="de",
        topic_id=3,
        topic_label="Politik",
        topic_confidence=0.8,
        model_hash="h",
    )
    insert_topic_assignment_rows(pool, [row], 999)
    pool.insert.assert_called_once()
    args, kwargs = pool.insert.call_args
    assert args[0] == "aer_gold.topic_assignments"
    assert kwargs["column_names"] == TOPIC_ASSIGNMENT_COLUMNS
    assert "999" in kwargs["deduplication_token"]


def test_extraction_loop_serialises_on_shared_lock(monkeypatch):
    """Phase 148c — a heavy corpus sweep cannot start while another holds the
    shared ``extraction_lock``. Proves the mutex that stops co-occurrence /
    topic / baseline / revision-diff from saturating CPU+RAM concurrently.

    Deterministic (not timing-dependent): while the test holds the lock the loop
    physically cannot enter the guarded ``to_thread`` block, so the sweep is
    never dispatched; it runs only once the lock is released.
    """
    calls: list[int] = []

    def fake_sweep(_ch_pool, _extractor, _window):
        calls.append(1)
        return SimpleNamespace(rows_written=0)

    # The baseline loop imports _run_baseline_sweep into its own module namespace.
    import internal.corpus_baseline_topic as cbt

    monkeypatch.setattr(cbt, "_run_baseline_sweep", fake_sweep)

    async def run() -> None:
        lock = asyncio.Lock()
        cfg = BaselineConfig(
            enabled=True,
            interval_seconds=100.0,
            window_seconds=1.0,
            initial_delay_seconds=0.0,
        )
        stop = asyncio.Event()
        extractor = SimpleNamespace(name="metric_baseline")

        await lock.acquire()  # simulate another heavy sweep holding the mutex
        task = asyncio.create_task(
            baseline_extraction_loop(object(), extractor, cfg, stop, extraction_lock=lock)
        )
        await asyncio.sleep(0.1)  # give the loop time to reach the lock
        assert calls == []  # blocked — the sweep cannot run while the lock is held

        lock.release()
        for _ in range(100):  # let the now-unblocked sweep run
            if calls:
                break
            await asyncio.sleep(0.02)
        assert calls == [1]  # ran exactly once the lock was free

        stop.set()
        await asyncio.wait_for(task, timeout=2.0)

    asyncio.run(run())

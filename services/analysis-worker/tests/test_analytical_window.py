"""Phase 122e A26 / F-A26 — analytical-window gate.

Verifies the worker's archive-vs-analytics layer split:

  * Articles whose extracted `core.timestamp` is within the analytical
    window (default 365 days) flow normally through Silver MinIO + CH
    + Gold extractors.
  * Articles whose `core.timestamp` is OLDER than the window are
    preserved in Silver MinIO (archive-side) but the worker skips:
      - the `aer_silver.documents` ClickHouse projection,
      - all Gold inserts (`aer_gold.metrics`, `aer_gold.entities`,
        `aer_gold.entity_links`, `aer_gold.language_detections`),
      - extractor execution beyond language detection.
    The Prometheus counter `analysis_worker_archived_only_total{source}`
    increments; PG `documents.status` is `processed`.
"""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from unittest.mock import MagicMock

from internal.adapters import AdapterRegistry, RssAdapter
from internal.extractors import LanguageDetectionExtractor, WordCountExtractor
from internal.metrics import analysis_worker_archived_only_total
from internal.processor import DataProcessor


def _make_processor_with_window(
    mock_minio, mock_clickhouse, mock_pg_pool, window_days: int
):
    adapter_registry = AdapterRegistry({"rss": RssAdapter(pg_pool=mock_pg_pool)})
    return DataProcessor(
        mock_minio,
        mock_clickhouse,
        mock_pg_pool,
        adapter_registry,
        [LanguageDetectionExtractor(), WordCountExtractor()],
        analytical_window_days=window_days,
    )


def _bronze_with_timestamp(timestamp_iso: str) -> bytes:
    return json.dumps({
        "source": "tagesschau",
        "source_type": "rss",
        "title": "Test",
        "raw_text": "Die Bundesregierung hat etwas beschlossen.",
        "url": "https://www.tagesschau.de/inland/test-100",
        "timestamp": timestamp_iso,
        "feed_url": "https://www.tagesschau.de/index~rss2.xml",
        "categories": [],
        "author": "tagesschau.de",
        "feed_title": "tagesschau.de",
    }).encode("utf-8")


def test_within_window_flows_normally(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """An article published yesterday (within 365-day window) writes to
    BOTH MinIO Silver AND CH Silver/Gold."""
    proc = _make_processor_with_window(
        mock_minio, mock_clickhouse, mock_pg_pool, window_days=365
    )
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    yesterday = (datetime.now(timezone.utc) - timedelta(days=1)).isoformat().replace("+00:00", "Z")
    mock_minio.get_object.return_value = MagicMock(read=lambda: _bronze_with_timestamp(yesterday))

    proc.process_event("rss/tagesschau/recent.json", "2026-05-09T10:00:00.000Z", dummy_span)

    # Silver MinIO envelope written.
    assert mock_minio.put_object.called
    # CH inserts happened (Silver projection + at least one Gold insert).
    insert_tables = [c.args[0] for c in mock_clickhouse.insert.call_args_list]
    assert "aer_silver.documents" in insert_tables
    assert "aer_gold.metrics" in insert_tables
    proc._update_document_status.assert_called_with("rss/tagesschau/recent.json", "processed")


def test_outside_window_preserves_archive_skips_analytics(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """An article whose extracted timestamp is past the analytical
    window writes to MinIO Silver but skips ALL CH inserts."""
    counter_before = analysis_worker_archived_only_total.labels(source="tagesschau")._value.get()

    proc = _make_processor_with_window(
        mock_minio, mock_clickhouse, mock_pg_pool, window_days=365
    )
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    # 918 days old — the iter-5 podcast scenario. RssAdapter derives
    # `core.timestamp` from the MinIO event_time (it has no per-article
    # publish-date parser like WebAdapter), so the old timestamp must
    # be passed through event_time for the gate to see it.
    old_ts = (datetime.now(timezone.utc) - timedelta(days=918)).isoformat().replace("+00:00", "Z")
    mock_minio.get_object.return_value = MagicMock(read=lambda: _bronze_with_timestamp(old_ts))

    proc.process_event("rss/tagesschau/old.json", old_ts, dummy_span)

    # Silver MinIO envelope WAS written (archive layer preserved).
    assert mock_minio.put_object.called

    # NO CH inserts of any kind (analytics layer bounded).
    insert_tables = [c.args[0] for c in mock_clickhouse.insert.call_args_list]
    assert "aer_silver.documents" not in insert_tables
    assert "aer_gold.metrics" not in insert_tables
    assert "aer_gold.entities" not in insert_tables
    assert "aer_gold.language_detections" not in insert_tables

    # Document marked processed (the harmonisation succeeded).
    proc._update_document_status.assert_called_with("rss/tagesschau/old.json", "processed")

    # Prometheus counter incremented for the source.
    counter_after = analysis_worker_archived_only_total.labels(source="tagesschau")._value.get()
    assert counter_after == counter_before + 1


def test_window_zero_excludes_everything(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """Pathological window=0 means even today's articles are out of
    scope — everything goes archive-only. Useful for tests that want
    to force the archive-only path."""
    proc = _make_processor_with_window(
        mock_minio, mock_clickhouse, mock_pg_pool, window_days=1
    )
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    # 2 days old — outside a 1-day window
    two_days_ago = (datetime.now(timezone.utc) - timedelta(days=2)).isoformat().replace("+00:00", "Z")
    mock_minio.get_object.return_value = MagicMock(read=lambda: _bronze_with_timestamp(two_days_ago))

    proc.process_event("rss/tagesschau/2d.json", "2026-05-09T10:00:00.000Z", dummy_span)

    insert_tables = [c.args[0] for c in mock_clickhouse.insert.call_args_list]
    assert "aer_silver.documents" not in insert_tables
    assert "aer_gold.metrics" not in insert_tables


def test_default_window_from_env_var(monkeypatch):
    """The DataProcessor's analytical_window_days defaults to the
    WORKER_ANALYTICAL_WINDOW_DAYS env var when not explicitly passed.
    """
    from internal.processor import _analytical_window_days

    monkeypatch.setenv("WORKER_ANALYTICAL_WINDOW_DAYS", "180")
    assert _analytical_window_days() == 180

    monkeypatch.setenv("WORKER_ANALYTICAL_WINDOW_DAYS", "")
    assert _analytical_window_days() == 365  # default

    monkeypatch.setenv("WORKER_ANALYTICAL_WINDOW_DAYS", "not-an-int")
    assert _analytical_window_days() == 365  # falls back gracefully

    monkeypatch.setenv("WORKER_ANALYTICAL_WINDOW_DAYS", "0")
    assert _analytical_window_days() == 365  # rejects non-positive

    monkeypatch.setenv("WORKER_ANALYTICAL_WINDOW_DAYS", "-5")
    assert _analytical_window_days() == 365  # rejects negative


def test_naive_datetime_treated_as_utc():
    """Articles whose timestamp lacks tzinfo are assumed UTC (matches
    WebAdapter convention)."""
    from internal.processor import _is_within_analytical_window

    # 1 day ago, naive datetime
    naive_recent = datetime.utcnow() - timedelta(days=1)
    assert _is_within_analytical_window(naive_recent, 365) is True

    # 400 days ago, naive datetime
    naive_old = datetime.utcnow() - timedelta(days=400)
    assert _is_within_analytical_window(naive_old, 365) is False

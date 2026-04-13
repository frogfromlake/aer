"""Phase 74 — Worker Idempotency dedup gate.

Integration test with a real ClickHouse container that verifies
ReplacingMergeTree collapses duplicate rows produced by NATS redelivery
after a partial success. Processes the same Bronze event twice through
a real DataProcessor with a real ClickHouse pool, runs ``OPTIMIZE TABLE
... FINAL`` on each Gold fact table, and asserts that exactly one row
survives per ``(article_id, metric_name)`` key.
"""

import time
import urllib.error
import urllib.request
from pathlib import Path
from unittest.mock import MagicMock

import pytest
import yaml
from testcontainers.core.container import DockerContainer

from conftest import DUMMY_EVENT_TIME, VALID_RSS_BRONZE_DATA
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter
from internal.extractors import (
    LanguageDetectionExtractor,
    WordCountExtractor,
)
from internal.processor import DataProcessor
from internal.storage import init_clickhouse


def _get_compose_image(service_name: str) -> str:
    compose_path = Path(__file__).resolve().parents[3] / "compose.yaml"
    with open(compose_path, "r", encoding="utf-8") as f:
        compose = yaml.safe_load(f)
    return compose["services"][service_name]["image"]


@pytest.fixture(scope="module")
def ch_dedup_container():
    container = (
        DockerContainer(_get_compose_image("clickhouse"))
        .with_env("CLICKHOUSE_USER", "aer_admin")
        .with_env("CLICKHOUSE_PASSWORD", "aer_secret")
        .with_env("CLICKHOUSE_DB", "aer_gold")
        .with_exposed_ports(8123)
    )
    with container:
        start = time.time()
        ready = False
        while time.time() - start < 60:
            try:
                host = container.get_container_host_ip()
                port = container.get_exposed_port(8123)
                resp = urllib.request.urlopen(f"http://{host}:{port}/ping", timeout=1)
                if resp.getcode() == 200:
                    ready = True
                    break
            except (urllib.error.URLError, ConnectionError, TimeoutError):
                time.sleep(1)
        if not ready:
            raise TimeoutError("ClickHouse container did not become ready within 60s.")
        yield container


@pytest.fixture
def ch_pool(ch_dedup_container, monkeypatch):
    monkeypatch.setenv("CLICKHOUSE_HOST", ch_dedup_container.get_container_host_ip())
    monkeypatch.setenv("CLICKHOUSE_PORT", str(ch_dedup_container.get_exposed_port(8123)))
    monkeypatch.setenv("CLICKHOUSE_USER", "aer_admin")
    monkeypatch.setenv("CLICKHOUSE_PASSWORD", "aer_secret")
    monkeypatch.setenv("CLICKHOUSE_DB", "aer_gold")

    pool = init_clickhouse(pool_size=2)
    client = pool.getconn()
    try:
        # Phase 74 schema — mirrors infra/clickhouse/migrations/000010_*.sql.
        # We intentionally (re)create here so the test is self-contained and
        # does not depend on the migration runner image.
        client.command("DROP TABLE IF EXISTS aer_gold.metrics")
        client.command("DROP TABLE IF EXISTS aer_gold.entities")
        client.command("DROP TABLE IF EXISTS aer_gold.language_detections")
        client.command(
            """
            CREATE TABLE aer_gold.metrics (
                timestamp DateTime,
                value Float64,
                source String DEFAULT '',
                metric_name String DEFAULT '',
                article_id Nullable(String),
                discourse_function String DEFAULT '',
                ingestion_version UInt64 DEFAULT 0
            )
            ENGINE = ReplacingMergeTree(ingestion_version)
            ORDER BY (article_id, metric_name)
            SETTINGS allow_nullable_key = 1
            """
        )
        client.command(
            """
            CREATE TABLE aer_gold.entities (
                timestamp DateTime,
                source String,
                article_id Nullable(String),
                entity_text String,
                entity_label String,
                start_char UInt32,
                end_char UInt32,
                discourse_function String DEFAULT '',
                ingestion_version UInt64 DEFAULT 0
            )
            ENGINE = ReplacingMergeTree(ingestion_version)
            ORDER BY (article_id, entity_label, start_char, end_char)
            SETTINGS allow_nullable_key = 1
            """
        )
        client.command(
            """
            CREATE TABLE aer_gold.language_detections (
                timestamp DateTime,
                source String,
                article_id Nullable(String),
                detected_language String,
                confidence Float64,
                rank UInt8,
                ingestion_version UInt64 DEFAULT 0
            )
            ENGINE = ReplacingMergeTree(ingestion_version)
            ORDER BY (article_id, rank)
            SETTINGS allow_nullable_key = 1
            """
        )
    finally:
        pool.putconn(client)

    yield pool


def _build_processor(ch_pool, mock_minio, mock_pg_pool):
    # NER excluded (requires spaCy model at runtime — skip for speed).
    extractors = [WordCountExtractor(), LanguageDetectionExtractor()]
    registry = AdapterRegistry(
        {"legacy": LegacyAdapter(), "rss": RssAdapter()}
    )
    processor = DataProcessor(
        mock_minio, ch_pool, mock_pg_pool, registry, extractors
    )
    # Bypass PostgreSQL idempotency gate — we want both deliveries to reach
    # the ClickHouse inserts so ReplacingMergeTree does the deduplication.
    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()
    return processor


def test_replacing_merge_tree_collapses_redelivered_event(
    ch_pool, mock_minio, mock_pg_pool, dummy_span
):
    processor = _build_processor(ch_pool, mock_minio, mock_pg_pool)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    obj_key = "rss/tagesschau/dedup/2023-10-25.json"

    # Two identical deliveries of the same NATS event.
    processor.process_event(obj_key, DUMMY_EVENT_TIME, dummy_span)
    processor.process_event(obj_key, DUMMY_EVENT_TIME, dummy_span)

    client = ch_pool.getconn()
    try:
        # Pre-OPTIMIZE: duplicates exist in raw parts.
        raw_metrics = client.query(
            "SELECT count() FROM aer_gold.metrics"
        ).result_rows[0][0]
        assert raw_metrics >= 2, "Expected duplicate inserts before OPTIMIZE FINAL"

        client.command("OPTIMIZE TABLE aer_gold.metrics FINAL")
        client.command("OPTIMIZE TABLE aer_gold.language_detections FINAL")

        # Post-OPTIMIZE: exactly one row per (article_id, metric_name).
        dupes = client.query(
            """
            SELECT article_id, metric_name, count() AS c
            FROM aer_gold.metrics FINAL
            GROUP BY article_id, metric_name
            HAVING c > 1
            """
        ).result_rows
        assert dupes == [], f"Expected no duplicates after dedup, got {dupes}"

        total_metrics = client.query(
            "SELECT count() FROM aer_gold.metrics FINAL"
        ).result_rows[0][0]
        assert total_metrics >= 1

        lang_dupes = client.query(
            """
            SELECT article_id, rank, count() AS c
            FROM aer_gold.language_detections FINAL
            GROUP BY article_id, rank
            HAVING c > 1
            """
        ).result_rows
        assert lang_dupes == []
    finally:
        ch_pool.putconn(client)

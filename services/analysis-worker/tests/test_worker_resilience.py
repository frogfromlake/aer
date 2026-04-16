"""Phase 91 tests — Worker Resilience: timeouts, thread safety, partial failure.

Covers:
- ClickHousePool.getconn() raises TimeoutError when the pool is exhausted.
- RssAdapter classification cache is safe under concurrent access.
- DataProcessor marks documents as "processed" even on partial Gold insert failure.
"""

import threading
from unittest.mock import MagicMock, patch

from internal.storage.clickhouse_client import ClickHousePool
from internal.adapters.rss import RssAdapter
from conftest import VALID_RSS_BRONZE_DATA, _make_processor


# ---------------------------------------------------------------------------
# ClickHouse pool timeout
# ---------------------------------------------------------------------------

class TestClickHousePoolTimeout:
    def test_getconn_raises_timeout_when_pool_exhausted(self):
        """A pool with zero available clients must raise TimeoutError promptly."""
        with patch.dict("os.environ", {"CLICKHOUSE_POOL_TIMEOUT_SECONDS": "1"}):
            with patch(
                "internal.storage.clickhouse_client.clickhouse_connect.get_client",
                return_value=MagicMock(),
            ):
                pool = ClickHousePool(size=1)

        # Drain the single client so the pool is empty.
        pool.getconn()

        try:
            pool.getconn()
            assert False, "Expected TimeoutError"
        except TimeoutError as e:
            assert "exhausted" in str(e)

    def test_getconn_succeeds_when_client_returned_in_time(self):
        """If a client is returned before the timeout, getconn() must succeed."""
        with patch.dict("os.environ", {"CLICKHOUSE_POOL_TIMEOUT_SECONDS": "5"}):
            with patch(
                "internal.storage.clickhouse_client.clickhouse_connect.get_client",
                return_value=MagicMock(),
            ):
                pool = ClickHousePool(size=1)

        client = pool.getconn()
        # Return the client from another thread after a short delay.
        timer = threading.Timer(0.1, pool.putconn, args=(client,))
        timer.start()

        recovered = pool.getconn()
        assert recovered is client
        timer.join()


# ---------------------------------------------------------------------------
# RssAdapter cache thread safety
# ---------------------------------------------------------------------------

class TestRssAdapterCacheThreadSafety:
    def test_concurrent_cache_access_no_corruption(self):
        """Spawn N threads calling _get_classification_cached simultaneously.
        No RuntimeError or lost entries should occur."""
        mock_pool = MagicMock()
        sources = [f"source_{i}" for i in range(20)]
        errors: list[Exception] = []

        with patch("internal.adapters.rss.get_source_classification") as mock_get:
            mock_get.return_value = {
                "primary_function": "epistemic_authority",
                "secondary_function": None,
                "emic_designation": "test",
            }
            adapter = RssAdapter(pg_pool=mock_pool)

            def worker(source: str):
                try:
                    for _ in range(50):
                        adapter._get_classification_cached(source)
                except Exception as e:
                    errors.append(e)

            threads = [threading.Thread(target=worker, args=(s,)) for s in sources]
            for t in threads:
                t.start()
            for t in threads:
                t.join()

        assert errors == [], f"Cache access raised errors: {errors}"
        # All sources should be in the cache.
        assert len(adapter._classification_cache) == len(sources)


# ---------------------------------------------------------------------------
# Processor partial Gold insert failure
# ---------------------------------------------------------------------------

class TestProcessorPartialInsertFailure:
    def test_document_marked_processed_on_partial_insert_failure(
        self, mock_minio, mock_pg_pool, adapter_registry, dummy_span
    ):
        """If the entities insert fails, the document must still be marked
        'processed' — not left in a state that triggers wasteful redelivery."""
        mock_ch = MagicMock()
        call_count = {"n": 0}

        def insert_side_effect(table, rows, column_names):
            call_count["n"] += 1
            if table == "aer_gold.entities":
                raise Exception("ClickHouse write timeout (simulated)")

        mock_ch.insert.side_effect = insert_side_effect

        from internal.extractors import WordCountExtractor
        from internal.extractors.base import ExtractionResult, GoldEntity

        class EntityProducingExtractor:
            @property
            def name(self) -> str:
                return "entity_stub"

            def extract_all(self, core, article_id):
                return ExtractionResult(
                    metrics=[],
                    entities=[
                        GoldEntity(
                            timestamp=core.timestamp,
                            source=core.source,
                            article_id=article_id,
                            entity_text="Berlin",
                            entity_label="LOC",
                            start_char=0,
                            end_char=6,
                        )
                    ],
                )

        proc = _make_processor(
            mock_minio, mock_ch, mock_pg_pool, adapter_registry,
            [WordCountExtractor(), EntityProducingExtractor()],
        )

        mock_response = MagicMock()
        mock_response.read.return_value = VALID_RSS_BRONZE_DATA
        mock_minio.get_object.return_value = mock_response
        proc._get_document_status = MagicMock(return_value=None)
        proc._update_document_status = MagicMock()

        obj_key = "rss/tagesschau/abc123/2026-04-05.json"
        proc.process_event(obj_key, "2026-04-05T10:00:00.000Z", dummy_span)

        # The document must be marked processed despite the entities insert failure.
        proc._update_document_status.assert_called_once_with(obj_key, "processed")

    def test_document_marked_processed_when_all_inserts_fail(
        self, mock_minio, mock_pg_pool, adapter_registry, dummy_span
    ):
        """Even if every ClickHouse insert fails, the document must still be
        marked processed to prevent wasteful reprocessing."""
        mock_ch = MagicMock()
        mock_ch.insert.side_effect = Exception("ClickHouse down (simulated)")

        from internal.extractors import WordCountExtractor

        proc = _make_processor(
            mock_minio, mock_ch, mock_pg_pool, adapter_registry,
            [WordCountExtractor()],
        )

        mock_response = MagicMock()
        mock_response.read.return_value = VALID_RSS_BRONZE_DATA
        mock_minio.get_object.return_value = mock_response
        proc._get_document_status = MagicMock(return_value=None)
        proc._update_document_status = MagicMock()

        obj_key = "rss/tagesschau/abc123/2026-04-05.json"
        proc.process_event(obj_key, "2026-04-05T10:00:00.000Z", dummy_span)

        proc._update_document_status.assert_called_once_with(obj_key, "processed")

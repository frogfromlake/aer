"""Phase 103b: Silver projection (`aer_silver.documents`) write path."""

from datetime import datetime, timezone
from unittest.mock import MagicMock

from internal.silver_projection import (
    SILVER_DOCS_TABLE,
    raw_entity_count,
    upload_silver_projection,
)


class _Core:
    def __init__(self, **kw):
        self.document_id = kw.get("document_id", "doc-1")
        self.source = kw.get("source", "tagesschau")
        self.timestamp = kw.get("timestamp", datetime(2026, 4, 24, 10, 0, tzinfo=timezone.utc))
        self.language = kw.get("language", "de")
        self.cleaned_text = kw.get("cleaned_text", "Hallo Welt aus Berlin.")
        self.word_count = kw.get("word_count", 4)


def test_raw_entity_count_picks_capitalized_tokens():
    # Counts each capitalized run as one token; sentence-initial words are
    # not filtered (undercounting matters more than the false positive on
    # the first word — see silver_projection module docstring).
    assert raw_entity_count("Hallo Welt aus Berlin.") == 3
    assert raw_entity_count("Der Berliner Senat tagte heute.") == 3
    assert raw_entity_count("") == 0
    assert raw_entity_count("nichts großgeschrieben hier") == 0


def test_upload_silver_projection_inserts_one_row_with_expected_columns():
    ch = MagicMock()
    core = _Core(cleaned_text="Hallo Welt aus Berlin.")
    upload_silver_projection(ch, core, ingestion_version=1234567890)

    ch.insert.assert_called_once()
    args, kwargs = ch.insert.call_args
    assert args[0] == SILVER_DOCS_TABLE
    rows = args[1]
    assert len(rows) == 1
    row = rows[0]
    column_names = kwargs["column_names"]
    assert column_names == [
        "timestamp",
        "source",
        "article_id",
        "language",
        "cleaned_text_length",
        "word_count",
        "raw_entity_count",
        "ingestion_version",
    ]
    # Spot-check derived fields.
    by_name = dict(zip(column_names, row))
    assert by_name["source"] == "tagesschau"
    assert by_name["article_id"] == "doc-1"
    assert by_name["language"] == "de"
    assert by_name["cleaned_text_length"] == len("Hallo Welt aus Berlin.")
    assert by_name["word_count"] == 4
    assert by_name["raw_entity_count"] >= 2
    assert by_name["ingestion_version"] == 1234567890


def test_upload_silver_projection_swallows_clickhouse_errors():
    ch = MagicMock()
    ch.insert.side_effect = RuntimeError("ch down")
    # Must not propagate — projection failures are non-fatal per Phase 103b.
    upload_silver_projection(ch, _Core(), ingestion_version=1)


def test_upload_silver_projection_handles_missing_language():
    ch = MagicMock()
    core = _Core(language="")
    upload_silver_projection(ch, core, ingestion_version=1)
    row = ch.insert.call_args[0][1][0]
    assert row[3] == "und"

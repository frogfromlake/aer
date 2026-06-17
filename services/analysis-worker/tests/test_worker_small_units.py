"""Coverage for small worker units that were previously untested:
``metadata_coverage`` row-building + upload, ``ProbeLanguageScope`` config
loading + scope checks, and ``configure_logging``.
"""

from __future__ import annotations

from datetime import datetime, timezone
from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from internal.adapters.web_meta import WebMeta
from internal.metadata_coverage import (
    NULL_METHOD,
    build_coverage_rows,
    is_extraction_present,
    upload_metadata_coverage,
)
from internal.models.probe_scope import ProbeLanguageScope
import internal.logging as worker_logging


# --- metadata_coverage ------------------------------------------------------


def test_build_coverage_rows_normalises_methods():
    rows = build_coverage_rows(
        source="tagesschau",
        article_id="a1",
        extraction_methods={"section": "json_ld", "author": None, "tags": "made_up"},
        ingestion_version=7,
        ingestion_at=datetime(2025, 1, 1, tzinfo=timezone.utc),
        fields=["section", "author", "tags"],
    )
    by_field = {r[2]: r[3] for r in rows}
    assert by_field["section"] == "json_ld"  # in-vocabulary → kept
    assert by_field["author"] == NULL_METHOD  # None → null
    assert by_field["tags"] == NULL_METHOD  # out-of-vocabulary → null
    # Column order mirrors the migration: source, article_id, field, method, …
    assert rows[0][0] == "tagesschau" and rows[0][1] == "a1"
    assert rows[0][4] == 7


def test_is_extraction_present():
    methods = {"section": "json_ld", "author": None, "tags": "bogus"}
    assert is_extraction_present(methods, "section") is True
    assert is_extraction_present(methods, "author") is False
    assert is_extraction_present(methods, "tags") is False
    assert is_extraction_present(methods, "absent") is False


def test_upload_metadata_coverage_web_meta_inserts():
    ch = MagicMock()
    meta = WebMeta(source_type="web", extraction_methods={"section": "json_ld"})
    core = SimpleNamespace(source="tagesschau", document_id="a1")
    upload_metadata_coverage(ch, core, meta, ingestion_version=3, ingestion_at=datetime(2025, 1, 1, tzinfo=timezone.utc))
    ch.insert.assert_called_once()
    assert ch.insert.call_args.args[0] == "aer_gold.metadata_coverage_raw"


def test_upload_metadata_coverage_non_web_is_noop():
    ch = MagicMock()
    meta = SimpleNamespace()  # not a WebMeta
    core = SimpleNamespace(source="x", document_id="a1")
    upload_metadata_coverage(ch, core, meta, ingestion_version=1, ingestion_at=datetime(2025, 1, 1, tzinfo=timezone.utc))
    ch.insert.assert_not_called()


def test_upload_metadata_coverage_insert_failure_is_swallowed():
    ch = MagicMock()
    ch.insert.side_effect = RuntimeError("clickhouse down")
    meta = WebMeta(source_type="web", extraction_methods={"section": "json_ld"})
    core = SimpleNamespace(source="tagesschau", document_id="a1")
    # Must not raise — coverage is best-effort, never blocks the Silver write.
    upload_metadata_coverage(ch, core, meta, ingestion_version=1, ingestion_at=datetime(2025, 1, 1, tzinfo=timezone.utc))


# --- ProbeLanguageScope -----------------------------------------------------


def write_scope_yaml(tmp_path, body):
    p = tmp_path / "probe_language_scope.yaml"
    p.write_text(body, encoding="utf-8")
    return p


def test_probe_scope_load_and_checks(tmp_path):
    path = write_scope_yaml(tmp_path, "sources:\n  tagesschau: [de]\n  franceinfo: [fr, en]\n")
    scope = ProbeLanguageScope.load(path)
    # In-scope / out-of-scope language gating.
    assert scope.is_in_scope("tagesschau", "de") is True
    assert scope.is_in_scope("tagesschau", "fr") is False
    # Unknown source → no restriction.
    assert scope.is_in_scope("unknown", "xx") is True
    # Empty / undetermined language is never filtered.
    assert scope.is_in_scope("tagesschau", "und") is True
    assert scope.is_in_scope("tagesschau", "") is True
    assert scope.allowed_languages("franceinfo") == ["fr", "en"]
    assert scope.allowed_languages("unknown") is None
    assert set(scope.sources_with_scope()) == {"tagesschau", "franceinfo"}


def test_probe_scope_load_empty_file(tmp_path):
    path = write_scope_yaml(tmp_path, "")
    scope = ProbeLanguageScope.load(path)
    assert list(scope.sources_with_scope()) == []


def test_probe_scope_rejects_non_mapping_sources(tmp_path):
    path = write_scope_yaml(tmp_path, "sources: [tagesschau, franceinfo]\n")
    with pytest.raises(ValueError, match="must be a mapping"):
        ProbeLanguageScope.load(path)


def test_probe_scope_rejects_non_list_langs(tmp_path):
    path = write_scope_yaml(tmp_path, "sources:\n  tagesschau: de\n")
    with pytest.raises(ValueError, match="must be a list"):
        ProbeLanguageScope.load(path)


# --- configure_logging ------------------------------------------------------


def test_configure_logging_production_uses_json_renderer():
    import structlog

    worker_logging.configure_logging(environment="production", level="INFO")
    procs = structlog.get_config()["processors"]
    # Production/staging must render JSON (machine-ingestible logs), and the
    # trace-id correlator must be in the chain.
    assert isinstance(procs[-1], structlog.processors.JSONRenderer)
    assert worker_logging.add_trace_id in procs


def test_configure_logging_development_uses_console_renderer():
    import structlog

    worker_logging.configure_logging(environment="development", level="DEBUG")
    procs = structlog.get_config()["processors"]
    # Non-prod renders the human-readable console format.
    assert isinstance(procs[-1], structlog.dev.ConsoleRenderer)
    assert not isinstance(procs[-1], structlog.processors.JSONRenderer)


def test_configure_logging_restores_json_for_staging():
    # Re-configuring is idempotent and the prod/staging branch (not just
    # 'production') selects JSON — guards the env-string membership check.
    import structlog

    worker_logging.configure_logging(environment="development")
    worker_logging.configure_logging(environment="staging")
    assert isinstance(structlog.get_config()["processors"][-1], structlog.processors.JSONRenderer)


# --- enrichment re-attempt sweep + loop guards ------------------------------


def test_run_reattempt_sweep_collects_per_task_summaries():
    from collections import Counter

    from internal.reattempt import run_reattempt_sweep

    ok_task = SimpleNamespace(name="entity_linking", run=lambda limit: Counter({"reprocessed": 3}))

    def _boom(limit):
        raise RuntimeError("task exploded")

    bad_task = SimpleNamespace(name="topics", run=_boom)

    summary = run_reattempt_sweep([ok_task, bad_task], batch_limit=100)
    assert summary["entity_linking"] == Counter({"reprocessed": 3})
    # A failing task is contained and reported, not propagated.
    assert summary["topics"] == Counter({"task_failed": 1})


def test_reattempt_loop_disabled_returns():
    import asyncio

    from internal.reattempt import ReAttemptConfig, enrichment_reattempt_loop

    cfg = ReAttemptConfig(enabled=False)
    asyncio.run(enrichment_reattempt_loop([], asyncio.Event(), cfg))


def test_reattempt_loop_no_tasks_returns():
    import asyncio

    from internal.reattempt import ReAttemptConfig, enrichment_reattempt_loop

    cfg = ReAttemptConfig(enabled=True)
    asyncio.run(enrichment_reattempt_loop([], asyncio.Event(), cfg))

"""Migration 000025 silent-failure guard — `aer_gold.wayback_lookups`.

These tests pin the invariant that makes the bundesregierung/elysee regression
impossible to recur SILENTLY: every WebMeta article produces exactly one
lookup-outcome row, and a non-completed lookup (circuit_open / rate_limited /
failed / skipped / disabled / unknown) is stored as a value DISTINCT from
`no_snapshots`, so "we could not look" can never be read as "no edits".
"""

from __future__ import annotations

from datetime import datetime, timezone
from types import SimpleNamespace

from internal.adapters.web_meta import WebMeta
from internal import wayback_lookups as wl


class _FakeCH:
    """Captures `insert` calls so the upload path is testable without ClickHouse."""

    def __init__(self, *, raise_on_insert: bool = False) -> None:
        self.calls: list[dict] = []
        self._raise = raise_on_insert

    def insert(self, table, rows, column_names=None, deduplication_token=None):
        if self._raise:
            raise RuntimeError("clickhouse down")
        self.calls.append(
            {
                "table": table,
                "rows": rows,
                "column_names": column_names,
                "deduplication_token": deduplication_token,
            }
        )


def _core(source: str = "bundesregierung", document_id: str = "abc123"):
    return SimpleNamespace(source=source, document_id=document_id)


def _at() -> datetime:
    return datetime(2026, 5, 31, 12, 0, tzinfo=timezone.utc)


# --- normalise_status -------------------------------------------------------


def test_normalise_status_passes_known_values_through() -> None:
    for s in ("ok", "no_snapshots", "failed", "circuit_open", "rate_limited", "skipped", "disabled"):
        assert wl.normalise_status(s) == s


def test_normalise_status_empty_and_garbage_collapse_to_unknown() -> None:
    assert wl.normalise_status("") == "unknown"
    assert wl.normalise_status(None) == "unknown"
    assert wl.normalise_status("totally-bogus") == "unknown"


def test_non_answer_statuses_are_distinct_from_no_snapshots() -> None:
    """The core invariant — incomplete lookups must NEVER equal a real answer."""
    completed = wl.COMPLETED_STATUSES
    assert completed == {"ok", "no_snapshots"}
    for s in ("failed", "circuit_open", "rate_limited", "skipped", "disabled", "unknown"):
        assert s not in completed
        assert s != "no_snapshots"


# --- build_wayback_lookup_row ----------------------------------------------


def test_build_row_shape_and_status_normalised() -> None:
    row = wl.build_wayback_lookup_row(
        source="bundesregierung",
        article_id="abc123",
        canonical_url="https://www.bundesregierung.de/breg-de/aktuelles/x-1",
        status="circuit_open",
        ingestion_version=7,
        looked_up_at=_at(),
    )
    # Order must match the column_names in upload_wayback_lookup.
    assert row == [
        "bundesregierung",
        "abc123",
        "https://www.bundesregierung.de/breg-de/aktuelles/x-1",
        "circuit_open",
        _at(),
        7,
    ]


def test_build_row_empty_url_and_blank_status() -> None:
    row = wl.build_wayback_lookup_row(
        source="elysee",
        article_id="d",
        canonical_url="",
        status="",  # WebMeta default when the lookup never ran
        ingestion_version=1,
        looked_up_at=_at(),
    )
    assert row[2] == ""  # canonical_url
    assert row[3] == "unknown"  # blank → unknown, never silently dropped


# --- upload_wayback_lookup --------------------------------------------------


def test_upload_writes_exactly_one_row_for_webmeta() -> None:
    ch = _FakeCH()
    meta = WebMeta(source_type="web", canonical_url="https://x/y", wayback_lookup_status="circuit_open")
    wl.upload_wayback_lookup(ch, _core(), meta, ingestion_version=3, looked_up_at=_at())

    assert len(ch.calls) == 1
    call = ch.calls[0]
    assert call["table"] == "aer_gold.wayback_lookups"
    assert len(call["rows"]) == 1
    # status column (index 3 per column_names) is the recorded outcome.
    assert call["rows"][0][3] == "circuit_open"
    assert call["column_names"][3] == "status"


def test_upload_is_noop_for_non_webmeta() -> None:
    ch = _FakeCH()
    # An RSS / legacy meta has no Wayback step — represented here by a plain object.
    wl.upload_wayback_lookup(ch, _core(), SimpleNamespace(), ingestion_version=1, looked_up_at=_at())
    assert ch.calls == []


def test_upload_never_raises_on_insert_failure() -> None:
    """A blind guard must not take down the pipeline (fail-silent for processing)."""
    ch = _FakeCH(raise_on_insert=True)
    meta = WebMeta(source_type="web", canonical_url="https://x/y", wayback_lookup_status="ok")
    # Must not raise.
    wl.upload_wayback_lookup(ch, _core(), meta, ingestion_version=1, looked_up_at=_at())

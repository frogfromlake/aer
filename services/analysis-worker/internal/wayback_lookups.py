"""
Per-article Wayback CDX lookup outcome → ``aer_gold.wayback_lookups``.

Why this module exists
----------------------
Before it, the per-article Wayback lookup outcome was written to Gold ONLY
when revisions actually existed (`aer_gold.article_revisions`). A lookup that
did not COMPLETE — IA unreachable, circuit breaker open, local rate-limit —
produced no row and no signal, so "we could not look" was indistinguishable
from "no edits observed". A transient IA outage therefore became permanent,
SILENT, source-skewed data loss.

This module writes ONE row per WebMeta article, UNCONDITIONALLY, into
``aer_gold.wayback_lookups`` (migration 000025). "0 rows for source X" becomes
impossible: the worker records the outcome for every web article it processes.
``ok`` / ``no_snapshots`` mean "we know"; ``failed`` / ``circuit_open`` /
``rate_limited`` / ``skipped`` / ``disabled`` / ``unknown`` mean "we do NOT
know" — and that gap is now queryable per source (operational monitoring +
the Phase-122d.2 Negative-Space surface), never silent.

Failure mode mirrors :mod:`metadata_coverage`: a missing row only blinds the
observability guard for that one article; it never hard-fails the pipeline
(the canonical Silver record is untouched). The guard losing a row is itself
logged at ERROR so even THAT degradation is visible.
"""

from __future__ import annotations

from datetime import datetime

import structlog

from internal.adapters.web_meta import WebMeta

logger = structlog.get_logger()

WAYBACK_LOOKUPS_TABLE = "aer_gold.wayback_lookups"

# The full status vocabulary the CDX client can emit (mirrors the STATUS_*
# constants in `internal.wayback.client`). "ok"/"no_snapshots" are the only
# two that mean "we looked and this is the answer"; every other value means
# "we did not get an answer" and must never be collapsed into "no edits".
KNOWN_STATUSES: frozenset[str] = frozenset(
    {"ok", "no_snapshots", "failed", "circuit_open", "rate_limited", "skipped", "disabled"}
)

# Statuses where the lookup genuinely completed. Anything NOT in this set is an
# "unknown" outcome eligible for recovery on the next crawl (and surfaced as
# Negative Space rather than as "no revisions").
COMPLETED_STATUSES: frozenset[str] = frozenset({"ok", "no_snapshots"})


def normalise_status(raw: str | None) -> str:
    """Storage form of a `wayback_lookup_status`.

    Empty (e.g. the integration was disabled so the WebAdapter never set it)
    or out-of-vocabulary collapses to ``"unknown"`` — defensive, because a row
    is ALWAYS written and an unrecognised value must still be honest about
    being a non-answer rather than masquerade as a real status.
    """
    if not raw:
        return "unknown"
    return raw if raw in KNOWN_STATUSES else "unknown"


def build_wayback_lookup_row(
    source: str,
    article_id: str,
    canonical_url: str,
    status: str | None,
    ingestion_version: int,
    looked_up_at: datetime,
) -> list[object]:
    """Construct the single `wayback_lookups` row for one article.

    Column order matches the ``column_names`` passed to ``insert`` below (not
    the physical table order, which ``insert`` maps by name).
    """
    return [
        source,
        article_id,
        canonical_url or "",
        normalise_status(status),
        looked_up_at,
        ingestion_version,
    ]


def upload_wayback_lookup(
    ch_client,
    core,
    meta,
    ingestion_version: int,
    looked_up_at: datetime,
) -> None:
    """Insert one row recording this article's Wayback lookup outcome.

    Only `WebMeta` envelopes carry a `wayback_lookup_status` — RSS / legacy
    `SilverMeta` subclasses have no Wayback step, so the function no-ops for
    them. For every web article it writes a row regardless of outcome — that
    unconditional write is the whole point: it is what makes a non-answer
    impossible to lose silently.
    """
    if not isinstance(meta, WebMeta):
        return

    try:
        row = build_wayback_lookup_row(
            source=core.source,
            article_id=core.document_id,
            canonical_url=getattr(meta, "canonical_url", ""),
            status=getattr(meta, "wayback_lookup_status", ""),
            ingestion_version=ingestion_version,
            looked_up_at=looked_up_at,
        )
        ch_client.insert(
            WAYBACK_LOOKUPS_TABLE,
            [row],
            column_names=[
                "source",
                "article_id",
                "canonical_url",
                "status",
                "looked_up_at",
                "ingestion_version",
            ],
            deduplication_token=f"{WAYBACK_LOOKUPS_TABLE}:{core.document_id}:{ingestion_version}",
        )
    except Exception as exc:
        logger.error(
            "Wayback lookup-status insert failed; the silent-failure guard is "
            "blind for this article.",
            source=core.source,
            article_id=core.document_id,
            error=str(exc),
        )

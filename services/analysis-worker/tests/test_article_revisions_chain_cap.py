"""Phase 148c — the per-article revision chain is capped so a live-ticker /
continuously-updated page (hundreds of Wayback CDX content-versions) cannot
monopolise the diff/enrichment budget for zero analytical value. Past the cap
the chain is truncated; the article then surfaces as the `live_ticker`
Negative-Space class in the UI (revision count >= the matching frontend floor).
"""

from __future__ import annotations

from internal.article_revisions import (
    MAX_CDX_REVISIONS_PER_ARTICLE,
    _build_chain,
)


def _raw_cdx_entries(n: int) -> list[dict]:
    """n distinct Wayback CDX revision dicts (unique content hashes + timestamps)."""
    return [
        {
            "snapshot_at": f"2026-01-01T{i // 60:02d}:{i % 60:02d}:00Z",
            "content_hash": f"content-hash-{i}",
            "archive_url": f"https://web.archive.org/web/2026{i:04d}/x",
        }
        for i in range(n)
    ]


def test_build_chain_caps_a_runaway_ticker_chain():
    chain = _build_chain(_raw_cdx_entries(MAX_CDX_REVISIONS_PER_ARTICLE + 50), None)
    assert len(chain) == MAX_CDX_REVISIONS_PER_ARTICLE
    # Truncation keeps the chronologically-first N (sorted, coherent prev-hash chain).
    assert chain[0]["content_hash"] == "content-hash-0"


def test_build_chain_leaves_a_normal_article_intact():
    chain = _build_chain(_raw_cdx_entries(5), None)
    assert len(chain) == 5


def test_build_chain_cap_counts_the_republication_pseudo_revision():
    # The republication pseudo-revision is part of the chain and counts toward
    # the cap, so a ticker that also re-lists never exceeds the bound.
    republication = {
        "snapshot_at": __import__("datetime").datetime(2026, 1, 1, tzinfo=__import__("datetime").timezone.utc),
        "content_hash": "republication-hash",
        "trigger": "republication_trigger",
    }
    chain = _build_chain(_raw_cdx_entries(MAX_CDX_REVISIONS_PER_ARTICLE + 10), republication)
    assert len(chain) == MAX_CDX_REVISIONS_PER_ARTICLE

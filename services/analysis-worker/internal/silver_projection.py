"""
Silver-layer ClickHouse projection (Phase 103b).

Silver itself lives as JSON envelopes in MinIO. To support the
`/api/v1/silver/aggregations/{aggregationType}` endpoints (distributional,
heatmap, correlation queries) without paying a per-request MinIO scan, the
analysis worker writes a one-row-per-document projection of the fields
needed for those aggregations into `aer_silver.documents` at the same point
Silver is uploaded to MinIO.

Idempotent via `ingestion_version` (mirrors aer_gold.metrics): NATS
redelivery emits the same (timestamp, source, article_id) with the same
version, and ReplacingMergeTree collapses on merge / FINAL.
"""

import re
import structlog

logger = structlog.get_logger()

SILVER_DOCS_TABLE = "aer_silver.documents"

# Pre-NER token-based candidate-entity heuristic per ROADMAP Phase 103b: a
# capitalized-word run, including German umlauts and the non-initial ß. This
# is deliberately a cheap deterministic count (Ockham's Razor) — it is not
# the spaCy NER count (which lives in aer_gold.entities). Sentence-initial
# capitalization is intentionally not filtered: undercounting matters more
# than the false-positive on the first word.
_CAPITALIZED_TOKEN = re.compile(r"\b[A-ZÄÖÜ][A-Za-zÄÖÜäöüß]+\b")


def raw_entity_count(cleaned_text: str) -> int:
    if not cleaned_text:
        return 0
    return len(_CAPITALIZED_TOKEN.findall(cleaned_text))


def upload_silver_projection(ch_client, core, ingestion_version: int) -> None:
    """
    Insert a single projection row into aer_silver.documents.

    Failure is logged and swallowed: a missing projection row only impacts
    the aggregation endpoints, and the canonical Silver record is the MinIO
    envelope. Hard-failing here would jeopardize the rest of the pipeline.
    """
    try:
        row = [
            core.timestamp,
            core.source,
            core.document_id,
            core.language or "und",
            len(core.cleaned_text or ""),
            int(core.word_count),
            raw_entity_count(core.cleaned_text or ""),
            ingestion_version,
        ]
        ch_client.insert(
            SILVER_DOCS_TABLE,
            [row],
            column_names=[
                "timestamp",
                "source",
                "article_id",
                "language",
                "cleaned_text_length",
                "word_count",
                "raw_entity_count",
                "ingestion_version",
            ],
        )
        logger.info(
            "Silver projection updated",
            source=core.source,
            article_id=core.document_id,
            cleaned_text_length=len(core.cleaned_text or ""),
        )
    except Exception as e:
        logger.error(
            "Silver projection insert failed; aggregation endpoints will be missing this row.",
            source=core.source,
            article_id=core.document_id,
            error=str(e),
        )

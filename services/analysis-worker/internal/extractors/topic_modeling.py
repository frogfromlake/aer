"""
Topic Modeling corpus-level extractor (Phase 120).

Implements the Episteme-pillar BERTopic pipeline as the second
``CorpusExtractor`` after Phase 102's entity co-occurrence sweep. Topic
discovery runs on a configurable rolling window (default 30 days) and is
**partitioned by detected language** per WP-004 §3.4: BERTopic is fit
once per language partition, never across languages. Cross-cultural
topic alignment is explicitly out of scope.

Reproducibility commitments (Tier 2 per WP-002 §8.1 Option C):
  * Embedding model and revision pinned via the Language Capability
    Manifest (``shared.topic_modeling`` block — defaults to
    ``intfloat/multilingual-e5-large``).
  * UMAP and HDBSCAN seeded; ``random_state=42`` is the canonical seed.
  * ``model_hash`` recorded per row composes the BERTopic version, the
    sentence-transformer model id + revision, both random seeds, and the
    language partition. Any change in any of those bumps the hash and
    therefore the topic identity space.

The extractor itself is pure: ``extract_topics`` consumes a list of
``DocumentRecord`` and returns a list of ``TopicAssignmentRow``. The
caller (``corpus.topic_extraction_loop``) owns Silver IO from MinIO and
the bulk insert into ``aer_gold.topic_assignments``.

Graceful degradation: if BERTopic / sentence-transformers / UMAP /
HDBSCAN are unavailable at runtime, the extractor logs a structured
warning and returns an empty list — consistent with the rest of the
extractor pipeline. Tier-1 metrics keep flowing.
"""

from __future__ import annotations

import hashlib
import os
from collections import defaultdict
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Iterable

import structlog
import yaml

from internal.extractors.base import TimeWindow

logger = structlog.get_logger()


_DEFAULT_EMBEDDING_MODEL = "intfloat/multilingual-e5-large"
_DEFAULT_EMBEDDING_REVISION = "ab10c1a7f42e74530fe7ae5be82e6d4f11a719eb"
_DEFAULT_UMAP_SEED = 42
_DEFAULT_HDBSCAN_SEED = 42
_BERTOPIC_OUTLIER_TOPIC_ID = -1
_MIN_DOCS_PER_LANGUAGE = 10  # BERTopic / HDBSCAN underflow guard.


@dataclass(frozen=True, slots=True)
class DocumentRecord:
    """A single Silver document presented to the topic-modeling sweep.

    The corpus loop assembles these from ``aer_silver.documents`` (article
    list) plus the Silver envelope fetched from MinIO (cleaned text). The
    extractor itself is IO-free — it only consumes records.
    """

    article_id: str
    source: str
    language: str
    cleaned_text: str


@dataclass(frozen=True, slots=True)
class TopicAssignmentRow:
    """One row of ``aer_gold.topic_assignments``.

    ``topic_id`` is unique only within ``(window_start, language)`` — never
    cross-language. ``topic_id == -1`` is BERTopic's outlier class and is
    stored unchanged so the BFF can surface it as an "uncategorised"
    observation (Phase 121 design constraint).
    """

    window_start: datetime
    window_end: datetime
    source: str
    article_id: str
    language: str
    topic_id: int
    topic_label: str
    topic_confidence: float
    model_hash: str


class TopicModelingExtractor:
    """Corpus-level BERTopic extractor (Phase 120).

    Implements the ``CorpusExtractor`` protocol's spirit (batch over a
    window) but exposes ``extract_topics`` rather than the
    ``extract_batch`` signature, because BERTopic produces topic
    assignments — not ``GoldMetric`` rows. The ``aer_gold.topic_assignments``
    table is the dedicated output sink, mirroring Phase 102's
    ``aer_gold.entity_cooccurrences`` arrangement.
    """

    def __init__(
        self,
        embedding_model: str | None = None,
        embedding_revision: str | None = None,
        umap_seed: int = _DEFAULT_UMAP_SEED,
        hdbscan_seed: int = _DEFAULT_HDBSCAN_SEED,
        manifest_path: str | None = None,
    ) -> None:
        resolved_model, resolved_revision = self._resolve_embedding_config(
            manifest_path,
            embedding_model,
            embedding_revision,
        )
        self.embedding_model = resolved_model
        self.embedding_revision = resolved_revision
        self.umap_seed = umap_seed
        self.hdbscan_seed = hdbscan_seed

    @property
    def name(self) -> str:
        return "topic_modeling"

    @staticmethod
    def _resolve_embedding_config(
        manifest_path: str | None,
        explicit_model: str | None,
        explicit_revision: str | None,
    ) -> tuple[str, str]:
        """Manifest-driven model selection (ADR-024 pattern).

        Constructor arguments win (test injection). Otherwise the
        ``shared.topic_modeling`` block in
        ``configs/language_capabilities.yaml`` is consulted. Falls back to
        the documented Phase-120 default (``multilingual-e5-large``) when
        the manifest does not declare topic-modeling — keeps the extractor
        usable in unit tests that do not mount the manifest.
        """
        if explicit_model and explicit_revision:
            return explicit_model, explicit_revision

        if manifest_path is None:
            manifest_path = os.getenv(
                "LANGUAGE_CAPABILITY_MANIFEST_PATH",
                str(
                    Path(__file__).resolve().parents[2]
                    / "configs"
                    / "language_capabilities.yaml"
                ),
            )

        try:
            data = yaml.safe_load(Path(manifest_path).read_text(encoding="utf-8"))
        except (FileNotFoundError, OSError):
            return (
                explicit_model or _DEFAULT_EMBEDDING_MODEL,
                explicit_revision or _DEFAULT_EMBEDDING_REVISION,
            )

        shared = (data or {}).get("shared", {}) or {}
        topic_block = shared.get("topic_modeling") or {}
        return (
            explicit_model or topic_block.get("model", _DEFAULT_EMBEDDING_MODEL),
            explicit_revision
            or topic_block.get("model_revision", _DEFAULT_EMBEDDING_REVISION),
        )

    def model_hash(self, language_partition: str, bertopic_version: str) -> str:
        """Composite reproducibility hash, written per row.

        Captures every input that influences topic identity: the BERTopic
        version, the embedding model id + revision, both seeds, and the
        language partition. Bumping any one of them rotates the hash and
        signals to downstream consumers that ``topic_id`` no longer
        corresponds to the prior topic-identity space.
        """
        material = ":".join(
            [
                self.embedding_model,
                self.embedding_revision,
                bertopic_version,
                str(self.umap_seed),
                str(self.hdbscan_seed),
                language_partition,
            ]
        ).encode("utf-8")
        return hashlib.sha256(material).hexdigest()

    @staticmethod
    def partition_by_language(
        records: Iterable[DocumentRecord],
    ) -> dict[str, list[DocumentRecord]]:
        """Group records by ``language``. Empty / ``und`` languages are dropped.

        Per WP-004 §3.4 we never feed mixed-language batches to BERTopic.
        Documents without a confidently-detected language (Phase 116
        ``und`` fallback) are excluded so the topic space is never
        polluted by adapter defaults; they reappear in the next sweep
        once language detection succeeds.
        """
        partitions: dict[str, list[DocumentRecord]] = defaultdict(list)
        for rec in records:
            if not rec.cleaned_text or not rec.cleaned_text.strip():
                continue
            language = (rec.language or "").strip()
            if not language or language == "und":
                continue
            partitions[language].append(rec)
        return partitions

    def extract_topics(
        self,
        records: Iterable[DocumentRecord],
        window: TimeWindow,
    ) -> list[TopicAssignmentRow]:
        """Run BERTopic per language partition and emit assignment rows.

        Returns ``[]`` (never raises) when:
          * the corpus is empty after partitioning,
          * a language partition has fewer than ``_MIN_DOCS_PER_LANGUAGE``
            documents (BERTopic / HDBSCAN underflow on tiny corpora),
          * BERTopic / sentence-transformers are not importable at
            runtime (graceful degradation per the extractor contract).
        """
        partitions = self.partition_by_language(records)
        if not partitions:
            logger.info("topic_modeling.empty_corpus")
            return []

        try:
            bertopic_module, bertopic_version = self._import_bertopic()
        except RuntimeError as exc:
            logger.warning(
                "topic_modeling.dependencies_missing",
                error=str(exc),
            )
            return []

        rows: list[TopicAssignmentRow] = []
        for language, docs in partitions.items():
            if len(docs) < _MIN_DOCS_PER_LANGUAGE:
                logger.info(
                    "topic_modeling.partition_below_min_docs",
                    language=language,
                    n_docs=len(docs),
                    min_docs=_MIN_DOCS_PER_LANGUAGE,
                )
                continue
            try:
                rows.extend(
                    self._fit_partition(
                        bertopic_module,
                        bertopic_version,
                        language,
                        docs,
                        window,
                    )
                )
            except Exception as exc:  # pragma: no cover — defensive
                logger.error(
                    "topic_modeling.partition_failed",
                    language=language,
                    n_docs=len(docs),
                    error=str(exc),
                    error_type=type(exc).__name__,
                )

        return rows

    @staticmethod
    def _import_bertopic():
        """Late import so the worker boots even without BERTopic installed."""
        try:
            import bertopic  # type: ignore[import-not-found]
        except ImportError as exc:
            raise RuntimeError(f"BERTopic not importable: {exc}") from exc
        version = getattr(bertopic, "__version__", "unknown")
        return bertopic, version

    def _fit_partition(
        self,
        bertopic_module,
        bertopic_version: str,
        language: str,
        docs: list[DocumentRecord],
        window: TimeWindow,
    ) -> list[TopicAssignmentRow]:
        """Fit BERTopic on one language partition and emit assignment rows."""
        from sentence_transformers import SentenceTransformer  # type: ignore[import-not-found]
        from umap import UMAP  # type: ignore[import-not-found]
        from hdbscan import HDBSCAN  # type: ignore[import-not-found]

        embedder = SentenceTransformer(
            self.embedding_model,
            revision=self.embedding_revision,
        )

        umap_model = UMAP(
            n_neighbors=15,
            n_components=5,
            min_dist=0.0,
            metric="cosine",
            random_state=self.umap_seed,
        )
        hdbscan_model = HDBSCAN(
            min_cluster_size=10,
            metric="euclidean",
            cluster_selection_method="eom",
            prediction_data=True,
        )

        BERTopic = bertopic_module.BERTopic
        topic_model = BERTopic(
            embedding_model=embedder,
            umap_model=umap_model,
            hdbscan_model=hdbscan_model,
            calculate_probabilities=False,
            verbose=False,
        )

        texts = [rec.cleaned_text for rec in docs]
        topics, _ = topic_model.fit_transform(texts)

        topic_info = topic_model.get_topic_info()
        label_by_topic_id: dict[int, str] = {}
        for _, row in topic_info.iterrows():
            topic_id = int(row["Topic"])
            # BERTopic's Name column is the c-TF-IDF / KeyBERT representation.
            label_by_topic_id[topic_id] = str(row.get("Name", "") or "")

        partition_hash = self.model_hash(
            language_partition=language,
            bertopic_version=bertopic_version,
        )

        rows: list[TopicAssignmentRow] = []
        for rec, topic_id in zip(docs, topics, strict=True):
            topic_id_int = int(topic_id)
            label = label_by_topic_id.get(
                topic_id_int,
                "outlier" if topic_id_int == _BERTOPIC_OUTLIER_TOPIC_ID else "",
            )
            # Confidence: 1.0 for assigned topics, 0.0 for outliers. We
            # explicitly set ``calculate_probabilities=False`` above
            # because the soft-clustering path is non-deterministic on
            # small corpora; the fixed-seed hard assignment is the Tier-2
            # reproducibility commitment.
            confidence = 0.0 if topic_id_int == _BERTOPIC_OUTLIER_TOPIC_ID else 1.0
            rows.append(
                TopicAssignmentRow(
                    window_start=window.start,
                    window_end=window.end,
                    source=rec.source,
                    article_id=rec.article_id,
                    language=language,
                    topic_id=topic_id_int,
                    topic_label=label,
                    topic_confidence=confidence,
                    model_hash=partition_hash,
                )
            )

        logger.info(
            "topic_modeling.partition_complete",
            language=language,
            n_docs=len(docs),
            n_topics=len({t for t in topics if int(t) != _BERTOPIC_OUTLIER_TOPIC_ID}),
            n_outliers=sum(1 for t in topics if int(t) == _BERTOPIC_OUTLIER_TOPIC_ID),
            model_hash=partition_hash,
        )
        return rows

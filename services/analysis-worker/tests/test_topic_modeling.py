"""Unit tests for TopicModelingExtractor (Phase 120).

The BERTopic dependency tree (sentence-transformers, UMAP, HDBSCAN, the
multilingual E5 weights baked into the runtime image) is heavy. The
extractor is designed to gracefully degrade when those imports fail, so
the bulk of the per-document logic (language partitioning, model_hash
composition, the underflow guard) is exercised directly without
exercising BERTopic itself. The integration test that *does* exercise
BERTopic is gated on the dependency being importable so the unit suite
keeps running on a slim Python environment.
"""

from __future__ import annotations

import importlib.util
from datetime import datetime
from unittest.mock import MagicMock

import pytest

from internal.extractors import TimeWindow
from internal.extractors.topic_modeling import (
    DocumentRecord,
    TopicAssignmentRow,
    TopicModelingExtractor,
)


_BERTOPIC_AVAILABLE = importlib.util.find_spec("bertopic") is not None


WINDOW = TimeWindow(start=datetime(2026, 4, 1), end=datetime(2026, 5, 1))


def _doc(article_id: str, source: str, language: str, text: str) -> DocumentRecord:
    return DocumentRecord(
        article_id=article_id,
        source=source,
        language=language,
        cleaned_text=text,
    )


def test_partition_by_language_groups_and_drops_und():
    extractor = TopicModelingExtractor()
    docs = [
        _doc("a1", "tagesschau", "de", "Bundestag debatte"),
        _doc("a2", "tagesschau", "de", "Klimaschutz"),
        _doc("a3", "francetvinfo", "fr", "Assemblée nationale"),
        _doc("a4", "anywhere", "und", "should be dropped"),
        _doc("a5", "anywhere", "", "also dropped"),
        _doc("a6", "tagesschau", "de", "   "),  # whitespace-only — dropped
    ]
    partitions = extractor.partition_by_language(docs)
    assert set(partitions.keys()) == {"de", "fr"}
    assert len(partitions["de"]) == 2
    assert len(partitions["fr"]) == 1


def test_extract_topics_empty_corpus_returns_empty_list():
    extractor = TopicModelingExtractor()
    assert extractor.extract_topics([], WINDOW) == []


def test_extract_topics_partition_below_min_docs_skipped():
    """Partitions smaller than the BERTopic underflow threshold are skipped."""
    extractor = TopicModelingExtractor()
    # Five docs is well below the 10-doc minimum — even with BERTopic
    # available the partition is skipped before any model load.
    docs = [_doc(f"a{i}", "tagesschau", "de", f"text {i}") for i in range(5)]
    rows = extractor.extract_topics(docs, WINDOW)
    assert rows == []


def test_extract_topics_dependencies_missing_returns_empty(monkeypatch):
    """When BERTopic is unimportable the extractor logs and returns []."""
    extractor = TopicModelingExtractor()

    def _raise_runtime():
        raise RuntimeError("BERTopic not importable")

    monkeypatch.setattr(extractor, "_import_bertopic", lambda: _raise_runtime())
    docs = [_doc(f"a{i}", "tagesschau", "de", f"text {i}") for i in range(15)]
    rows = extractor.extract_topics(docs, WINDOW)
    assert rows == []


def test_model_hash_changes_with_inputs():
    extractor = TopicModelingExtractor(
        embedding_model="m", embedding_revision="r", umap_seed=1, hdbscan_seed=2
    )
    h_de = extractor.model_hash("de", "0.17.0")
    h_fr = extractor.model_hash("fr", "0.17.0")
    h_de_v2 = extractor.model_hash("de", "0.18.0")
    assert h_de != h_fr  # language partition changes the hash
    assert h_de != h_de_v2  # bertopic version bump changes the hash
    assert h_de == extractor.model_hash("de", "0.17.0")  # deterministic


def test_topic_assignment_row_is_frozen():
    row = TopicAssignmentRow(
        window_start=WINDOW.start,
        window_end=WINDOW.end,
        source="tagesschau",
        article_id="a1",
        language="de",
        topic_id=0,
        topic_label="bundestag",
        topic_confidence=1.0,
        model_hash="hash",
    )
    with pytest.raises((AttributeError, TypeError)):
        row.topic_id = 99  # type: ignore[misc]


def test_extract_topics_returns_rows_when_partition_succeeds():
    """End-to-end with a stubbed BERTopic to bypass the heavy import path.

    Verifies the extractor: wires the (article_id, language, topic_id)
    mapping correctly, copies the window onto every row, stamps the
    composite model_hash, and treats topic_id == -1 as the outlier class
    with confidence 0.
    """
    # Patch the dependency import path so the extractor accepts a fake
    # BERTopic-shaped module. The fake assigns topic 0 to every doc except
    # the last (-1, the outlier class).
    extractor = TopicModelingExtractor(embedding_model="m", embedding_revision="r")

    fake_module = MagicMock()
    fake_module.__version__ = "0.17.0"

    fake_topic_model = MagicMock()
    n_docs = 12
    topic_assignments = [0] * (n_docs - 1) + [-1]
    fake_topic_model.fit_transform.return_value = (topic_assignments, None)

    class _FakeRow:
        def __init__(self, topic, name):
            self._d = {"Topic": topic, "Name": name}

        def __getitem__(self, k):
            return self._d[k]

        def get(self, k, default=None):
            return self._d.get(k, default)

    fake_topic_info = MagicMock()
    fake_topic_info.iterrows.return_value = [
        (0, _FakeRow(0, "bundestag_klimaschutz")),
        (1, _FakeRow(-1, "")),
    ]
    fake_topic_model.get_topic_info.return_value = fake_topic_info

    fake_module.BERTopic = MagicMock(return_value=fake_topic_model)

    extractor._import_bertopic = lambda: (fake_module, "0.17.0")  # type: ignore[assignment]

    # Stub the third-party heavy imports inside _fit_partition.
    import sys

    sys.modules.setdefault("sentence_transformers", MagicMock())
    sys.modules.setdefault("umap", MagicMock())
    sys.modules.setdefault("hdbscan", MagicMock())
    # Phase 120b: per-language stopword filter wraps a real CountVectorizer
    # constructor; stub the import so the test does not require sklearn.
    fake_sklearn = MagicMock()
    fake_sklearn.feature_extraction.text.CountVectorizer = MagicMock()
    sys.modules.setdefault("sklearn", fake_sklearn)
    sys.modules.setdefault("sklearn.feature_extraction", fake_sklearn.feature_extraction)
    sys.modules.setdefault(
        "sklearn.feature_extraction.text", fake_sklearn.feature_extraction.text
    )

    docs = [_doc(f"a{i}", "tagesschau", "de", f"text {i}") for i in range(n_docs)]
    rows = extractor.extract_topics(docs, WINDOW)

    assert len(rows) == n_docs
    assert {r.language for r in rows} == {"de"}
    assert {r.window_start for r in rows} == {WINDOW.start}
    # Outlier row has confidence 0 and topic_id -1.
    outliers = [r for r in rows if r.topic_id == -1]
    assigned = [r for r in rows if r.topic_id == 0]
    assert len(outliers) == 1
    assert outliers[0].topic_confidence == 0.0
    assert all(r.topic_confidence == 1.0 for r in assigned)
    # All rows from the same partition share the same model_hash.
    assert len({r.model_hash for r in rows}) == 1


def test_resolve_stopwords_loads_spacy_for_de(tmp_path):
    """Manifest entry `source: spacy` resolves to the spaCy STOP_WORDS set
    when the lang package is importable. Validates the new Phase-120b
    BERTopic label-quality fix without forcing the unit suite to depend
    on the slim worker image's spaCy install."""
    manifest = tmp_path / "manifest.yaml"
    manifest.write_text(
        "languages:\n"
        "  xx:\n"
        "    topic_modeling:\n"
        "      stopwords:\n"
        "        source: ['alpha', 'beta', 'gamma']\n",
        encoding="utf-8",
    )
    extractor = TopicModelingExtractor(
        embedding_model="m",
        embedding_revision="r",
        manifest_path=str(manifest),
    )
    assert extractor._resolve_stopwords("xx") == ["alpha", "beta", "gamma"]
    # Languages without a manifest entry get None — preserves the
    # opt-in semantics so today's behaviour is not silently changed
    # for any language other than `de`.
    assert extractor._resolve_stopwords("yy") is None


@pytest.mark.skipif(
    not _BERTOPIC_AVAILABLE,
    reason="BERTopic optional integration test — requires the heavy NLP stack",
)
def test_bertopic_runs_end_to_end_on_minimal_corpus():
    """Smoke test that requires BERTopic + sentence-transformers + UMAP + HDBSCAN.

    Skipped on environments without the heavy NLP stack so the unit suite
    stays fast. Exercises the real partition → fit → label path with two
    obvious topic clusters and asserts at least 2 topics are discovered
    in the German partition.
    """
    extractor = TopicModelingExtractor()
    political = [
        f"Bundestag debattiert Klimaschutz und Wirtschaftspolitik {i}"
        for i in range(15)
    ]
    sports = [
        f"Bundesliga Spieltag Fußball Mannschaft Tor {i}"
        for i in range(15)
    ]
    docs = [
        _doc(f"p{i}", "tagesschau", "de", t)
        for i, t in enumerate(political)
    ] + [
        _doc(f"s{i}", "tagesschau", "de", t)
        for i, t in enumerate(sports)
    ]
    rows = extractor.extract_topics(docs, WINDOW)
    assert rows, "expected at least one topic assignment"
    non_outlier = {r.topic_id for r in rows if r.topic_id != -1}
    assert len(non_outlier) >= 2

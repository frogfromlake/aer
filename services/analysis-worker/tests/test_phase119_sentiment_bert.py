"""Phase 119 — Tier-2 / Tier-2.5 BERT sentiment extractors (ADR-023).

The two production deps (`torch`, `transformers`) are heavy; running real
inference here would require the multi-gigabyte model weights plus a pinned
HuggingFace cache. These tests therefore split into two slices:

  * **Manifest gating + language guard** — runs everywhere. Mocks the score
    function so the test focuses on the routing logic the extractor adds
    around the model call. This is the slice that protects against regressions
    in the manifest-driven feature flags.

  * **Determinism gate** — skipped when `transformers` is absent. When it is
    present, the gate executes the documented Phase 119 contract: two
    consecutive scoring calls in the same process produce byte-identical
    output. Mirrors the in-process leg of the operations playbook's
    determinism runbook; the cross-process leg lives in the integration
    suite (out of scope for unit tests).
"""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

import pytest

from internal.extractors import sentiment_bert_de_news as de_news_mod
from internal.extractors import sentiment_bert_multilingual as multilingual_mod
from internal.extractors.sentiment_bert_de_news import GermanNewsBertSentimentExtractor
from internal.extractors.sentiment_bert_multilingual import (
    MultilingualBertSentimentExtractor,
)
from internal.models import SilverCore
from internal.models.language_capability import (
    CapabilityManifest,
    LanguageCapability,
    MultilingualBertCapability,
    NegationConfig,
    SentimentTier1Capability,
    SentimentTier2DefaultCapability,
    SentimentTier2RefinementCapability,
    SharedCapability,
    load_manifest,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _core(text: str, *, language: str = "de") -> SilverCore:
    return SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text=text,
        cleaned_text=text,
        language=language,
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=len(text.split()),
    )


def _manifest_with_multilingual(supported: list[str]) -> CapabilityManifest:
    return CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier1=SentimentTier1Capability(
                    method="lexicon",
                    lexicon="sentiws_v2.0",
                    negation=NegationConfig(),
                    metric_name="sentiment_score_sentiws",
                ),
                sentiment_tier2_default=SentimentTier2DefaultCapability(
                    method="multilingual_bert",
                    provided_by="shared.multilingual_bert",
                    metric_name="sentiment_score_bert_multilingual",
                ),
            ),
        },
        shared=SharedCapability(
            multilingual_bert=MultilingualBertCapability(
                model="cardiffnlp/twitter-xlm-roberta-base-sentiment",
                model_revision="abc123",
                supported_languages=supported,
            ),
        ),
    )


def _manifest_with_de_news() -> CapabilityManifest:
    return CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier2_refinement=SentimentTier2RefinementCapability(
                    method="news_domain_bert",
                    model="mdraw/german-news-sentiment-bert",
                    model_revision="def456",
                    metric_name="sentiment_score_bert_de_news",
                ),
            ),
        },
    )


# ---------------------------------------------------------------------------
# Multilingual extractor — manifest gating + language guard
# ---------------------------------------------------------------------------


def test_multilingual_disabled_when_manifest_lacks_block(monkeypatch):
    # An empty `shared` block — typical of a pre-Phase-119 manifest — must
    # leave the extractor disabled rather than crash. Graceful degradation
    # is the contract; a startup ImportError or attribute error would take
    # the worker down.
    manifest = CapabilityManifest(manifest_version=1, languages={})
    extractor = MultilingualBertSentimentExtractor(manifest=manifest)
    assert extractor._enabled is False
    assert extractor.extract_all(_core("Hallo Welt"), "p1").metrics == []


def test_multilingual_skips_unsupported_language(monkeypatch):
    extractor = MultilingualBertSentimentExtractor(
        manifest=_manifest_with_multilingual(["de", "en", "fr"]),
        tokenizer=object(),
        model=object(),
    )
    monkeypatch.setattr(multilingual_mod, "score_three_class_to_scalar", lambda *a, **k: 0.5)

    # Swahili is not in the supported set — no metric row.
    out = extractor.extract_all(_core("Habari yako", language="sw"), "p1").metrics
    assert out == []


def test_multilingual_emits_metric_for_supported_language(monkeypatch):
    extractor = MultilingualBertSentimentExtractor(
        manifest=_manifest_with_multilingual(["de", "en"]),
        tokenizer=object(),
        model=object(),
    )
    monkeypatch.setattr(
        multilingual_mod, "score_three_class_to_scalar", lambda *a, **k: 0.4242
    )

    out_de = extractor.extract_all(_core("Es ist ein guter Tag.", language="de"), "p1").metrics
    out_en = extractor.extract_all(_core("It is a good day.", language="en"), "p2").metrics

    for out in (out_de, out_en):
        assert len(out) == 1
        assert out[0].metric_name == "sentiment_score_bert_multilingual"
        assert out[0].value == pytest.approx(0.4242)


def test_multilingual_legacy_und_falls_through(monkeypatch):
    # Pre-detection documents tag language as "und". Tier-2 default still
    # emits a row so coverage does not regress when Phase 116 is rolled out
    # against a backlog.
    extractor = MultilingualBertSentimentExtractor(
        manifest=_manifest_with_multilingual(["de"]),
        tokenizer=object(),
        model=object(),
    )
    monkeypatch.setattr(
        multilingual_mod, "score_three_class_to_scalar", lambda *a, **k: 0.1
    )

    out = extractor.extract_all(_core("Etwas Text.", language="und"), "p1").metrics
    assert len(out) == 1


def test_multilingual_inference_failure_skips_document(monkeypatch):
    extractor = MultilingualBertSentimentExtractor(
        manifest=_manifest_with_multilingual(["de"]),
        tokenizer=object(),
        model=object(),
    )

    def boom(*_a, **_k):
        raise RuntimeError("simulated CUDA OOM")

    monkeypatch.setattr(multilingual_mod, "score_three_class_to_scalar", boom)
    out = extractor.extract_all(_core("Beispiel", language="de"), "p1").metrics
    assert out == []  # Graceful degradation — Tier-1 SentiWS still produces a row.


def test_multilingual_metric_name_constant():
    # ADR-016 dual-metric naming is part of the BFF contract
    # (`/api/v1/metrics/available` exposes this constant verbatim).
    assert MultilingualBertSentimentExtractor.METRIC_NAME == "sentiment_score_bert_multilingual"


# ---------------------------------------------------------------------------
# German news extractor — manifest gating + strict language match
# ---------------------------------------------------------------------------


def test_de_news_disabled_when_no_refinement_block():
    # Same defensive contract — an absent `sentiment_tier2_refinement` block
    # leaves the extractor inert without raising.
    manifest = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(iso_code="de", display_name="German"),
        },
    )
    extractor = GermanNewsBertSentimentExtractor(manifest=manifest)
    assert extractor._enabled is False
    assert extractor.extract_all(_core("Hallo"), "p1").metrics == []


def test_de_news_skips_non_german_language(monkeypatch):
    extractor = GermanNewsBertSentimentExtractor(
        manifest=_manifest_with_de_news(),
        tokenizer=object(),
        model=object(),
    )
    monkeypatch.setattr(de_news_mod, "score_three_class_to_scalar", lambda *a, **k: 0.5)

    out = extractor.extract_all(_core("It is a good day.", language="en"), "p1").metrics
    assert out == []


def test_de_news_skips_und_language(monkeypatch):
    # Strict match — `und` is *not* allowed to fall through here. The whole
    # point of Tier-2.5 is in-domain quality on confirmed-German text; an
    # undetected document gets only the Tier-2 multilingual row.
    extractor = GermanNewsBertSentimentExtractor(
        manifest=_manifest_with_de_news(),
        tokenizer=object(),
        model=object(),
    )
    monkeypatch.setattr(de_news_mod, "score_three_class_to_scalar", lambda *a, **k: 0.5)

    out = extractor.extract_all(_core("Etwas Text", language="und"), "p1").metrics
    assert out == []


def test_de_news_emits_metric_for_german(monkeypatch):
    extractor = GermanNewsBertSentimentExtractor(
        manifest=_manifest_with_de_news(),
        tokenizer=object(),
        model=object(),
    )
    monkeypatch.setattr(de_news_mod, "score_three_class_to_scalar", lambda *a, **k: -0.31)

    out = extractor.extract_all(_core("Schreckliche Nachrichten.", language="de"), "p1").metrics
    assert len(out) == 1
    assert out[0].metric_name == "sentiment_score_bert_de_news"
    assert out[0].value == pytest.approx(-0.31)


def test_de_news_metric_name_constant():
    assert GermanNewsBertSentimentExtractor.METRIC_NAME == "sentiment_score_bert_de_news"


# ---------------------------------------------------------------------------
# Manifest collision guard — Tier-2 metric_name must be unique across
# methods. This protects the BFF's metric-name parameterisation from
# silently merging rows produced by semantically incompatible extractors.
# ---------------------------------------------------------------------------


def test_bundled_manifest_exposes_phase119_metrics():
    manifest = load_manifest()
    assert manifest.shared.multilingual_bert is not None
    assert manifest.shared.multilingual_bert.model.startswith("cardiffnlp/")
    # German de_news refinement is wired up.
    de = manifest.languages["de"]
    assert de.sentiment_tier2_default is not None
    assert de.sentiment_tier2_default.metric_name == "sentiment_score_bert_multilingual"
    assert de.sentiment_tier2_refinement is not None
    assert de.sentiment_tier2_refinement.metric_name == "sentiment_score_bert_de_news"


# ---------------------------------------------------------------------------
# Provenance / version_hash
# ---------------------------------------------------------------------------


def test_version_hash_stable_across_calls():
    extractor = MultilingualBertSentimentExtractor(
        manifest=_manifest_with_multilingual(["de"])
    )
    h1 = extractor.version_hash
    h2 = extractor.version_hash
    assert h1 == h2
    # SHA-256 hex; not the truncated 16-char hash SentiWS uses for its
    # lexicon — Tier-2 keeps the full digest because it lives in Silver
    # provenance, not in per-row Gold tables.
    assert len(h1) == 64
    assert all(c in "0123456789abcdef" for c in h1)


def test_version_hash_changes_with_model_revision():
    a = MultilingualBertSentimentExtractor(
        manifest=_manifest_with_multilingual(["de"])
    ).version_hash

    rev_b_manifest = _manifest_with_multilingual(["de"])
    rev_b_manifest.shared.multilingual_bert.model_revision = "different-rev"
    b = MultilingualBertSentimentExtractor(manifest=rev_b_manifest).version_hash

    assert a != b


# ---------------------------------------------------------------------------
# Determinism gate — skipped when transformers is absent
# ---------------------------------------------------------------------------


_SCRIPT_DIR = Path(__file__).parent


def test_determinism_in_process_byte_identical():
    """Same process, two consecutive calls per extractor → byte-identical
    metric values. This is the Tier-2 reproducibility guarantee.

    Skipped when `transformers` is not importable (the dev environment may
    not have it installed locally; CI installs the full requirements).
    """
    pytest.importorskip("transformers")
    pytest.importorskip("torch")

    sentences = [
        "Es ist ein hervorragender Tag für die Demokratie.",
        "Das ist eine schlechte Entscheidung der Bundesregierung.",
    ]

    multi = MultilingualBertSentimentExtractor()
    if not multi._enabled:
        pytest.skip("multilingual model unavailable in this environment")

    out_a = [multi.extract_all(_core(s, language="de"), f"p{i}").metrics
             for i, s in enumerate(sentences)]
    out_b = [multi.extract_all(_core(s, language="de"), f"p{i}").metrics
             for i, s in enumerate(sentences)]

    for a, b in zip(out_a, out_b):
        assert a and b
        assert a[0].value == b[0].value, (
            "Multilingual BERT produced non-deterministic output across "
            "two calls in the same process — Phase 119 determinism gate failed."
        )

    de_news = GermanNewsBertSentimentExtractor()
    if not de_news._enabled:
        pytest.skip("German news model unavailable in this environment")

    out_c = [de_news.extract_all(_core(s, language="de"), f"p{i}").metrics
             for i, s in enumerate(sentences)]
    out_d = [de_news.extract_all(_core(s, language="de"), f"p{i}").metrics
             for i, s in enumerate(sentences)]

    for c, d in zip(out_c, out_d):
        assert c and d
        assert c[0].value == d[0].value

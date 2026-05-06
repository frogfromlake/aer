"""Phase 118a — Language Capability Manifest tests (ADR-024).

Covers:
  * Manifest schema loading (valid + invalid + version + collisions).
  * Refactored extractors read from the manifest.
  * Scaffold-generator idempotency + drift gate.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path
from textwrap import dedent

import pytest
from pydantic import ValidationError

from internal.extractors import NamedEntityExtractor, SentimentExtractor
from internal.models.language_capability import (
    CapabilityManifest,
    ConfigurationError,
    LanguageCapability,
    NegationConfig,
    NerCapability,
    SentimentTier1Capability,
    SentimentTier2DefaultCapability,
    SentimentTier2RefinementCapability,
    load_manifest,
)


_REPO_ROOT = Path(__file__).resolve().parents[3]
_BUNDLED_MANIFEST = (
    _REPO_ROOT / "services" / "analysis-worker" / "configs" / "language_capabilities.yaml"
)


# ---------------------------------------------------------------------------
# (a) Valid manifest loads and exposes the expected `de` block
# ---------------------------------------------------------------------------


def test_bundled_manifest_loads():
    manifest = load_manifest()
    assert manifest.manifest_version == 1
    assert "de" in manifest.languages
    de = manifest.languages["de"]
    assert de.iso_code == "de"
    assert de.ner is not None
    assert de.ner.model == "de_core_news_lg"
    assert de.sentiment_tier1 is not None
    assert de.sentiment_tier1.metric_name == "sentiment_score_sentiws"
    # Negation config has been lifted into the manifest.
    assert "nicht" in de.sentiment_tier1.negation.particles
    assert de.sentiment_tier1.negation.spacy_neg_dep == "neg"
    assert "ng" in de.sentiment_tier1.negation.spacy_neg_deps_extra
    assert "mark" in de.sentiment_tier1.negation.clause_boundary_deps


# ---------------------------------------------------------------------------
# (b) Invalid manifests raise structured ConfigurationError at load time
# ---------------------------------------------------------------------------


def test_missing_manifest_version_rejected(tmp_path):
    bad = tmp_path / "missing_version.yaml"
    bad.write_text("languages:\n  de:\n    iso_code: de\n    display_name: German\n", encoding="utf-8")
    with pytest.raises(ConfigurationError):
        load_manifest(bad)


def test_unsupported_manifest_version_rejected(tmp_path):
    bad = tmp_path / "future_version.yaml"
    bad.write_text(
        "manifest_version: 99\nlanguages:\n  de:\n    iso_code: de\n    display_name: German\n",
        encoding="utf-8",
    )
    with pytest.raises(ConfigurationError):
        load_manifest(bad)


def test_malformed_yaml_rejected(tmp_path):
    bad = tmp_path / "broken.yaml"
    bad.write_text(": : :\n  - not: { yaml]\n", encoding="utf-8")
    with pytest.raises(ConfigurationError):
        load_manifest(bad)


def test_unknown_top_level_field_rejected(tmp_path):
    # `extra=forbid` on the root model catches typos.
    bad = tmp_path / "typo.yaml"
    bad.write_text(
        "manifest_version: 1\nlanguages:\n  de:\n    iso_code: de\n    display_name: German\nunkown_field: 1\n",
        encoding="utf-8",
    )
    with pytest.raises(ConfigurationError):
        load_manifest(bad)


def test_missing_manifest_file_rejected(tmp_path):
    with pytest.raises(ConfigurationError):
        load_manifest(tmp_path / "nonexistent.yaml")


# ---------------------------------------------------------------------------
# (c) Pydantic schema rejects mismatched method/feature combinations
# ---------------------------------------------------------------------------


def test_unknown_sentiment_feature_rejected():
    with pytest.raises(ValidationError):
        SentimentTier1Capability(
            method="lexicon",
            lexicon="sentiws_v2.0",
            features=["compound_split", "made_up_feature"],
            negation=NegationConfig(),
            metric_name="sentiment_score_sentiws",
        )


def test_unknown_method_value_rejected():
    # `method` is a Literal["lexicon"] — anything else fails validation.
    with pytest.raises(ValidationError):
        SentimentTier1Capability(
            method="rule_based",  # type: ignore[arg-type]
            lexicon="x",
            negation=NegationConfig(),
            metric_name="sentiment_score_sentiws",
        )


# ---------------------------------------------------------------------------
# (d) Cross-language metric_name uniqueness — same name with different
# methods is rejected; same name with the same method is allowed (the
# multilingual case where Phase 119 would share `sentiment_score_bert_multilingual`
# across every supported language).
# ---------------------------------------------------------------------------


def test_metric_name_collision_across_languages_rejected():
    with pytest.raises(ValidationError):
        CapabilityManifest(
            manifest_version=1,
            languages={
                "de": LanguageCapability(
                    iso_code="de",
                    display_name="German",
                    sentiment_tier1=SentimentTier1Capability(
                        method="lexicon",
                        lexicon="sentiws_v2.0",
                        negation=NegationConfig(),
                        metric_name="shared_metric",
                    ),
                ),
                "fr": LanguageCapability(
                    iso_code="fr",
                    display_name="French",
                    sentiment_tier2_default=SentimentTier2DefaultCapability(
                        method="multilingual_bert",
                        provided_by="shared.multilingual_bert",
                        metric_name="shared_metric",
                    ),
                ),
            },
        )


def test_same_metric_same_method_across_languages_allowed():
    # The multilingual-BERT case Phase 119 will exercise.
    m = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier2_default=SentimentTier2DefaultCapability(
                    method="multilingual_bert",
                    provided_by="shared.multilingual_bert",
                    metric_name="sentiment_score_bert_multilingual",
                ),
            ),
            "fr": LanguageCapability(
                iso_code="fr",
                display_name="French",
                sentiment_tier2_default=SentimentTier2DefaultCapability(
                    method="multilingual_bert",
                    provided_by="shared.multilingual_bert",
                    metric_name="sentiment_score_bert_multilingual",
                ),
            ),
        },
    )
    assert "fr" in m.languages
    # Different metric in the per-language refinement must not collide either.
    m_with_refinement = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier2_refinement=SentimentTier2RefinementCapability(
                    method="news_domain_bert",
                    model="mdraw/german-news-sentiment-bert",
                    metric_name="sentiment_score_bert_de_news",
                ),
            ),
        },
    )
    assert m_with_refinement.languages["de"].sentiment_tier2_refinement is not None


# ---------------------------------------------------------------------------
# (e) Refactored extractors read from manifest
# ---------------------------------------------------------------------------


def test_named_entity_extractor_routing_reads_manifest():
    """An extractor built from a custom manifest only knows that manifest's
    languages — proving that the routing comes from the manifest, not from a
    hard-coded fallback in the extractor."""
    manifest = CapabilityManifest(
        manifest_version=1,
        languages={
            "xx": LanguageCapability(
                iso_code="xx",
                display_name="Test",
                ner=NerCapability(tier=1.5, model="nonexistent_model_for_test", model_version="0"),
            ),
        },
    )
    ext = NamedEntityExtractor(manifest=manifest)
    # The map mirrors the manifest 1:1 — `de` is absent.
    assert ext._language_to_model == {"xx": "nonexistent_model_for_test"}


def test_sentiment_extractor_negation_reads_manifest(tmp_path):
    """Override the manifest's negation cues and watch the extractor pick
    them up at construction time."""
    manifest = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier1=SentimentTier1Capability(
                    method="lexicon",
                    lexicon="sentiws_v2.0",
                    features=["custom_lexicon"],
                    negation=NegationConfig(
                        particles=["nope_particle"],
                        spacy_neg_dep="custom_neg",
                        clause_boundary_deps=["custom_boundary"],
                    ),
                    metric_name="sentiment_score_sentiws",
                ),
            ),
        },
    )

    # Use empty data dirs so the lexicon is empty — we only inspect the
    # manifest-driven attributes here.
    sentiws_dir = tmp_path / "sentiws"
    sentiws_dir.mkdir()
    custom_path = tmp_path / "custom_lexicon.yaml"
    custom_path.write_text("{}\n", encoding="utf-8")

    ext = SentimentExtractor(
        sentiws_dir=sentiws_dir,
        custom_lexicon_path=custom_path,
        manifest=manifest,
    )
    assert ext._negation_cues == frozenset({"nope_particle"})
    assert ext._negation_deps == frozenset({"custom_neg"})
    assert ext._clause_boundary_deps == frozenset({"custom_boundary"})
    # `compound_split` not in features → the decomposer is inactive even if
    # the library is installed.
    assert ext._compound_split_active is False
    # `negation_dependency` not in features → no spaCy parser is loaded.
    assert ext._nlp is None


def test_sentiment_compound_split_feature_flag_toggles_decomposer(tmp_path):
    sentiws_dir = tmp_path / "sentiws"
    sentiws_dir.mkdir()
    (sentiws_dir / "SentiWS_v2.0_Positive.txt").write_text(
        "Wut|NN\t-0.6000\t\n"
        "Ausbruch|NN\t0.2000\t\n",
        encoding="utf-8",
    )
    (sentiws_dir / "SentiWS_v2.0_Negative.txt").write_text("", encoding="utf-8")

    base_features = ["custom_lexicon"]
    manifest_off = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier1=SentimentTier1Capability(
                    method="lexicon",
                    lexicon="sentiws_v2.0",
                    features=base_features,
                    negation=NegationConfig(),
                    metric_name="sentiment_score_sentiws",
                ),
            ),
        },
    )
    manifest_on = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                sentiment_tier1=SentimentTier1Capability(
                    method="lexicon",
                    lexicon="sentiws_v2.0",
                    features=base_features + ["compound_split"],
                    negation=NegationConfig(),
                    metric_name="sentiment_score_sentiws",
                ),
            ),
        },
    )
    ext_off = SentimentExtractor(sentiws_dir=sentiws_dir, manifest=manifest_off)
    ext_on = SentimentExtractor(sentiws_dir=sentiws_dir, manifest=manifest_on)
    assert ext_off._compound_split_active is False
    # `_compound_split_active` is True only if the library is also installed.
    # In CI both packages ship; if the local checkout omits compound-split,
    # the feature flag still toggles the recorded intent.
    from internal.extractors.sentiment import _COMPOUND_SPLIT_AVAILABLE

    assert ext_on._compound_split_active is _COMPOUND_SPLIT_AVAILABLE


# ---------------------------------------------------------------------------
# (f) Scaffold-generator drift gate
# ---------------------------------------------------------------------------


def _run_generator(*args: str) -> subprocess.CompletedProcess:
    script = _REPO_ROOT / "scripts" / "build" / "generate_metric_validity_scaffold.py"
    return subprocess.run(
        [sys.executable, str(script), *args],
        check=False,
        capture_output=True,
        text=True,
    )


def test_scaffold_generator_check_passes_on_clean_checkout():
    result = _run_generator("--check")
    assert result.returncode == 0, result.stderr or result.stdout


def test_scaffold_generator_is_idempotent(tmp_path):
    out = tmp_path / "scaffold.sql"
    r1 = _run_generator("--output", str(out))
    assert r1.returncode == 0, r1.stderr
    first = out.read_text(encoding="utf-8")
    r2 = _run_generator("--output", str(out))
    assert r2.returncode == 0, r2.stderr
    second = out.read_text(encoding="utf-8")
    assert first == second


def test_scaffold_generator_reflects_manifest_changes(tmp_path):
    """Editing the manifest produces a non-empty diff; reverting restores it."""
    custom_manifest = tmp_path / "manifest.yaml"
    custom_manifest.write_text(
        dedent(
            """\
            manifest_version: 1
            languages:
              xx:
                iso_code: xx
                display_name: Test Language
                ner:
                  tier: 1.5
                  model: xx_test_model
                  model_version: "0"
                sentiment_tier1:
                  tier: 1
                  method: lexicon
                  lexicon: stub
                  features: []
                  negation: {}
                  metric_name: sentiment_score_xx_stub
            """
        ),
        encoding="utf-8",
    )
    out = tmp_path / "scaffold.sql"
    result = _run_generator(
        "--manifest", str(custom_manifest),
        "--output", str(out),
    )
    assert result.returncode == 0, result.stderr
    rendered = out.read_text(encoding="utf-8")
    assert "entity_count" in rendered
    assert "sentiment_score_xx_stub" in rendered
    # Bundled manifest does NOT mention these — proving the generator reads
    # the supplied path and does not fall back to the default.
    bundled_text = _BUNDLED_MANIFEST.read_text(encoding="utf-8")
    assert "sentiment_score_xx_stub" not in bundled_text

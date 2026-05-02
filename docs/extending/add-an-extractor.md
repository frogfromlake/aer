# Adding an Extractor

This guide answers: *"I want AĒR to compute a new metric — a new measurement, a new algorithm, a new analytical dimension — what do I touch?"*

Extractors are the workhorses of the Gold layer. Every metric the dashboard shows, every entity row in the Probe Dossier, every topic in the topic distribution — all flow from an extractor.

---

## Two protocols, two scopes

AĒR has two extractor types with explicit different scopes:

| Protocol | Input | Output | Cadence | Examples |
| :--- | :--- | :--- | :--- | :--- |
| **`MetricExtractor`** | Single Silver document | One or more `GoldMetric` rows + optional `GoldEntity` rows | Per-document, sync, in the analysis-worker pipeline | `SentimentExtractor`, `NamedEntityExtractor`, `TemporalDistributionExtractor`, `LanguageDetectionExtractor` |
| **`CorpusExtractor`** | A configurable rolling window of Silver documents | Bulk inserts to a Gold-layer table | NATS-cron on a weekly cadence | `EntityCoOccurrenceExtractor`, `TopicModelingExtractor` (Phase 120) |

Pick by the question your extractor answers:

- **Per-document fact** ("what is the sentiment of this article?", "what entities appear in this article?") → `MetricExtractor`.
- **Cross-document pattern** ("which entities co-occur in the corpus?", "what topics dominate the last 30 days?") → `CorpusExtractor`.

---

## What gets touched

### MetricExtractor

| Component | Change | Effort |
| :--- | :--- | :--- |
| `services/analysis-worker/internal/extractors/<your_extractor>.py` | New extractor class implementing the protocol | 2–6 h |
| `services/analysis-worker/main.py` | Register in the extractor list | 1 min |
| `services/analysis-worker/tests/test_extractor_<your>.py` | Pytest unit tests with deterministic inputs | 1–2 h |
| `services/bff-api/configs/metric_provenance.yaml` | Tier classification, algorithm description, known limitations, version hash anchor | 30 min |
| `infra/clickhouse/seed/metric_validity_scaffold.sql` | Initial `metric_validity` row with `validation_status='unvalidated'` | 5 min |
| Documentation: CLAUDE.md "Registered extractors" | One-paragraph description | 5 min |

**Realistic effort: 0.5–1.5 days for a Tier 1 deterministic metric.**

### CorpusExtractor

Everything from MetricExtractor plus:

| Component | Change | Effort |
| :--- | :--- | :--- |
| ClickHouse migration | New Gold table for the bulk output | 30 min |
| Cron trigger | NATS-cron schedule entry | 15 min |
| BFF API endpoint | New `GET /api/v1/<concept>/...` route + handler + OpenAPI spec | 2–4 h |
| Frontend cell (optional) | New view-mode cell consuming the endpoint | 1 day |
| Determinism / reproducibility tests | Especially for Tier 2 corpus extractors with model inference | 1–2 h |

**Realistic effort: 2–4 days for a Tier 2 corpus extractor with frontend integration.**

---

## Tier classification (ADR-016)

Every extractor must declare its tier. The tier governs validation expectations, version-hash provenance requirements, and dashboard rendering (Epistemic Weight per Brief §7.8):

| Tier | Definition | Reproducibility | Examples |
| :--- | :--- | :--- | :--- |
| **Tier 1** | Deterministic, no model inference, no randomness | Bit-for-bit reproducible given pinned data | `TemporalDistributionExtractor`, `LanguageDetectionExtractor`, SentiWS sentiment, dependency-parse-based negation handler |
| **Tier 1.5** | Deterministic, but uses external pre-built artefacts (alias DBs, lexicons) | Reproducible given the artefact hash | `NamedEntityExtractor` with Wikidata alias linking (Phase 118) |
| **Tier 2** | Model-based, deterministic with seeds and pinned model revision | Reproducible cross-process given pinned model + flags, but not bit-for-bit guaranteed across hardware (CUDA non-determinism) | `GermanNewsBertSentimentExtractor` (Phase 119), `TopicModelingExtractor` (Phase 120) |
| **Tier 3** *(reserved)* | Stochastic, requires variance reporting | Distributional reproducibility only | None yet |

ADR-016 mandates the **dual-metric pattern**: Tier 2 extractors register *alongside* their Tier 1 baselines, never replacing them. The dashboard renders both with Epistemic Weight distinguishing them. This is why Phase 119's BERT sentiment is `sentiment_score_bert`, not a replacement of `sentiment_score_sentiws`.

---

## The MetricExtractor protocol in detail

```python
# services/analysis-worker/internal/extractors/base.py

class MetricExtractor(Protocol):
    """Per-document metric extraction."""
    
    @property
    def name(self) -> str:
        """Extractor name for logging and provenance."""
        ...
    
    @property
    def version_hash(self) -> str:
        """
        Provenance anchor: a short hash uniquely identifying the extractor's
        algorithm, lexicon, model, and configuration. Written to
        SilverEnvelope.extraction_provenance per Phase 46. Required for ALL
        tiers. For Tier 1: hash of the lexicon contents. For Tier 2: 
        sha256({model_name}:{model_revision}:{transformers_version}:...).
        """
        ...
    
    def extract_all(
        self,
        core: SilverCore,
        article_id: str | None,
    ) -> ExtractionResult:
        """
        Compute metrics for a single document. Returns zero or more
        GoldMetric rows and zero or more GoldEntity rows. Empty result
        is valid — represents legitimate absence (e.g. language guard
        skipped extraction per Phase 116).
        
        MUST NOT raise on bad input. If the document is malformed, return
        an empty ExtractionResult and emit a structured warning log.
        Raising halts the pipeline for the document; AĒR's contract is
        graceful degradation.
        """
        ...
```

### Reference implementation: a hypothetical TextLengthExtractor

```python
class TextLengthExtractor:
    """Tier 1 deterministic metric: cleaned text length in characters."""
    
    @property
    def name(self) -> str:
        return "text_length"
    
    @property
    def version_hash(self) -> str:
        # Tier 1 with no external artefact: hash the algorithm version
        return hashlib.sha256(b"text_length_v1").hexdigest()[:16]
    
    def extract_all(self, core, article_id):
        if not core.cleaned_text:
            return ExtractionResult()  # Honest absence, not zero
        
        return ExtractionResult(metrics=[
            GoldMetric(
                timestamp=core.timestamp,
                source=core.source,
                metric_name="text_length_chars",
                value=float(len(core.cleaned_text)),
                article_id=article_id,
            )
        ])
```

Note the architectural discipline:

- Empty input → empty result, **not** a zero-valued metric. "No text" and "zero-length text" are different facts.
- Algorithm version in the hash, even for trivial extractors. This protects against silent algorithm drift.
- No raises. The pipeline continues even if this extractor encounters something unexpected.

---

## The CorpusExtractor protocol in detail

```python
class CorpusExtractor(Protocol):
    """Cross-document corpus-level extraction."""
    
    @property
    def name(self) -> str:
        ...
    
    @property
    def version_hash(self) -> str:
        ...
    
    def extract(
        self,
        window_start: datetime,
        window_end: datetime,
    ) -> int:
        """
        Read Silver documents in the window, compute corpus-level
        observations, write to the Gold layer. Returns the number
        of rows written. Idempotent — re-running with the same window
        produces the same result via ReplacingMergeTree's ingestion_version.
        
        Triggered by NATS-cron, not per-document NATS events.
        """
        ...
```

### Critical disciplines for CorpusExtractors

1. **Idempotency via ReplacingMergeTree.** Always write with an `ingestion_version` column (UInt64, monotonically increasing — `toUnixTimestamp(now())` works). Re-runs of the same window produce a new version that supersedes the previous; the latest version wins on read.

2. **Window discipline.** The window is a parameter, not a configuration. The cron job passes a window; the extractor honours it exactly. This makes back-fills and reprocessing trivial.

3. **Determinism (Tier 2 specifically).** Set `random_state=42` on UMAP, `seed=42` on HDBSCAN, `torch.manual_seed(42)` etc. for any model-based extractor. The determinism CI gate (per Phase 119 pattern) catches drift.

4. **Per-language partitioning.** Per WP-004 §3.4, corpus extractors that operate on *content* (topics, embedding clusters) must partition by `detected_language` before fitting. Cross-language alignment is an explicit out-of-scope — it is a manual scientific workflow per WP-004.

---

## metric_provenance.yaml entry

Every new metric needs a provenance entry consumed by `GET /api/v1/metrics/{name}/provenance` (Phase 67, ADR-017 Principle 1). Skeleton:

```yaml
# services/bff-api/configs/metric_provenance.yaml
metrics:
  text_length_chars:
    tier: 1
    algorithm: "Cleaned-text character count via Python `len()`."
    known_limitations:
      - "Counts characters including whitespace and punctuation."
      - "No semantic interpretation — text length does not correlate with informativeness."
    extractor_module: "extractors.text_length"
    cultural_context_notes: "Language-independent. No cultural calibration required."
```

If your metric *does* have language-dependent calibration (most sentiment metrics), include the per-language notes here. The provenance endpoint exposes this to consumers; the dashboard methodology tray renders it (Phase 108).

---

## metric_validity scaffold

Every new metric ships with a row in `aer_gold.metric_validity` declaring its initial validation state. For an unvalidated Tier 1 metric:

```sql
-- infra/clickhouse/seed/metric_validity_scaffold.sql
INSERT INTO aer_gold.metric_validity
  (metric_name, context_key, validation_date, alpha_score, correlation,
   n_annotated, error_taxonomy, valid_until)
VALUES
  ('text_length_chars', '*:*:*', now(), NULL, NULL, 0,
   '{"validation_status": "unvalidated", "reason": "no annotation study yet — engineering default"}',
   toDateTime('2099-12-31 00:00:00'));
```

`context_key='*:*:*'` means "applies to all language × source-type × discourse-function contexts". For metrics that *are* language-bound (sentiment), use specific context keys: `'de:rss:epistemic_authority'`.

The scaffold is the architecturally honest default. The metric goes live as `unvalidated` and consumers see the limitation. Promoting to `validated` is an out-of-band scientific workflow (Workflow 2 in the Scientific Operations Guide), not an engineering action.

---

## What you do *not* do

- **Do not silence errors.** `extract_all` returns empty results on bad input; it does not log-and-pretend-everything-is-fine. Use structured warnings.
- **Do not skip the version hash.** Even for trivial extractors. Without a hash, you cannot detect algorithm drift.
- **Do not invent confidence scores you cannot defend.** If your metric has a confidence dimension, document the heuristic in the extractor docstring and in `error_taxonomy`. WP-002's recommendation: prefer absence of confidence (caller assumes 1.0) over fabricated confidence.
- **Do not bypass the dual-metric pattern for Tier 2.** Adding a BERT-based variant of an existing Tier 1 metric: register under a new metric name, never overwrite. The Tier 1 baseline stays available.
- **Do not put corpus-level logic in MetricExtractors.** Per-document extractors that secretly maintain global state are a maintenance trap. If you need cross-document logic, use CorpusExtractor.

---

## Cross-references

- [ADR-016: Hybrid Tier Architecture](../arc42/09_architecture_decisions.md) — the tier classification system.
- [ADR-017: Reflexive Architecture](../arc42/09_architecture_decisions.md) — Principle 1 (Methodological Transparency) drives the provenance requirement.
- [Scientific Operations Guide → Workflow 2 (Validating a Metric)](../scientific_operations_guide.md) — the canonical procedure for promoting a metric to `validated`.
- [Arc42 §13.3: Tier 1 / Tier 2 Method Inventory](../arc42/13_scientific_foundations.md) — the running register of which methods fall in which tier.
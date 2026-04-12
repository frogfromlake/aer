# Scientific Operations Guide

> **Audience.** This document is the bridge between AĒR's two operational audiences:
>
> * The **Operations Playbook** answers *"what do I type?"* — commands, schemas, file paths.
> * The **Working Papers** (`docs/methodology/`) answer *"why this methodology?"* — theoretical and empirical justification.
>
> This guide answers the question that sits between them: **"At which points does scientific judgment enter the AĒR pipeline, who is responsible, what is the procedure, and where does the result land?"**
>
> Each workflow is end-to-end. It names the trigger, the role responsible, the Working Paper that anchors the procedure, the technical steps (with a deep link into the Operations Playbook for the exact commands), the templates to use, the outputs that get persisted, and a concrete walkthrough using **Probe 0 — German Institutional RSS** with real values.

---

## Table of Contents

1. [Workflow 1: Classifying a New Probe](#workflow-1-classifying-a-new-probe) — WP-001 §4.4
2. [Workflow 2: Validating a Metric](#workflow-2-validating-a-metric) — WP-002 §6.2
3. [Workflow 3: Establishing Metric Equivalence](#workflow-3-establishing-metric-equivalence) — WP-004 §5.2
4. [Workflow 4: Computing and Updating Baselines](#workflow-4-computing-and-updating-baselines) — WP-004 §6.1
5. [Workflow 5: Assessing Bias for a Data Source](#workflow-5-assessing-bias-for-a-data-source) — WP-003 §8.1
6. [Workflow 6: Updating the Cultural Calendar](#workflow-6-updating-the-cultural-calendar) — WP-005 §4.3
7. [Probe 0 End-to-End Walkthrough](#probe-0-end-to-end-walkthrough)
8. [Provenance Inventory](#provenance-inventory)

Each workflow heading is a stable anchor and is referenced by the corresponding section of the [Operations Playbook](operations_playbook.md). The link is bidirectional: every Playbook section that touches a scientific touchpoint contains a "Scientific rationale" pointer back into this guide.

---

## Workflow 1: Classifying a New Probe

**Anchor:** `#workflow-1-classifying-a-new-probe`
**Working Paper:** WP-001 §4.4 — *Five-Step Probe Classification Process*
**Roles:** Area Expert, Peer Reviewer, Engineering Lead, Ethical Reviewer
**Templates:** [`docs/templates/probe_registration_template.yaml`](templates/probe_registration_template.yaml), [`docs/templates/observer_effect_assessment.yaml`](templates/observer_effect_assessment.yaml)
**Outputs:**

* New row in PostgreSQL `source_classifications`
* New Probe Dossier directory under `docs/probes/<probe-id>/`
* New entry in `services/bff-api/configs/metric_provenance.yaml` *only if* a new metric is introduced alongside the probe
* `mkdocs.yml` Probes navigation entry

### Trigger

Any of the following:

* A research collaborator proposes a new data source that AĒR does not yet ingest.
* The engineering team adds a source for pipeline calibration purposes (e.g., the rationale that produced Probe 0; see [Arc42 §13.10](arc42/13_scientific_foundations.md)).
* An existing source's discourse function is reclassified (this produces a *new* row in `source_classifications`, never an UPDATE — the composite primary key `(source_id, classification_date)` is designed for temporal tracking).

### The Five Steps (WP-001 §4.4)

| Step | Activity | Role | Artifact |
| :--- | :--- | :--- | :--- |
| 1 | **Area Expert Nomination.** A domain specialist nominates the source, asserts a `primary_function` (and optionally `secondary_function`), and writes the `emic_designation` and `emic_context`. | Area Expert | Filled `probe_registration_template.yaml` |
| 2 | **Peer Review.** A second domain specialist reviews the nomination. Disagreements are documented in the registration template under `peer_review_notes`. Quantification of `function_weights` (e.g. `{"primary": 0.7, "secondary": 0.3}`) requires both Steps 1 and 2 to be complete — until then, `function_weights` stays `NULL`. | Peer Reviewer | Updated registration template |
| 3 | **Technical Feasibility.** Engineering assesses crawler viability — feed availability, format, terms of service, rate limits, expected volume, authentication, deduplication strategy. | Engineering Lead | Notes appended to the registration template |
| 4 | **Ethical Review.** The proposer (with the ethical reviewer) fills out [`observer_effect_assessment.yaml`](templates/observer_effect_assessment.yaml). The completed assessment is committed to the Probe Dossier as `observer_effect.md`. | Ethical Reviewer | `docs/probes/<probe-id>/observer_effect.md` |
| 5 | **Registration.** Engineering inserts a row into `source_classifications` with `review_status = 'provisional_engineering'` (or `'pending'` if all of Steps 1–4 are complete and the proposal awaits final sign-off), creates the Probe Dossier directory, and adds it to `mkdocs.yml`. | Engineering Lead | PostgreSQL row + dossier files |

### Technical Steps (Operations Playbook references)

* **PostgreSQL INSERT:** see [Operations Playbook → Source Classifications (WP-001)](operations_playbook.md#source-classifications-wp-001) for the SQL template, the Probe 0 example, and the rules for advancing a row through `provisional_engineering` → `pending` → `reviewed` (or `contested`).
* **Probe Dossier creation:** see [Operations Playbook → Probe Dossier](operations_playbook.md#probe-dossier) for the mandatory file list and the kebab-case slug convention.
* **Metric Provenance entry (only if a new metric is registered alongside the probe):** see [Operations Playbook → Metric Provenance Config](operations_playbook.md#metric-provenance-config).

### `review_status` Lifecycle

```
provisional_engineering   →   pending   →   reviewed
                                       ↘   contested
```

* **`provisional_engineering`** — engineering team set the classification without expert nomination or peer review. `function_weights` is `NULL`. Metrics from sources in this state must be marked provisional in any consumer-facing context. This is the state Probe 0 is currently in.
* **`pending`** — Steps 1–4 are complete and the row is awaiting final sign-off.
* **`reviewed`** — A complete review cycle has been performed. `function_weights` is populated.
* **`contested`** — A reviewer has formally objected to the classification. The probe remains usable but consumers are alerted via the `validation_status` field.

Why `function_weights = NULL` until Steps 1–2 complete: weights are a quantitative claim about the share of a source's output that performs each discourse function. WP-001 §4.4 holds that this claim is only meaningful after at least two independent domain specialists have agreed on the assignment — anything else would be an engineering guess masquerading as a measurement.

### Probe 0 Walkthrough

Probe 0 was classified **out of order**: registration (Step 5) preceded expert nomination and peer review (Steps 1–2) because Probe 0 was a deliberate engineering calibration probe, not a research-motivated probe. The chronology is recorded in [Arc42 §13.10](arc42/13_scientific_foundations.md) and reflected in the dossier.

| Step | Status | Notes |
| :--- | :--- | :--- |
| 1 — Area Expert Nomination | **outstanding** | No domain specialist has been engaged. The classification (`epistemic_authority` for `tagesschau.de`, `power_legitimation` for `bundesregierung.de`) is an engineering judgement. |
| 2 — Peer Review | **outstanding** | Therefore `function_weights = NULL` for both sources. |
| 3 — Technical Feasibility | **complete** | Both feeds are public, low-volume, free of authentication and engagement signals, and the RSS adapter exists. See [`docs/probes/probe-0-de-institutional-rss/README.md`](probes/probe-0-de-institutional-rss/README.md). |
| 4 — Ethical Review | **complete** | [`docs/probes/probe-0-de-institutional-rss/observer_effect.md`](probes/probe-0-de-institutional-rss/observer_effect.md). |
| 5 — Registration | **complete (as `provisional_engineering`)** | Migration `infra/postgres/migrations/000006_probe_0_classification.up.sql` inserted both rows; the Probe Dossier exists. |

The two `source_classifications` rows produced by Migration 000006 (one shown — see [Operations Playbook → Source Classifications](operations_playbook.md#source-classifications-wp-001) for the second):

```sql
-- Probe 0, source: tagesschau.de
-- This is a real row in production PostgreSQL.
SELECT source_id, primary_function, secondary_function,
       function_weights, review_status
  FROM source_classifications
 WHERE source_id = (SELECT id FROM sources WHERE name = 'tagesschau');
-- → epistemic_authority | power_legitimation | NULL | provisional_engineering
```

**Open Probe 0 actions** (these are the unblocking steps for the `provisional_engineering` → `pending` transition):

1. Engage at least two independent area experts for German public broadcasting / institutional discourse to perform Steps 1–2 against the existing classification.
2. Quantify `function_weights` from the expert/peer review process and INSERT a *new* row (do not UPDATE — the composite primary key tracks temporal transitions).
3. Probe 0's exit criteria are documented in the dossier `README.md` under "Exit Criteria".

---

## Workflow 2: Validating a Metric

**Anchor:** `#workflow-2-validating-a-metric`
**Working Paper:** WP-002 §6.2 — *Five-Step Validation Protocol*
**Roles:** Annotation Lead, Annotators (≥ 3, independent), Methodologist
**Template:** [`docs/templates/validation_study_template.yaml`](templates/validation_study_template.yaml)
**Output:** Row in `aer_gold.metric_validity` with `(metric_name, context_key, alpha_score, correlation, n_annotated, error_taxonomy, valid_until)`.

### Trigger

* A metric is currently `unvalidated` in the BFF response (`GET /api/v1/metrics/available`) and a research collaborator wants to use it for substantive claims.
* An existing validation row's `valid_until` date is approaching expiry.
* A new context (language × source type × discourse function) requires its own validation — validity does not transfer across contexts (WP-002 §6.2).

### The Five Steps (WP-002 §6.2)

1. **Annotation Study.** ≥ 3 independent annotators annotate a representative sample. Krippendorff's alpha must be ≥ 0.667 (WP-002 §6.2 acceptance threshold). Capture the sample size, instructions, and disagreement data in `validation_study_template.yaml`.
2. **Baseline Comparison.** Run the metric against the annotated sample. Compute correlation (Pearson or Spearman, whichever fits the metric scale) between the metric output and the annotator consensus.
3. **Error Taxonomy.** Classify the metric's failures into named categories. For SentiWS sentiment, the taxonomy already used in `metric_provenance.yaml` is `negation_failure`, `compound_failure`, `genre_drift`, `no_compositionality`, `irony_blindness`. The taxonomy is stored as a semicolon-separated list in `metric_validity.error_taxonomy`.
4. **Cross-Context Transfer.** Document what counts as "another context" for this metric. The default `context_key` is `<lang>:<source_type>:<discourse_function>`, e.g. `de:rss:epistemic_authority`. A validation result for `de:rss:epistemic_authority` does **not** transfer to `de:rss:power_legitimation` — both require separate studies.
5. **Longitudinal Stability.** Re-run the metric on the same sample after a minimum 6-month window to detect concept drift. Set `valid_until` accordingly (typical: 12 months from the validation date).

### Technical Steps (Operations Playbook references)

* **INSERT into `metric_validity`:** see [Operations Playbook → Metric Validity (WP-002)](operations_playbook.md#metric-validity-wp-002) for the SQL template and the hypothetical Probe 0 example.
* **What happens at expiry:** when `valid_until` passes, the BFF API automatically reverts the metric to `validation_status = 'unvalidated'` on the next request. There is no scheduled job — the join in the BFF query handles it. No manual cleanup is required.
* **Verifying the BFF reflects the new row:** `curl -H "X-API-Key: $BFF_API_KEY" http://localhost:8080/api/v1/metrics/<metric_name>/provenance` and confirm `validationStatus` is `validated`.

### Probe 0 Walkthrough (hypothetical)

> **No actual validation study has been performed for any Probe 0 metric.** Every metric reports `validation_status = 'unvalidated'` in production. The walkthrough below is a complete dry-run of what Workflow 2 would produce for `sentiment_score` against the `de:rss:epistemic_authority` context.

| Step | Hypothetical activity for `sentiment_score` × `de:rss:epistemic_authority` |
| :--- | :--- |
| 1 | Sample 250 `tagesschau.de` documents stratified by topic (politics, economy, sports, culture). Three annotators label each on a 5-point polarity scale. Krippendorff's α = 0.71. Sample, instructions, and per-document agreement go into `validation_study_template.yaml`. |
| 2 | Run `SentimentExtractor` over the same 250 documents. Spearman correlation between metric and annotator consensus = 0.62 (moderate; interpretable but not strong). |
| 3 | Errors fall into three categories: `negation_failure` (28% of mismatches), `compound_failure` (19%), `genre_drift` (53%) — the dominant failure mode is the lexicon being calibrated on news/product reviews, not formal institutional discourse. |
| 4 | `context_key = 'de:rss:epistemic_authority'`. The result does not transfer to `de:rss:power_legitimation` (i.e. `bundesregierung.de`) — that source has a different register and would need its own study. |
| 5 | `valid_until = 2027-04-12` (12 months). Re-run scheduled for that date. |

The resulting INSERT (the same statement appears in [Operations Playbook → Metric Validity](operations_playbook.md#metric-validity-wp-002), marked as hypothetical):

```sql
INSERT INTO aer_gold.metric_validity
    (metric_name, context_key, validation_date, alpha_score, correlation,
     n_annotated, error_taxonomy, valid_until)
VALUES
    ('sentiment_score', 'de:rss:epistemic_authority', now(),
     0.71, 0.62, 250,
     'negation_failure;compound_failure;genre_drift',
     toDate('2027-04-12'));
```

After this INSERT, `GET /api/v1/metrics/sentiment_score/provenance` would return `validationStatus = 'validated'` and the limitation list from `metric_provenance.yaml` would still be returned alongside it — the provenance config and the validation status are independent fields.

---

## Workflow 3: Establishing Metric Equivalence

**Anchor:** `#workflow-3-establishing-metric-equivalence`
**Working Paper:** WP-004 §5.2 — *Three Levels of Cross-Cultural Equivalence*
**Roles:** Methodologist, Cross-Cultural Comparability Lead
**Output:** Row in `aer_gold.metric_equivalence` with `(etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence)`.

### Trigger

Two probes from different cultural contexts produce the same metric and a downstream consumer wants to compare their values. WP-004 §5.2 requires an explicit equivalence claim before this comparison is allowed; the BFF API enforces this by returning HTTP 400 to `?normalization=zscore` requests for which no equivalence row exists.

### The Three Equivalence Levels (WP-004 §5.2)

| Level | Claim | Required Evidence |
| :--- | :--- | :--- |
| `temporal` | The metric tracks the *direction* of change consistently across cultures. | Time-series correlation of within-culture deviations from baseline; no requirement on absolute scale. |
| `deviation` | The metric tracks the *magnitude* of deviation from a culture-specific baseline. | Same as `temporal` plus comparable variance structure (e.g., similar coefficients of variation under stable conditions). |
| `absolute` | The metric values are directly comparable across cultures without normalization. | Strong evidence — typically not achievable for lexicon- or model-based metrics. |

The level is recorded in `metric_equivalence.equivalence_level`. The BFF normalization gate accepts any non-empty row regardless of level — the level itself is exposed via `GET /api/v1/metrics/available` so consumers can interpret the comparison appropriately.

### Technical Steps (Operations Playbook references)

* **INSERT into `metric_equivalence`:** see [Operations Playbook → Metric Baselines & Equivalence (WP-004)](operations_playbook.md#metric-baselines-equivalence-wp-004) for the SQL template.
* **The BFF normalization gate:** `?normalization=zscore` requires *both* a `metric_baselines` row (from Workflow 4) *and* a `metric_equivalence` row. Missing either yields HTTP 400 with a descriptive error message. Default behaviour (`?normalization=raw`) is unaffected.

### Probe 0 Walkthrough

**Workflow 3 is not applicable to Probe 0.** Probe 0 is monolingual (German) and monocultural (Germany). Cross-cultural equivalence is defined over *at least two* probes from distinct cultural contexts; a single probe cannot establish it.

The expected behaviour against Probe 0 sources:

```bash
# Expected: HTTP 400, "no equivalence row for metric_name=sentiment_score"
curl -H "X-API-Key: $BFF_API_KEY" \
     "http://localhost:8080/api/v1/metrics?source=tagesschau&metricName=sentiment_score&normalization=zscore"
```

This is **not a bug**. It is the validation gate from WP-004 §7.3 functioning as designed: refusing to serve a comparison whose validity has not been demonstrated. Workflow 3 will become relevant when AĒR ingests its second probe from a different cultural context.

---

## Workflow 4: Computing and Updating Baselines

**Anchor:** `#workflow-4-computing-and-updating-baselines`
**Working Paper:** WP-004 §6.1 — *Baseline as the Within-Culture Reference*
**Role:** Engineering Lead (with Methodologist for staleness assessment)
**Output:** Rows in `aer_gold.metric_baselines`, one per `(metric_name, source, language)`.

### Trigger

Any of the following:

* Significant corpus growth (rule of thumb: corpus has at least doubled since the last baseline computation).
* A new source added to an existing probe.
* A new metric registered for which no baseline yet exists.
* Periodic refresh — the baseline window in `compute_baselines.py` defaults to 90 days, so quarterly recomputation keeps the baselines aligned with the rolling window.

### How Baselines Are Computed

`scripts/compute_baselines.py` queries `aer_gold.metrics` for the configured `--window` (default 90 days), groups by `(metric_name, source, language)`, computes mean and standard deviation, and inserts one row per group into `aer_gold.metric_baselines`. Single-value groups produce `std = 0` and are inserted unchanged — downstream consumers must handle the divide-by-zero case.

Baseline staleness affects z-score reliability: as the corpus drifts, the baseline becomes a less accurate within-culture reference. There is no automated alarm — operators are expected to recompute on the triggers above.

### Technical Steps (Operations Playbook references)

* **Run the script:** see [Operations Playbook → Metric Baselines & Equivalence (WP-004)](operations_playbook.md#metric-baselines-equivalence-wp-004) for the exact CLI invocation, environment requirements, and the `--dry-run` flag.
* **Inspect existing baselines:** `SELECT * FROM aer_gold.metric_baselines FINAL ORDER BY metric_name, source;`

### Probe 0 Walkthrough

Probe 0 baselines **can be computed today** against the live data — this is the workflow with the lowest blocking dependency for Probe 0. Concrete invocation:

```bash
cd services/analysis-worker
python scripts/compute_baselines.py --metric word_count --window 90 --dry-run
# Inspect the printed rows, then run without --dry-run to insert.
python scripts/compute_baselines.py --metric word_count --window 90
```

Expected output shape (one line per `(metric, source, language)`; actual numbers depend on the corpus at runtime):

```
word_count | tagesschau      | de | mean=312.4 | std=158.7 | n=4500
word_count | bundesregierung | de | mean=487.2 | std=203.1 | n=450
```

Each line is one INSERT into `aer_gold.metric_baselines`. After the run:

```sql
SELECT metric_name, source, language, mean, std, n
  FROM aer_gold.metric_baselines FINAL
 WHERE metric_name = 'word_count'
 ORDER BY source;
```

Note: even after this baseline is computed, `?normalization=zscore` for Probe 0 sources still returns HTTP 400 — Workflow 3 (Equivalence) is the second precondition and cannot be satisfied for a single probe.

---

## Workflow 5: Assessing Bias for a Data Source

**Anchor:** `#workflow-5-assessing-bias-for-a-data-source`
**Working Paper:** WP-003 §8.1 — *BiasContext Documentation Framework*
**Roles:** Engineering Lead (objective platform fields), Domain Researcher (interpretive fields, where required)
**Outputs:**

* `BiasContext` field values set in the source adapter (`services/analysis-worker/internal/adapters/<source>.py`) — emitted into every Silver record
* Prose narrative in the Probe Dossier (`bias_assessment.md`) covering the structural biases that the structured fields cannot capture

### Trigger

A new source adapter is being authored, or an existing adapter is being audited as part of a probe re-classification.

### The Six `BiasContext` Fields

| Field | Type | Who fills it? |
| :--- | :--- | :--- |
| `platform_type` | enum | Engineering — objective protocol/platform identifier (`rss`, `twitter`, `mastodon`, ...) |
| `access_method` | enum | Engineering — describes how AĒR fetches data (`public_rss`, `authenticated_api`, ...) |
| `visibility_mechanism` | enum | Engineering for protocol-determined cases (`chronological` for RSS); Domain Researcher when the platform has algorithmic ranking (`algorithmic`, `engagement_weighted`, ...) |
| `moderation_context` | enum | Engineering when the protocol enforces it (`editorial` for RSS); Domain Researcher otherwise |
| `engagement_data_available` | bool | Engineering — feature presence/absence on the platform |
| `account_metadata_available` | bool | Engineering — feature presence/absence on the platform |

For RSS, all six fields can be set by Engineering alone because RSS is a constrained, well-understood protocol with no algorithmic amplification. For platforms with opaque ranking algorithms (e.g., recommendation feeds), `visibility_mechanism` and `moderation_context` may require domain expertise to set defensibly.

WP-003's "document, don't filter" principle: `BiasContext` is metadata, not an exclusion criterion. Bias is recorded so consumers can interpret metrics with awareness of selection effects — never to drop records.

### Technical Steps (Operations Playbook references)

* **`BiasContext` in adapter code:** the structured fields are set in the adapter (e.g. `services/analysis-worker/internal/adapters/rss.py`) and propagate via `RssMeta` into every Silver record. See [Operations Playbook → Analysis Worker (Python)](operations_playbook.md#analysis-worker-python).
* **Prose narrative:** lives in the Probe Dossier `bias_assessment.md`. See [Operations Playbook → Probe Dossier](operations_playbook.md#probe-dossier).

### Probe 0 Walkthrough

The six `BiasContext` values for Probe 0 RSS sources (set by `RssAdapter`, identical for both `tagesschau.de` and `bundesregierung.de` because the bias profile is determined by the RSS protocol, not the publisher):

| Field | Value |
| :--- | :--- |
| `platform_type` | `rss` |
| `access_method` | `public_rss` |
| `visibility_mechanism` | `chronological` |
| `moderation_context` | `editorial` |
| `engagement_data_available` | `false` |
| `account_metadata_available` | `false` |

The full prose treatment — including the five structural biases of the RSS protocol and the per-source biases (state-funding bias for `tagesschau.de`, government communication bias for `bundesregierung.de`) — is in [`docs/probes/probe-0-de-institutional-rss/bias_assessment.md`](probes/probe-0-de-institutional-rss/bias_assessment.md). For Probe 0, no domain-expertise split was needed: RSS is fully described by its protocol properties, and the per-source operator characteristics are publicly known.

---

## Workflow 6: Updating the Cultural Calendar

**Anchor:** `#workflow-6-updating-the-cultural-calendar`
**Working Paper:** WP-005 §4.3 — *Cultural Calendar as Interpretation Aid*
**Role:** Domain Researcher (with Engineering for the file commit)
**Output:** Updated YAML file under `configs/cultural_calendars/<region>.yaml`. *Currently a static lookup — not consumed by any service in the POC.*

### Trigger

* A new probe region is added — a new cultural calendar file is required.
* A known event has occurred or been scheduled (e.g., an announced election date).
* A previously listed movable feast needs its concrete year-specific date computed.

### Content

The calendar enumerates dates whose presence is expected to perturb discourse metrics. Categories: public holidays, elections, religious feasts, recurring major media events, commemorations. Each entry carries a qualitative `expected_discourse_effect` note.

### Format

```yaml
- date: "YYYY-MM-DD"        # or "MM-DD" with recurrence: annual
  recurrence: annual         # optional: annual | movable
  name: "<local name>"
  type: public_holiday       # public_holiday | election | media_event | commemoration
  expected_discourse_effect: "<qualitative note>"
```

### Technical Steps (Operations Playbook references)

See [Operations Playbook → Cultural Calendar Files](operations_playbook.md#cultural-calendar-files) for the file location, the per-entry schema, and the steps for adding a new region.

### Consumption

**Currently a static lookup.** The calendar is consulted manually by analysts when interpreting metric anomalies — no service in the POC reads it. WP-005 §4.3 anticipates a future integration where the calendar feeds into the `min_meaningful_resolution` interpretation and into temporal anomaly explanations on the BFF API; that work is out of scope until the visualization layer arrives.

### Probe 0 Walkthrough

The Probe 0 calendar is `configs/cultural_calendars/de.yaml`. It currently includes German federal public holidays (Neujahr, Tag der Arbeit, Tag der Deutschen Einheit, Weihnachten, ...), Easter-based movable feasts, federal election dates, and recurring media events (Berlinale, Frankfurter Buchmesse).

Adding a future entry — for example, the next Bundestag election:

```yaml
  - date: "2029-09-30"   # placeholder; actual date set by Bundeswahlleiter
    name: "Bundestagswahl 2029"
    type: election
    expected_discourse_effect: |
      6-week pre-election period: sustained volume increase, heavy
      concentration on party manifestos, candidate framing, and polling
      coverage. Discourse function shifts toward power_legitimation across
      all institutional sources.
```

After editing, bump `last_updated` at the top of the file. No service restart is required — nothing reads the file at runtime.

---

## Probe 0 End-to-End Walkthrough

This section reads the six workflows back-to-back as a chronological narrative for Probe 0. It is the answer to the question: *"If I am responsible for Probe 0, what is done, what can I do today, and what am I waiting on collaborators for?"*

| # | Workflow | Status | What it would take |
| :--- | :--- | :--- | :--- |
| 1 | **Workflow 1 — Classification** | **partially done** (`provisional_engineering`; Steps 3 + 4 + 5 complete; Steps 1–2 outstanding) | Engage two independent area experts on German institutional discourse. Quantify `function_weights`. INSERT a new `source_classifications` row with `review_status = 'reviewed'`. |
| 2 | **Workflow 5 — Bias Assessment** | **done** | `BiasContext` is set in `RssAdapter`; the prose narrative is in `bias_assessment.md`. |
| 3 | **Workflow 6 — Cultural Calendar** | **done** | `configs/cultural_calendars/de.yaml` is populated with the recurring entries. New entries should be appended as events are scheduled. |
| 4 | **Workflow 4 — Baseline Computation** | **can do now** | Run `python scripts/compute_baselines.py --metric word_count --window 90` against the live ClickHouse instance — the only Probe 0 workflow that requires no human collaborators and can be executed by Engineering today. |
| 5 | **Workflow 2 — Metric Validation** | **outstanding (requires collaborators)** | Recruit ≥ 3 annotators, design the annotation study per WP-002 §6.2, run it, INSERT into `aer_gold.metric_validity`. The hypothetical INSERT in Workflow 2 above is the target shape. |
| 6 | **Workflow 3 — Equivalence** | **not applicable** | A single probe cannot establish cross-cultural equivalence. This workflow becomes relevant only when AĒR ingests its second probe from a different cultural context. |

The current concrete unblocking action for Probe 0 is **Workflow 4** — Engineering can execute it without external dependencies, and it is the prerequisite (alongside future Workflow 3 work) for unlocking the `?normalization=zscore` query path.

---

## Provenance Inventory

Every value below is **manually set** rather than derived from the data. This is the complete inventory of scientific judgment encoded into Probe 0 today. It is a *living* table — every new manually set value across the system should be added here at the moment it is set.

| Value | Where (Table / File / Config) | Set by | Date | Authority | Review Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `primary_function = 'epistemic_authority'` (tagesschau) | PostgreSQL `source_classifications` | Engineering | 2026-04-11 | WP-001 §4.4 (Steps 1–2 outstanding) | `provisional_engineering` |
| `secondary_function = 'power_legitimation'` (tagesschau) | PostgreSQL `source_classifications` | Engineering | 2026-04-11 | WP-001 §4.4 (Steps 1–2 outstanding) | `provisional_engineering` |
| `function_weights = NULL` (tagesschau) | PostgreSQL `source_classifications` | Engineering (deliberate `NULL`) | 2026-04-11 | WP-001 §4.4 — quantification requires Steps 1–2 | `provisional_engineering` |
| `primary_function = 'power_legitimation'` (bundesregierung) | PostgreSQL `source_classifications` | Engineering | 2026-04-11 | WP-001 §4.4 (Steps 1–2 outstanding) | `provisional_engineering` |
| `secondary_function` (bundesregierung) | PostgreSQL `source_classifications` | Engineering | 2026-04-11 | WP-001 §4.4 (Steps 1–2 outstanding) | `provisional_engineering` |
| `function_weights = NULL` (bundesregierung) | PostgreSQL `source_classifications` | Engineering (deliberate `NULL`) | 2026-04-11 | WP-001 §4.4 | `provisional_engineering` |
| `BiasContext` six static fields (RSS, both sources) | `services/analysis-worker/internal/adapters/rss.py` | Engineering | Phase 64 | WP-003 §8.1 — protocol-determined for RSS | accepted as protocol fact |
| `known_limitations` for `word_count` (1 entry) | `services/bff-api/configs/metric_provenance.yaml` | Engineering | Phase 67 | WP-002 §3 | provisional |
| `known_limitations` for `sentiment_score` (5 entries) | `services/bff-api/configs/metric_provenance.yaml` | Engineering | Phase 67 | WP-002 §3 | provisional |
| `known_limitations` for `language_confidence` (3 entries) | `services/bff-api/configs/metric_provenance.yaml` | Engineering | Phase 67 | WP-002 §3 | provisional |
| `known_limitations` for `entity_count` (4 entries) | `services/bff-api/configs/metric_provenance.yaml` | Engineering | Phase 67 | WP-002 §3 + Arc42 Ch. 11 R-10 | provisional |
| `known_limitations` for `publication_hour` / `publication_weekday` | `services/bff-api/configs/metric_provenance.yaml` | Engineering | Phase 67 | WP-005 §3 — empty list (deterministic) | accepted |
| `min_meaningful_resolution` heuristic (tagesschau) | Probe Dossier `temporal_profile.md` | Engineering | Phase 68 | WP-005 §3.3 — heuristic from publication rate, not from a measured signal-to-noise study | provisional |
| `min_meaningful_resolution` heuristic (bundesregierung) | Probe Dossier `temporal_profile.md` | Engineering | Phase 68 | WP-005 §3.3 | provisional |
| Cultural Calendar entries (`de.yaml`) | `configs/cultural_calendars/de.yaml` | Engineering | 2026-04-12 | WP-005 §4.3 — public-domain federal calendar | factual |
| Probe Dossier `observer_effect.md` | `docs/probes/probe-0-de-institutional-rss/observer_effect.md` | Engineering (template completed) | Phase 68 | WP-006 §4 — `observer_effect_assessment.yaml` template | provisional |

**Update protocol.** Whenever a new manually set value enters the system — a new metric in `metric_provenance.yaml`, a new `BiasContext` field in a new adapter, a new Probe Dossier file, a new row in `source_classifications` — append a row to this table in the same commit. The provenance inventory is the answer to the auditor's question *"where did this number come from?"* and only stays accurate if it is updated atomically with the change.

---

## Cross-References

* [Operations Playbook](operations_playbook.md) — the "what to type" companion. Each section that touches a scientific touchpoint links back here under "Scientific rationale".
* [Operations Playbook → Scientific Touchpoints Index](operations_playbook.md#scientific-touchpoints-index) — table mapping every touchpoint to (Playbook section, this guide's workflow).
* [Working Papers](methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md) — the "why this methodology" companion.
* [Arc42 §8.15 — Probe Dossier Pattern](arc42/08_concepts.md) — cross-cutting concept that this guide operationalises.
* [Arc42 §13.10 — Probe 0 Source Selection Rationale](arc42/13_scientific_foundations.md) — the engineering-calibration justification for Probe 0.
* [Phase 68 templates](templates/probe_registration_template.yaml) — the YAML templates used by Workflows 1, 2, and the ethical review step.

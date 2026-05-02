# Probe 0 — German Institutional RSS

**Probe ID:** `probe-0-de-institutional-rss`
**Cultural region:** Germany (`DE`), German-language (`de`)
**Sources:** `tagesschau.de`, `bundesregierung.de`
**Status:** Engineering calibration probe — *not* scientifically motivated

---

## Purpose

Probe 0 is AĒR's first real data source. Its purpose is **engineering pipeline calibration**, not scientific representativeness or hypothesis testing. The two German institutional RSS feeds were chosen because they are public, structured, low-volume, in a single language, and free of authentication, engagement signals, and algorithmic amplification — the simplest possible end-to-end load for the Bronze → Silver → Gold pipeline.

The full rationale, including the explicit acknowledgment that source selection was driven by engineering convenience rather than research design, is documented in **Arc42 §13.10**. The Probe Classification Process (WP-001 §4.4) has *not* been executed for these sources; their `source_classifications` row carries `review_status = 'provisional_engineering'` and `function_weights = NULL`.

## Sources

| Source | Operator | Funding | Feed URL | Volume |
| :--- | :--- | :--- | :--- | :--- |
| `tagesschau.de` | ARD (public broadcasting) | Rundfunkbeitrag | `https://www.tagesschau.de/index~rss2.xml` | ≈ 50 articles/day |
| `bundesregierung.de` | Federal Press Office | Federal budget | `https://www.bundesregierung.de/breg-de/feed` | ≈ 5 articles/day |

## Engineering Calibration Status

The following decisions are recorded as **engineering defaults**, not scientific choices. They will be revisited under interdisciplinary CSS collaboration (Arc42 §13.5).

- **Etic classification** assigned by the engineering team without expert nomination or peer review (WP-001 §4.4 Steps 1–2 outstanding).
- **`function_weights = NULL`** — quantification of primary/secondary discourse function shares requires the missing classification process.
- **`BiasContext` values** are derived from the structural properties of the RSS protocol and from publicly known operator characteristics — not from independent platform audits.
- **`min_meaningful_resolution`** is a heuristic derived from publication-rate division rather than from a measured signal-to-noise study (WP-005 §3.3).
- **No validation studies** have been conducted for any extractor in the Probe 0 context. Every metric reports `validation_status = unvalidated`.

## Exit Criteria

Probe 0 graduates from `provisional_engineering` to `pending` (and ultimately `reviewed`) only when:

1. WP-001 §4.4 Steps 1–2 are completed for both sources (area expert nomination + peer review).
2. `function_weights` are quantified and inserted as a new `source_classifications` row (the composite primary key allows temporal tracking; old rows are not overwritten).
3. At least one validation study has been recorded in `aer_gold.metric_validity` for the `de:rss:epistemic_authority` and `de:rss:power_legitimation` context keys.
4. An ethical review has been completed against `docs/templates/observer_effect_assessment.yaml` (see `observer_effect.md`).

Until then, all metrics derived from Probe 0 must be presented to consumers as **provisional** via the `validation_status` field in `GET /api/v1/metrics/available` and the `/provenance` endpoint.

---

## WP Coverage Matrix

The Probe Dossier Pattern documents per-probe scientific context along the six Working Paper axes. Per-metric methodological provenance is system-wide (`metric_provenance.yaml`) and per-validation context is system-wide (`metric_validity`, `metric_baselines`, `metric_equivalence`) — those are referenced from this dossier rather than duplicated inside it.

| Working Paper | Topic | Probe-specific location |
| :--- | :--- | :--- |
| **WP-001** | Functional Probe Taxonomy | [`classification.md`](classification.md) |
| **WP-002** | Metric Validity & Sentiment Calibration | System-wide: `aer_gold.metric_validity` (per `(metric_name, context_key)` pair). No probe-specific file — validation studies are scoped to context keys, not probes. |
| **WP-003** | Platform Bias & Non-Human Actors | [`bias_assessment.md`](bias_assessment.md) |
| **WP-004** | Cross-Cultural Comparability | System-wide: `aer_gold.metric_baselines` and `aer_gold.metric_equivalence`. Cross-probe by definition — a single probe cannot establish equivalence. |
| **WP-005** | Temporal Granularity | [`temporal_profile.md`](temporal_profile.md) |
| **WP-006** | Observer Effect & Reflexivity | [`observer_effect.md`](observer_effect.md) |

---

## See Also

- Arc42 §13.10 — full rationale for Probe 0 source selection.
- Arc42 §8.15 — Probe Dossier Pattern as a cross-cutting concept.
- Operations Playbook → "Probe Dossier" section — how to author a new dossier.
- [Scientific Operations Guide](../../operations/scientific_operations_guide.md) — workflows that produce the data referenced from this dossier; see in particular the [Probe 0 End-to-End Walkthrough](../../operations/scientific_operations_guide.md#probe-0-end-to-end-walkthrough).

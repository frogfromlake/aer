# Visualization Guidelines — Non-Prescriptive Rendering of AĒR Metrics

**Status:** Requirements specification for future frontend work.
**Authority:** WP-006 §6.2 (Non-Prescriptive Visualization) and ADR-017.
**Audience:** Dashboard developers, data-journalism consumers of the BFF API.

AĒR's BFF API exposes metrics from a pipeline that is, by design, methodologically cautious. The dashboard that consumes this API must match that caution: visual choices can smuggle in normative judgments that the underlying data does not support. These guidelines translate the WP-006 §6.2 principles into concrete rules for any frontend — in-repo or third-party — that renders AĒR output.

They are rules, not suggestions. A dashboard that violates them is not a neutral view of the pipeline; it is a separate interpretive instrument that AĒR does not endorse.

---

## 1. Color

* **Use the `viridis` scale (or one of its siblings: `cividis`, `magma`, `inferno`) for all sequential encodings.** These scales are perceptually uniform, colorblind-safe, and — critically — carry no culturally-loaded valence.
* **Never use red/green encoding for sentiment, trust, or any metric whose interpretation is contested.** Red-green carries "bad/good" connotations in most Western dashboards and will be read that way whether or not the underlying metric supports that reading. It is also the most common colorblindness failure mode.
* **Do not hand-pick custom palettes that place "warm" colors on one extreme.** Warmth has cultural valence even without red/green.

## 2. Labels and Framing

* **Do not apply normative labels to metric values.** Sentiment scores are `-1.0`, `0.0`, `+1.0`, not `"negative"`, `"neutral"`, `"positive"`. If a label is required for accessibility, use the raw value.
* **Do not rank sources on a "quality" or "credibility" axis the underlying data does not support.** AĒR's `source_classifications` table stores discourse functions, not quality scores.
* **Do not invent thresholds.** "Significant spike" and "normal range" must come from the `metric_baselines` / `metric_validity` tables, not from the dashboard.

## 3. Uncertainty

* **Always show uncertainty alongside point estimates.** This means confidence bands, interquartile ranges, or explicit validation-status indicators — never a bare number.
* **Surface `validationStatus` from `/metrics/available` and `/metrics/{metricName}/provenance` in every view that displays a metric.** An `unvalidated` metric must be visually distinguishable from a `validated` one (e.g., via a status badge, hatched fill, or muted opacity). The user must not have to open a details panel to learn that a metric is provisional.
* **Expose `knownLimitations` in a reachable position.** Every metric view must link to or inline the limitations list from the provenance endpoint. One click away is the maximum distance.

## 4. Multiple Visualization Modes

* **Offer at least two modes for every aggregated view.** A time series should be available as both raw counts and z-score normalized (gated by `metric_equivalence`); a categorical breakdown should be available as both absolute and relative.
* **Do not pick a default that privileges one interpretive frame.** Where a choice must be made, default to the rawer, less interpreted mode.
* **Do not smooth by default.** Smoothing hides volatility, which is itself a signal. If smoothing is offered, it must be a user-initiated toggle with a visible indicator.

## 5. Provenance Surfacing

* **Every chart must expose the source's `documentation_url`.** The `/sources` endpoint returns this field for each source; the dashboard must make it reachable from any view that displays data from that source.
* **Metric provenance (tier, algorithm description, known limitations) must be reachable from every metric view.** This is the point of the `/metrics/{metricName}/provenance` endpoint — it is not enough to have the data available; it must be visible to the consumer at the moment of interpretation.

## 6. Refusal

Some questions cannot be answered from the current pipeline state. These include — but are not limited to:

* Cross-cultural comparisons for metrics without a `metric_equivalence` entry (the BFF already refuses these at the API layer; the dashboard must not route around that refusal).
* "Quality" or "credibility" rankings of sources.
* Causal claims from any current metric.

**The dashboard must refuse these questions rather than answering them with a plausible-looking but unsupported visualization.** Refusal is a feature, not a failure state.

---

## Relationship to the Backend

These guidelines constrain the frontend, not the backend. The BFF already enforces a subset of the same principles via validation gates (ADR-016): z-score normalization requires a registered baseline and equivalence entry, and is refused with HTTP 400 otherwise. The dashboard is expected to honor that refusal verbatim — it must not fall back to a local approximation when the backend refuses.

Violations of these guidelines should be caught in code review of the frontend repository, not at the API layer. The API cannot see how its responses are rendered.

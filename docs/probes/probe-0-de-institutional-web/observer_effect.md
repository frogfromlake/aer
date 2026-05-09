# Probe 0 — Observer Effect Assessment (WP-006)

Initial observer-effect assessment for the two Probe 0 sources, completed against the `docs/templates/observer_effect_assessment.yaml` template introduced in Phase 68. For Probe 0 the assessed risk is **low**: both sources are public RSS feeds with no authentication, no engagement signal, and no feedback channel through which AĒR's observation could perturb the observed discourse.

> **Status: provisional.** This assessment was completed by the engineering team rather than by a constituted ethics review board. It is recorded here so the pipeline can run while the WP-001 §4.4 Step 4 (Ethical Review) process catches up. See [Scientific Operations Guide → **Workflow 1, Step 4**](../../operations/scientific_operations_guide.md#workflow-1-classifying-a-new-probe).

---

## Cultural and Geographic Scope

| Field | Value |
| :--- | :--- |
| `cultural_region.region` | Germany |
| `cultural_region.country_codes` | `DE` |
| `cultural_region.language_communities` | `de` |
| `cultural_region.context_notes` | German-language institutional discourse. Both sources operate under German federal media law (Rundfunkstaatsvertrag for tagesschau.de; federal press regulations for bundesregierung.de). |

## Anticipated Effects

### Beneficial

| Effect | Beneficiary | Likelihood | Evidence |
| :--- | :--- | :--- | :--- |
| Improved transparency on institutional framing patterns over time | German-speaking analysts, journalism researchers | medium | Existing CSS literature on framing analysis of German public-service broadcasting establishes that systematic longitudinal observation produces interpretable findings. |
| Reproducible baseline for cross-comparison once additional probes (other languages / source types) are added | Cross-cultural discourse researchers | low (today) → medium (after second probe) | Requires WP-004 equivalence work; not realisable until probe-1 exists. |

### Harmful

| Effect | Affected party | Likelihood | Severity | Evidence |
| :--- | :--- | :--- | :--- | :--- |
| Re-identification of individual journalists via metadata patterns | Editorial staff at ARD / BPA | low | low | RSS feeds expose `author` strings only when the source publisher chooses to. The `RssAdapter` does not currently strip them, but the volume of bylines per author is too low to enable behavioral profiling at Probe 0 scale. |
| Mis-interpretation of provisional metrics as validated findings | Downstream consumers of the BFF API | medium | medium | The BFF surface (`validation_status`, `/provenance`) makes the provisional state machine-readable, but consumers must actually read it. Mitigated by the validation-gate pattern (HTTP 400 on `?normalization=zscore`). |

## Vulnerable Populations

No vulnerable populations are directly affected by Probe 0. Both sources are institutional publishers; neither carries user-generated content, minority-language reporting, or content from groups at elevated risk of re-identification.

This will *not* generalise to subsequent probes. The vulnerable-populations field exists in the template precisely because it must be re-evaluated for every new probe, not because it is presumed empty by default.

## Recommended Safeguards

| Safeguard | Addresses | Responsible role | Status | Reference |
| :--- | :--- | :--- | :--- | :--- |
| Drop author byline before Silver write | Re-identification of editorial staff | developer | deferred | `RssAdapter.harmonize` — currently retains `author` in `RssMeta`. Defer until a second probe with bylines actually exposes a behavioral surface. |
| Surface `validation_status` on every metric response | Mis-interpretation as validated | developer | implemented | `GET /api/v1/metrics/available` and `/provenance` (Phases 63, 67). |
| Document provisional status in every consumer-facing surface | Mis-interpretation as validated | developer | implemented | `metric_provenance.yaml` `known_limitations` per metric. |
| Refuse `?normalization=zscore` without an equivalence entry | False cross-cultural comparison | developer | implemented | Validation-gate pattern from ADR-016. |

## Reflexive Considerations (WP-006 §6)

- **Observer position.** The engineering team selected these sources for pipeline calibration, not for representativeness of German discourse. The assessment is therefore performed from an *engineering* standpoint and is explicitly not a substitute for an external ethics review.
- **Power asymmetry.** Both sources are large institutional publishers with substantial resources. The asymmetry runs the *opposite* direction from the typical observer/observed concern: AĒR is a small open-source project observing two of the most powerful information producers in Germany. Reflexivity here is about avoiding *over-claiming*, not about protecting the observed party from scrutiny.
- **Consent status.** Public-record. Both sources publish RSS feeds explicitly intended for syndication and downstream re-use. No consent infrastructure is required at the protocol layer.
- **Feedback to observed.** None planned for Probe 0. AĒR is currently a research instrument, not a public dashboard. If a public dashboard is built, the WP-006 governance principles (Principles 4–5) and the `docs/design/visualization_guidelines.md` non-prescriptive visualization rules apply.

## Review Outcome

| Field | Value |
| :--- | :--- |
| `decision` | `pending` |
| `conditions` | Convene an external review for any subsequent probe involving user-generated content, vulnerable populations, or non-public sources. |
| `reviewers` | (none — engineering team self-assessment) |
| `decision_date` | (pending) |
| `notes` | This dossier file is the placeholder until WP-001 §4.4 Step 4 (Ethical Review) is executed against an external panel. |

## Cross-References

- Template: `docs/templates/observer_effect_assessment.yaml`
- WP-006 §8.4 Q7 — observer effect assessment requirement
- WP-001 §4.4 Step 4 — Ethical Review
- ADR-017 — Reflexive Architecture
- [Scientific Operations Guide → **Workflow 1: Classifying a New Probe**, Step 4](../../operations/scientific_operations_guide.md#workflow-1-classifying-a-new-probe)

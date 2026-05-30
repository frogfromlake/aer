# Probe 1 — French Institutional Web

**Probe ID:** `probe-1-fr-institutional-web`
**Collection method:** Web crawl (Phase 122 / ADR-028) — single configurable binary, French config.
**Status:** Provisional engineering. Every Probe-1 metric reports `validation_status = unvalidated`; classifications are `provisional_engineering` (`function_weights = NULL`) pending WP-001 §4.4.

## Purpose

Probe 1 is AĒR's **first non-German cultural context** (Phase 123). It mirrors Probe 0's discourse-function coverage in a second polity so the two can later be compared on the shared multilingual Tier-2 backbone (Phase 124). It is a cross-cultural calibration probe, not a scientifically representative sample of French discourse.

## Sources

| Source | Operator | Funding | Primary function | Discovery surface | Volume |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **franceinfo** (`franceinfo.fr`) | France Télévisions / franceinfo | Public broadcaster | Epistemic Authority | dated news sitemap + RSS | high (continuous) |
| **elysee** (`elysee.fr`) | Présidence de la République | State | Power Legitimation | RSS feed only (publication sitemap unusable for rolling window — see bias_assessment #7) | low, event-driven bursts |

These mirror Probe 0's pair (tagesschau = EA, bundesregierung = PL). Cohesion & Identity and Subversion & Friction are **structurally unobserved on both probes** — the symmetry the Phase-124 equivalence grant requires.

## Source-selection rationale (live-audited 2026-05-29)

Two source choices diverge from the ROADMAP's literal naming. Both are collection-method decisions, disclosed here per WP-006 — see [`bias_assessment.md`](bias_assessment.md) for the full structural-bias treatment.

- **`francetvinfo.fr` → `franceinfo.fr`.** The ROADMAP named `francetvinfo.fr`; that domain 301-redirects to the publisher's canonical domain `franceinfo.fr` (same publisher, France Télévisions / franceinfo). No substantive change.
- **`gouvernement.fr` → `elysee.fr`.** The ROADMAP named `gouvernement.fr`; that domain 301-redirects to `info.gouv.fr` (the Premier-ministre / SIG government portal — the institutionally exact twin of bundesregierung's BPA). `info.gouv.fr` sits behind an **active Cloudflare JS bot-challenge** and is therefore not collectable by AĒR's polite, JS-free crawler (ADR-028); circumventing an active challenge would be adversarial and contrary to the Manifesto's non-surveillance posture. `elysee.fr` (Présidence de la République) was chosen instead — freely collectable via its RSS feed (its publication sitemap is unusable for rolling collection: every `<lastmod>` is the nightly regeneration time, not the article date), and a first-rank Power-Legitimation voice. The resulting **PL-locus asymmetry** (head of state vs. head of government) is a documented cross-cultural parameter (WP-004), not a defect.

## Engineering calibration status

- Source selection is engineering/pragmatic, not the outcome of the Manifesto's interdisciplinary Probe Principle dialogue.
- Etic classifications are `provisional_engineering`; `function_weights = NULL` until WP-001 §4.4 (expert nomination + peer review).
- NER ships on `fr_core_news_lg`; sentiment ships on the **shared multilingual backbone** (`cardiffnlp/twitter-xlm-roberta-base-sentiment`). French Tier-1 (FEEL) and Tier-2.5 (CamemBERT) are deferred — within-frame only, never the cross-probe basis (ADR-023 amendment).

## Exit criteria

Probe 1 graduates from `provisional_engineering` only when WP-001 §4.4 Steps 1–2 are completed and `function_weights` are populated, mirroring Probe 0's lifecycle (`provisional_engineering → pending → reviewed`).

## WP coverage matrix

| Working Paper | Topic | Probe-specific location |
| :--- | :--- | :--- |
| **WP-001** | Functional Probe Taxonomy | [`classification.md`](classification.md) |
| **WP-002** | Metric Validity & Sentiment Calibration | System-wide: `aer_gold.metric_validity` (per `(metric_name, context_key)`). No probe file. |
| **WP-003** | Platform Bias & Non-Human Actors | [`bias_assessment.md`](bias_assessment.md) |
| **WP-004** | Cross-Cultural Comparability | System-wide: `aer_gold.metric_baselines` / `metric_equivalence`. PL-locus asymmetry noted in [`classification.md`](classification.md) + [`bias_assessment.md`](bias_assessment.md). |
| **WP-005** | Temporal Granularity | [`temporal_profile.md`](temporal_profile.md) |
| **WP-006** | Observer Effect & Reflexivity | [`observer_effect.md`](observer_effect.md) |

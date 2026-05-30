# Probe 1 — Classification (WP-001)

> **Status:** `provisional_engineering` — engineering team assigned, no peer review. `function_weights = NULL` pending WP-001 §4.4.

This file is the human-readable mirror of the `source_classifications` rows seeded by migration `000021`. The migration is the machine-readable form; this file is the explanation. Both advance together when WP-001 §4.4 moves a row through `provisional_engineering → pending → reviewed`.

## franceinfo

| Field | Value |
| :--- | :--- |
| `primary_function` | `epistemic_authority` |
| `secondary_function` | `power_legitimation` |
| `function_weights` | `NULL` (requires WP-001 §4.4 Steps 1–2) |
| `emic_designation` | `franceinfo` |
| `emic_context` | Public broadcaster (France Télévisions / franceinfo). Norm-setting through the informational baseline; editorial independence under a public-service mandate. |
| `emic_language` | `fr` |
| `classified_by` | `WP-001/Probe-1` (engineering team) |
| `classification_date` | `2026-05-29` |
| `review_status` | `provisional_engineering` |

**Qualitative justification.** As the news service of the French public broadcaster, franceinfo sets an informational baseline that other French-language outlets reference — its primary function is *epistemic authority*. Its secondary function is *power legitimation*: public-service governance structurally shapes editorial framing. It is the structural twin of Probe 0's tagesschau.

## Élysée

| Field | Value |
| :--- | :--- |
| `primary_function` | `power_legitimation` |
| `secondary_function` | `epistemic_authority` |
| `function_weights` | `NULL` (requires WP-001 §4.4 Steps 1–2) |
| `emic_designation` | `Élysée (Présidence de la République)` |
| `emic_context` | Official communication channel of the President of the Republic. Structural power legitimation through head-of-state agenda-setting and framing in the French semi-presidential system. |
| `emic_language` | `fr` |
| `classified_by` | `WP-001/Probe-1` (engineering team) |
| `classification_date` | `2026-05-29` |
| `review_status` | `provisional_engineering` |

**Qualitative justification.** elysee.fr is the official communication channel of the head of state. Its primary function is *power legitimation* — head-of-state agenda-setting and framing (communiqués, allocutions, discours); its secondary function is *epistemic authority* by institutional position. It is the Power-Legitimation twin of Probe 0's bundesregierung.

## Cross-cultural note — the PL locus differs (WP-004)

The two probes are deliberately symmetric in discourse *function* (EA + PL, with CI/SF unobserved), but the **institutional locus** of executive Power Legitimation differs across the polities:

- **Germany (Probe 0):** head of **government** — the Federal Government / Bundespresseamt (`bundesregierung`).
- **France (Probe 1):** head of **state** — the Présidence (`elysee`), because France is a semi-presidential system *and* because the institutionally exact analogue (the SIG government portal, `info.gouv.fr`) is not collectable (Cloudflare bot-wall; see [`bias_assessment.md`](bias_assessment.md)).

This asymmetry is a recorded comparative parameter, not noise: it is itself a statement about where power-legitimation discourse is produced in each system. Cross-probe interpretation must hold it in view.

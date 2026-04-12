# Probe 0 — Classification (WP-001)

This document records the etic/emic discourse-function classification of the two Probe 0 sources. The values mirror the rows inserted by `infra/postgres/migrations/000006_seed_probe0_classification.up.sql` into the `source_classifications` table — this file is the human-readable explanation of those rows.

> **Status: `provisional_engineering`.** WP-001 §4.4 Steps 1–2 (area expert nomination, peer review) have not been executed. `function_weights = NULL` until that process produces quantified shares. See [Scientific Operations Guide → Workflow 1](../../scientific_operations_guide.md#workflow-1-classifying-a-new-probe) for the path from `provisional_engineering` to `reviewed`.

---

## tagesschau.de

| Field | Value |
| :--- | :--- |
| `primary_function` | `epistemic_authority` |
| `secondary_function` | `power_legitimation` |
| `function_weights` | `NULL` (requires WP-001 §4.4 Steps 1–2) |
| `emic_designation` | `Tagesschau` |
| `emic_context` | State-funded public broadcaster (ARD). Norm-setting through informational baseline. Editorial independence structurally influenced by inter-party proportional governance. |
| `emic_language` | `de` |
| `classified_by` | `WP-001/Probe-0` (engineering team) |
| `classification_date` | `2026-04-11` |
| `review_status` | `provisional_engineering` |

**Qualitative justification.** As a publicly funded broadcaster with a legal mandate for balanced reporting (Staatsvertrag), tagesschau.de's primary discourse function is *epistemic authority* — it sets the informational baseline that other German-language outlets reference, paraphrase, or rebut. Its secondary function is *power legitimation*: editorial independence is structurally bounded by an inter-party proportional governance model (Rundfunkrat / Verwaltungsrat), and the institutional dependency on the broadcasting fee creates an incentive structure favoring stability narratives. These two functions are not equally weighted, but the missing WP-001 §4.4 process means the system records `NULL` rather than a guessed split.

## bundesregierung.de

| Field | Value |
| :--- | :--- |
| `primary_function` | `power_legitimation` |
| `secondary_function` | `epistemic_authority` |
| `function_weights` | `NULL` (requires WP-001 §4.4 Steps 1–2) |
| `emic_designation` | `Bundesregierung` |
| `emic_context` | Official government communication channel. Structural power legitimation through agenda-setting and framing. |
| `emic_language` | `de` |
| `classified_by` | `WP-001/Probe-0` (engineering team) |
| `classification_date` | `2026-04-11` |
| `review_status` | `provisional_engineering` |

**Qualitative justification.** bundesregierung.de is the official communication channel of the German federal government. Its content is authored or approved by the Federal Press Office (BPA) and exists explicitly to set the government's agenda and framing of events — its primary discourse function is *power legitimation*. The secondary *epistemic authority* function follows from its institutional position: government statements are routinely cited as authoritative by downstream outlets even when they are explicitly partisan in framing. As with tagesschau.de, the relative weights are unspecified pending the WP-001 §4.4 process.

---

## What `provisional_engineering` Means

The `review_status` lifecycle is defined by the `chk_review_status` constraint in migration 000005:

```
provisional_engineering  →  pending  →  reviewed   (or  contested)
```

- **`provisional_engineering`** — Classification was assigned by the AĒR engineering team without expert nomination or peer review. Recorded so the pipeline can run, *not* a substitute for the formal WP-001 §4.4 process.
- **`pending`** — A WP-001 §4.4 process has been initiated; awaiting peer review.
- **`reviewed`** — Steps 1–4 are complete and `function_weights` are populated. The classification is canonical for the recorded `classification_date`.
- **`contested`** — A subsequent review disagrees with an earlier `reviewed` classification. Both rows remain in the table (composite PK `(source_id, classification_date)`); the `contested` row supersedes the older one only after a tie-break per Workflow 1.

## Cross-References

- Migration: `infra/postgres/migrations/000006_seed_probe0_classification.up.sql`
- Schema: `infra/postgres/migrations/000005_source_classifications.up.sql`
- WP-001 §4.4 — Five-step probe classification process
- [Scientific Operations Guide → **Workflow 1: Classifying a New Probe**](../../scientific_operations_guide.md#workflow-1-classifying-a-new-probe)
- [Operations Playbook → **Source Classifications (WP-001)**](../../operations_playbook.md#source-classifications-wp-001)

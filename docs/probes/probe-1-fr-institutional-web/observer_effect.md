# Probe 1 — Observer Effect Assessment (WP-006)

> **Status:** Provisional (engineering team self-assessment, awaiting external review).

## Cultural and geographic scope

| Field | Value |
| :--- | :--- |
| `cultural_region.region` | France |
| `cultural_region.country_codes` | `FR` |
| `cultural_region.language_communities` | `fr` |
| `cultural_region.context_notes` | French-language institutional discourse. franceinfo operates under French public-broadcasting law (France Télévisions); the Élysée is the constitutional communication of the Présidence de la République. |

## Anticipated effects

### Beneficial

| Effect | Beneficiary | Likelihood | Evidence |
| :--- | :--- | :--- | :--- |
| First cross-cultural calibration enabling DE↔FR comparison discipline | Researchers / methodology | high | Phase 124 equivalence work builds directly on this probe. |
| Transparent disclosure of source-selection constraints as a teaching surface | Analysts / public | medium | Per-source content cards (Phase 123) surface the rationale live in-app. |

### Harmful

| Effect | Affected party | Likelihood | Severity | Evidence |
| :--- | :--- | :--- | :--- | :--- |
| Mis-reading institutional voice as "French opinion" | Public | medium | medium | Mitigated by milieu-bias disclosure (`bias_assessment.md`) + `validation_status=unvalidated`. |
| Mis-comparison across the PL-locus asymmetry | Researchers | medium | medium | Mitigated by the WP-004 note in `classification.md` + `bias_assessment.md` + source cards. |

## Vulnerable populations

None directly. Both sources are institutional public data — a national public broadcaster and the head-of-state communication channel — with no re-identification surface (no user accounts, no engagement data, no comments).

## Recommended safeguards

| Safeguard | Addresses | Responsible role | Status | Reference |
| :--- | :--- | :--- | :--- | :--- |
| Surface `validation_status` on every metric response | Mis-interpretation as validated | developer | implemented | `GET /api/v1/metrics/*`. |
| Disclose source-selection rationale + bias per source | Hidden collection-method bias | developer | implemented | per-source content cards + `bias_assessment.md` (Phase 123). |
| Drop author byline before Silver write | Re-identification of editorial staff | developer | deferred | shared with Probe 0; revisit when a byline surface materially appears. |

## Reflexive considerations (WP-006 §6)

- **Observer position.** Source selection is engineering-driven and partly collection-method-constrained (Cloudflare). Disclosed, not hidden.
- **Power asymmetry.** Observing official state and public-broadcaster communication — institutions with their own substantial communicative power; the observation does not expose private individuals.
- **Consent status.** Public institutional content intended for public consumption; polite crawl honouring robots.txt.
- **Feedback to observed.** None in the POC.

## Silver eligibility

Both sources are seeded `silver_eligible = true` (migration `000022`) under the same WP-006 §7 source-class rationale as Probe 0: institutional public data, no re-identification risk, per Manifesto §VI.

## Review outcome

- **Decision:** provisional engineering approval for POC calibration use.
- **Conditions:** all metrics `unvalidated`; classifications `provisional_engineering`; external WP-006 review outstanding.
- **Reviewers:** Engineering Team (self-assessment).
- **Decision date:** 2026-05-29.

# Adding a Probe — Solo-Developer Quickstart

> **Audience.** A single engineer (the project's current state) who needs to add a new probe end-to-end without an interdisciplinary team. This guide is **not a substitute** for the [Scientific Operations Guide](../operations/scientific_operations_guide.md) Workflow 1 — it is an index that sequences the existing workflows for the solo case and makes the scientific honesty mechanism (`provisional_engineering` status) explicit.
>
> **Scientific honesty contract.** When you add a probe alone, you cannot perform Steps 1–2 of WP-001 §4.4 (Area Expert Nomination + Peer Review). The architecture already handles this: the row goes in as `review_status = 'provisional_engineering'` with `function_weights = NULL`, and every consumer of the metrics sees `validation_status = unvalidated`. **Do not invent expert names, do not fabricate function weights, do not change the review status.** The unblocking action is to engage external experts later — that is the only legitimate path to `pending` and `reviewed`.

---

## Prerequisites

Before you start adding a probe, confirm:

- [ ] **Phase 116 is complete.** Multilingual NLP foundation must be in production before any non-German probe — otherwise the new probe's documents are mis-processed by the German-only pipeline and the Gold-layer baselines are contaminated. (Even for a German Probe 1, Phase 116 is recommended because Probe 0 already contains English articles.)
- [ ] **Phase 115 is complete.** Cross-cultural normalization gate, refusal surfaces, and `equivalence_reviews` Postgres table must exist — otherwise the probe ships without the methodological discipline that protects it from naive cross-cultural absolute claims.
- [ ] **The probe scope decision is documented.** Before any code: write a short ADR-style note (or a bullet list in the dossier `README.md` draft) answering: *which cultural-linguistic frame, which two WP-001 discourse functions are minimally covered, why this frame and not another, what is the engineering exit criteria?* This is the equivalent of the rationale Arc42 §13.10 records for Probe 0.

---

## The Solo Sequence

Each step links to the canonical procedure. **Do not duplicate** content from those procedures here — read the linked sections and execute them. This guide only sequences.

### Step A — Scope decision (no code)

1. **Pick the cultural frame.** Document the choice in your scope note. WP-001 §5.1 MVPS criteria are the test: at least two of four discourse functions covered by the candidate sources, public RSS feeds with no ToS restriction, documentable emic context.
2. **Pick the sources.** For an engineering POC, the sources should mirror Probe 0's discourse-function coverage (Epistemic Authority + Power Legitimation) so cross-probe parallel-stream rendering has overlapping lanes. CI / SF coverage is legitimate scope for a later probe with a real ethical-review partner.
3. **Pick the probe ID.** Kebab-case slug, format `probe-N-<iso-region>-<descriptor>`. Example: `probe-1-fr-institutional-rss`.

### Step B — Fill the registration template (Workflow 1, partial)

1. **Copy** [`docs/templates/probe_registration_template.yaml`](../templates/probe_registration_template.yaml) to a working draft (do *not* commit yet).
2. **Fill** the sections you can fill yourself:
   - Probe identity, geographic scope, languages.
   - Source list, RSS URLs, expected volume, ToS check.
   - Discourse functions: assert `primary_function` per source as your engineering judgement. Leave `function_weights: null` — this is the signal that Steps 1–2 of WP-001 §4.4 are outstanding.
   - Emic designation: untranslated local name (e.g. `Service d'Information du Gouvernement`, not `French Government Information Service`).
   - Emic context: brief cultural-context paragraph in your own words.
3. **Leave outstanding** the `area_expert` and `peer_reviewer` sections empty. Add a comment: `# Outstanding — provisional_engineering status. Engage external experts to advance to 'pending'.`

### Step C — Ethical review (Workflow 1, Step 4 — adapted for solo)

1. **Copy** [`docs/templates/observer_effect_assessment.yaml`](../templates/observer_effect_assessment.yaml) into the to-be-created dossier directory as `observer_effect.md`.
2. **Fill it honestly.** WP-006 §5.2 is non-negotiable, but for institutional public-RSS sources the assessment is short — there is no vulnerable population, no quasi-identifier, no engagement signal. Probe 0's `observer_effect.md` is the reference template.
3. **If the candidate sources include any non-institutional content** (community forums, social media, activist blogs), **stop here.** You cannot perform a credible solo ethical review for those source types. Defer them to a probe with a real ethical-review partner. This is the architectural answer, not a workaround.

### Step D — Probe Dossier creation (Operations Playbook → Probe Dossier)

Follow the procedure in [`docs/operations_playbook.md` → Probe Dossier](../operations/operations_playbook.md#probe-dossier) verbatim:

1. `mkdir docs/probes/<probe-id>/`
2. Copy the five files from `docs/probes/../probes/probe-0-de-institutional-rss/` and replace the content. Keep the headings stable.
3. The `README.md` Exit Criteria section names the same four conditions as Probe 0's: Steps 1–2 of WP-001 §4.4, function-weight quantification, at least one validation study in the new context-keys, completed external ethical review.
4. Add the dossier to `mkdocs.yml` under the `Probes` nav entry.

### Step E — Engineering work (ROADMAP Phase 122 checklist)

The ROADMAP Phase 122 entry is the canonical engineering checklist. The solo developer executes the full list — there are no items that require a second person:

- [ ] PostgreSQL seed migration (sources + source_classifications, both `provisional_engineering`).
- [ ] New crawler binary in `crawlers/<probe-id>-rss/`, own `go.mod`, registered in `go.work` and Makefile.
- [ ] Source Adapter in `services/analysis-worker/internal/adapters/`, registered in `adapters/registry.py`.
- [ ] Probe Dossier content directory in `services/bff-api/configs/content/<probe-id>/`.
- [ ] spaCy model in `requirements.txt` (Phase 116 router picks it up automatically — no extractor change).
- [ ] Sentiment coverage: Tier 1 lexicon if available (Phase 117 pattern), Tier 2 BERT if available (Phase 119 pattern). Document gaps in `aer_gold.metric_validity` rather than skipping silently.
- [ ] PostgreSQL update of `sources.documentation_url` to point at the new dossier directory (mirrors Probe 0 migration `000008`).

### Step F — Validation

The Phase 122 validation block plus the cross-cutting checks from the Operations Playbook:

- [ ] `make lint && make test && make fe-check` green.
- [ ] `make crawl-<probe-id>` fetches articles and they reach `aer_gold.metrics`.
- [ ] Manual: probe appears as a second luminous point on the Surface I globe; Phase 114 scope bar composes Probe 0 + new probe and produces parallel streams in lanes.
- [ ] `GET /api/v1/probes/<probe-id>/equivalence` (Phase 115 endpoint) returns Level-1-only — empty equivalence registry until Phase 123 grants the temporal level.
- [ ] All metrics from the new probe report `validation_status = unvalidated` in `GET /api/v1/metrics/available`. This is correct.

### Step G — Honesty pass

After everything is green:

- [ ] **Provenance Inventory update.** Append rows to the table in [`docs/scientific_operations_guide.md` → Provenance Inventory](../operations/scientific_operations_guide.md#provenance-inventory) for every manually-set value: per-source `BiasContext` fields, `min_meaningful_resolution` heuristics, the Cultural Calendar entries (if you added one).
- [ ] **Cross-link from Probe 0's `README.md`** to the new probe (so a reader landing on Probe 0's dossier discovers the second probe).
- [ ] **ROADMAP update.** Mark Phase 122 as `[x] DONE` with the date, and note in the dossier README that Probe 1 is now operational. Do *not* mark Phase 123 — that is a separate phase that sequences the cross-probe operations.

---

## What you do *not* do alone

These items are deliberately out of scope for the solo developer. They are unblockers for advancing the probe from `provisional_engineering` to `reviewed`, not for adding it:

| Item | Why not solo | When to do it |
| :--- | :--- | :--- |
| WP-001 §4.4 Steps 1–2 (Area Expert + Peer Review) | Requires two independent domain specialists. Engineering judgement is *not* the same epistemic act. | When external experts are engaged. Apply the Workflow 1 update procedure (insert a *new* `source_classifications` row, do not UPDATE). |
| WP-002 validation studies | Requires ≥ 3 independent annotators and Krippendorff's alpha ≥ 0.667. | When an annotation team is funded or volunteered. |
| WP-004 Level 2/3 equivalence grants | Requires interdisciplinary methodological review. | Phase 123 grants Level 1 (temporal — always valid) for the first probe pair; Levels 2/3 are out-of-band scientific decisions. |
| Ethical review for non-institutional sources | Requires a credible second reviewer per WP-006 §5.2. | When an ethical-review partner is engaged. |

---

## Cross-references

- [Scientific Operations Guide → Workflow 1](../operations/scientific_operations_guide.md#workflow-1-classifying-a-new-probe) — the canonical end-to-end probe classification process.
- [Operations Playbook → Source Classifications (WP-001)](../operations/operations_playbook.md#source-classifications-wp-001) — the SQL templates.
- [Operations Playbook → Probe Dossier](../operations/operations_playbook.md#probe-dossier) — the dossier-creation procedure.
- [Probe 0 reference dossier](../probes/probe-0-de-institutional-rss/README.md) — the reference implementation. Copy this structure.
- [Arc42 §8.15 Probe Dossier Pattern](../arc42/08_concepts.md) — the cross-cutting concept that this guide operationalises for the solo case.
- [Arc42 §13.10 Probe 0 Source Selection Rationale](../arc42/13_scientific_foundations.md) — the engineering-calibration justification, the precedent for solo-developer probe addition.
- [ROADMAP Phase 122](https://github.com/frogfromlake/aer/blob/main/ROADMAP.md) — the canonical engineering checklist for Probe 1.
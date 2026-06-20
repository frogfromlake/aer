# Proposal: Probe / Source Terminology Reconciliation

> **Status: PROPOSAL — opens as a review PR, does not land with Phase 129.**
> Phase 129 (Documentation Sweep) is required to *open* this question as a review
> artifact, not to decide or implement it. This document is the decision input.
> Authored 2026-06-20. Supersedes the deferred placeholder in
> `docs/design/design_brief.md` §6 ("Path B decision, 2026-04-24") and the
> "post-Step-2 ADR" promised there.

## 1. The problem (precisely)

AĒR uses two words — *Probe* and *Source* — whose meaning is **swapped** between
the scientific foundation (WP-001) and the engineering/code/API surface. This is
not a vague-word problem; it is a clean inversion:

| Concept | WP-001 (science) | Engineering (code, schema, API, UI) |
| :--- | :--- | :--- |
| A single observation point (1:1 with one publisher, one etic tag, one emic designation) | **probe** | **source** (PostgreSQL `sources`, `source_classifications`) |
| The multi-source grouping defined by a shared cultural/discursive scope | **probe constellation** / **Minimum Viable Probe Set (MVPS)** (WP-001 §5.1) | **Probe** (`probe-<n>-<iso2>-<corpus-class>`, `?selectedProbes=`, `GET /api/v1/probes`) |

A reader who comes to the dashboard directly from WP-001 must mentally translate:
*"probe" in the app = "probe constellation" in WP-001; "source" in the app =
"probe" in WP-001* (design_brief §6).

This was knowingly accepted as **Path B** on 2026-04-24 ("keep current usage for
Iteration 5; name the drift; defer reconciliation to a later ADR") because
terminology surgery mid-rewrite was high-risk for little value. Two things have
changed since, which is why Phase 129 re-opens it:

1. **The audience changed (Phase 134 / ADR-040).** The deployment target is
   *invited researchers* behind auth — exactly the WP-001-fluent readers for whom
   the swap is most jarring. The mismatch graduated from "internal doc-clarity
   nit" to "first-contact confusion for the primary user."
2. **The display layer now exists.** Probes carry a `displayName` + `shortName`
   presentation layer (BFF `Probe` schema + config); the UI never renders the raw
   machine id. This makes a **labels-only** reconciliation feasible without
   touching the load-bearing identifier — an option that did not exist in 2026-04.

## 2. Footprint (measured 2026-06-20)

"Probe" as a **load-bearing identifier** is deep:

- **API contract:** ~22 `probe` references in `services/bff-api/api/openapi.yaml`
  (incl. `probeId`, `GET /api/v1/probes`, `?selectedProbes=`, `?comparedTo=`,
  `/probes/{id}/equivalence`, `/probes/{id}/lead-lag`). Contract-first → a rename
  is a wire-contract break (Hard Rule 4).
- **Frontend:** ~109 `src/` files reference `probe` (URL grammar `?selectedProbes=`,
  the cross-surface "shopping cart", the Workbench ScopeGroup tree, the globe).
- **Database:** the `sources` / `source_classifications` tables, the
  `primary_function` enum, the `probe-<n>-<iso2>-<corpus-class>` id convention,
  BFF content-catalog keys, the per-probe dossier directories under `docs/probes/`.
- **Cached state:** dashboard deep-link URLs and BFF content keys embed `probeId`
  (Probe 0's id is *deliberately* frozen as `probe-0-de-institutional-web` for
  exactly this reason — see `docs/extending/add-a-probe.md`).

"Source" is comparatively small (~15 openapi refs) but is the *other half* of the
swap, so it cannot be reconciled independently.

## 3. The options

### Path A — Full reconciliation (rename code to match WP-001)

Make the code speak WP-001: engineering "Probe" → "constellation" (or "probe set"),
engineering "source" → "probe". Touches the API contract, DB schema, URL grammar,
109 frontend files, content keys, dossier paths, cached URLs.

- **Pro:** one vocabulary across science + code + UI; no translation tax for
  researchers; the most defensible long-term endpoint.
- **Con:** the single largest rename in the project's history; breaks the wire
  contract and every cached URL; high regression surface; collides with the
  frozen-`probeId` backward-compatibility stance; enormous cost for a POC whose
  metrics are still provisional. Realistically a multi-phase epic, not a sweep.

### Path B — Status quo + sharpen (the 2026-04-24 decision, continued)

Keep engineering usage; invest only in *documentation clarity*: a single canonical
glossary entry, a one-line translation note wherever the app meets WP-001 readers,
and consistency enforcement.

- **Pro:** ~zero engineering risk; the architecture behind the words is already
  coherent; preserves the wire contract and cached URLs.
- **Con:** the translation tax persists for the exact audience (researchers) we are
  now onboarding; "name the drift" notes accrete instead of resolving it.

### Path C — Labels-only reconciliation (RECOMMENDED for evaluation)

Keep every **machine identifier** load-bearing and unchanged (`probeId`, the
`sources` table, the API contract, URL keys), and reconcile only the **human-facing
vocabulary** through the *already-existing* `displayName`/`shortName` display layer
plus content-catalog copy and UI strings (now localized, Phase 144). Decide the
*user-facing* words deliberately — e.g. keep "Probe" but always gloss it on first
contact, or surface "constellation"/"probe set" + "probe" in the UI while the code
keeps `probe`/`source`.

- **Pro:** resolves the *user-visible* half of the problem (the part that actually
  bites researchers) at a fraction of Path A's cost and risk; rides infrastructure
  that already exists (display layer + ADR-042 localization tiers); reversible;
  no wire-contract break.
- **Con:** the science↔code mismatch persists *inside* the codebase (developers
  still translate); a labels-only layer can itself drift from the identifiers if
  not disciplined; does not satisfy a purist who wants one vocabulary end-to-end.

## 4. Recommendation (for the review, not a decision)

**Evaluate Path C first**, with Path B as the fallback and Path A explicitly
deferred to a dedicated post-POC epic (it is not a documentation-sweep task and
its cost is only justified once the metrics leave "provisional"). Concretely, the
review PR should decide:

1. The **user-facing** word for the grouping and for the individual point (keep
   "Probe"/"Source" with a first-contact gloss, or adopt WP-001's
   "constellation"/"probe"). This is a UX + scientific-communication call for the
   operator + (ideally) a WP-001 author.
2. Whether the **machine identifiers stay frozen** (strong default: yes — the
   `probeId` is load-bearing and deliberately backward-compatible).
3. If Path A is ever chosen, it is scoped as its own numbered epic with a contract
   migration plan (versioned API, URL-redirect shims, content-key migration), not
   folded into a sweep.

## 5. Non-goal

This proposal does **not** rename anything. No code, schema, API, or URL changes
land with it. It exists to convert the 2026-04-24 "defer to a later ADR" promise
into an explicit, costed, reviewable decision.

## References

- `docs/design/design_brief.md` §6 (Path B record, the original framing)
- `docs/methodology/en/WP-001-*` §3–§5, §5.1 (probe / probe constellation / MVPS)
- `services/bff-api/api/openapi.yaml` (the `Probe` schema, `displayName`/`shortName`)
- `docs/extending/add-a-probe.md` (the frozen-`probeId` backward-compatibility stance)
- ADR-040 (the auth/researcher audience that re-opens the question)
- ADR-042 (UI localization tiers — the seam a labels-only Path C would use)

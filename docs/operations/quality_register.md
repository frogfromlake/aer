# Quality Register — Iteration 11 Consolidation

*Single living register produced by **Phase 138 (Quality Inventory & Worklists)**. It is the assessment stage of the assess→fix pass: it **fixes nothing**. Each item is tagged `scope · effort · ratchet-target` and grouped by the consuming phase. The remediation phases (139/140/141/143 + 129) check off against their section here.*

**Generated:** 2026-06-14 · Phase 138, fan-out of five tool-driven inventories (long-file census, dead-code scan, naming scan, comment/doc-gap census, stale-docs sweep). Methods + tool-availability notes are inline per section.

**Status convention:** `[ ]` open · `[x]` done · `[~]` reclassified/deferred-with-reason. Counts are real (grep/AST/`go test`/`knip`/`ruff`/`deadcode`), not estimates.

> **Excluded from every scan** (so counts mean source code): generated files (`generated.go`, `*_gen.go`, `src/lib/api/types.ts`, openapi output), vendored trees (`.venv`, `.audit-venv`, `node_modules`), lockfiles, build output, `__pycache__`. Test files are reported separately (they feed Phase 142, not 141/143).

---

## Summary scoreboard

| Inventory | Consuming phase | Counted result |
|---|---|---|
| Long-file census | **141** | **33 production** files over threshold (Svelte 16, Go 5, Py 6, TS 6; 1 data-file exemption) + 7 long test files (→142). *Paths refreshed post-Phase-140.* |
| Dead-code scan | **139** | ✅ **DONE** — 8 dead FE files + 2 unused deps removed; 3 ratchets proven (knip/golangci-`unused`/ruff-`F`); ~127 TS export backlog deferred (rules off) |
| Naming-inconsistency | **140** | ✅ **DONE** — Go initialisms via codegen normalizer (wire contract intact) + gofmt ratchet; ~250 retired-vocabulary renames (ViewMode→Presentation, viewmodes/→presentations/, lanes/ split); 3 ratchets active. Surface II/III deferred |
| Comment/doc gaps | **143** | Doc coverage Go **89.9%** / Py **76.8%** / TS **43.2%**; **0** TODO/FIXME; **0** commented-out blocks; **2** English-only violations |
| Stale-docs | **129** | **23** definitely-stale doc references + **3** to-verify; 5 groups of legitimate-historical (do NOT correct) |

**Cross-cutting note (highest leverage):** the same two retirements dominate three inventories — (a) the **RSS-crawler → web-crawler** rename surfaces in stale-docs (§5) and is the reason `lanes/` survives in naming (§3); (b) the **Function-Lane / `/lanes/` / `viewMode=` → Workbench/Pillar** retirement is the bulk of both the naming worklist (§3) and the stale-docs worklist (§5). Sequencing 140 and 129 to share that vocabulary canon avoids double work.

---

## 1. Long-file census (→ Phase 141)

*Method: non-blank LOC (`grep -cve '^[[:space:]]*$'`) over source only. Generated `types.ts` (8058) and `generated.go` excluded.*

### Threshold confirmation (operator decision point)

Proposed thresholds (~500 Go/Py, ~400 Svelte, ~500 TS) **match current norms** — each language has a small over-threshold tail. **Recommendation: ratchet at 500 for all four languages**, treating Svelte 400 as an advisory soft target only (Svelte has 12 files in 400–500; enforcing 400 would create 23 simultaneous violations). Distribution:

| Bucket (non-blank LOC) | Go prod (78) | Py prod (72) | Svelte (106) | TS prod (64) |
|---|---|---|---|---|
| 300–400 | 3 | 3 | 14 | 4 |
| 400–500 | 4 | 1 | 9 | 0 |
| 500–700 | 2 | 3 | 9 | 1 |
| 700–1000 | 0 | 0 | 3 | 4 |
| >1000 | 3 | 3 | 4 | 1 |

*(>500 rows re-verified post-Phase-140; the only change is TS 500–700 dropping 2→1 after `viridis.ts` was deleted in Phase 139. The 300–500 rows are the original pre-140 snapshot — unaffected by the path moves, which preserve LOC.)*

### Production files over threshold (ranked) — split hypotheses anchor on existing seams

> **Paths refreshed 2026-06-14 (post-Phase-140).** Phase 140 moved `src/lib/viewmodes/`→`presentations/`, `src/lib/components/viewmodes/`→`components/presentations/`, and split `components/lanes/` into `source/`/`article/`/`evidence/`/`charts/`. Paths + LOC below are re-verified against the current tree. `viridis.ts` (a former exemption candidate) was deleted in Phase 139, so the count dropped from 35→33.

**Svelte (ratchet 500):**
- [x] `src/lib/components/workbench/PanelControls.svelte` — **2208 → 2002 → 356** · **DONE (Phase 141, 2026-06-16).** Phase 1 lifted pure derivation + view-switch reconciliation → `panel-controls-derive.ts` (+21t). Phase 2 (Tier-2b) decomposed the markup giant into per-lever child components under `workbench/levers/` behind a new Workbench characterization e2e (`tests/e2e/workbench.spec.ts` — seeds the base64url pillar grammar, asserts the lever strip renders + a Split click re-encodes `?aleph=`): shared primitives **`LeverRow`** (117, owns the `.ctrl-row`/`.ctrl-eyebrow`/`.ctrl-options`/`.config-*` styling, `:global` so children reuse it) + **`LeverButton`** (113, the `.ctrl-btn` + variants); levers **`CompositionControls`** (113) · **`ViewControls`** (61, owns `reconcilePanelForView`) · **`MetricControls`** (333, metric/field pickers + withheld hints + the two reconcile effects) · **`ResolutionControls`** (40) · **`ConfigValueLevers`** (236, sliders/toggles) · **`ConfigChannelLevers`** (385, channel/set/facet selects) · **`WindowControls`** (112) · **`LayerCompareControls`** (166). Parent is now a 356-LOC orchestrator owning the 3 shared availability queries + shared derivations, passing them as props. Allowlist entry removed (ratchet locks it). svelte-check + fe-lint(+knip) + fe-test (366) + workbench e2e green · workbench · **L** · 500
- [x] `src/lib/components/workbench/PanelHost.svelte` — **1427 → 1364 → 312** · **DONE (Phase 141, 2026-06-16).** Phase 1 lifted the shared-axis comparison discipline (Phase 124/126) + dimension-availability filtering (ADR-038) → `panel-host-layout.ts` (`extractDimensionAvail`/`dimensionSourceFilter`/`droppedSources`/`cellDimensionOptions`/`scalarMetricOptionsFromAvailable`/`renderedProbeCount`/`isIntensiveMetric`/`effectiveCellScale`/`computeSharedCellKeys`/`unionExtent`/`computeSharedDomains`) + **21 unit tests**. Phase 2 (Tier-2b) decomposed the markup giant into per-region child components behind the Workbench characterization e2e (extended with toolbar + cell-grid fan-out assertions): **`PanelToolbar`** (144, header eyebrow/metric/lock/actions) · **`PanelScopeChips`** (90, multi-group chips, panel-bound) · **`PanelDisclosureNotes`** (99, the 4 soft banners) · **`PanelCellGrid`** (520, the panel-body: probe/facet fan-out + lazy cell-load + at-scale branch + shared-axis union + per-cell open-state) · **`PanelCell`** (371, one rendered cell — cfg/sources/dimension-peek/eyebrow/Cell/methodology). Parent is now a 312-LOC orchestrator owning the 2 shared availability queries + derived availability, passing the resolved data as props. Allowlist entry removed (ratchet locks it). svelte-check + fe-lint(+knip) + fe-test (366) + workbench e2e (4) green · workbench · **L** · 500
- [ ] `src/lib/components/evidence/L5EvidenceReader.svelte` — **1286** · markup-dominated; extract revision-row / metadata-strip / diff-segment child components → `evidence/` · evidence · **M** · 500
- [~] `src/lib/components/presentations/CoOccurrenceNetworkCell.svelte` — **1222** · **ALLOWLIST (AtScale twin, operator decision 2026-06-16).** The node-sizing/colour-channel + relabel + how-to-read/export logic is ALREADY fully in `cooccurrence-network-shared.ts` (the component holds only thin reactive wrappers `nodeFill`/`nodeStrokeColor`/…, tested via `cooccurrence-network-internals.test.ts`); the residual is **irreducible d3-force/SVG simulation + pan/zoom/node-drag interaction glue** + 310 scoped-CSS. Forcing ≤530 would mean carving the simulation/interaction core for no quality gain. Not split — mirrors `CoOccurrenceNetworkAtScale`.
- [ ] `src/lib/components/workbench/ScopeEditor.svelte` — **997** · move group-mutation/validation into `scope-editor-draft.ts`; extract `ScopeGroupCard` · workbench · **M** · 500
- [ ] `src/lib/components/account/AnalysesOverlay.svelte` — **974** · extract list-row + per-analysis-action sub-components; lift API-orchestration · account · **M** · 500
- [~] `src/lib/components/presentations/CoOccurrenceNetworkAtScale.svelte` — **728** · **ALLOWLIST (no change, operator decision 2026-06-15).** Pure logic ALREADY fully in `cooccurrence-network-shared.ts` (`buildNetworkNodes`/`buildNetworkEdges`/`communityHeads`/`computeCommunities`/`nodeFillColor`/`buildExportPayload`…, tested via `cooccurrence-network-internals.test.ts`); the residual is **irreducible imperative sigma.js/ForceAtlas2/WebGL render `$effect`** + thin reactive shell. Floor ~628 even with risky effect-carving → reaching 530 would require gutting the render effect for no quality gain. Not split.
- [x] `src/lib/components/presentations/TopicEvolutionCell.svelte` — **691 → 493** · extracted `topic-evolution-internals.ts` (computeBuckets/buildSeriesMeta/selectTopSeriesIds/collectLanguages/buildPlotRows/buildLegendEntries — pure) + **14 new unit tests**; the Observable-Plot DOM `$effect` left byte-identical (its `seriesId`/`seriesMetaValues`/`OUTLIER_COLOUR` resolve to imports). svelte-check + fe-lint + fe-test (316) green.
- [~] `src/lib/components/atmosphere/AtmosphereSurface.svelte` — **682 → 642** · **safe-extract + ALLOWLIST (operator decision 2026-06-15).** Extracted pure transforms → `atmosphere-surface-internals.ts` (`buildProbeMarkers`/`computeWindow`/`computeActivity`/`resolveFlyTo`/`buildFlatProbes`) + **8 unit tests**; reactivity preserved ($derived calls the pure fns; `void flyToNonce`/`now`/`lookbackMs` stay at the call site). Residual >530 is **render/markup-dominated** (script 285 / markup 158 / **scoped-CSS 239**) — cohesive CSS + interaction handlers + markup; Tier-2b sub-componentisation deferred (not forced). svelte-check + fe-lint + fe-test (324) green.
- [ ] `src/lib/components/workbench/CellConfigPopover.svelte` — **660** · extract per-`configurableParams` field renderers · workbench · **S/M** · 500
- [ ] `src/routes/(app)/reflection/wp/[id]/+page.svelte` — **642** · move markdown/section rendering to a component · routes · **S** · 500
- [x] `src/lib/components/presentations/DistributionCell.svelte` — **587 → 526** · extracted `distribution-cell-internals.ts` (buildPlotRows/isDegenerate/computePlotDomain/fmtBinRange — pure) + **8 new unit tests**; reactivity preserved ($derived calls the pure fns). svelte-check + fe-lint + fe-test green. *(pattern-establisher for medium pure-extractions)*
- [ ] `src/lib/components/dossier/ProbeCard.svelte` — **568** · extract capability-matrix sub-component · dossier · **S** · 500
- [~] `src/lib/components/chrome/SideRail.svelte` — **533** · **WITHIN TOLERANCE** (≤530-ish) — leave; markup-dominated, splitting for 3 LOC would degrade cohesion.
- [~] `src/lib/components/source/SourceCard.svelte` — **526** · **WITHIN TOLERANCE** — leave.
- [~] `src/lib/components/article/ArticleListModal.svelte` — **502** · **WITHIN TOLERANCE** — leave.

> **Svelte verification reality (operator decision 2026-06-15):** no Svelte component-render test setup exists (vitest is `node`-env). Components verify via svelte-check (types/template) + E2E (Playwright, image not pulled) only. So ALL component work uses the **Tier-2 approach: svelte-check + careful manual review, git as the safety net** — distinct from the unit-tested TS modules above. Sub-tiers: **(a) pure-logic extraction** (the 568–728 components — lift `$derived`/helpers into a companion `.ts` + ADD unit tests for the pure helper; reactivity/template untouched → low risk) · **(b) markup-giant sub-componentisation** (PanelControls/PanelHost/L5EvidenceReader/CoOccurrenceNetworkCell/ScopeEditor/AnalysesOverlay — props/state/CSS-scope/bindings risk; svelte-check + review only).

**Go (ratchet 500):** — ✅ all split (golangci clean, bff tests green incl. Testcontainers)
- [x] `view_mode_handlers.go` 1334 → `view_mode_handlers.go` (132, shared scope/window) + `distribution_handlers.go` (384) + `heatmap_correlation_handlers.go` (381) + `cooccurrence_handlers.go` (455)
- [x] `metrics_query.go` 1163 → `metrics_query.go` (465) + `equivalence_query.go` (252) + `scope_equivalence_query.go` (462)
- [x] `handler.go` 1110 → `handler.go` (426, server+health+gate) + `metrics_handler.go` (287) + `probes_handler.go` (372) + `content_handler.go` (46)
- [x] `cooccurrence_query.go` 669 → `cooccurrence_query.go` (338) + `cooccurrence_subqueries.go` (339)
- [x] `revisions_handler.go` 505 → `revisions_handler.go` (433) + `revisions_resolution.go` (78)

**Python (ratchet 500):** — corpus + audit_source + web_extract split (12 new modules, all tests green); the 3 remaining (worker main, processor, crawler main) are justified exceptions (marginal + cohesive entrypoints/processor), allowlisted in the ratchet as tracked debt.
- [x] `services/analysis-worker/internal/corpus.py` — **1279** → `corpus.py` (383, shared+cooccurrence) + `corpus_baseline_topic.py` (323) + `corpus_revision_io.py` (362) + `corpus_revision_diff.py` (248). main.py imports rewired. ruff clean, 326 worker tests green.
- [x] `crawlers/web-crawler/audit_source.py` — **1825** → **7 modules, all <500** (operator decision 2026-06-15): `audit_probe.py` (329, HTTP/feed/sitemap/RSS probing + constants) + `audit_pattern.py` (416, URL-pattern inference + CMS detection — mutually coupled, kept together) + `audit_datewalk.py` (111, date-walk verification) + `audit_core.py` (220, `audit_source()` orchestrator + `extract_discovered_urls`) + `audit_yaml.py` (330, sources.yaml diff/render/apply) + `audit_reaudit.py` (269, re-audit flow) + `audit_source.py` (277, CLI entry `aer-audit-source:cli` + re-export facade via `__all__`). Cycle (`_run_reaudit`→`audit_source()`) broken by the `audit_core` layer. **Test patch-targets updated to follow moved fns** (`audit_reaudit.audit_source`, `audit_core._try_trafilatura_*`). ruff clean, **121 crawler tests green**.
- [x] `services/analysis-worker/internal/adapters/web_extract.py` — **1078** → **5 modules, all <500** (operator decision 2026-06-15): `web_extract_deps.py` (31, the optional-dep `try/except` + `EXTRACTION_AVAILABLE` + handles — shared, set once) + `web_extract_images.py` (82, image-URL extraction/identity/dedup — heavily unit-tested) + `web_extract_sources.py` (292, jsonld/og/microdata parsers + date/url/lang helpers) + `web_extract_fields.py` (467, `_resolve_*` field resolvers) + `web_extract.py` (310, `extract_web_document` + body/extruct/title; re-export facade via `__all__`). Layering deps<sources<images<fields<web_extract (no cycles). Production contract (`web.py`: `EXTRACTION_AVAILABLE`/`canonical_url_or`/`extract_web_document`) + test helpers (`_extract_image_url`/`_dedupe_images`/`_image_identity`) re-exported. **Required installing the pinned NLP deps into the local venv to un-skip `test_web_adapter.py`** (no repo/req change). ruff clean, **full worker suite 348 passed (identical to baseline)**.
- [~] `services/analysis-worker/main.py` — **673** · **EXCEPTION.** Service entrypoint (NATS wiring + lifecycle + setup). Moderate size; tests import `init_extractors`/`DEFAULT_EXTRACTOR_CLASSES`/`_handle_message` directly, so an init-block extraction needs re-export machinery + cycle-care for marginal benefit (≈493 after split). *Allowlisted.*
- [~] `services/analysis-worker/internal/processor.py` — **642** · **EXCEPTION.** The core extractor-pipeline processor — one cohesive unit (language-detection-first ordering + Gold enrichment). The census itself flagged it "marginal; extract only if it grows." *Allowlisted.*
- [~] `crawlers/web-crawler/main.py` — **577** · **EXCEPTION.** Crawler entrypoint, only marginally over (Scrapy bootstrap + CLI + per-channel orchestration are tightly coupled to the run setup). *Allowlisted.*

**Ratchet threshold = 530 (operator decision 2026-06-15): 500 + a ±20–30 line tolerance — do NOT degrade code quality just to force <500.** Files within ~530 that are cohesive stay as-is (not split, not "exceptions" — within tolerance).

**TS — unit-tested modules (the safe split tier), all DONE + green (svelte-check 0 errors, fe-test + fe-lint pass):**
- [x] `src/lib/api/queries.ts` — **1239** → `queries/` barrel-dir: `shared.ts`(325) + `probes.ts`(255) + `articles.ts`(208) + `analytics.ts`(354) + `revisions.ts`(158) + `index.ts`(6). Stable `$lib/api/queries` path preserved (dir index). 3 query test files green.
- [x] `src/lib/state/url-internals.ts` — **985** → `url-types.ts`(499, types+consts+Compact) + `url-codec.ts`(391, compact/expand/encode/decode/guards/base64) + `url-internals.ts`(127, read/write + re-exports). `import type` circular-safe; 4 url test files green.
- [x] `src/lib/presentations/registry.ts` — **784** → `registry.ts`(465, types+accessors) + `registry-data.ts`(324, PRESENTATIONS table). SoT export surface preserved.
- [~] `src/lib/workbench/panel-queries.ts` — **526** · **WITHIN TOLERANCE** (≤530) — cohesive panel→query mapping (CLAUDE.md SoT); not split.
- [ ] `packages/engine-3d/src/engine.ts` — **937** · → **Tier 2** (Three.js; behaviour verifiable only via E2E/WebGL, no node unit path) — handle with svelte-check + review, not in the safe tier.
- [~] `src/lib/reflection/open-questions.ts` — **743** · **data-dominated** (content array); relocate to `.json`/generated OR keep as data exemption. (`viridis.ts` 570, the other candidate, was deleted dead in Phase 139.)

### Long test files (→ Phase 142, listed only — NOT Phase 141)
- [ ] Go: `bff-api/internal/handler/metrics_handler_test.go` 988 · `…/storage/metrics_query_test.go` 944 · `…/handler/view_mode_handlers_test.go` 671 · `…/storage/view_mode_queries_test.go` 637
- [ ] Python: `crawlers/web-crawler/tests/test_audit_source.py` 749 · `…/tests/test_main.py` 547
- [ ] TS: `dashboard/tests/unit/url-panels.test.ts` 504

**Count (post-Phase-140): 33 production files over threshold (1 exemption: `open-questions.ts`) — Svelte 16, Go 5, Python 6, TS 6 — + 7 long test files (→142).** Ratchet recommendation: 500 all languages. *(Was 35/2-exemptions pre-Phase-139; `viridis.ts` deleted.)*

---

## 2. Dead-code scan (→ Phase 139) — ✅ REMEDIATED 2026-06-14

> **Phase 139 outcome:** 8 dead FE files deleted + 2 unused deps removed (all verified independently before deletion). Three dead-code ratchets now proven (planted-symbol test → exit 1, clean → exit 0): **TS** `knip` (new, wired into `make fe-lint` + `knip.json`), **Go** golangci-lint `unused`/`staticcheck`/`ineffassign` (already default-active — no `.golangci.yml`, so defaults run), **Python** `ruff` default `F` rules (`F401/F811/F841`, already active via `make lint`). Validation: `make fe-lint` + `make fe-test` (294) green; CLAUDE.md stale `WorkbenchDatasetShape` line corrected. The ~127-item unused-exports backlog is intentionally NOT ratcheted (knip `exports`/`types` rules `off` — they are dynamic-dispatch / SoT-registry / not-yet-mounted FPs). Go/Python full Testcontainer + E2E suites not re-run (zero backend-source change) — CI confirms on push.

*Tooling that ran: Go `deadcode@latest` per-module via go.work + `go mod tidy` dry-run; Python `ruff 0.15.8` (`F401,F811,F841`) + `vulture`; TS `knip 5.88.1` (temp-installed, reverted, hand-validated by grep). staticcheck not pinned (deadcode used instead).*

### Go — 1 candidate (NOT dead, keep)
- [~] `services/bff-api/internal/auth/password.go:30` `DefaultArgon2Params()` — test-only fixture (called from `auth_test.go` + `auth_handlers_test.go`); deadcode flags it because callers are tests. **Keep or relocate to `export_test.go`**, do not delete. · bff/auth · low confidence

### Python — 0 real (2 vulture FPs, ruff clean)
- [~] `extractors/base.py:166` param `cores` — Protocol method signature, contract. Keep.
- [~] `main.py:623` `*args` in `shutdown_signal` — `add_signal_handler` callback contract. Keep.
- ruff `F401/F811/F841` across worker + crawler: **all pass** (zero unused imports/vars).

### TS/Svelte — 8 dead files/barrels (all DELETED, verified unreferenced first)
- [x] `src/lib/components/workbench/WorkbenchDatasetShape.svelte` — deleted (only comment refs). CLAUDE.md "mounted by WindowHost" line **corrected**.
- [x] `src/lib/components/TimeScrubber.svelte` — deleted (only comment refs).
- [x] `src/lib/components/lanes/NormalizationControl.svelte` — deleted (zero refs).
- [x] `src/lib/components/lanes/SilverIneligiblePanel.svelte` — deleted (verified: only barrel-referenced, 0 direct imports).
- [x] `src/lib/design/viridis.ts` — deleted (no module import; `VIRIDIS_256`/`CIVIDIS_256`/`viridis()`/`cividis()` unused; CSS uses own token stops, Plot uses its built-in `scheme:'viridis'`). **Resolves the §1 exemption — deleted, not exempted.**
- [x] `src/lib/api/client.ts` — deleted (verified: `createApiClient` 0 importers in src + tests; app uses `queries.ts`/`auth.ts`/`analyses.ts`).
- [x] `src/lib/components/lanes/index.ts` + `src/lib/components/workbench/index.ts` — deleted (unused barrels; confirmed every other re-exported component has direct importers, so none orphaned).

### Unused dependencies — 2 removed
- [x] `@opentelemetry/context-zone` — removed (verified: no `ZoneContextManager`/`context-zone` usage anywhere).
- [x] `@types/diff` — removed (verified: `diff@9.0.0` ships own types `libcjs/index.d.ts`; runtime `diff` dep kept — used in L5EvidenceReader).
- [~] `@fontsource-variable/inter` + `@fontsource/ibm-plex-mono` — **NOT dead** (consumed via `url()` in `fonts.css`; knip raw-flags them → added to `ignoreDependencies` in `knip.json`). Go `go.mod` — no unused deps (tidy clean).

### Manual-review backlog (do NOT bulk-delete) — deferred-with-reason
- [~] knip's ~56 unused exports + ~71 unused exported types are **mostly false positives** (barrel re-exports imported directly, auth/analyses surfaces wired via dynamic call sites not yet UI-mounted, generated `types.ts`, string-keyed SoT registries). **Deferred:** knip `exports`/`types` rules set `off` so the ratchet stays zero-FP. A future targeted pass (e.g. after Phase 134 auth surfaces fully mount) can revisit specific exports; not a Phase-139 sweep.

### Live extension seams — explicitly NOT dead
AdapterRegistry + MetricExtractor registry + `extractors/__init__.py` exports; all exported `pkg/` APIs; `generated.go`; `types.ts`; the string-keyed SoT maps (`registry.ts`, `metric-presentation.ts`, `negative-space.ts`, `discourse-function.ts`).

### Ratchet — IMPLEMENTED & proven (planted-symbol test passed for all three)
- [x] **Go** → golangci-lint `unused`/`staticcheck`/`ineffassign` already default-active (no `.golangci.yml` ⇒ defaults run). Proven: planted unused unexported func → `unused` exit 1; clean → exit 0. Catches unexported dead symbols (exported-dead is left to the extension-seam allowance — `deadcode` not added, it FP'd on the test-only `DefaultArgon2Params`).
- [x] **Python** → `ruff` default `F` rules (`F401/F811/F841`) already active via `make lint`. Proven: planted unused import → `F401` exit 1; clean → exit 0. (`vulture` NOT added — it FP'd on 2 Protocol-contract params.)
- [x] **TS** → `knip 5.88.1` added (devDep, from local store, zero download) + `knip.json` (rules `files`/`dependencies`/`devDependencies`/`unlisted`/`binaries`/`unresolved` = error; `exports`/`types`/etc. = off; `ignoreDependencies` for the 2 fontsource CSS-`url()` deps). Wired into `make fe-lint` (`pnpm run knip`) ⇒ runs in CI (CI calls `make fe-lint`). Proven: planted dead file → exit 1; clean → exit 0.

**Count: 8 dead FE files/barrels DELETED + 2 unused deps REMOVED; Go 0 / Python 0 real (kept-with-reason); ~127-item exports backlog deferred (rules off). All three ratchets active.**

---

## 3. Naming-inconsistency scan (→ Phase 140) — ✅ REMEDIATED 2026-06-14

> **Phase 140 outcome.** Codified `docs/development/conventions.md`, then applied the renames in compiler-verified tranches. **Two register assumptions were wrong and are corrected below (verify-first paid off):**
> 1. *"81 safe Go renames"* — WRONG. The `Id`/`Url` fields are anonymous structs that **structurally mirror oapi-codegen output**; renaming one breaks the build (`append` type-mismatch). The real fix was to set oapi-codegen `name-normalizer: ToCamelCaseWithInitialisms` (both `codegen.yaml`), regenerate, and chase ~119 references — **json tags (the wire contract) are 100% unchanged**, codegen is deterministic, all Go tests green.
> 2. *"Python ruff N = 0 violations, free ratchet"* — WRONG. `ruff N` found 23 violations, all defensible false positives (local UPPER_SNAKE constants, test stub classes, `Client as NATS` alias, descriptive test names). Enabled `N` with targeted ignores (N806/N814 global, N802 in tests) so the valuable rules (N801/N803/N818) stay sharp at 0 violations.
>
> Deferred-with-reason: **"Surface II/III" numbering** (33 occ incl. generated `types.ts` from OpenAPI descriptions + the descent-model) — distinct from the retired "Function Lanes" surface *name*; purging it would require openapi.yaml contract-description edits. Validation: `make lint` green (Go ×3, Python ×2, fe) · svelte-check 0 errors · fe-test 294 · Go tests green incl. Testcontainers · all three ratchets proven with planted-symbol tests.

*Per-language discipline is strong; the real surface is Go initialism drift + retired Phase-106 vocabulary in the dashboard.*

### Per-language convention drift
**Go:**
- [x] `Id`→`ID` / `Url`→`URL` across the BFF + ingestion — done via the **codegen name-normalizer** (not per-field renames, which were impossible — see outcome note). `ProbeID`/`SourceID`/`ScopeID`/`ArticleID`/`DocumentationURL`/`ArchiveURL`/`GetSourceByID` etc. now idiomatic in generated AND hand-written mirrors; one hand-written `scopeId` param also fixed. Ratchet: `stylecheck ST1003` (`.golangci.yml`).
- [x] **Bonus (in-scope per phase "formatter settings"):** 16 pre-existing gofmt-dirty files formatted; `gofmt` added to golangci-lint as a ratchet (drift had accumulated because the default set doesn't check formatting).
- [~] `API`/`JSON`/`HTTP`/`SQL` already canonical; receivers consistent; `Err*` vars consistent — no action.

**Python:** [x] `ruff` `N` (pep8-naming) enabled in both services with targeted ignores (see outcome note — the "0 violations" claim was wrong). N801/N803/N818 enforced at 0 violations; proven (planted N801 caught).

**TS/Svelte:** [~] 0 drift on filenames, `$lib` imports, camelCase locals, constants — confirmed, no action needed (the drift was domain-term, below).

### Domain-term drift (the real work — ~250 occurrences of retired vocabulary) — DONE
- [x] `ViewMode` type (163 occ across state/registry/queries/components) → **`Presentation`** (`ViewModeCellProps`→`PresentationCellProps`, `pillarForViewMode`→`pillarForPresentation`, etc.). Lowercase URL key `'viewMode'` (legacy redirect contract) preserved.
- [x] `src/lib/viewmodes/` + `src/lib/components/viewmodes/` (32 files) → **`presentations/`** (82 import paths updated). 
- [x] `ViewingMode` type (50 occ) → **`PillarId`**. URL key `viewingMode`/`activePillar` preserved.
- [x] Live user-facing **"Function Lane(s)"** / "Surface II" strings (globe primer, ScopeBar, side-rail story) → **Workbench** / surface names. ScopeBar's dead `/lanes/` breadcrumb branches removed.
- [x] `components/lanes/` (the grab-bag) **split** into coherent dirs: `source/` (SourceCard, MetadataCoveragePanel, DiscoveryCoveragePanel), `article/` (ArticleListModal, ArticlePreviewList, ArticleRow), `evidence/` (L5EvidenceReader), `charts/` (`OverlayLaneChart`→`OverlayLineChart`, `SourceLaneChart`→`SourceLineChart`). [operator decision — no clean single rename existed]
- [x] Stale `WorkbenchScopeBar` comments scrubbed where present-tense-wrong (legit "retired X" historical notes kept).
- [~] `scopeGroup` (1, lowercase prose in a tree-hierarchy comment) — consistent prose, left as-is. Go `probeId` local vars — idiomatic, no change. `cooccurrence` vs `co-occurrence` — legitimate.
- [~] **Deferred:** "Surface II/III" numbering (33 occ, incl. generated `types.ts` + descent-model) — see outcome note.

### Contract identifiers — deferred-with-reason (do NOT rename)
OpenAPI/JSON fields (`probeId`/`sourceId`/`scopeId`/`comparedTo`/`viewerLanguage`); all of `generated.go`; content-catalog `entityType` keys (`view_mode`/`empty_lane`/`discourse_function`/`open_research_question`); URL-grammar keys (`bn`/`ch`/`sb`/`fs`/`dl`, `aleph`/`episteme`/`rhizome`, `selectedProbes`, `dossier`, `activePillar`); DB/storage identifiers (ClickHouse table/column names, Postgres `primary_function` enum, the machine `probeId`); discourse-function enum keys.

### Ratchets — IMPLEMENTED & proven (planted-symbol test passed for all three)
- [x] **Go**: `.golangci.yml` enables `stylecheck ST1003` (initialisms) + `gofmt`, on top of the default set. No `generated.go` exclusion needed (the normalizer makes generated names idiomatic). Proven (planted `Id` field / unformatted file → fail).
- [x] **TS**: eslint `no-restricted-syntax` bans `ViewMode`/`ViewingMode`/`WorkbenchScopeBar` identifiers + `Function Lane` string/template literals (comments untouched, so legit historical notes survive; the config file is exempt from its own rule). Proven (planted identifier + string → fail).
- [x] **Python**: `ruff N` in both services with targeted ignores. Proven (planted N801 class → fail).

**Count: Go initialism unification via codegen normalizer (~119 refs, wire contract intact) + gofmt ratchet; ~250 retired-vocabulary occurrences renamed (ViewMode→Presentation, viewmodes/→presentations/, lanes/ split, ViewingMode→PillarId); 3 ratchets active. "Surface II/III" deferred-with-reason.**

---

## 4. Comment/doc-coverage gaps (→ Phase 143)

*Framing (per phase): document intent/invariants/units/gotchas — NOT signature restatements. The lists below already exclude protocol-method restatements and grouped enum members.*

### Exported-symbol doc coverage
- **Go — 89.9%** (364/405; 41 undoc). High-value gaps to fill (skip the ~22 grouped enum members):
  - [ ] `services/ingestion-api/internal/handler/handler.go:60,100,104,127` — `IngestDocuments`/`GetHealthz`/`GetReadyz`/`GetSourceByName` (the ingest entrypoint) · ingestion · S
  - [ ] `services/bff-api/internal/notify/sender.go:24,29` — `SendInvite`/`SendPasswordReset` (side-effecting email) · bff/notify · S
  - [ ] storage constructors/types: `ClickHouseStorage`/`NewClickHouseStorage`, `MinioClient`/`NewMinioClient`, `PostgresDB`/`NewPostgresDB`, `NewWebAuthnStore`, `NewAnalysesStore` · S
- **Python — 76.8%** (199/259; 60 undoc) + **21 modules missing module docstring**. High-value (skip ~30 protocol restatements):
  - [ ] `internal/processor.py:164 process_event` (pipeline entrypoint, + module docstring) · worker · S
  - [ ] `main.py:502 main`/`connect_nats`/`message_handler` (lifecycle) · worker · S
  - [ ] `internal/models/probe_scope.py:40,56,64,68` — `load`/`is_in_scope`/`allowed_languages`/`sources_with_scope` · worker · S
  - [ ] `internal/storage/clickhouse_client.py:39,47` `getconn`/`putconn` (+ module docstring) · worker · S
  - [ ] domain dataclasses: `entity_linking.py:47 LinkCandidate`, `metric_baseline.py:140 BaselineSweepResult`, `language_capability.py:126 CulturalCalendarRef` · worker · S
  - [ ] crawler contracts: `internal/ingestion/client.py` (4/4 undoc) + `internal/state/dedup.py` · crawler · S
- **TS — 43.2%** (167/387; 220 undoc — functions 35% / types 74%). Highest-value (skip ~50 `*Dto` aliases + generated `types.ts`):
  - [ ] `state/url-internals.ts` (17) — `Panel`/`ScopeGroup`/`UrlState`, `encode/decodePillarState`, `MAX_PANELS_PER_WINDOW`/`MAX_WINDOWS_PER_PILLAR` caps, `readFromSearch`/`writeToSearch` (URL grammar SoT) · state · M
  - [ ] `negative-space.ts` (7) + `discourse-function.ts` (4) — the two SoT taxonomy modules named in CLAUDE.md, undocumented at export level · dashboard · S
  - [ ] `api/queries.ts` — the ~20 `*Query` functions (refusal/equivalence semantics); the ~50 `*Dto` aliases are low-value · api · M

### TODO/FIXME census — 0 actionable
- [x] Full `TODO|FIXME|XXX|HACK` sweep: **no real markers** (only a test-fixture string, a lockfile hash, and two docs describing the convention). Nothing to action.

### Commented-out-code blocks — 0 genuine
- [x] Heuristic fired only on wrapped explanatory prose + SvelteKit scaffold boilerplate (`app.d.ts`). No removable dead-code blocks.

### English-only violations — 2
- [ ] `src/lib/workbench/scope-editor-draft.ts:7-10` — 4-line **German comment** (quoted requirement) → translate to English · dashboard · S
- [~] `src/lib/components/chrome/PillarSwitch.svelte:5-6` — comment references German UI strings (`Was ist jetzt da?` …); borderline (documentation-of-UI-content, surrounding comment is English) → lower priority

### Ratchet recommendation
- [ ] **English-only CI/pre-commit grep** (cheap, enforces a hard CLAUDE.md invariant): fail on German stopwords (`\b(und|nicht|wird|werden|über|zurück|gespeichert|gehe)\b`) inside `//`/`#` across `*.go *.ts *.py *.svelte`, proper-noun allowlist.
- [ ] **TS doc-lint** scoped (`jsdoc/require-jsdoc`) to `$lib/api/*Query`, `url-internals.ts`, `negative-space.ts`, `discourse-function.ts` — NOT blanket (would flag 50+ DTO aliases).
- Go: no ratchet (89.9%, enum-member noise). No TODO-lint needed (already clean).

**Count: Go 89.9% / Py 76.8% / TS 43.2% coverage; 0 TODO; 0 commented-out; 2 English-only violations.**

---

## 5. Stale-docs list (→ Phase 129)

*Two clusters dominate: (A) RSS-crawler → web-crawler rename; (B) Function-Lane/`/lanes/`/`viewMode=` → Workbench/base64url-grammar. Plus one structural defect (duplicate §8.12/8.13/8.14 numbering).*

### Definitely-stale (23)
**arc42:**
- [ ] `05_building_block_view.md:155,157` — RSS Crawler described as live primary source → web-crawler (ADR-028) · definitely-stale
- [ ] `08_concepts.md:758-877` — §8.12/8.13 "Surface II Architecture" + view-mode matrix in present tense (`/lanes/[probeId]/...`, `ViewModeSwitcher`) → three-surface Workbench grammar (no supersession banner present) · definitely-stale
- [ ] `08_concepts.md:855` — `?viewMode=` + `/lanes/` grammar → base64url-json pillar grammar · definitely-stale
- [ ] `08_concepts.md:822,947,949` — SideRail "Function Lanes → `/lanes/.../dossier`" → Dossier global overlay (`?dossier=open`) · definitely-stale
- [ ] `08_concepts.md:553,555` — SideRail second anchor "Function Lanes"; `?viewingMode=`; lane switcher → Workbench · definitely-stale
- [ ] `08_concepts.md:517,539,545` — "Function Lanes (four discourse functions as horizontal lanes)"; `view=analysis`/`?viewMode=` · definitely-stale
- [ ] `08_concepts.md:420/436/452 vs 758/826/881` — **duplicate `## 8.12/8.13/8.14` section numbers** (structural) → renumber · definitely-stale
- [ ] `03_system_scope_and_context.md:17,58` — `wikipedia-scraper` as example crawler (dir removed) → web-crawler · definitely-stale
- [ ] `12_glossary.md:18` — Dashboard "three surfaces (Atmosphere, Function Lanes, Reflection)" → Workbench · definitely-stale
- [ ] `12_glossary.md:56` — Surface entry "Function Lanes" → Workbench · definitely-stale
- [ ] `12_glossary.md:59` — RSS Crawler glossary entry in present tense → mark retired / web-crawler · definitely-stale
- [ ] `12_glossary.md:32` — Go Workspace lists `crawlers/wikipedia-scraper/` (removed) · definitely-stale
- [ ] `13_scientific_foundations.md:463` — "RSS crawler remains operational" → web-crawler · definitely-stale

**design:**
- [ ] `design_system.md:203` — chrome anchors "Links to `/`, `/lanes`, `/reflection`" → `/workbench` (no banner on this file) · definitely-stale
- [ ] `design_system.md:219` — "II — Function Lanes | Lane switcher (Phase 106)" → Workbench · definitely-stale

**operations:**
- [ ] `developer_quickstart.md:54` — `make crawl # runs rss-crawler` → web crawler (`crawl-probe0`) · definitely-stale
- [ ] `developer_quickstart.md:55` — references non-existent `make crawl-reset` target · definitely-stale
- [ ] `operations_playbook.md:1832` — "`make crawl` | Run the RSS crawler" → web crawler · definitely-stale

**extending:**
- [ ] `add-a-source-type.md:140` — "new standalone Go binary analogous to the RSS crawler" contradicts line 31 (Python-preferred, ADR-028) → rewrite against web-crawler · definitely-stale (internally inconsistent)

**README (top-level):**
- [ ] `README.md:7` — "ingesting German institutional RSS feeds" → web crawler (raw HTML, `source_type:"web"`) · definitely-stale
- [ ] `README.md:35` — "Currently includes the RSS crawler" → web-crawler · definitely-stale
- [ ] `README.md:198` — "`make crawl` | Build and run the RSS crawler" → web crawler · definitely-stale
- [ ] `README.md:360-382` — entire "### RSS Crawler" section (`go build -o bin/rss-crawler`, `feeds.yaml`) → document Python web-crawler + `make crawl-probe0` · definitely-stale

### To-verify (3)
- [ ] `05_building_block_view.md:159` — Wikipedia Scraper co-text (historical framing OK; flag only live-crawler context) · verify
- [ ] `operations_playbook.md:1402,1427` — e2e prose "runs the RSS crawler against fixtures" → check `scripts/build/e2e_smoke_test.sh` (likely stale) · verify
- [ ] `add-a-source-type.md:140` — confirm Go-vs-Python internal inconsistency (cross-listed above) · verify

### Legitimate historical — NOT stale (Phase 129 must NOT "correct")
- `09_architecture_decisions.md` — ADR-020/033/034 records of four-surface / `/lanes/` / ProbeFilterModal / WorkbenchScopeBar→PanelControls / CellControls / flyout / flat-form URLs are **explicitly marked superseded with Phase addenda** (lines 536, 2346, 2364, …).
- `design_brief.md` + `reframing-note.md` — Function-Lanes/four-surface prose protected by **up-front supersession banners** → ADR-033.
- `extending/README.md:47` + `operations_playbook.md:1193-1195,1443` — RSS crawler / `/lanes/*` described as retired/archived/quarantined.
- `_archived/rss-crawler/` references (as archived) + `compose.go`/`get_compose_image()` SSoT mentions.

**Count: 23 definitely-stale + 3 verify; 5 legitimate-historical groups preserved.**

---

## Cross-references back into CLAUDE.md (surfaced during the scan — for the maintainer)

- [x] CLAUDE.md line "`WorkbenchDatasetShape` … mounted by WindowHost" was **stale** — the component is dead (§2). **Corrected in Phase 139** (the clause was removed; CLAUDE.md is a gitignored local file, so the fix is in the working tree, not the committed diff).

---

*End of register. Phase 138 Definition of Done: every category above has a counted, non-empty worklist (or a justified "nothing found" — see §4 TODO/commented-out). Downstream phases 139/140/141/143/129 each have a concrete worklist. This register is the SoT the remediation phases check off against.*

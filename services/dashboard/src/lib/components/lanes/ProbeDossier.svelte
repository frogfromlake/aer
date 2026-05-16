<script lang="ts">
  // Probe Dossier — Surface II's default landing (Brief §4.2.1).
  //
  // Phase 113c rework:
  //   - Single collapsible probe section. Default expanded; future N-probe
  //     dossiers will list multiple sections with one expanded at a time.
  //   - Folds in the framing copy that used to live in Surface I's L3
  //     companion panel (now retired): emic semantic paragraph, structural
  //     meta, methodology-tray entry, reach disclaimer (Brief §5.7).
  //   - Promotes the four-function coverage indicator into the central
  //     interaction of this layer: a Function Selector with prominent
  //     tiles that link to `/lanes/{probeId}/{functionKey}` — the path
  //     onward into Surface II L3.
  //   - Source cards remain as the "what's in this probe" inventory.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    contentQuery,
    type ProbeDossierDto,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import SourceCard from './SourceCard.svelte';
  import MetadataCoveragePanel from './MetadataCoveragePanel.svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { goto } from '$app/navigation';
  import {
    DISCOURSE_FUNCTIONS,
    FUNCTION_DEFINITIONS,
    type DiscourseFunction
  } from '$lib/discourse-function';

  interface Props {
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
  }

  let { dossier, ctx, windowStart, windowEnd }: Props = $props();

  let expanded = $state(true);
  let sourcesExpanded = $state(true);

  const url = $derived(urlState());
  let narrowedSourceIds = $derived(url.sourceIds);
  let hasNarrowedScope = $derived(narrowedSourceIds.length > 0);
  let composedProbeIds = $derived(url.probeIds);
  let hasComposition = $derived(composedProbeIds.length > 0);

  // Destination function for "Analyze": use the first narrowed source's
  // primary function, falling back to the first covered probe function.
  let analyzeTarget = $derived.by<string>(() => {
    for (const id of narrowedSourceIds) {
      const src = dossier.sources.find((s) => s.name === id);
      if (src?.primaryFunction) return src.primaryFunction;
    }
    const covered = dossier.functionCoverage.functions as string[];
    return covered[0] ?? 'epistemic_authority';
  });

  // Discourse-function metadata is sourced from $lib/discourse-function
  // (Phase 122h / ADR-033 §4 — single source of truth across the dashboard).
  const ALL_FUNCTIONS: readonly DiscourseFunction[] = DISCOURSE_FUNCTIONS;
  let coveredFunctions = $derived(new Set(dossier.functionCoverage.functions as string[]));

  // Source-to-function mapping (Finding 1.5b). For each WP-001 discourse
  // function, list the sources whose primaryFunction matches it — so the
  // user can read at a glance which sources will be in scope when they
  // click the Open-Workbench tile for that function.
  const sourcesByFunction = $derived.by<Record<string, string[]>>(() => {
    const m: Record<string, string[]> = {};
    for (const fn of ALL_FUNCTIONS) m[fn] = [];
    for (const s of dossier.sources) {
      if (s.primaryFunction && m[s.primaryFunction]) {
        m[s.primaryFunction]!.push(s.emicDesignation ?? s.name);
      }
    }
    return m;
  });

  // Parallel mapping by canonical source name (used as URL `sourceId` param).
  // `sourcesByFunction` above carries emicDesignation for display; the
  // Workbench needs the source `name` so its scope state aligns with the
  // SourceLaneChart query keys and the WorkbenchScopeBar chips.
  const sourceNamesByFunction = $derived.by<Record<string, string[]>>(() => {
    const m: Record<string, string[]> = {};
    for (const fn of ALL_FUNCTIONS) m[fn] = [];
    for (const s of dossier.sources) {
      if (s.primaryFunction && m[s.primaryFunction]) {
        m[s.primaryFunction]!.push(s.name);
      }
    }
    return m;
  });

  let coveragePercent = $derived(
    Math.round((dossier.functionCoverage.covered / dossier.functionCoverage.total) * 100)
  );

  // Aggregate publication rate across all sources (docs/day). The Dossier
  // payload already exposes per-source publicationFrequencyPerDay over the
  // requested window; sum is a faithful approximation that does not
  // require a second metrics call.
  let publicationRatePerDay = $derived.by<number | null>(() => {
    let any = false;
    let sum = 0;
    for (const s of dossier.sources) {
      if (s.publicationFrequencyPerDay !== null && s.publicationFrequencyPerDay !== undefined) {
        any = true;
        sum += s.publicationFrequencyPerDay;
      }
    }
    return any ? sum : null;
  });

  const probeContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'probe', dossier.probeId, 'en');
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  function toggleExpanded() {
    expanded = !expanded;
  }

  function toggleSources() {
    sourcesExpanded = !sourcesExpanded;
  }

  function clearScope() {
    setUrl({ sourceIds: [] });
  }

  function clearComposition() {
    setUrl({ probeIds: [] });
  }

  function clearAllScope() {
    setUrl({ sourceIds: [], probeIds: [] });
  }

  let hasAnyScope = $derived(hasComposition || hasNarrowedScope);
</script>

<article class="dossier" aria-labelledby="dossier-title">
  <!-- Brief orientation strip (Finding 1) — explains what the user sees
       before the source-card flood hits them. Renders once per Dossier. -->
  <header class="dossier-intro" aria-label="What is this page?">
    <p class="dossier-eyebrow">Probe Dossier</p>
    <p class="dossier-lede">
      Inventory of a probe — its sources, the discourse functions they cover, and the metadata AĒR
      has on each. From here, open the Workbench with a function as scope filter, or narrow to a
      single source.
    </p>
  </header>

  <section class="probe-section" class:collapsed={!expanded}>
    <!-- Probe header — always visible. Click toggles expansion. -->
    <header class="probe-header">
      <button
        type="button"
        class="probe-toggle"
        aria-expanded={expanded}
        aria-controls="probe-body-{dossier.probeId}"
        onclick={toggleExpanded}
      >
        <span class="chevron" aria-hidden="true" class:expanded>›</span>
        <span class="probe-glyph" aria-hidden="true">◎</span>
        <h1 id="dossier-title" class="probe-title">{dossier.probeId}</h1>
        <span class="lang-badge" aria-label="Primary language: {dossier.language}">
          {dossier.language}
        </span>
        <span class="summary" aria-hidden={expanded ? 'true' : 'false'}>
          {dossier.sources.length} source{dossier.sources.length !== 1 ? 's' : ''} ·
          {dossier.functionCoverage.covered}/{dossier.functionCoverage.total} functions
        </span>
      </button>
      {#if dossier.windowStart && dossier.windowEnd}
        <p
          class="window-note"
          aria-label="Article counts cover the window from {dossier.windowStart} to {dossier.windowEnd}"
        >
          Window:
          <time datetime={dossier.windowStart}
            >{new Date(dossier.windowStart).toLocaleDateString('en-CA')}</time
          >
          →
          <time datetime={dossier.windowEnd}
            >{new Date(dossier.windowEnd).toLocaleDateString('en-CA')}</time
          >
        </p>
      {/if}
    </header>

    {#if expanded}
      <div class="probe-body" id="probe-body-{dossier.probeId}">
        <!-- Scope summary — appears only when the user has narrowed scope
             (sources) or built a composition set (probes). Replaces the
             prior sticky-bottom compose-bar + scope-bar. Single source of
             truth for "what is the current analysis scope on this probe". -->
        {#if hasAnyScope}
          <section
            class="scope-summary"
            role="status"
            aria-live="polite"
            aria-label="Active analysis scope"
          >
            <div class="scope-summary-left">
              <span class="scope-summary-label">Scope</span>
              {#if hasComposition}
                <span class="scope-summary-group">
                  <span class="scope-glyph" aria-hidden="true">⊗</span>
                  <span class="scope-count">
                    {composedProbeIds.length} probe{composedProbeIds.length === 1 ? '' : 's'} composed
                  </span>
                  <ul class="scope-chips" role="list">
                    {#each composedProbeIds as id (id)}
                      <li>
                        <button
                          type="button"
                          class="scope-chip"
                          aria-label="Remove {id} from composition"
                          onclick={() =>
                            setUrl({ probeIds: composedProbeIds.filter((p) => p !== id) })}
                        >
                          {id}
                          <span aria-hidden="true" class="chip-remove">×</span>
                        </button>
                      </li>
                    {/each}
                  </ul>
                </span>
              {/if}
              {#if hasNarrowedScope}
                <span class="scope-summary-group">
                  <span class="scope-glyph" aria-hidden="true">⊂</span>
                  <span class="scope-count">
                    {narrowedSourceIds.length} source{narrowedSourceIds.length === 1 ? '' : 's'} narrowed
                  </span>
                  <ul class="scope-chips" role="list">
                    {#each narrowedSourceIds as id (id)}
                      <li>
                        <button
                          type="button"
                          class="scope-chip"
                          aria-label="Deselect {id}"
                          onclick={() =>
                            setUrl({ sourceIds: narrowedSourceIds.filter((s) => s !== id) })}
                        >
                          {id}
                          <span aria-hidden="true" class="chip-remove">×</span>
                        </button>
                      </li>
                    {/each}
                  </ul>
                </span>
              {/if}
            </div>
            <div class="scope-summary-actions">
              {#if hasComposition && hasNarrowedScope}
                <button type="button" class="scope-mini-clear" onclick={clearComposition}>
                  Clear probes
                </button>
                <button type="button" class="scope-mini-clear" onclick={clearScope}>
                  Clear sources
                </button>
              {/if}
              <button type="button" class="scope-clear-all" onclick={clearAllScope}>
                Clear all
              </button>
              <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Workbench route (Phase 122h) -->
              <button
                type="button"
                class="scope-open-lane"
                onclick={() => {
                  // Carry the active narrow-scope into the Workbench. If
                  // the user has narrowed sources here, those become the
                  // initial source-scope; otherwise the Workbench falls
                  // back to its own DF-derived narrowing on landing.
                  const baseParts = [
                    `probeId=${encodeURIComponent(dossier.probeId)}`,
                    `functionKey=${encodeURIComponent(analyzeTarget)}`,
                    'viewingMode=aleph'
                  ];
                  if (narrowedSourceIds.length > 0) {
                    baseParts.push(`sourceId=${encodeURIComponent(narrowedSourceIds.join(','))}`);
                  } else {
                    const fallback = sourceNamesByFunction[analyzeTarget] ?? [];
                    if (fallback.length > 0) {
                      baseParts.push(`sourceId=${encodeURIComponent(fallback.join(','))}`);
                    }
                  }
                  void goto(`/workbench?${baseParts.join('&')}`);
                }}
              >
                Open Workbench ›
              </button>
              <!-- eslint-enable svelte/no-navigation-without-resolve -->
            </div>
          </section>
        {/if}

        <!-- Emic framing paragraph (semantic register) -->
        {#if probeContentQ.data?.kind === 'success'}
          <p class="emic-frame">{probeContentQ.data.data.registers.semantic.long}</p>
        {/if}

        <!-- Structural meta -->
        <dl class="meta">
          <div>
            <dt>Language</dt>
            <dd>{dossier.language.toUpperCase()}</dd>
          </div>
          <div>
            <dt>Sources</dt>
            <dd>{dossier.sources.length}</dd>
          </div>
          <div>
            <dt>Publication rate</dt>
            <dd>
              {publicationRatePerDay !== null
                ? `${publicationRatePerDay.toFixed(1)} docs/day`
                : '—'}
            </dd>
          </div>
          <div>
            <dt>Function coverage</dt>
            <dd>
              {dossier.functionCoverage.covered}/{dossier.functionCoverage.total} ({coveragePercent}%)
            </dd>
          </div>
        </dl>

        <!-- Function Selector — the central interaction of this layer.
             Each tile opens the Workbench with the function preselected as
             a scope filter and Aleph as the active Pillar (Phase 122h /
             ADR-033 §3). Click on a covered function = direct path into
             the analytical surface. -->
        <section class="function-selector" aria-labelledby="fn-heading">
          <h2 id="fn-heading" class="section-title">Discourse Functions</h2>
          <p class="section-lede">
            Choose a function to open the Workbench with that function preselected as a scope
            filter.
          </p>
          <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Workbench routes (Phase 122h) -->
          <ul class="fn-grid" role="list">
            {#each ALL_FUNCTIONS as fn (fn)}
              {@const meta = FUNCTION_DEFINITIONS[fn]}
              {@const covered = coveredFunctions.has(fn)}
              {@const fnSources = sourcesByFunction[fn] ?? []}
              {@const fnSourceNames = sourceNamesByFunction[fn] ?? []}
              {@const fnSourceParam =
                fnSourceNames.length > 0
                  ? `&sourceId=${encodeURIComponent(fnSourceNames.join(','))}`
                  : ''}
              <li>
                <a
                  class="fn-tile"
                  class:fn-covered={covered}
                  style:--fn-color={meta.color}
                  aria-label="{meta.label}: {covered
                    ? `covered by ${fnSources.join(', ')}`
                    : 'not covered by this probe'} — open Workbench"
                  href="/workbench?probeId={encodeURIComponent(
                    dossier.probeId
                  )}&functionKey={fn}&viewingMode=aleph{fnSourceParam}"
                  data-sveltekit-preload-data="hover"
                >
                  <span class="fn-tile-head">
                    <span class="fn-abbr">{meta.abbr}</span>
                    <span class="fn-state" aria-hidden="true">
                      {covered ? '● covered' : '○ uncovered'}
                    </span>
                  </span>
                  <span class="fn-label">{meta.label}</span>
                  <span class="fn-desc">{meta.description}</span>
                  {#if fnSources.length > 0}
                    <span class="fn-sources" aria-hidden="true">
                      <span class="fn-sources-eyebrow">Sources</span>
                      <span class="fn-sources-list">{fnSources.join(' · ')}</span>
                    </span>
                  {:else}
                    <span class="fn-sources fn-sources-empty" aria-hidden="true">
                      <span class="fn-sources-eyebrow">Sources</span>
                      <span class="fn-sources-list">none in this probe</span>
                    </span>
                  {/if}
                  <span class="fn-cta" aria-hidden="true">Open Workbench →</span>
                </a>
              </li>
            {/each}
          </ul>
          <!-- eslint-enable svelte/no-navigation-without-resolve -->
        </section>

        <!-- Sources -->
        <section class="sources-section" aria-labelledby="sources-heading">
          <button
            type="button"
            class="sources-toggle"
            aria-expanded={sourcesExpanded}
            aria-controls="sources-body"
            onclick={toggleSources}
          >
            <span class="chevron" aria-hidden="true" class:expanded={sourcesExpanded}>›</span>
            <h2 id="sources-heading" class="section-title">
              Sources
              <span class="source-count">({dossier.sources.length})</span>
            </h2>
          </button>

          {#if sourcesExpanded}
            <div id="sources-body">
              {#if dossier.sources.length === 0}
                <p class="muted">No sources configured for this probe.</p>
              {:else}
                <ul class="source-grid" role="list" aria-label="Sources in this probe">
                  {#each dossier.sources as source (source.name)}
                    <li>
                      <SourceCard {source} {ctx} {windowStart} {windowEnd} />
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          {/if}
        </section>

        <!-- Phase 122f: per-source-per-field metadata-coverage matrix.
             Operationalises WP-003 §3.2 (metadata-richness asymmetry as
             a structural bias) at runtime; the field-level Negative-Space
             rendering (Brief §7.7) flips structurally-absent cells into
             methodological-register prose when the overlay is on. -->
        <section class="dossier-section" aria-labelledby="metadata-coverage-heading">
          <MetadataCoveragePanel probeId={dossier.probeId} {ctx} />
        </section>
      </div>
    {/if}
  </section>
</article>

<style>
  .dossier {
    width: 100%;
    padding: 0 var(--space-6); /* Guarantees an equal gap on both the left and right edges */
    margin: 0 auto;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .probe-section {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-bg);
  }

  .probe-section.collapsed {
    background: var(--color-surface);
  }

  .probe-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }

  .probe-section.collapsed .probe-header {
    border-bottom: none;
  }

  .probe-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0;
    background: transparent;
    border: none;
    cursor: pointer;
    color: inherit;
    text-align: left;
    flex-wrap: wrap;
    width: 100%;
  }

  .probe-toggle:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-radius: var(--radius-sm);
  }

  .chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1rem;
    height: 1rem;
    color: var(--color-fg-muted);
    transform: rotate(0deg);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }

  .chevron.expanded {
    transform: rotate(90deg);
  }

  .probe-glyph {
    color: var(--color-accent);
    font-size: 1.1em;
  }

  .probe-title {
    font-size: var(--font-size-lg);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-medium);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0;
  }

  .lang-badge {
    display: inline-block;
    padding: 2px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .summary {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    margin-left: auto;
  }

  .window-note {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    margin: 0;
    padding-left: 1.75rem;
  }

  .probe-body {
    padding: var(--space-5) var(--space-6) var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .emic-frame {
    margin: 0;
    color: var(--color-fg);
    font-size: var(--font-size-base);
    line-height: 1.55;
    border-left: 2px solid var(--color-accent);
    padding: var(--space-2) var(--space-3);
  }

  dl.meta {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: var(--space-2) var(--space-4);
    margin: 0;
    font-size: var(--font-size-xs);
  }
  dl.meta > div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  dt {
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-size: 10px;
  }
  dd {
    margin: 0;
    color: var(--color-fg);
    font-family: var(--font-mono);
  }

  .section-title {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-2) 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .section-lede {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0 0 var(--space-3) 0;
    /* Full width (Finding 1.3 / 1.4) — short description does not need to
       wrap to multiple lines; saves vertical real estate. */
    line-height: var(--line-height-loose);
  }

  /* Top-of-dossier intro (Finding 1). Single, brief orientation block so
     users know what they are looking at before being hit by source cards. */
  .dossier-intro {
    padding: var(--space-3) var(--space-6) 0;
  }

  .dossier-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-1) 0;
  }

  .dossier-lede {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
    max-width: 78ch;
  }

  .source-count {
    font-family: var(--font-mono);
    font-weight: var(--font-weight-regular);
    color: var(--color-fg-muted);
    letter-spacing: 0;
  }

  .function-selector {
    display: flex;
    flex-direction: column;
  }

  .fn-grid {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
    gap: var(--space-3);
  }

  .fn-tile {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--color-border);
    border-radius: var(--radius-md);
    text-decoration: none;
    color: var(--color-fg);
    transition:
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      background var(--motion-duration-fast) var(--motion-ease-standard);
    height: 100%;
  }

  .fn-tile.fn-covered {
    border-left-color: var(--fn-color);
    background: color-mix(in srgb, var(--fn-color) 8%, var(--color-surface));
  }

  .fn-tile:hover,
  .fn-tile:focus-visible {
    border-color: var(--fn-color);
    background: color-mix(in srgb, var(--fn-color) 14%, var(--color-surface));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .fn-tile-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .fn-abbr {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 32px;
    padding: 2px var(--space-2);
    background: color-mix(in srgb, var(--fn-color) 18%, transparent);
    border: 1px solid var(--fn-color);
    border-radius: var(--radius-sm);
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    color: var(--fn-color);
    letter-spacing: 0.04em;
  }

  .fn-state {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .fn-tile.fn-covered .fn-state {
    color: var(--fn-color);
  }

  .fn-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }

  .fn-desc {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  /* Source-to-function mapping (Finding 1.5b) — each tile lists which
     sources in this probe cover its function, so the user can read at
     a glance which sources will be in scope when they open Workbench.
     `margin-top: auto` pushes this row + the CTA below it to the
     bottom of the tile so all four tiles align regardless of how
     long the description is. */
  .fn-sources {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-2) 0 0;
    border-top: 1px dashed color-mix(in srgb, var(--fn-color) 30%, transparent);
    margin-top: auto;
  }

  .fn-sources-eyebrow {
    font-family: var(--font-mono);
    font-size: 9.5px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .fn-sources-list {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--fn-color);
    font-weight: var(--font-weight-medium);
  }

  .fn-sources-empty .fn-sources-list {
    color: var(--color-fg-subtle);
    font-style: italic;
    font-weight: var(--font-weight-regular);
  }

  .fn-cta {
    padding-top: var(--space-2);
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    font-family: var(--font-mono);
  }

  .sources-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .sources-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0;
    background: transparent;
    border: none;
    cursor: pointer;
    color: inherit;
    text-align: left;
    width: 100%;
  }

  .sources-toggle .chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1rem;
    height: 1rem;
    color: var(--color-fg-muted);
    transform: rotate(0deg);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }

  .sources-toggle .chevron.expanded {
    transform: rotate(90deg);
  }

  .sources-toggle:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-radius: var(--radius-sm);
  }

  /* --- Top scope summary (Probe-Dossier head) ---
     A single row directly under the probe header that summarises ALL
     active scope narrowings (probe composition + source narrowing) plus
     a single navigation action into the lane. Replaces the prior two
     sticky bottom bars; appears only when scope is actually narrowed. */
  .scope-summary {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-3) var(--space-4);
    background: color-mix(in srgb, var(--color-surface) 92%, var(--color-accent-muted));
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-md);
    flex-wrap: wrap;
  }

  .scope-summary-left {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    flex: 1;
    min-width: 0;
  }

  .scope-summary-label {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
    flex-shrink: 0;
  }

  .scope-summary-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .scope-glyph {
    color: var(--color-accent);
    font-size: var(--font-size-base);
    line-height: 1;
    flex-shrink: 0;
  }

  .scope-count {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    flex-shrink: 0;
  }

  .scope-summary-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    flex-shrink: 0;
  }

  .scope-mini-clear,
  .scope-clear-all {
    padding: var(--space-1) var(--space-3);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .scope-mini-clear:hover,
  .scope-mini-clear:focus-visible,
  .scope-clear-all:hover,
  .scope-clear-all:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .scope-open-lane {
    padding: var(--space-1) var(--space-4);
    background: var(--color-accent);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    font-family: var(--font-mono);
    color: var(--color-bg);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .scope-open-lane:hover,
  .scope-open-lane:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, white);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .scope-chips {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .scope-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px var(--space-2);
    background: rgba(82, 131, 184, 0.12);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .scope-chip:hover,
  .scope-chip:focus-visible {
    background: rgba(82, 131, 184, 0.22);
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .chip-remove {
    color: var(--color-accent-muted);
    font-size: 0.8em;
    line-height: 1;
  }

  .source-grid {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(22rem, 1fr));
    gap: var(--space-4);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .chevron,
    .sources-toggle .chevron,
    .fn-tile,
    .scope-chip,
    .scope-mini-clear,
    .scope-clear-all,
    .scope-open-lane {
      transition: none;
    }
  }
</style>

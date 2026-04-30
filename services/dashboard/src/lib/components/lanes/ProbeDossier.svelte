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
  import ValidComparisonsPanel from './ValidComparisonsPanel.svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { goto } from '$app/navigation';

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

  const FUNCTION_META: Record<
    string,
    { label: string; abbr: string; color: string; description: string }
  > = {
    epistemic_authority: {
      label: 'Epistemic Authority',
      abbr: 'EA',
      color: '#5283b8',
      description: 'Sources that produce and legitimate knowledge claims (WP-001 §3).'
    },
    power_legitimation: {
      label: 'Power Legitimation',
      abbr: 'PL',
      color: '#7ec4a0',
      description: 'Sources that frame, justify, or contest political power (WP-001 §3).'
    },
    cohesion_identity: {
      label: 'Cohesion & Identity',
      abbr: 'CI',
      color: '#c8a85a',
      description: 'Sources that articulate collective identity and social cohesion (WP-001 §3).'
    },
    subversion_friction: {
      label: 'Subversion & Friction',
      abbr: 'SF',
      color: '#9a8fb8',
      description: 'Sources that challenge dominant frames or introduce friction (WP-001 §3).'
    }
  };

  const ALL_FUNCTIONS = Object.keys(FUNCTION_META);
  let coveredFunctions = $derived(new Set(dossier.functionCoverage.functions as string[]));

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
</script>

<article class="dossier" aria-labelledby="dossier-title">
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
             Each tile descends to /lanes/{probeId}/{functionKey} (Surface
             II L3). Equivalent affordance is also present in the
             ScopeBar's lane tabs, but the tiles are the primary path. -->
        <section class="function-selector" aria-labelledby="fn-heading">
          <h2 id="fn-heading" class="section-title">Discourse Functions</h2>
          <p class="section-lede">
            Choose a function to enter Surface II · L3 with charts, view modes, and methodology for
            this probe.
          </p>
          <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface II routes -->
          <ul class="fn-grid" role="list">
            {#each ALL_FUNCTIONS as fn (fn)}
              {@const meta = FUNCTION_META[fn]!}
              {@const covered = coveredFunctions.has(fn)}
              <li>
                <a
                  class="fn-tile"
                  class:fn-covered={covered}
                  style:--fn-color={meta.color}
                  aria-label="{meta.label}: {covered
                    ? 'covered by this probe'
                    : 'not covered by this probe'} — open lane"
                  href="/lanes/{dossier.probeId}/{fn}"
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
                  <span class="fn-cta" aria-hidden="true">Open lane →</span>
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

        <!-- Phase 115: per-metric Level-1/2/3 availability matrix —
             surfaces the methodological boundary of the probe before
             the user encounters a refusal in a function lane. -->
        <section class="dossier-section" aria-labelledby="valid-comparisons-heading">
          <h2 id="valid-comparisons-heading" class="section-title">Valid comparisons</h2>
          <ValidComparisonsPanel probeId={dossier.probeId} {ctx} />
        </section>
      </div>
    {/if}
  </section>

  <!-- Probe composition bar — sticky above the source scope bar.
       Appears when the user has shift+clicked probes on the globe. Shows
       the full composition set and a Compose CTA to enter multi-probe
       analysis in this probe's function lanes. -->
  {#if hasComposition}
    <div class="compose-bar" role="status" aria-live="polite" aria-label="Probe composition">
      <div class="compose-bar-left">
        <span class="compose-bar-label" aria-hidden="true">⊗</span>
        <span class="compose-bar-count">
          {composedProbeIds.length === 1 ? '1 probe' : `${composedProbeIds.length} probes`} in composition
        </span>
        <ul class="scope-chips" role="list">
          {#each composedProbeIds as id (id)}
            <li>
              <button
                type="button"
                class="scope-chip"
                aria-label="Remove {id} from composition"
                onclick={() => setUrl({ probeIds: composedProbeIds.filter((p) => p !== id) })}
              >
                {id}
                <span aria-hidden="true" class="chip-remove">×</span>
              </button>
            </li>
          {/each}
        </ul>
      </div>
      <div class="scope-bar-actions">
        <button type="button" class="scope-bar-clear" onclick={clearComposition}>Clear</button>
        <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface II route -->
        <button
          type="button"
          class="scope-bar-analyze compose-primary"
          onclick={() =>
            void goto(`/lanes/${encodeURIComponent(dossier.probeId)}/${analyzeTarget}`)}
        >
          Compose ›
        </button>
        <!-- eslint-enable svelte/no-navigation-without-resolve -->
      </div>
    </div>
  {/if}

  <!-- Scope action bar — sticky at the bottom of the dossier scroll area.
       Appears when one or more sources are narrowed. Lets the user build up
       a multi-source selection before committing to analysis. -->
  {#if hasNarrowedScope}
    <div class="scope-bar" role="status" aria-live="polite" aria-label="Scope selection">
      <div class="scope-bar-left">
        <span class="scope-bar-label" aria-hidden="true">⊂</span>
        <span class="scope-bar-count">
          {narrowedSourceIds.length === 1 ? '1 source' : `${narrowedSourceIds.length} sources`}
        </span>
        <ul class="scope-chips" role="list">
          {#each narrowedSourceIds as id (id)}
            <li>
              <button
                type="button"
                class="scope-chip"
                aria-label="Deselect {id}"
                onclick={() => setUrl({ sourceIds: narrowedSourceIds.filter((s) => s !== id) })}
              >
                {id}
                <span aria-hidden="true" class="chip-remove">×</span>
              </button>
            </li>
          {/each}
        </ul>
      </div>
      <div class="scope-bar-actions">
        <button type="button" class="scope-bar-clear" onclick={clearScope}>Clear</button>
        <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface II route -->
        <button
          type="button"
          class="scope-bar-analyze"
          onclick={() =>
            void goto(`/lanes/${encodeURIComponent(dossier.probeId)}/${analyzeTarget}`)}
        >
          Analyze ›
        </button>
        <!-- eslint-enable svelte/no-navigation-without-resolve -->
      </div>
    </div>
  {/if}
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
    max-width: 60ch;
    line-height: var(--line-height-loose);
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

  .fn-cta {
    margin-top: auto;
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

  /* --- Probe composition bar --- */

  .compose-bar {
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-3) var(--space-5);
    background: var(--color-surface);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-lg);
    box-shadow: 0 -2px 16px rgba(82, 131, 184, 0.25);
    flex-wrap: wrap;
  }

  .compose-bar-left {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    flex: 1;
    min-width: 0;
  }

  .compose-bar-label {
    color: var(--color-accent);
    font-size: var(--font-size-base);
    flex-shrink: 0;
  }

  .compose-bar-count {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    flex-shrink: 0;
  }

  .compose-primary {
    background: var(--color-accent);
    border-color: var(--color-accent);
    color: var(--color-bg);
  }

  .compose-primary:hover,
  .compose-primary:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, white);
  }

  /* --- Scope action bar --- */

  .scope-bar {
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-3) var(--space-5);
    background: var(--color-surface);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-lg);
    box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.3);
    flex-wrap: wrap;
  }

  .scope-bar-left {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    flex: 1;
    min-width: 0;
  }

  .scope-bar-label {
    color: var(--color-accent);
    font-size: var(--font-size-base);
    flex-shrink: 0;
  }

  .scope-bar-count {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    flex-shrink: 0;
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

  .scope-bar-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .scope-bar-clear {
    padding: var(--space-1) var(--space-3);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .scope-bar-clear:hover,
  .scope-bar-clear:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .scope-bar-analyze {
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

  .scope-bar-analyze:hover,
  .scope-bar-analyze:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, white);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
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
    .scope-bar-clear,
    .scope-bar-analyze {
      transition: none;
    }
  }
</style>

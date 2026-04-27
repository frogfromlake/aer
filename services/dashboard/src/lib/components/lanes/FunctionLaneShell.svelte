<script lang="ts">
  // Function Lane Shell — view-mode-driven render (Phase 107).
  //
  // The Phase 106 baseline (one uPlot time-series per source) is now one
  // cell of the View-Mode Matrix. The shell:
  //   1. Resolves the active presentation from the URL via $lib/viewmodes.
  //   2. Lazy-imports the cell component (Plot / d3-force chunks defer).
  //   3. Renders the cell with the resolved scope (probe vs. source) and
  //      the lane's default metric.
  // Empty-lane Dual-Register invitation behaviour is preserved when no
  // sources match the function key.
  import { createQuery } from '@tanstack/svelte-query';
  import type { Component } from 'svelte';
  import {
    contentQuery,
    type ProbeDossierDto,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import {
    DEFAULT_METRIC_NAME,
    cellContentId,
    getPresentation,
    type ViewModeCellProps
  } from '$lib/viewmodes';
  import { urlState } from '$lib/state/url.svelte';
  import { setFocusedMetric } from '$lib/state/metric.svelte';
  import { negativeSpaceActive } from '$lib/state/tray.svelte';
  import SilverIneligiblePanel from './SilverIneligiblePanel.svelte';
  import LensBar from './LensBar.svelte';

  interface Props {
    functionKey: string;
    dossier: ProbeDossierDto | null;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    sourceIds: string[];
  }

  let { functionKey, dossier, ctx, windowStart, windowEnd, sourceIds }: Props = $props();

  const FUNCTION_META: Record<string, { label: string; abbr: string; description: string }> = {
    epistemic_authority: {
      label: 'Epistemic Authority',
      abbr: 'EA',
      description: 'Sources that produce and legitimate knowledge claims (WP-001 §3).'
    },
    power_legitimation: {
      label: 'Power Legitimation',
      abbr: 'PL',
      description: 'Sources that frame, justify, or contest political power (WP-001 §3).'
    },
    cohesion_identity: {
      label: 'Cohesion & Identity',
      abbr: 'CI',
      description: 'Sources that articulate collective identity and social cohesion (WP-001 §3).'
    },
    subversion_friction: {
      label: 'Subversion & Friction',
      abbr: 'SF',
      description: 'Sources that challenge dominant frames or introduce friction (WP-001 §3).'
    }
  };

  let meta = $derived(FUNCTION_META[functionKey]);

  // Sources in this function lane.
  // Default: filter by primaryFunction (normal single-function browsing).
  // Manual selection: when the user explicitly narrowed scope via source cards,
  // show ALL selected sources regardless of their discourse function — they made
  // a cross-function selection deliberately. The function lane becomes the
  // analysis context rather than a filter.
  let laneSources = $derived.by(() => {
    if (!dossier) return [];
    if (sourceIds.length > 0) return dossier.sources.filter((s) => sourceIds.includes(s.name));
    return dossier.sources.filter((s) => s.primaryFunction === functionKey);
  });

  // Discourse functions represented by the current selection. Used by LensBar
  // to show secondary "has selection" markers on non-active function buttons.
  let activeFunctionKeys = $derived.by<string[]>(() => {
    const keys: string[] = [functionKey];
    if (sourceIds.length > 0 && dossier) {
      for (const id of sourceIds) {
        const fn = dossier.sources.find((s) => s.name === id)?.primaryFunction;
        if (fn && !keys.includes(fn)) keys.push(fn);
      }
    }
    return keys;
  });

  let isEmpty = $derived(laneSources.length === 0);

  // Empty-lane Dual-Register content from the Content Catalog (Phase 104).
  const emptyContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'empty_lane', functionKey);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: isEmpty
    };
  });

  const url = $derived(urlState());
  let presentation = $derived(getPresentation(url.viewMode));
  let metricName = $derived(url.metric ?? DEFAULT_METRIC_NAME);

  let scope = $derived<'probe' | 'source'>(sourceIds.length === 1 ? 'source' : 'probe');
  // When exactly one source is narrowed, the cell queries use source scope.
  // With multiple sources, probe scope is used and the cell receives the filtered source list.
  let scopeId = $derived<string>(sourceIds.length === 1 ? sourceIds[0]! : (dossier?.probeId ?? ''));

  // Phase 111 — Silver-layer routing.
  let dataLayer = $derived<'gold' | 'silver'>(url.layer === 'silver' ? 'silver' : 'gold');

  // Phase 112 — Negative Space overlay.
  const negSpace = $derived(negativeSpaceActive());

  // For Silver-eligibility, single-source scope gives us the record to check.
  let singleSourceRecord = $derived(
    sourceIds.length === 1 ? (dossier?.sources.find((s) => s.name === sourceIds[0]) ?? null) : null
  );
  let silverEligible = $derived(
    dataLayer === 'gold' || sourceIds.length === 0 || (singleSourceRecord?.silverEligible ?? false)
  );

  // Per-cell view-mode content from the catalog. Lookup by composed cell
  // id so each (presentation × metric) pair carries its own Dual-Register
  // entry (see Arc42 §8.13 / 8.11). The query is non-blocking — the cell
  // renders even if content is missing, and the methodology tray (Phase
  // 108) consumes the same entry.
  let contentId = $derived(cellContentId(presentation.id, metricName));
  const viewModeContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', contentId);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: !isEmpty
    };
  });

  // Lazy-load the cell component so each presentation's heavy chunk
  // (Observable Plot / d3-force) defers to selection.
  let CellComponent = $state<Component<ViewModeCellProps> | null>(null);
  let cellLoadError = $state<string | null>(null);
  let loadToken = 0;

  $effect(() => {
    const id = presentation.id;
    const t = ++loadToken;
    // Do NOT null CellComponent here — keep the old cell visible while
    // the new one loads to avoid a blank flash (L3 item 4 flicker fix).
    cellLoadError = null;
    presentation
      .loadComponent()
      .then((Comp) => {
        if (t !== loadToken) return;
        CellComponent = Comp;
      })
      .catch((err: unknown) => {
        if (t !== loadToken) return;
        CellComponent = null;
        cellLoadError = err instanceof Error ? err.message : `Failed to load ${id}`;
      });
  });

  let cellSources = $derived(
    laneSources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }))
  );

  $effect(() => {
    if (isEmpty) return;
    const ctxParts: string[] = [`probe ${dossier?.probeId ?? '—'}`, presentation.label];
    if (sourceIds.length > 0) ctxParts.push(`source ${sourceIds.join(', ')}`);
    if (dataLayer === 'silver') ctxParts.push('Silver layer');
    setFocusedMetric({
      metricName,
      chartContext: ctxParts.join(' · ')
    });
  });

  // Methodology panel — expandable section default open (L3 item 8).
  let methodologyExpanded = $state(true);
</script>

<section class="lane" class:neg-space={negSpace} aria-labelledby="lane-heading-{functionKey}">
  <!-- Lane header -->
  <header class="lane-header">
    <div class="lane-identity">
      <span class="fn-abbr" aria-hidden="true">{meta?.abbr ?? functionKey}</span>
      <h2 id="lane-heading-{functionKey}" class="fn-label">
        {meta?.label ?? functionKey}
      </h2>
      <span class="cell-id" aria-label="Active view-mode cell">
        <span class="cell-id-view">{presentation.label}</span>
        <span class="cell-id-sep" aria-hidden="true">·</span>
        <code class="cell-id-metric">{metricName}</code>
      </span>
    </div>

    <!-- Source list — prominently visible to show what data is displayed (L3 item 5) -->
    {#if laneSources.length > 0}
      <div class="source-list" aria-label="Sources in this lane">
        <span class="source-list-label">Sources:</span>
        {#each laneSources as s (s.name)}
          <span class="source-chip">{s.emicDesignation ?? s.name}</span>
        {/each}
        {#if sourceIds.length > 0}
          <span class="source-scope-indicator" title="Scope narrowed from probe">⊂ scoped</span>
        {/if}
      </div>
    {/if}

    <!-- Phase 113c: prominent link into Surface III for the function's
         long-form treatment. The `from=lane&probe=…&fn=…` hint is read
         by the Working Paper page to render a "Back to Function Lane"
         affordance. -->
    <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface III route -->
    <p class="wp-link">
      <a
        href="/reflection/wp/wp-001?section={functionKey}&from=lane&probe={dossier?.probeId ??
          ''}&fn={functionKey}"
      >
        Read the full Working Paper · WP-001 · {meta?.label ?? functionKey} →
      </a>
    </p>
    <!-- eslint-enable svelte/no-navigation-without-resolve -->
  </header>

  <!-- Lens bar: metric × view-mode pair (Phase 113c). -->
  {#if !isEmpty}
    <LensBar {activeFunctionKeys} />
  {/if}

  <!-- Lane body -->
  <div class="lane-body">
    {#if negSpace && !isEmpty}
      <!-- Surface II — Negative Space mode: demographic-skew annotation (Phase 112).
           WP-003 §6.1 documents the demographic opacity of the current source selection. -->
      <aside class="demographic-annotation" aria-label="Demographic scope note">
        <span class="annotation-glyph" aria-hidden="true">∅</span>
        <div class="annotation-body">
          <p class="annotation-lead">Demographic scope boundary</p>
          <p class="annotation-text">
            Sources in this lane may not reflect the full demographic spectrum of the discourse
            domain. Speaker identity, age, gender, class, and regional variation are not captured by
            current probes. —
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal WP route -->
            <a class="annotation-ref" href="/reflection/wp/wp-003?section=6.1">WP-003 §6.1</a>
          </p>
        </div>
      </aside>
    {/if}

    {#if isEmpty}
      <!-- Empty-lane invitation from Content Catalog -->
      <div class="empty-lane" class:neg-space-prominent={negSpace} role="status">
        {#if emptyContentQ.isPending}
          <p class="muted" aria-busy="true">…</p>
        {:else if emptyContentQ.data?.kind === 'success'}
          {@const registers = emptyContentQ.data.data.registers}
          <div class="invitation">
            <p class="invitation-semantic">{registers.semantic.short}</p>
            <details class="invitation-detail">
              <summary>Methodological context</summary>
              <p class="invitation-methodological">{registers.methodological.long}</p>
            </details>
          </div>
        {:else}
          <p class="muted">
            No sources currently assigned to this discourse function in this probe.
          </p>
        {/if}
        <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface II route -->
        <a
          class="back-to-dossier"
          href="/lanes/{dossier?.probeId}/dossier"
          data-sveltekit-preload-data="off"
        >
          ← Back to Probe Dossier
        </a>
        <!-- eslint-enable svelte/no-navigation-without-resolve -->
      </div>
    {:else if dataLayer === 'silver' && sourceIds.length === 0}
      <!-- Silver mode requires a single source — probe scope is undefined for Silver -->
      <div class="silver-scope-prompt" role="status">
        <p class="silver-scope-msg">
          Silver-layer data is available per source. Narrow the scope to a single source using the
          <strong>⊂ Narrow scope</strong> action on a source card in the Probe Dossier, then re-select
          the Silver layer.
        </p>
      </div>
    {:else if dataLayer === 'silver' && !silverEligible && singleSourceRecord}
      <!-- Active source not Silver-eligible -->
      <SilverIneligiblePanel source={singleSourceRecord} />
    {:else if cellLoadError}
      <p class="muted">Could not load view: {cellLoadError}</p>
    {:else if !CellComponent}
      <p class="muted" aria-busy="true">Loading {presentation.label.toLowerCase()}…</p>
    {:else if scopeId === ''}
      <p class="muted">Resolving scope…</p>
    {:else}
      {@const Cell = CellComponent}
      <Cell
        {ctx}
        scopeProbeId={dossier?.probeId ?? ''}
        {scope}
        {scopeId}
        {windowStart}
        {windowEnd}
        {metricName}
        sources={cellSources}
        {dataLayer}
      />

      <!-- Methodology section — expandable, default open (L3 item 8) -->
      {#if viewModeContentQ.data?.kind === 'success'}
        {@const methodContent = viewModeContentQ.data.data}
        <div class="methodology-section">
          <button
            type="button"
            class="methodology-toggle"
            aria-expanded={methodologyExpanded}
            aria-controls="methodology-body"
            onclick={() => (methodologyExpanded = !methodologyExpanded)}
          >
            <span
              class="methodology-chevron"
              aria-hidden="true"
              class:expanded={methodologyExpanded}>›</span
            >
            <span class="methodology-title">Methodology — {presentation.label} × {metricName}</span>
          </button>
          {#if methodologyExpanded}
            <div id="methodology-body" class="methodology-body">
              <p class="methodology-text">{methodContent.registers.methodological.long}</p>
              <!-- eslint-disable svelte/no-navigation-without-resolve -- internal WP link -->
              <a
                class="methodology-wp-link"
                href="/reflection/wp/wp-001?section={functionKey}&from=lane&probe={dossier?.probeId ??
                  ''}&fn={functionKey}"
              >
                Read the full Working Paper · WP-001 · {meta?.label ?? functionKey} →
              </a>
              <!-- eslint-enable svelte/no-navigation-without-resolve -->
            </div>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</section>

<style>
  .lane {
    display: flex;
    flex-direction: column;
    min-height: 100%;
  }

  .lane-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-5) var(--space-6) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }

  .lane-identity {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .fn-abbr {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    background: rgba(82, 131, 184, 0.15);
    border: 1px solid #5283b8;
    border-radius: var(--radius-sm);
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    color: #5283b8;
    flex-shrink: 0;
  }

  .fn-label {
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
  }

  .cell-id {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    padding: 2px var(--space-2);
    background: rgba(82, 131, 184, 0.12);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-sm);
  }

  .cell-id-view {
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    font-weight: var(--font-weight-medium);
  }

  .cell-id-sep {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .cell-id-metric {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .source-list {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-1);
    margin-top: 2px;
  }

  .source-list-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    flex-shrink: 0;
  }

  .source-chip {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    padding: 1px var(--space-2);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-pill);
    color: var(--color-fg);
  }

  .source-scope-indicator {
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-accent);
    padding: 1px var(--space-1);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-pill);
  }

  .back-to-dossier {
    display: inline-flex;
    align-items: center;
    margin-left: auto;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-3);
    text-decoration: none;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .back-to-dossier:hover,
  .back-to-dossier:focus-visible {
    color: var(--color-accent);
    border-color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .methodology-section {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
  }

  .methodology-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg-elevated);
    border: none;
    cursor: pointer;
    color: var(--color-fg-muted);
    text-align: left;
    width: 100%;
    transition: background var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .methodology-toggle:hover,
  .methodology-toggle:focus-visible {
    background: var(--color-surface);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .methodology-chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1rem;
    height: 1rem;
    color: var(--color-fg-subtle);
    transform: rotate(0deg);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }

  .methodology-chevron.expanded {
    transform: rotate(90deg);
  }

  .methodology-title {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
  }

  .methodology-body {
    padding: var(--space-4) var(--space-5);
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    border-top: 1px solid var(--color-border);
  }

  .methodology-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .methodology-wp-link {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent-muted);
    align-self: flex-start;
  }

  .methodology-wp-link:hover,
  .methodology-wp-link:focus-visible {
    color: var(--color-fg);
    border-bottom-style: solid;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .wp-link {
    margin: var(--space-2) 0 0;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
  }
  .wp-link a {
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent-muted);
  }
  .wp-link a:hover,
  .wp-link a:focus-visible {
    color: var(--color-fg);
    border-bottom-style: solid;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .lane-body {
    padding: var(--space-5) var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    flex: 1;
  }

  .empty-lane {
    display: flex;
    align-items: flex-start;
    padding: var(--space-5);
    background: var(--color-bg-elevated);
    border: 1px dashed var(--color-border-strong);
    border-radius: var(--radius-lg);
  }

  /* Negative Space mode — empty lanes become visually prominent (Phase 112).
     An absence IS data; empty lanes gain a positive visual weight rather than
     fading into the background. */
  .empty-lane.neg-space-prominent {
    background: rgba(82, 131, 184, 0.06);
    border-style: solid;
    border-color: rgba(82, 131, 184, 0.4);
  }

  /* Demographic-skew annotation — shown in the lane body when negSpace is on. */
  .demographic-annotation {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-4);
    background: rgba(82, 131, 184, 0.06);
    border: 1px solid rgba(82, 131, 184, 0.3);
    border-left: 3px solid rgba(82, 131, 184, 0.6);
    border-radius: var(--radius-md);
  }

  .annotation-glyph {
    font-size: 1.2rem;
    color: rgba(82, 131, 184, 0.7);
    line-height: 1.4;
    flex-shrink: 0;
  }

  .annotation-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .annotation-lead {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: rgba(82, 131, 184, 0.9);
    font-weight: var(--font-weight-semibold);
    margin: 0;
  }

  .annotation-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
    max-width: 60ch;
  }

  .annotation-ref {
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent);
  }

  .annotation-ref:hover,
  .annotation-ref:focus-visible {
    color: var(--color-fg);
    border-bottom-color: var(--color-fg);
  }

  .invitation {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    max-width: 36rem;
  }

  .invitation-semantic {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .invitation-detail {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .invitation-detail summary {
    cursor: pointer;
    color: var(--color-accent-muted);
    list-style: none;
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .invitation-detail summary:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .invitation-methodological {
    margin: var(--space-2) 0 0;
    line-height: var(--line-height-loose);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .silver-scope-prompt {
    display: flex;
    align-items: flex-start;
    padding: var(--space-5);
    background: rgba(126, 196, 160, 0.06);
    border: 1px dashed #7ec4a0;
    border-radius: var(--radius-lg);
    max-width: 42rem;
  }

  .silver-scope-msg {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .silver-scope-msg strong {
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }
</style>

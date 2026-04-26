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

  interface Props {
    functionKey: string;
    dossier: ProbeDossierDto | null;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    sourceId: string | null;
  }

  let { functionKey, dossier, ctx, windowStart, windowEnd, sourceId }: Props = $props();

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

  // Sources in this function lane — filtered by primaryFunction.
  // Narrowed to sourceId when source scope is active.
  let laneSources = $derived.by(() => {
    if (!dossier) return [];
    const matched = dossier.sources.filter((s) => s.primaryFunction === functionKey);
    if (sourceId) return matched.filter((s) => s.name === sourceId);
    return matched;
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

  let scope = $derived<'probe' | 'source'>(sourceId ? 'source' : 'probe');
  let scopeId = $derived<string>(sourceId ?? dossier?.probeId ?? '');

  // Phase 111 — Silver-layer routing.
  let dataLayer = $derived<'gold' | 'silver'>(url.layer === 'silver' ? 'silver' : 'gold');

  // Phase 112 — Negative Space overlay.
  const negSpace = $derived(negativeSpaceActive());

  // Active source record from the dossier (null when using probe scope).
  let activeSourceRecord = $derived(
    sourceId ? (dossier?.sources.find((s) => s.name === sourceId) ?? null) : null
  );
  // Eligibility: true when Gold layer, or when Silver + no source-scope (probe
  // scope; eligibility is evaluated per source, not per probe — the lane body
  // will show a "narrow to a source" prompt instead), or when Silver + the
  // active source has passed the WP-006 §5.2 review.
  let silverEligible = $derived(
    dataLayer === 'gold' || !sourceId || (activeSourceRecord?.silverEligible ?? false)
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
    CellComponent = null;
    cellLoadError = null;
    presentation
      .loadComponent()
      .then((Comp) => {
        if (t !== loadToken) return;
        CellComponent = Comp;
      })
      .catch((err: unknown) => {
        if (t !== loadToken) return;
        cellLoadError = err instanceof Error ? err.message : `Failed to load ${id}`;
      });
  });

  let cellSources = $derived(
    laneSources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }))
  );

  // Phase 108: every (probe, function-key, view-mode, metric, source-scope,
  // data-layer) change in the lane retargets the methodology tray.
  $effect(() => {
    if (isEmpty) return;
    const ctxParts: string[] = [`probe ${dossier?.probeId ?? '—'}`, presentation.label];
    if (sourceId) ctxParts.push(`source ${sourceId}`);
    if (dataLayer === 'silver') ctxParts.push('Silver layer');
    setFocusedMetric({
      metricName,
      chartContext: ctxParts.join(' · ')
    });
  });
</script>

<section class="lane" class:neg-space={negSpace} aria-labelledby="lane-heading-{functionKey}">
  <!-- Lane header -->
  <header class="lane-header">
    <div class="lane-identity">
      <span class="fn-abbr" aria-hidden="true">{meta?.abbr ?? functionKey}</span>
      <h2 id="lane-heading-{functionKey}" class="fn-label">
        {meta?.label ?? functionKey}
      </h2>
      <span class="source-count">
        {laneSources.length} source{laneSources.length !== 1 ? 's' : ''}
      </span>
      <span class="cell-id" aria-label="Active view-mode cell">
        {presentation.label} · <code>{metricName}</code>
      </span>
    </div>
    {#if meta?.description}
      <p class="fn-description">{meta.description}</p>
    {/if}
    {#if !isEmpty && viewModeContentQ.data?.kind === 'success'}
      <p class="cell-semantic">
        {viewModeContentQ.data.data.registers.semantic.short}
      </p>
    {/if}
  </header>

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
      </div>
    {:else if dataLayer === 'silver' && !sourceId}
      <!-- Silver mode requires a single source — probe scope is undefined for Silver -->
      <div class="silver-scope-prompt" role="status">
        <p class="silver-scope-msg">
          Silver-layer data is available per source. Narrow the scope to a single source using the
          <strong>⊂ Narrow scope</strong> action on a source card, then re-select the Silver layer.
        </p>
      </div>
    {:else if dataLayer === 'silver' && !silverEligible && activeSourceRecord}
      <!-- Active source not Silver-eligible -->
      <SilverIneligiblePanel source={activeSourceRecord} />
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

  .source-count {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
  }

  .cell-id {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin-left: auto;
  }

  .cell-id code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }

  .fn-description {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 0;
    line-height: var(--line-height-loose);
  }

  .cell-semantic {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
    padding-top: var(--space-2);
    max-width: 64ch;
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

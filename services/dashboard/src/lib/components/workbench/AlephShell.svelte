<script lang="ts">
  // AlephShell — Phase 122h / ADR-033 §2 (Aleph paragraph).
  //
  // The Aleph Pillar renders a synchronic snapshot of the dataset.
  // Layout:
  //   - Top strip: Dataset-Shape (probe count, source count, article count,
  //     language mix). Reads from `/probes/{id}/dossier` for the active
  //     probe scope.
  //   - Body: a single focus Cell occupying the full width. Cells are
  //     restricted to Aleph-allowed presentations (`time_series`,
  //     `distribution`) per `presentations/registry.ts:PILLAR_DEFINITIONS`.
  //   - Per-Cell controls (Metric, Darstellung, Layer, Vergleich) live as
  //     a header strip directly above the Cell body.
  //
  // Parallel side-by-side Cells are reserved for a follow-up enhancement;
  // the first ship renders a single focus Cell.
  import { createQuery } from '@tanstack/svelte-query';
  import type { Component } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import CellLoadingState from '$lib/components/base/CellLoadingState.svelte';
  import { formatNumber } from '$lib/localization/format';
  import {
    probeDossierQuery,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import {
    DEFAULT_METRIC_NAME,
    resolvePresentation,
    type PresentationCellProps
  } from '$lib/presentations';
  import { urlState } from '$lib/state/url.svelte';
  import { metricLabel, registerSourceLabels } from '$lib/state/labels.svelte';
  import PanelControls from './PanelControls.svelte';
  import MeasureDetail from './MeasureDetail.svelte';
  import WindowHost from './WindowHost.svelte';

  interface Props {
    probeIds: string[];
  }

  let { probeIds }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  // Active probe is the first composed probe; cross-probe queries pass
  // the full `probeIds` array to the BFF as `?probeIds=`. Phase 122i:
  // when the URL carries a PillarState, fall back to its first ScopeGroup's
  // first probe so the Dossier can hydrate without the legacy probeIds
  // prop being populated.
  const activeProbeId = $derived.by(() => {
    if (probeIds[0]) return probeIds[0];
    const fromPillar = url.pillars?.aleph?.windows[0]?.panels[0]?.scopes[0]?.probeIds[0];
    return fromPillar ?? '';
  });

  // Default window = the WHOLE dataset: undefined bounds ⇒ no time filter (the
  // BFF treats absent start/end as unbounded). Time-limiting is an optional
  // feature, engaged only when the URL carries from/to. Mirrors DossierOverlay.
  const windowMs = $derived.by<{ start: string | undefined; end: string | undefined }>(() => {
    const fromMs = url.from ? Date.parse(url.from) : NaN;
    const toMs = url.to ? Date.parse(url.to) : NaN;
    return {
      start: Number.isFinite(fromMs) ? new Date(fromMs).toISOString() : undefined,
      end: Number.isFinite(toMs)
        ? new Date(toMs).toISOString()
        : url.from
          ? new Date().toISOString()
          : undefined
    };
  });

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, activeProbeId, {
        windowStart: windowMs.start,
        windowEnd: windowMs.end
      });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: activeProbeId !== ''
      };
    }
  );

  const dossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );

  // Phase 148g — seed the global source-label resolver so cell titles (and every
  // other source-name surface) render the emic designation, not the raw id.
  $effect(() => {
    if (dossier) registerSourceLabels(dossier.sources);
  });

  // Phase 122i / ADR-034 — Multi-Panel state path. When the URL carries a
  // `?aleph=<encoded>` payload, render the Multi-Panel Workbench via the
  // WindowHost → PanelHost tree. Otherwise fall through to the Phase-122h
  // legacy single-Cell shell below.
  const pillarState = $derived(url.pillars?.aleph ?? null);

  // Phase 122k — legacy single-cell fallback (when pillarState is null)
  // renders an empty Aleph with Aleph defaults. Scope is fully governed by
  // pillar state; this fallback is only reached on a fresh /workbench
  // visit before the user has configured anything. The K3 wiring will
  // open the ScopeEditor instead of rendering this fallback.
  const presentation = $derived(resolvePresentation(null, 'aleph'));
  const metricName = $derived(DEFAULT_METRIC_NAME);
  const dataLayer = $derived<'gold' | 'silver'>('gold');

  const cellSources = $derived(
    dossier
      ? dossier.sources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }))
      : []
  );

  const scope = $derived<'probe' | 'source'>('probe');
  const scopeId = $derived<string>(activeProbeId);

  // Dataset-Shape strip values. Phase 122i revision (A4): when the URL
  // carries pillar-state, the strip reflects the FOCUSED PANEL's
  // resolved scope (intersect dossier.sources with the panel's scope
  // groups), so the numbers describe the workbench the user is looking
  // at — not the whole probe. In DF-locked mode the probes counter is
  // dropped (a single locked DF over a single probe is the implicit
  // shape). Falls back to whole-probe stats for legacy flat-URL
  // workbenches.
  const datasetShape = $derived.by(() => {
    if (!dossier) return null;
    const ps = url.pillars?.aleph;
    const win = ps?.windows[ps.activeWindowIndex] ?? ps?.windows[0];
    const focused = win?.panels[win.focusedPanelIndex] ?? win?.panels[0] ?? null;
    if (focused) {
      const probeList: string[] = [];
      const sourceList: string[] = [];
      let anyEmptySourceList = false;
      for (const g of focused.scopes) {
        for (const p of g.probeIds) if (!probeList.includes(p)) probeList.push(p);
        if (g.sourceIds.length === 0) anyEmptySourceList = true;
        else for (const s of g.sourceIds) if (!sourceList.includes(s)) sourceList.push(s);
      }
      // If any group is "whole probe" (empty sourceIds), the resolved
      // source set is the union of all probe sources, not the explicit
      // sourceList. Intersect with dossier.sources to drop unknowns.
      const resolvedSources = anyEmptySourceList
        ? dossier.sources
        : dossier.sources.filter((s) => sourceList.includes(s.name));
      const articleCount = resolvedSources.reduce((sum, s) => sum + (s.articlesInWindow ?? 0), 0);
      return {
        // Drop the probes counter in DF-locked mode: confusing
        // ("Probes: 0" inside a single-probe locked workbench).
        probes: focused.locked === true ? null : probeList.length,
        sources: resolvedSources.length,
        articlesInWindow: articleCount,
        language: dossier.language,
        coverage: dossier.functionCoverage
      };
    }
    // Legacy flat URL form.
    const sources = dossier.sources;
    const articleCount = sources.reduce((sum, s) => sum + (s.articlesInWindow ?? 0), 0);
    return {
      probes: probeIds.length,
      sources: sources.length,
      articlesInWindow: articleCount,
      language: dossier.language,
      coverage: dossier.functionCoverage
    };
  });

  // Cell component lazy-load — same pattern as the legacy FunctionLaneShell.
  // Each Cell ships its own chunk so heavy chart libraries land on demand.
  let CellComponent = $state<Component<PresentationCellProps> | null>(null);
  let loadError = $state<string | null>(null);
  let loadToken = 0;

  $effect(() => {
    const t = ++loadToken;
    loadError = null;
    presentation
      .loadComponent()
      .then((Comp) => {
        if (t !== loadToken) return;
        CellComponent = Comp;
      })
      .catch((err: unknown) => {
        if (t !== loadToken) return;
        CellComponent = null;
        loadError = err instanceof Error ? err.message : m.errors_generic();
      });
  });
</script>

<section class="aleph-shell" aria-label={m.workbench_aleph_aria_label()}>
  {#if dossierQ.isPending}
    <CellLoadingState label={m.workbench_aleph_loading_dataset()} />
  {:else if datasetShape}
    {#if pillarState && dossier}
      <!-- Phase 122i / ADR-034 — Multi-Panel rendering path. The
           dataset-shape strip is rendered inside WindowHost so all
           three pillars (Aleph, Episteme, Rhizome) surface it
           consistently. -->
      <WindowHost
        pillar="aleph"
        {pillarState}
        {dossier}
        {ctx}
        windowStart={windowMs.start}
        windowEnd={windowMs.end}
      />
    {:else}
      <!-- Phase 122h legacy single-Cell path. Preserved verbatim so
           existing bookmarks load unchanged. The legacy path keeps the
           inline dataset-shape strip because WindowHost is not part of
           the legacy rendering. -->
      <header class="dataset-shape" aria-label={m.workbench_aleph_dataset_shape_aria_label()}>
        {#if datasetShape.probes !== null}
          <div class="shape-item">
            <span class="shape-label">{m.workbench_aleph_shape_probes()}</span>
            <span class="shape-value">{datasetShape.probes}</span>
          </div>
        {/if}
        <div class="shape-item">
          <span class="shape-label">{m.workbench_aleph_shape_sources()}</span>
          <span class="shape-value">{datasetShape.sources}</span>
        </div>
        <div class="shape-item">
          <span class="shape-label">{m.workbench_aleph_shape_articles()}</span>
          <span class="shape-value">{formatNumber(datasetShape.articlesInWindow)}</span>
        </div>
        <div class="shape-item">
          <span class="shape-label">{m.workbench_aleph_shape_language()}</span>
          <span class="shape-value">{datasetShape.language.toUpperCase()}</span>
        </div>
        <div class="shape-item">
          <span class="shape-label">{m.workbench_aleph_shape_function_coverage()}</span>
          <span class="shape-value">
            {datasetShape.coverage.covered}/{datasetShape.coverage.total}
          </span>
        </div>
      </header>

      <PanelControls pillar="aleph" />

      <div class="cell-frame">
        <header class="cell-header">
          <span class="cell-eyebrow">{m.workbench_aleph_cell_eyebrow()}</span>
          <span class="cell-presentation">{presentation.label}</span>
          <span class="cell-sep" aria-hidden="true">·</span>
          <code class="cell-metric">{metricLabel(metricName)}</code>
        </header>

        <div class="cell-body">
          {#if loadError}
            <p class="muted">{m.workbench_aleph_cell_failed({ error: loadError })}</p>
          {:else if !CellComponent}
            <CellLoadingState
              label={m.workbench_aleph_loading_presentation({ presentation: presentation.label })}
            />
          {:else if cellSources.length === 0}
            <p class="muted">{m.workbench_aleph_no_sources()}</p>
          {:else if dossier}
            {@const Cell = CellComponent}
            <Cell
              {ctx}
              scopeProbeId={dossier.probeId}
              {scope}
              {scopeId}
              windowStart={windowMs.start}
              windowEnd={windowMs.end}
              {metricName}
              sources={cellSources}
              {dataLayer}
              probeIds={probeIds.length > 1 ? probeIds : []}
            />
          {/if}
        </div>
      </div>

      <MeasureDetail
        subjects={[{ name: metricName, roles: ['primary'] }]}
        viewMode={presentation.id}
        viewLabel={presentation.label}
      />
    {/if}
  {:else if dossierQ.isError}
    <p class="muted">{m.workbench_aleph_dossier_failed()}</p>
  {/if}
</section>

<style>
  .aleph-shell {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    flex: 1;
  }

  .dataset-shape {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
    padding: var(--space-3) var(--space-4);
    background: color-mix(in srgb, #5283b8 6%, var(--color-surface));
    border: 1px solid color-mix(in srgb, #5283b8 30%, transparent);
    border-left: 3px solid #5283b8;
    border-radius: var(--radius-md);
  }

  .shape-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 6rem;
  }

  .shape-label {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .shape-value {
    font-family: var(--font-mono);
    font-size: var(--font-size-md);
    color: var(--color-fg);
    font-weight: var(--font-weight-semibold);
  }

  .cell-frame {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 24rem;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
  }

  .cell-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    font-family: var(--font-mono);
  }

  .cell-eyebrow {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .cell-presentation {
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
  }

  .cell-sep {
    color: var(--color-fg-subtle);
  }

  .cell-metric {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .cell-body {
    flex: 1;
    padding: var(--space-4);
    overflow: auto;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

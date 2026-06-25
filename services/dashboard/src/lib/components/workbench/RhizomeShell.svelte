<script lang="ts">
  // RhizomeShell — Phase 130 / ADR-035.
  //
  // The Rhizome Pillar renders the relational substrate (entity
  // co-occurrence now; lead-lag / correlation / configurable networks
  // arrive in Phases 124/125). Phase 130 removed the bespoke
  // entry-question navigation layer (Actors&Topics / Source Resonance /
  // Concept Migration / Free Composition) that no other pillar had: two of
  // the four were dead feature-gates, one pointed at `topic_distribution`
  // (which moved to Aleph), and the last was a vestige of the retired
  // card/edge canvas. The relational cells are now ordinary cell choices,
  // so Rhizome behaves exactly like Aleph and Episteme — a multi-panel
  // Workbench (pillar-state path) with a legacy single-Cell fallback.
  // (Guided "questions", if they return, come back as optional preset
  // cell-templates, not a navigation layer.)
  import { createQuery } from '@tanstack/svelte-query';
  import type { Component } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import CellLoadingState from '$lib/components/base/CellLoadingState.svelte';
  import {
    probeDossierQuery,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import {
    DEFAULT_METRIC_NAME,
    resolvePresentation,
    type PresentationCellProps
  } from '$lib/presentations';
  import PanelControls from './PanelControls.svelte';
  import MeasureDetail from './MeasureDetail.svelte';
  import WindowHost from './WindowHost.svelte';
  import { registerSourceLabels } from '$lib/state/labels.svelte';

  interface Props {
    probeIds: string[];
  }

  let { probeIds }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());
  // Phase 122i — PillarState-aware fallback for the active probe id.
  const activeProbeId = $derived.by(() => {
    if (probeIds[0]) return probeIds[0];
    const fromPillar = url.pillars?.rhizome?.windows[0]?.panels[0]?.scopes[0]?.probeIds[0];
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

  // Phase 148g — seed the global source-label resolver (emic designation).
  $effect(() => {
    if (dossier) registerSourceLabels(dossier.sources);
  });

  // Phase 122i / ADR-034 — Multi-Panel state path. See AlephShell for the
  // dual-path rationale.
  const pillarState = $derived(url.pillars?.rhizome ?? null);

  // Phase 130 — legacy fallback path (only reached when pillarState is null),
  // rendering an empty Rhizome with its default presentation
  // (cooccurrence_network).
  const presentation = $derived(resolvePresentation(null, 'rhizome'));
  const metricName = $derived(DEFAULT_METRIC_NAME);
  const dataLayer = $derived<'gold' | 'silver'>('gold');
  const cellSources = $derived(
    dossier
      ? dossier.sources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }))
      : []
  );
  const scope = $derived<'probe' | 'source'>('probe');
  const scopeId = $derived<string>(activeProbeId);

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

<section class="rhizome-shell" aria-label={m.workbench_rhizome_aria_label()}>
  {#if dossierQ.isPending}
    <CellLoadingState label={m.workbench_rhizome_loading_dataset()} />
  {:else if pillarState && dossier}
    <!-- Phase 122i / ADR-034 — Multi-Panel rendering path. -->
    <WindowHost
      pillar="rhizome"
      {pillarState}
      {dossier}
      {ctx}
      windowStart={windowMs.start}
      windowEnd={windowMs.end}
    />
  {:else if dossier}
    <!-- Legacy single-Cell fallback. -->
    <PanelControls pillar="rhizome" />
    <div class="cell-frame">
      <header class="cell-header">
        <span class="cell-eyebrow">{m.workbench_rhizome_cell_eyebrow()}</span>
        <span class="cell-presentation">{presentation.label}</span>
      </header>
      <div class="cell-body">
        {#if loadError}
          <p class="muted">{m.workbench_rhizome_cell_failed({ error: loadError })}</p>
        {:else if !CellComponent}
          <CellLoadingState
            label={m.workbench_rhizome_loading_presentation({ presentation: presentation.label })}
          />
        {:else if cellSources.length === 0}
          <p class="muted">{m.workbench_rhizome_no_sources()}</p>
        {:else}
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
  {:else if dossierQ.isError}
    <p class="muted">{m.workbench_rhizome_dossier_failed()}</p>
  {/if}
</section>

<style>
  .rhizome-shell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: 1;
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
    color: #9a8fb8;
    font-weight: var(--font-weight-semibold);
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

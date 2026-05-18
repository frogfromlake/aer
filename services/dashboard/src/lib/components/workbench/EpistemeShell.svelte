<script lang="ts">
  // EpistemeShell — Phase 122h / ADR-033 §2 (Episteme paragraph).
  //
  // The Episteme Pillar renders the diachronic record. Layout:
  //   - Global Resolution selector at the top — all strata snap to one
  //     resolution because they share a single bottom-anchored time-axis
  //     (ADR-033 §6).
  //   - Body: vertically stacked Strata, each one a Cell with its own
  //     header (Metric, Darstellung, Layer, Vergleich) and methodology
  //     anchor. First ship renders a single default Stratum using the
  //     URL-state's active metric + presentation; "+ Add stratum"
  //     is a placeholder for the per-Stratum URL-state extension that
  //     lands in a follow-up.
  import { createQuery } from '@tanstack/svelte-query';
  import type { Component } from 'svelte';
  import {
    probeDossierQuery,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { DEFAULT_METRIC_NAME, resolvePresentation, type ViewModeCellProps } from '$lib/viewmodes';
  import { urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import PanelControls from './PanelControls.svelte';
  import CellMethodology from './CellMethodology.svelte';
  import WindowHost from './WindowHost.svelte';

  interface Props {
    probeIds: string[];
  }

  let { probeIds }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());
  // Phase 122i — PillarState-aware fallback for the active probe id.
  const activeProbeId = $derived.by(() => {
    if (probeIds[0]) return probeIds[0];
    const fromPillar = url.pillars?.episteme?.windows[0]?.panels[0]?.scopes[0]?.probeIds[0];
    return fromPillar ?? '';
  });

  const windowMs = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      start: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS).toISOString(),
      end: new Date(Number.isFinite(toMs) ? toMs : now).toISOString()
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

  // Phase 122i / ADR-034 — Multi-Panel state path. See AlephShell for
  // the dual-path rationale.
  const pillarState = $derived(url.pillars?.episteme ?? null);

  // Phase 122k — legacy fallback path (only reached when pillarState is null).
  const presentation = $derived(resolvePresentation(null, 'episteme'));
  const metricName = $derived(DEFAULT_METRIC_NAME);
  const dataLayer = $derived<'gold' | 'silver'>('gold');

  const cellSources = $derived(
    dossier
      ? dossier.sources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }))
      : []
  );

  const scope = $derived<'probe' | 'source'>('probe');
  const scopeId = $derived<string>(activeProbeId);

  // Phase 122h Findings round 3 update: Resolution control was hoisted
  // into PanelControls (it is a per-Cell capability, not a per-Pillar
  // global). EpistemeShell no longer manages resolution state; the cell
  // honours `url.resolution` directly via SourceLaneChart.

  let CellComponent = $state<Component<ViewModeCellProps> | null>(null);
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
        loadError = err instanceof Error ? err.message : 'Cell failed to load';
      });
  });
</script>

<section class="episteme-shell" aria-label="Episteme — change over time">
  {#if dossierQ.isPending}
    <p class="muted" aria-busy="true">Loading dataset…</p>
  {:else if pillarState && dossier}
    <!-- Phase 122i / ADR-034 — Multi-Panel rendering path. -->
    <WindowHost
      pillar="episteme"
      {pillarState}
      {dossier}
      {ctx}
      windowStart={windowMs.start}
      windowEnd={windowMs.end}
    />
  {:else if dossier}
    <!-- Phase 122h legacy single-Stratum path. -->
    <PanelControls pillar="episteme" />
    <div class="stratum-stack">
      <article class="stratum" aria-label="Stratum — {presentation.label}">
        <header class="stratum-header">
          <span class="stratum-eyebrow">Stratum 1</span>
          <span class="stratum-presentation">{presentation.label}</span>
          <span class="stratum-sep" aria-hidden="true">·</span>
          <code class="stratum-metric">{metricName}</code>
        </header>
        <div class="stratum-body">
          {#if loadError}
            <p class="muted">Cell failed to load: {loadError}</p>
          {:else if !CellComponent}
            <p class="muted" aria-busy="true">Loading {presentation.label}…</p>
          {:else if cellSources.length === 0}
            <p class="muted">No sources in the active scope.</p>
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
      </article>

      <button
        type="button"
        class="add-stratum"
        disabled
        title="Multi-stratum stack lands in a follow-up iteration"
      >
        ＋ Add stratum
      </button>
    </div>

    <CellMethodology {metricName} viewMode={presentation.id} viewLabel={presentation.label} />
  {:else if dossierQ.isError}
    <p class="muted">Dossier failed to load.</p>
  {/if}
</section>

<style>
  .episteme-shell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: 1;
  }

  .stratum-stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: 1;
  }

  .stratum {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
    min-height: 18rem;
    display: flex;
    flex-direction: column;
  }

  .stratum-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    font-family: var(--font-mono);
  }

  .stratum-eyebrow {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .stratum-presentation {
    font-size: var(--font-size-xs);
    color: #c8a85a;
    font-weight: var(--font-weight-semibold);
  }

  .stratum-sep {
    color: var(--color-fg-subtle);
  }

  .stratum-metric {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .stratum-body {
    flex: 1;
    padding: var(--space-4);
    overflow: auto;
  }

  .add-stratum {
    appearance: none;
    border: 1px dashed var(--color-border);
    background: transparent;
    border-radius: var(--radius-md);
    padding: var(--space-3);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    cursor: not-allowed;
    opacity: 0.6;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

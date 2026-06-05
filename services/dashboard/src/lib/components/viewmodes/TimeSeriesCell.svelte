<script lang="ts">
  // NLP × time-series cell (Phase 107).
  // Per-source small-multiples baseline view. Reuses the existing
  // SourceLaneChart so the Phase 106 rendering survives unchanged when
  // this cell is selected.
  // Phase 111: not applicable in Silver layer (Silver documents have no
  // Gold-equivalent time-series; the distribution cell covers Silver).
  import { createQuery } from '@tanstack/svelte-query';
  import SourceLaneChart from '$lib/components/lanes/SourceLaneChart.svelte';
  import OverlayLaneChart from '$lib/components/lanes/OverlayLaneChart.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
  import { methodologyNotes } from '$lib/methodology-copy';
  import { metricsQuery, type MetricsResponseDto, type QueryOutcome } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import HowToRead from './HowToRead.svelte';
  import CellExport from './CellExport.svelte';

  let {
    ctx,
    sources,
    windowStart,
    windowEnd,
    metricName,
    scope,
    scopeId,
    dataLayer = 'gold',
    composition,
    showBand,
    resolution,
    normalization,
    reportExtent,
    sharedDomains,
    axisScaleState,
    configOverridden
  }: ViewModeCellProps = $props();

  // Phase 131 — ±1σ uncertainty band toggle (default shown).
  const bandShown = $derived(showBand ?? true);

  // Phase 131 (BUG4) — the chart's effective resolution: per-panel override,
  // else the global URL resolution, else hourly. Computed once here and passed
  // down so the charts AND the export query stay consistent.
  const url = $derived(urlState());
  const effectiveResolution = $derived(resolution ?? url.resolution ?? 'hourly');

  // Phase 122i revision (C6). Soft methodology note when the Aleph
  // time-series cell aggregates over multiple sources — the chart
  // reflects the union, not per-source framings. WP-004 §3.4 carries
  // the cross-frame interpretation guidance.
  const showMergedNote = $derived(composition === 'merged' && sources.length > 1);

  // Phase 122i revision (D1). Composition gates the fan-out semantic:
  //   merged → one SourceLaneChart over the unioned source list, BFF
  //            unions server-side via `?sourceIds=a,b,…`.
  //   split  → one SourceLaneChart per source (small-multiples) — the
  //            legacy Phase-106 behaviour.
  // Composition absent → fan-out per source (legacy callers, e.g. the
  // Phase-122h shells when the user hits a legacy flat URL).
  const sourceNames = $derived(sources.map((s) => s.name));

  // Phase 131 (BUG5) — export. One query over ALL sources (the BFF returns
  // per-source rows) gives the complete underlying data regardless of
  // composition; PNG captures the rendered uPlot canvas (no SVG — uPlot is
  // canvas-only).
  let bodyEl: HTMLDivElement | undefined = $state();
  const exportQ = createQuery<
    QueryOutcome<MetricsResponseDto>,
    Error,
    QueryOutcome<MetricsResponseDto>
  >(() => {
    const o = metricsQuery(ctx, {
      ...(windowStart ? { startDate: windowStart } : {}),
      ...(windowEnd ? { endDate: windowEnd } : {}),
      sourceIds: sourceNames.join(','),
      metricName,
      resolution: effectiveResolution,
      includeStddev: bandShown,
      ...(normalization && normalization !== 'raw' ? { normalization } : {})
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver' && sourceNames.length > 0
    };
  });
  const exportRows = $derived(
    exportQ.data?.kind === 'success'
      ? exportQ.data.data.data.map((p) => ({
          timestamp: p.timestamp,
          source: p.source,
          value: p.value,
          stddev: p.stddev,
          count: p.count
        }))
      : []
  );

  // Phase 124 — shared y-axis. Report this cell's y extent (spanning the
  // ±1σ band when shown) so PanelHost can union it across the panel's cells;
  // apply the returned union domain to every SourceLaneChart this cell renders.
  $effect(() => {
    if (!reportExtent) return;
    const rows = exportRows;
    if (rows.length === 0) {
      reportExtent('value', null);
      return;
    }
    let lo = Infinity;
    let hi = -Infinity;
    for (const r of rows) {
      if (typeof r.value !== 'number') continue;
      const s = bandShown && typeof r.stddev === 'number' ? r.stddev : 0;
      if (r.value - s < lo) lo = r.value - s;
      if (r.value + s > hi) hi = r.value + s;
    }
    reportExtent('value', lo <= hi ? [lo, hi] : null);
  });
  const sharedY = $derived<readonly [number, number] | null>(sharedDomains?.value ?? null);
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'time_series',
      metric: metricName,
      scope,
      scopeId,
      resolution: effectiveResolution,
      normalization: normalization ?? 'raw',
      band: bandShown ? '±1σ' : 'off',
      windowStart,
      windowEnd
    },
    howToRead: composeHowToRead('time_series', {
      showBand: bandShown,
      scales: axisScaleState,
      configOverridden
    }),
    rows: exportRows,
    columns: ['timestamp', 'source', 'value', 'stddev', 'count']
  });
  const exportFilenameParts = $derived([
    'time-series',
    metricName,
    scope === 'source' ? scopeId : 'probe'
  ]);
  function getNode(): HTMLElement | null {
    return bodyEl ?? null;
  }
</script>

<div class="cell-body" bind:this={bodyEl}>
  {#if dataLayer !== 'silver' && sources.length > 0 && exportRows.length > 0}
    <div class="ts-export-row">
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    </div>
  {/if}
  {#if dataLayer === 'silver'}
    <p class="notice">
      Time-series view is not available for Silver-layer data. Switch to Distribution to explore
      Silver-layer document characteristics.
    </p>
  {:else if sources.length === 0}
    <p class="empty">No sources in the active scope.</p>
  {:else if composition === 'merged'}
    {#if showMergedNote}
      {@const note = methodologyNotes.alephMergedTimeSeries(sources.length)}
      <MethodologyBanner anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
        <strong>{note.headline}</strong> — {note.body}
      </MethodologyBanner>
    {/if}
    <SourceLaneChart
      sourceNames={[...sourceNames]}
      emicDesignation={null}
      {ctx}
      {windowStart}
      {windowEnd}
      {metricName}
      showBand={bandShown}
      resolution={effectiveResolution}
      {normalization}
      yDomain={sharedY}
    />
    <HowToRead
      presentation="time_series"
      facts={{ showBand: bandShown, scales: axisScaleState, configOverridden }}
    />
  {:else if composition === 'overlay'}
    <!-- Phase 122k §14c finding 2 — Overlay: per-source independent
         queries, plotted as N viridis-coloured lines on a SHARED canvas.
         Visually one chart but with per-source structure preserved. -->
    <OverlayLaneChart
      sourceNames={[...sourceNames]}
      {ctx}
      {windowStart}
      {windowEnd}
      {metricName}
      resolution={effectiveResolution}
      {normalization}
    />
    <!-- Overlay has no ±1σ band (OverlayLaneChart plots per-source lines only),
         so the note must not claim one. -->
    <HowToRead presentation="time_series" facts={{ showBand: false, configOverridden }} />
  {:else}
    {#each sources as source (source.name)}
      <SourceLaneChart
        sourceName={source.name}
        emicDesignation={source.emicDesignation}
        {ctx}
        {windowStart}
        {windowEnd}
        {metricName}
        showBand={bandShown}
        resolution={effectiveResolution}
        {normalization}
        yDomain={sharedY}
      />
    {/each}
    <HowToRead
      presentation="time_series"
      facts={{ showBand: bandShown, scales: axisScaleState, configOverridden }}
    />
  {/if}
</div>

<style>
  .cell-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .ts-export-row {
    display: flex;
    justify-content: flex-end;
    margin-bottom: calc(-1 * var(--space-4));
  }

  .empty,
  .notice {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .notice {
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px dashed var(--color-border-strong);
    border-radius: var(--radius-md);
    max-width: 36rem;
  }
</style>

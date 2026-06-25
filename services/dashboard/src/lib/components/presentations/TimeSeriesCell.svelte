<script lang="ts">
  // NLP × time-series cell (Phase 107).
  // Per-source small-multiples baseline view. Reuses the existing
  // SourceLineChart so the Phase 106 rendering survives unchanged when
  // this cell is selected.
  // Phase 111: not applicable in Silver layer (Silver documents have no
  // Gold-equivalent time-series; the distribution cell covers Silver).
  import { createQuery } from '@tanstack/svelte-query';
  import SourceLineChart from '$lib/components/charts/SourceLineChart.svelte';
  import OverlayLineChart from '$lib/components/charts/OverlayLineChart.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
  import { methodologyNotes } from '$lib/methodology-copy';
  import { metricsQuery, type MetricsResponseDto, type QueryOutcome } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import CellExport from './CellExport.svelte';
  import CellTitleBar from './CellTitleBar.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { metricSubjectAndModel } from '$lib/state/labels.svelte';
  import type { CellTitleSpec } from '$lib/presentations/cell-title';

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
  }: PresentationCellProps = $props();

  // Phase 131 — ±1σ uncertainty band toggle (default shown).
  const bandShown = $derived(showBand ?? true);

  // Phase 131 (BUG4) — the chart's effective resolution: per-panel override,
  // else the global URL resolution, else the cell default. Computed once here
  // and passed down so the charts AND the export query stay consistent.
  // Phase 149 — the cell default is DAILY, not hourly: time_series is the
  // Episteme (diachronic "climate record") landing view, where daily is the
  // robust granularity across source cadences. Hourly is a rhythm grain (it
  // belongs to lead-lag, which buckets hourly) and renders a near-empty, spiky
  // line for low-cadence institutional sources (Élysée / Bundesregierung publish
  // a few items per week). Daily aligns time_series with its Episteme siblings
  // (RevisionTimeline / RevisionDiscourseShift default daily too); hourly stays
  // freely selectable in the resolution lever.
  const url = $derived(urlState());
  const effectiveResolution = $derived(resolution ?? url.resolution ?? 'daily');

  // Phase 148e — the cell title sits once at the cell level (eyebrow = Time
  // series, subject = metric with the model in its own dimmed slot, resolution
  // in the qualifier tail). Scope is `none` here: each SourceLineChart lane
  // carries its own scope pill (the source, or the ∪ union for a merged chart),
  // so the cell never double-renders the scope.
  const titleSubjectModel = $derived(metricSubjectAndModel(metricName));
  const titleSpec = $derived<CellTitleSpec>({
    presentation: m.domain_presentation_time_series_label(),
    subject: { kind: 'single', label: titleSubjectModel.subject },
    model: titleSubjectModel.model,
    scope: { kind: 'none' },
    qualifiers: [{ label: effectiveResolution }],
    idSeed: 'ts-title'
  });

  // Phase 122i revision (C6). Soft methodology note when the Aleph
  // time-series cell aggregates over multiple sources — the chart
  // reflects the union, not per-source framings. WP-004 §3.4 carries
  // the cross-frame interpretation guidance.
  const showMergedNote = $derived(composition === 'merged' && sources.length > 1);

  // Phase 122i revision (D1). Composition gates the fan-out semantic:
  //   merged → one SourceLineChart over the unioned source list, BFF
  //            unions server-side via `?sourceIds=a,b,…`.
  //   split  → one SourceLineChart per source (small-multiples) — the
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
  // apply the returned union domain to every SourceLineChart this cell renders.
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
  <CellTitleBar spec={titleSpec}>
    {#snippet actions()}
      {#if dataLayer !== 'silver' && sources.length > 0 && exportRows.length > 0}
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      {/if}
    {/snippet}
  </CellTitleBar>
  {#if sources.length === 0}
    <p class="empty">{m.cells_ts_no_sources()}</p>
  {:else if composition === 'merged'}
    {#if showMergedNote}
      {@const note = methodologyNotes.alephMergedTimeSeries(sources.length)}
      <MethodologyBanner anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
        <strong>{note.headline}</strong> — {note.body}
      </MethodologyBanner>
    {/if}
    <SourceLineChart
      sourceNames={[...sourceNames]}
      {ctx}
      {windowStart}
      {windowEnd}
      {metricName}
      showBand={bandShown}
      resolution={effectiveResolution}
      {normalization}
      yDomain={sharedY}
    />
  {:else if composition === 'overlay'}
    <!-- Phase 122k §14c finding 2 — Overlay: per-source independent
         queries, plotted as N viridis-coloured lines on a SHARED canvas.
         Visually one chart but with per-source structure preserved. -->
    <OverlayLineChart
      sourceNames={[...sourceNames]}
      {ctx}
      {windowStart}
      {windowEnd}
      {metricName}
      resolution={effectiveResolution}
      {normalization}
    />
  {:else}
    {#each sources as source (source.name)}
      <SourceLineChart
        sourceName={source.name}
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
  {/if}
</div>

<style>
  .cell-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .empty {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

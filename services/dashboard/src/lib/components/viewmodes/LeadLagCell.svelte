<script lang="ts">
  // Rhizome × Cross-Probe Temporal Lead-Lag — Phase 124.
  //
  // A relational artefact over a probe PAIR: the lagged cross-correlation of
  // the two probes' hourly publication activity. The reference probe is
  // probeIds[0]; the compared probe is probeIds[1]. A point at lag τ correlates
  // reference activity at hour h with compared activity at hour h+τ, so a peak
  // at τ>0 means the compared probe FOLLOWS (lags) the reference.
  //
  // The comparison is gated server-side on the temporal Level-1 equivalence
  // grant (WP-004 §6.3, Appendix B); an ungranted pair returns a refusal that
  // renders through RefusalSurface like every other equivalence refusal. The
  // grant block in the response drives the (server-authoritative) methodology
  // banner so the cell never asserts an ungranted claim.
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    probeLeadLagQuery,
    probesQuery,
    type ProbeLeadLagDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
  import { methodologyNotes } from '$lib/methodology-copy';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload, type ExportRow } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import { fmtValue, HIDDEN_READOUT, type ReadoutState } from '$lib/viewmodes/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import HowToRead from './HowToRead.svelte';

  let { ctx, probeIds, windowStart, windowEnd }: ViewModeCellProps = $props();

  // The reference + compared probe ids. The cell needs a PAIR; PanelHost only
  // passes a populated `probeIds` when a single rendered Cell pools >1 probe
  // (a merged composition over both probes). A single-probe scope yields an
  // empty list → the cell shows a "needs two probes" notice instead of an
  // empty chart.
  const referenceId = $derived(probeIds?.[0] ?? null);
  const comparedId = $derived(probeIds?.[1] ?? null);
  const hasPair = $derived(referenceId !== null && comparedId !== null);

  // Probe display labels (shortName) for the title + lead-lag sentence.
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  function labelFor(id: string | null): string {
    if (!id) return '—';
    if (probesQ.data?.kind === 'success') {
      const p = probesQ.data.data.find((x) => x.probeId === id);
      if (p) return p.shortName ?? p.displayName ?? id;
    }
    return id;
  }
  const referenceLabel = $derived(labelFor(referenceId));
  const comparedLabel = $derived(labelFor(comparedId));

  const leadLagQ = createQuery<QueryOutcome<ProbeLeadLagDto>, Error, QueryOutcome<ProbeLeadLagDto>>(
    () => {
      const o = probeLeadLagQuery(ctx, referenceId ?? '', {
        comparedTo: comparedId ?? '',
        start: windowStart,
        end: windowEnd
      });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: hasPair
      };
    }
  );

  const result = $derived<ProbeLeadLagDto | null>(
    leadLagQ.data?.kind === 'success' ? leadLagQ.data.data : null
  );

  type Point = { lagHours: number; correlation: number };
  // Only points with a defined correlation feed the line/dots.
  const definedPoints = $derived<Point[]>(
    (result?.points ?? [])
      .filter((p): p is { lagHours: number; correlation: number } => p.correlation != null)
      .map((p) => ({ lagHours: p.lagHours, correlation: p.correlation }))
  );
  const peakLagHours = $derived<number | null>(result?.peakLagHours ?? null);
  const peakCorrelation = $derived<number | null>(result?.peakCorrelation ?? null);
  const bucketsAtZero = $derived<number>(result?.bucketCountAtZero ?? 0);

  // Plain-language lead-lag sentence — the headline takeaway.
  const leadLagSentence = $derived.by<string>(() => {
    if (peakLagHours === null || peakCorrelation === null) {
      return 'No correlated publication rhythm found in the overlapping window.';
    }
    const r = peakCorrelation.toFixed(2);
    if (peakLagHours === 0) return `In step — rhythms align best at lag 0 (r = ${r}).`;
    const h = Math.abs(peakLagHours);
    return peakLagHours > 0
      ? `${comparedLabel} follows ${referenceLabel} by ${h}h (peak r = ${r}).`
      : `${comparedLabel} leads ${referenceLabel} by ${h}h (peak r = ${r}).`;
  });

  // --- Observable Plot render (line + dots + baselines + peak marker) ---
  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;
  // Live ref to the Plot x-scale's pixel→lag inverse, so the hover handler can
  // snap to the nearest lag across the whole column width (generous targeting)
  // instead of requiring an exact hit on a small dot. Updated on each render.
  const xInvertRef: { current: ((px: number) => number) | null } = { current: null };

  $effect(() => {
    const pts = definedPoints;
    const peakLag = peakLagHours;
    const peakCorr = peakCorrelation;
    if (!host || pts.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const corrs = pts.map((p) => p.correlation);
      const yMin = Math.max(-1, Math.min(0, ...corrs) - 0.05);
      const yMax = Math.min(1, Math.max(0, ...corrs) + 0.05);
      const peak =
        peakLag !== null && peakCorr !== null ? [{ lagHours: peakLag, correlation: peakCorr }] : [];
      const marks = [
        Plot.ruleY([0], { stroke: 'var(--color-border)' }),
        Plot.ruleX([0], { stroke: 'var(--color-border)', strokeDasharray: '3,3' }),
        Plot.line(pts, {
          x: 'lagHours',
          y: 'correlation',
          stroke: 'rgba(154, 143, 184, 0.95)',
          strokeWidth: 1.5
        }),
        Plot.dot(pts, { x: 'lagHours', y: 'correlation', r: 2, fill: 'rgba(154, 143, 184, 0.95)' })
      ];
      if (peak.length > 0) {
        marks.push(
          Plot.ruleX(peak, { x: 'lagHours', stroke: '#c8a85a', strokeDasharray: '2,2' }),
          Plot.dot(peak, {
            x: 'lagHours',
            y: 'correlation',
            r: 5,
            fill: '#c8a85a',
            stroke: 'var(--color-bg-elevated)',
            strokeWidth: 1.5
          })
        );
      }
      const next = Plot.plot({
        width: host.clientWidth,
        height: 240,
        marginLeft: 52,
        marginBottom: 40,
        x: { label: 'lag (hours) — compared relative to reference →', grid: true, nice: true },
        y: { label: 'correlation', domain: [yMin, yMax], grid: true },
        marks
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
      plotEl = next as unknown as HTMLElement;
      // Capture the x-scale inverse (pixel → lag) for the nearest-lag hover.
      const xScale = (
        next as unknown as { scale: (n: string) => { invert?: (v: number) => number } | undefined }
      ).scale('x');
      xInvertRef.current = xScale?.invert ?? null;
    })();
  });

  // Hover readout — snap to the nearest lag by inverting the mouse x through the
  // Plot x-scale. The whole column width is a hit target (generous, no need to
  // land exactly on a 2px dot), and the gold peak marker no longer blocks the
  // underlying point's readout.
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function onHostMove(ev: MouseEvent): void {
    const invert = xInvertRef.current;
    if (!host || !invert || definedPoints.length === 0) {
      readout = HIDDEN_READOUT;
      return;
    }
    const svg = host.querySelector('svg');
    if (!svg) {
      readout = HIDDEN_READOUT;
      return;
    }
    const lag = invert(ev.clientX - svg.getBoundingClientRect().left);
    let best = definedPoints[0]!;
    for (const p of definedPoints) {
      if (Math.abs(p.lagHours - lag) < Math.abs(best.lagHours - lag)) best = p;
    }
    readout = {
      visible: true,
      x: ev.clientX,
      y: ev.clientY,
      title: `lag ${best.lagHours >= 0 ? '+' : ''}${best.lagHours} h`,
      rows: [{ label: 'correlation', value: fmtValue(best.correlation) }],
      hint:
        best.lagHours > 0
          ? 'compared follows reference'
          : best.lagHours < 0
            ? 'compared leads'
            : 'in step'
    };
  }

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  // --- Export ---
  const exportRows = $derived<ExportRow[]>(
    (result?.points ?? []).map((p) => ({
      lag_hours: p.lagHours,
      correlation: p.correlation ?? ''
    }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'cross_probe_lead_lag',
      scope: 'probe',
      scopeId: `${referenceId ?? '?'}__${comparedId ?? '?'}`,
      windowStart,
      windowEnd
    },
    howToRead: composeHowToRead('cross_probe_lead_lag', { renderedCount: bucketsAtZero }),
    rows: exportRows,
    columns: ['lag_hours', 'correlation']
  });
  const exportFilenameParts = $derived(['lead-lag', referenceId ?? 'ref', comparedId ?? 'cmp']);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="leadlag-cell" aria-labelledby="leadlag-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="leadlag-title" class="cell-title">
      <span>Lead-lag</span>
      <span class="muted">
        — <strong class="scope-name">{referenceLabel}</strong> vs
        <strong class="scope-name">{comparedLabel}</strong>
      </span>
    </h3>
    {#if result && definedPoints.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if !hasPair}
    <p class="muted">
      Lead-lag compares two probes. Add a second probe to this panel's scope (merged composition) to
      see whether one culture's publication rhythm leads the other.
    </p>
  {:else if leadLagQ.isPending}
    <p class="muted" aria-busy="true">Computing lead-lag…</p>
  {:else if leadLagQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={leadLagQ.data} {ctx} />
  {:else if leadLagQ.isError || leadLagQ.data?.kind === 'network-error'}
    <p class="muted">Could not load lead-lag.</p>
  {:else if result && definedPoints.length === 0}
    <p class="muted">
      The two probes have no overlapping hourly activity in this window, so no lead-lag can be
      computed. Widen the time window or pick probes whose corpora overlap in time.
    </p>
  {:else if result}
    {@const note = methodologyNotes.rhizomeLeadLagGrant(result.grant.level)}
    <MethodologyBanner anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
      <strong>{note.headline}</strong> — {note.body}
    </MethodologyBanner>
    <p class="leadlag-takeaway">{leadLagSentence}</p>
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Lead-lag cross-correlation between {referenceLabel} and {comparedLabel}"
      onmousemove={onHostMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    <HowToRead presentation="cross_probe_lead_lag" facts={{ renderedCount: bucketsAtZero }} />
  {/if}
</section>

<style>
  .leadlag-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
  }
  .cell-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
    display: flex;
    gap: var(--space-2);
    align-items: baseline;
    flex-wrap: wrap;
  }
  .leadlag-takeaway {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    margin: 0;
    font-weight: var(--font-weight-medium);
  }
  .plot-host {
    width: 100%;
    min-height: 240px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  .plot-host :global(svg circle) {
    cursor: crosshair;
  }
  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>

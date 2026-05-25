<script lang="ts">
  // Publication-ready export toolbar — Phase 131 (reworked per BUG5).
  //
  // Uniform PNG / SVG / CSV / JSON export for every cell. PNG/SVG operate on
  // the cell's chart (SVG for Plot/d3 cells; a <canvas> for the uPlot
  // time-series, which has no SVG). The exported image matches the in-app cell
  // exactly (theme colours pinned in `serializeSvg`), and the data files are
  // self-describing: a metadata header + summary + the "how to read" note +
  // every data row. Filenames are scientific + timestamped. All serialisation
  // is native — no new dependency.
  import {
    downloadText,
    downloadSvg,
    downloadPng,
    downloadBlob,
    payloadToCsv,
    payloadToJson,
    exportFilename,
    type ExportPayload
  } from '$lib/viewmodes/cell-export';

  interface Props {
    /** Resolves the chart SVG at click time (Plot/d3 cells). */
    getSvg?: (() => SVGSVGElement | null | undefined) | undefined;
    /** Resolves the chart canvas at click time (uPlot time-series). */
    getCanvas?: (() => HTMLCanvasElement | null | undefined) | undefined;
    /** Self-describing data payload (meta + summary + how-to-read + rows). */
    payload: ExportPayload;
    /** Filename parts, e.g. ['distribution', 'word_count', 'tagesschau']. */
    filenameParts: Array<string | number | null | undefined>;
  }

  let { getSvg, getCanvas, payload, filenameParts }: Props = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  const hasData = $derived(payload.rows.length > 0);
  const hasSvg = $derived(!!getSvg);
  const hasImage = $derived(!!getSvg || !!getCanvas);
  const base = $derived(exportFilename(filenameParts));

  function canvasToPngBlob(canvas: HTMLCanvasElement): Promise<Blob> {
    return new Promise((resolve, reject) => {
      canvas.toBlob(
        (b) => (b ? resolve(b) : reject(new Error('canvas.toBlob returned null'))),
        'image/png'
      );
    });
  }

  async function onPng(e: MouseEvent) {
    e.stopPropagation();
    busy = true;
    error = null;
    try {
      const svg = getSvg?.();
      if (svg) {
        await downloadPng(`${base}.png`, svg);
        return;
      }
      const canvas = getCanvas?.();
      if (canvas) {
        downloadBlob(`${base}.png`, await canvasToPngBlob(canvas));
        return;
      }
      error = 'no chart to export';
    } catch (err) {
      error = err instanceof Error ? err.message : 'PNG export failed';
    } finally {
      busy = false;
    }
  }

  function onSvg(e: MouseEvent) {
    e.stopPropagation();
    const svg = getSvg?.();
    if (!svg) return;
    error = null;
    try {
      downloadSvg(`${base}.svg`, svg);
    } catch (err) {
      error = err instanceof Error ? err.message : 'SVG export failed';
    }
  }

  function onCsv(e: MouseEvent) {
    e.stopPropagation();
    if (!hasData) return;
    downloadText(`${base}.csv`, 'text/csv', payloadToCsv(payload));
  }

  function onJson(e: MouseEvent) {
    e.stopPropagation();
    if (!hasData) return;
    downloadText(`${base}.json`, 'application/json', payloadToJson(payload));
  }
</script>

<div class="cell-export" role="group" aria-label="Export this cell">
  <span class="export-eyebrow" aria-hidden="true">Export</span>
  {#if hasImage}
    <button
      type="button"
      class="export-btn"
      onclick={onPng}
      disabled={busy}
      title="Download PNG raster (matches the cell)"
    >
      PNG
    </button>
  {/if}
  {#if hasSvg}
    <button
      type="button"
      class="export-btn"
      onclick={onSvg}
      title="Download SVG vector (matches the cell)">SVG</button
    >
  {/if}
  <button
    type="button"
    class="export-btn"
    onclick={onCsv}
    disabled={!hasData}
    title="Download data + summary + how-to-read as CSV"
  >
    CSV
  </button>
  <button
    type="button"
    class="export-btn"
    onclick={onJson}
    disabled={!hasData}
    title="Download data + summary + how-to-read as JSON"
  >
    JSON
  </button>
  {#if error}
    <span class="export-error" role="status">{error}</span>
  {/if}
</div>

<style>
  .cell-export {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    flex-wrap: wrap;
  }

  .export-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    margin-right: var(--space-1);
  }

  .export-btn {
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 2px var(--space-2);
    cursor: pointer;
  }
  .export-btn:hover:not(:disabled),
  .export-btn:focus-visible:not(:disabled) {
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
  .export-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .export-error {
    font-size: 10px;
    color: var(--color-status-expired);
  }
</style>

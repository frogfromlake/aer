<script lang="ts">
  // Publication-ready export toolbar — Phase 131 (whole-cell, round-3).
  //
  // Two image families, each as PNG + SVG, captured from the WHOLE cell node so
  // the export matches the on-screen cell exactly (chart + summary + legend +
  // provenance):
  //   • Figure — publication figure: drops the provenance/methodology footer
  //     AND the how-to-read note (data-export-exclude="provenance").
  //   • Full   — the dashboard view: keeps everything (incl. how-to-read).
  // Only this interactive toolbar is dropped from both
  // (data-export-exclude="always"). Plus Data: CSV + JSON with metadata +
  // summary + how-to-read + rows. html-to-image is lazy-loaded on click.
  import {
    downloadText,
    downloadCellPng,
    downloadCellSvg,
    payloadToCsv,
    payloadToJson,
    exportFilename,
    type ExportPayload,
    type ImageVariant
  } from '$lib/presentations/cell-export';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    /** Resolves the cell's root element to capture (the whole cell section). */
    getNode: () => HTMLElement | null | undefined;
    /** Self-describing data payload (meta + summary + how-to-read + rows). */
    payload: ExportPayload;
    /** Filename parts, e.g. ['distribution', 'word_count', 'tagesschau']. */
    filenameParts: Array<string | number | null | undefined>;
  }

  let { getNode, payload, filenameParts }: Props = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  const hasData = $derived(payload.rows.length > 0);
  const base = $derived(exportFilename(filenameParts));

  async function onImage(e: MouseEvent, variant: ImageVariant, fmt: 'png' | 'svg') {
    e.stopPropagation();
    const node = getNode();
    if (!node) {
      error = m.cells_export_err_no_node();
      return;
    }
    busy = true;
    error = null;
    try {
      const name = `${base}_${variant}.${fmt}`;
      if (fmt === 'png') await downloadCellPng(name, node, variant);
      else await downloadCellSvg(name, node, variant);
    } catch (err) {
      error = err instanceof Error ? err.message : m.cells_export_err_image();
    } finally {
      busy = false;
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

<div
  class="cell-export"
  role="group"
  aria-label={m.cells_export_group_aria()}
  data-export-exclude="always"
>
  <span class="export-group">
    <span class="export-eyebrow" title={m.cells_export_figure_title()}
      >{m.cells_export_figure_label()}</span
    >
    <button
      type="button"
      class="export-btn"
      disabled={busy}
      onclick={(e) => onImage(e, 'figure', 'png')}>{m.cells_export_png()}</button
    >
    <button
      type="button"
      class="export-btn"
      disabled={busy}
      onclick={(e) => onImage(e, 'figure', 'svg')}>{m.cells_export_svg()}</button
    >
  </span>
  <span class="export-divider" aria-hidden="true"></span>
  <span class="export-group">
    <span class="export-eyebrow" title={m.cells_export_full_title()}
      >{m.cells_export_full_label()}</span
    >
    <button
      type="button"
      class="export-btn"
      disabled={busy}
      onclick={(e) => onImage(e, 'full', 'png')}>{m.cells_export_png()}</button
    >
    <button
      type="button"
      class="export-btn"
      disabled={busy}
      onclick={(e) => onImage(e, 'full', 'svg')}>{m.cells_export_svg()}</button
    >
  </span>
  <span class="export-divider" aria-hidden="true"></span>
  <span class="export-group">
    <span class="export-eyebrow" title={m.cells_export_data_title()}
      >{m.cells_export_data_label()}</span
    >
    <button type="button" class="export-btn" disabled={!hasData} onclick={onCsv}
      >{m.cells_export_csv()}</button
    >
    <button type="button" class="export-btn" disabled={!hasData} onclick={onJson}
      >{m.cells_export_json()}</button
    >
  </span>
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

  .export-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }

  .export-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    margin-right: 2px;
    cursor: help;
  }

  .export-divider {
    width: 1px;
    align-self: stretch;
    background: color-mix(in srgb, var(--color-border) 60%, transparent);
    margin: 0 var(--space-1);
  }

  .export-btn {
    appearance: none;
    /* Phase 128 — WCAG 2.2 (2.5.8) 24×24px minimum target size. */
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
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

<script lang="ts">
  // Exact-value hover readout box — Phase 132.
  //
  // A floating, pointer-following box that shows the EXACT datum (and every
  // bound visual channel) for the mark under the cursor. Shared across every
  // Workbench cell so the readout looks identical regardless of rendering
  // substrate (Observable Plot SVG, uPlot canvas, hand-rolled d3-force SVG).
  // Pure formatting / clamp logic lives in `$lib/presentations/cell-readout.ts`.
  //
  // `pointer-events: none` is load-bearing: the box must never intercept the
  // pointer, or it would steal events from the cell's click-drilldown /
  // drag / pan handlers (and could chase its own tail on every mousemove).
  import { clampReadoutPosition, type ReadoutState } from '$lib/presentations/cell-readout';

  // Named `readout` (not `state`) so it never shadows the `$state` rune.
  let { readout }: { readout: ReadoutState } = $props();

  let el: HTMLDivElement | undefined = $state();
  // Refined (viewport-clamped) position. Until the box is measured we fall
  // back to a raw pointer-offset placement so it appears next to the cursor
  // on the first frame rather than flashing at (0,0).
  let clamped = $state<{ left: number; top: number } | null>(null);

  $effect(() => {
    // Re-run on every readout change (track x/y/rows so the box repositions
    // as the pointer moves between marks).
    const s = readout;
    if (!s.visible || !el) {
      clamped = null;
      return;
    }
    clamped = clampReadoutPosition(
      s.x,
      s.y,
      el.offsetWidth,
      el.offsetHeight,
      window.innerWidth,
      window.innerHeight
    );
  });

  const left = $derived(clamped?.left ?? readout.x + 14);
  const top = $derived(clamped?.top ?? readout.y + 14);
</script>

{#if readout.visible && readout.rows.length > 0}
  <div
    bind:this={el}
    class="cell-readout"
    role="presentation"
    style:left="{left}px"
    style:top="{top}px"
  >
    {#if readout.title}
      <div class="readout-title">{readout.title}</div>
    {/if}
    <dl class="readout-rows">
      {#each readout.rows as row (row.label)}
        <div class="readout-row">
          <dt>
            {#if row.swatch}
              <span class="swatch" style:background={row.swatch} aria-hidden="true"></span>
            {/if}
            {row.label}
          </dt>
          <dd>{row.value}</dd>
        </div>
      {/each}
    </dl>
    {#if readout.hint}
      <div class="readout-hint">{readout.hint}</div>
    {/if}
  </div>
{/if}

<style>
  .cell-readout {
    position: fixed;
    z-index: 1100;
    pointer-events: none;
    max-width: min(60vw, 320px);
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-3);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: var(--line-height-snug, 1.35);
  }

  .readout-title {
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin-bottom: var(--space-1);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .readout-rows {
    margin: 0;
    display: grid;
    grid-template-columns: auto auto;
    gap: 1px var(--space-3);
    align-items: baseline;
  }

  .readout-row {
    display: contents;
  }

  dt {
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-size: 10px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  dd {
    margin: 0;
    color: var(--color-fg);
    text-align: right;
    white-space: nowrap;
  }

  .swatch {
    display: inline-block;
    width: 9px;
    height: 9px;
    border-radius: 2px;
    border: 1px solid color-mix(in srgb, currentColor 30%, transparent);
  }

  .readout-hint {
    margin-top: var(--space-2);
    padding-top: var(--space-1);
    border-top: 1px solid var(--color-border);
    color: var(--color-fg-subtle);
    font-style: italic;
    font-size: 10px;
    white-space: nowrap;
  }
</style>

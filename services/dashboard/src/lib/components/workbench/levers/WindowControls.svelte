<script lang="ts">
  // WindowControls — Phase 141 (extracted from PanelControls).
  //
  // The per-Panel time-window lever (Phase 122k F5): two native date inputs that
  // override the global default window, plus a Reset to drop the override. The
  // start anchors at 00:00 and the end at 23:59:59.999 so a single picked day is
  // a valid non-empty window; the pair can never invert (a would-be-inverted
  // pick snaps the other bound to the same day). Click events are stopped from
  // bubbling so the article-focus handler doesn't close the native picker.
  import type { DateWindow } from '$lib/workbench/panel-controls-derive';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    dateWindow: DateWindow;
    /** Today (YYYY-MM-DD) — forbids future dates, keeps TO ≥ FROM. */
    todayStr: string;
  }

  let { panelPath, dateWindow, todayStr }: Props = $props();

  function pickWindowStart(value: string) {
    if (!value) return;
    const start = new Date(`${value}T00:00:00.000Z`).toISOString();
    if (Number.isNaN(Date.parse(start))) return;
    updatePanel(panelPath, (p) => {
      const next = { ...p, windowStart: start };
      if (next.windowEnd && Date.parse(next.windowEnd) <= Date.parse(start)) {
        next.windowEnd = new Date(`${value}T23:59:59.999Z`).toISOString();
      }
      return next;
    });
  }
  function pickWindowEnd(value: string) {
    if (!value) return;
    const end = new Date(`${value}T23:59:59.999Z`).toISOString();
    if (Number.isNaN(Date.parse(end))) return;
    updatePanel(panelPath, (p) => {
      const next = { ...p, windowEnd: end };
      if (next.windowStart && Date.parse(next.windowStart) >= Date.parse(end)) {
        next.windowStart = new Date(`${value}T00:00:00.000Z`).toISOString();
      }
      return next;
    });
  }
  function resetWindowToGlobal() {
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      delete out.windowStart;
      delete out.windowEnd;
      return out;
    });
  }
</script>

<LeverRow eyebrow={m.levers_window_eyebrow()} role="group" ariaLabel={m.levers_window_aria()}>
  <div class="window-inputs" onclick={(e) => e.stopPropagation()} role="presentation">
    <input
      type="date"
      value={dateWindow.startDate}
      max={dateWindow.endDate ?? todayStr}
      onchange={(e) => pickWindowStart((e.currentTarget as HTMLInputElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_window_start_aria()}
    />
    <span class="window-sep" aria-hidden="true">→</span>
    <input
      type="date"
      value={dateWindow.endDate}
      min={dateWindow.startDate}
      max={todayStr}
      onchange={(e) => pickWindowEnd((e.currentTarget as HTMLInputElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_window_end_aria()}
    />
    {#if dateWindow.isPanelOverride}
      <LeverButton onclick={resetWindowToGlobal} title={m.levers_window_reset_title()}>
        {m.levers_window_reset()}
      </LeverButton>
    {/if}
  </div>
</LeverRow>

<style>
  .window-inputs {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .window-inputs input[type='date'] {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 3px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: text;
    color-scheme: dark;
  }
  .window-inputs input[type='date']:hover,
  .window-inputs input[type='date']:focus-visible {
    border-color: var(--color-accent);
  }

  .window-sep {
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }
</style>

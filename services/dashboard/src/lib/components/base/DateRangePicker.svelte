<script lang="ts">
  // Shared date-range picker — Phase 123a.
  //
  // Preset chips (Whole dataset / Last 7d / Last 30d / Custom…) over a
  // from/to ISO pair, plus revealed native date inputs in Custom mode.
  // Presentational: the parent owns the state and reacts via `onChange`.
  // Used by the Dossier overlay header (URL-backed `?from`/`?to`) and
  // available to the Workbench PanelControls (per-panel window).
  //
  // `Whole dataset` emits (null, null) → the BFF treats absent bounds as
  // "no filter" (in_window == total), per Phase 131a.
  interface Props {
    from: string | null;
    to: string | null;
    onChange: (from: string | null, to: string | null) => void;
  }

  let { from, to, onChange }: Props = $props();

  type Mode = 'whole' | '7d' | '30d' | 'custom';

  function deriveMode(f: string | null, t: string | null): Mode {
    return !f && !t ? 'whole' : 'custom';
  }
  // Local UI mode — initialised from props; chips set it. (7d/30d collapse to
  // 'custom' on reload: the chips are quick-sets, the concrete dates persist.)
  // svelte-ignore state_referenced_locally
  let mode = $state<Mode>(deriveMode(from, to));

  const DAY = 24 * 60 * 60 * 1000;

  function isoDate(iso: string | null): string {
    return iso ? iso.slice(0, 10) : '';
  }

  function setWhole() {
    mode = 'whole';
    onChange(null, null);
  }
  function setLast(days: number, m: Mode) {
    mode = m;
    const now = Date.now();
    onChange(new Date(now - days * DAY).toISOString(), new Date(now).toISOString());
  }
  function setCustom() {
    mode = 'custom';
  }
  // Start anchors at 00:00, end at 23:59:59.999 — a single day is a valid
  // window. The pair never inverts: a pick that would put end on/before start
  // snaps the other bound to the same day (single-day window) instead of
  // emitting an inverted range the BFF would reject.
  function pickFrom(v: string) {
    if (!v) {
      onChange(null, to);
      return;
    }
    const start = new Date(`${v}T00:00:00.000Z`).toISOString();
    const end =
      to && Date.parse(to) <= Date.parse(start) ? new Date(`${v}T23:59:59.999Z`).toISOString() : to;
    onChange(start, end);
  }
  function pickTo(v: string) {
    if (!v) {
      onChange(from, null);
      return;
    }
    const end = new Date(`${v}T23:59:59.999Z`).toISOString();
    const start =
      from && Date.parse(from) >= Date.parse(end)
        ? new Date(`${v}T00:00:00.000Z`).toISOString()
        : from;
    onChange(start, end);
  }
</script>

<div class="date-range-picker" role="group" aria-label="Time window">
  <div class="chips">
    <button type="button" class="chip" class:active={mode === 'whole'} onclick={setWhole}>
      Whole dataset
    </button>
    <button
      type="button"
      class="chip"
      class:active={mode === '7d'}
      onclick={() => setLast(7, '7d')}
    >
      Last 7d
    </button>
    <button
      type="button"
      class="chip"
      class:active={mode === '30d'}
      onclick={() => setLast(30, '30d')}
    >
      Last 30d
    </button>
    <button type="button" class="chip" class:active={mode === 'custom'} onclick={setCustom}>
      Custom…
    </button>
  </div>
  {#if mode === 'custom'}
    <div class="custom-inputs">
      <input
        type="date"
        value={isoDate(from)}
        onchange={(e) => pickFrom((e.currentTarget as HTMLInputElement).value)}
        aria-label="Window start"
      />
      <span class="sep" aria-hidden="true">→</span>
      <input
        type="date"
        value={isoDate(to)}
        onchange={(e) => pickTo((e.currentTarget as HTMLInputElement).value)}
        aria-label="Window end"
      />
    </div>
  {/if}
</div>

<style>
  .date-range-picker {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .chips {
    display: flex;
    gap: var(--space-1);
  }
  .chip {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-fg-muted);
    padding: 3px var(--space-3);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
  }
  .chip:hover,
  .chip:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  .chip.active {
    color: var(--color-bg);
    background: var(--color-accent);
    border-color: var(--color-accent);
  }
  .custom-inputs {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .custom-inputs input {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 3px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
  }
  .custom-inputs input:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-color: var(--color-accent);
  }
  .sep {
    color: var(--color-fg-subtle);
  }
</style>

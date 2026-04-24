<script lang="ts">
  // L2 Exploration controls (Design Brief §4.2 — "a bounded set of
  // structural levers"). A resolution selector and a pillar-mode toggle
  // for Aleph/Episteme/Rhizome (WP-001 → Design Brief §5.5). All state
  // is URL-backed via `$lib/state/url.svelte.ts`, so every lever is a
  // deep link and every drag is reproducible.
  //
  // Rhizome has no data today and Aleph/Episteme differ only in time-
  // window framing at the render layer; the toggle is exposed anyway
  // because the pillar grammar is part of Surface I's orientation
  // vocabulary, and the toggle's idempotence at L2 is a correct
  // rendering of the current system state (no opinion elided).
  import type { Resolution, ViewingMode } from '$lib/state/url-internals';
  import { setUrl, urlState } from '$lib/state/url.svelte';

  const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
  const PILLARS: readonly { id: ViewingMode; label: string; hint: string }[] = [
    {
      id: 'aleph',
      label: 'Aleph',
      hint: 'Totality — every observed probe, no filter'
    },
    {
      id: 'episteme',
      label: 'Episteme',
      hint: 'Knowledge register — epistemic-authority probes'
    },
    {
      id: 'rhizome',
      label: 'Rhizome',
      hint: 'Relational propagation — no data yet'
    }
  ];

  const url = $derived(urlState());
  let currentResolution = $derived<Resolution>(url.resolution ?? 'hourly');
  let currentPillar = $derived<ViewingMode>(url.viewingMode ?? 'aleph');
</script>

<div class="l2" role="group" aria-label="Exploration controls">
  <label class="resolution">
    <span class="label">Resolution</span>
    <select
      value={currentResolution}
      onchange={(e) =>
        setUrl({ resolution: (e.currentTarget as HTMLSelectElement).value as Resolution })}
    >
      {#each RESOLUTIONS as r (r)}
        <option value={r}>{r}</option>
      {/each}
    </select>
  </label>

  <div class="pillars" role="radiogroup" aria-label="Pillar mode">
    <span class="label">Pillar</span>
    {#each PILLARS as p (p.id)}
      <button
        type="button"
        role="radio"
        aria-checked={currentPillar === p.id}
        class:active={currentPillar === p.id}
        title={p.hint}
        onclick={() => setUrl({ viewingMode: p.id })}
      >
        {p.label}
      </button>
    {/each}
  </div>
</div>

<style>
  .l2 {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-2) var(--space-4);
    background: rgba(0, 0, 0, 0.6);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    backdrop-filter: blur(8px);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
  }
  .label {
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 10px;
    margin-right: var(--space-2);
  }
  .resolution {
    display: flex;
    align-items: center;
  }
  /* Native UA chrome on <select> renders a light grey control on most
     platforms, which clashes with the dark Atmosphere. Flatten it with
     appearance: none + an inline caret so the control itself is dark. */
  select {
    appearance: none;
    -webkit-appearance: none;
    background-color: rgba(0, 0, 0, 0.55);
    background-image:
      linear-gradient(45deg, transparent 50%, var(--color-fg-muted) 50%),
      linear-gradient(135deg, var(--color-fg-muted) 50%, transparent 50%);
    background-position:
      calc(100% - 14px) 50%,
      calc(100% - 9px) 50%;
    background-size:
      5px 5px,
      5px 5px;
    background-repeat: no-repeat;
    color: var(--color-fg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px 22px 2px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-family-mono);
    cursor: pointer;
  }
  select:hover,
  select:focus-visible {
    border-color: var(--color-accent);
    outline: none;
  }
  /* Native <option> inherits from the OS, not from the select — without
     this the dropdown list renders near-white text on a white background
     in dark-mode browsers. */
  select option {
    background: var(--color-surface);
    color: var(--color-fg);
  }
  .pillars {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .pillars button {
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .pillars button:hover {
    color: var(--color-fg);
  }
  .pillars button.active {
    color: var(--color-fg);
    background: rgba(82, 131, 184, 0.2);
    border-color: #5283b8;
  }
</style>

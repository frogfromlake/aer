<script lang="ts">
  // Phase 122i / ADR-034 — Probe Dossier Free-Compose entry path.
  //
  // The second entry path from the Probe Dossier into the Workbench
  // (the first being the DF-tile grid). Lets the user pick an arbitrary
  // subset of the probe's sources and open the Workbench in editable
  // mode — composition='merged', not locked, all controls live.
  //
  // Multi-probe selection is intentionally out of scope for the first
  // ship — current production runs a single probe. The `probeIds` array
  // passed to `buildFreeComposeUrl` is a single-element array today.
  import { goto } from '$app/navigation';
  import { SvelteSet } from 'svelte/reactivity';
  import { buildFreeComposeUrl } from '$lib/workbench/panel-queries';
  import type { ProbeDossierDto } from '$lib/api/queries';

  interface Props {
    dossier: ProbeDossierDto;
  }

  let { dossier }: Props = $props();

  const selected = new SvelteSet<string>();

  function toggle(name: string) {
    if (selected.has(name)) selected.delete(name);
    else selected.add(name);
  }

  function selectAll() {
    selected.clear();
    for (const s of dossier.sources) selected.add(s.name);
  }

  function clearAll() {
    selected.clear();
  }

  const selectedCount = $derived(selected.size);

  function open() {
    const sourceIds = [...selected];
    const qs = buildFreeComposeUrl({
      pillar: 'aleph',
      probeIds: [dossier.probeId],
      sourceIds
    });
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Workbench route
    void goto(`/workbench${qs}`);
  }
</script>

<div class="free-compose">
  <div class="picker">
    <div class="picker-header">
      <span class="picker-eyebrow">Sources in this probe</span>
      <div class="picker-actions">
        <button
          type="button"
          class="picker-action"
          onclick={selectAll}
          disabled={selectedCount === dossier.sources.length}
        >
          All
        </button>
        <button
          type="button"
          class="picker-action"
          onclick={clearAll}
          disabled={selectedCount === 0}
        >
          None
        </button>
      </div>
    </div>
    <ul class="source-list" role="list">
      {#each dossier.sources as source (source.name)}
        {@const checked = selected.has(source.name)}
        <li>
          <label class="source-row" class:checked>
            <input
              type="checkbox"
              {checked}
              onchange={() => toggle(source.name)}
              aria-label="Include {source.emicDesignation ?? source.name}"
            />
            <span class="source-label">
              <span class="source-name">{source.emicDesignation ?? source.name}</span>
              {#if source.emicDesignation && source.emicDesignation !== source.name}
                <code class="source-id">{source.name}</code>
              {/if}
            </span>
          </label>
        </li>
      {/each}
    </ul>
  </div>

  <div class="cta-row">
    <span class="cta-count">
      {selectedCount === 0
        ? 'No sources selected — Workbench will open with the whole probe.'
        : `${selectedCount} of ${dossier.sources.length} source${selectedCount === 1 ? '' : 's'} selected`}
    </span>
    <button type="button" class="cta" onclick={open}>Open Workbench (free compose) →</button>
  </div>
</div>

<style>
  .free-compose {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .picker-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-2);
  }

  .picker-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
  }

  .picker-actions {
    display: flex;
    gap: var(--space-1);
  }

  .picker-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .picker-action:hover:not(:disabled),
  .picker-action:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .picker-action:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .source-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: var(--space-1);
  }

  .source-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    background: var(--color-surface);
    transition: background var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .source-row.checked {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .source-row:hover {
    background: color-mix(in srgb, var(--color-accent) 6%, var(--color-surface));
  }

  .source-label {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .source-name {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }

  .cta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .cta-count {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
  }

  .cta {
    appearance: none;
    background: var(--color-accent);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-4);
    color: var(--color-bg);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    cursor: pointer;
  }

  .cta:hover,
  .cta:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 80%, var(--color-fg));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
</style>

<script lang="ts">
  // Surface II shared layout for a selected probe.
  // Renders the ScopeBar with:
  //   - Active probe label (truncated) + language badge
  //   - Dossier tab (default landing)
  //   - Four function-lane tabs (WP-001 taxonomy)
  //   - Scope indicator: probe vs. source mode
  //
  // Child pages (dossier, [functionKey]) render the main content area.
  // The ScopeBar is shared here so lane switching does not re-mount it.
  import type { Snippet } from 'svelte';
  import { page } from '$app/state';
  import { ScopeBar } from '$lib/components/chrome';
  import { SilverLayerToggle, ViewModeSwitcher } from '$lib/components/lanes';
  import { setUrl, urlState } from '$lib/state/url.svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  const FUNCTION_LANES = [
    { key: 'epistemic_authority', abbr: 'EA', label: 'Epistemic Authority' },
    { key: 'power_legitimation', abbr: 'PL', label: 'Power Legitimation' },
    { key: 'cohesion_identity', abbr: 'CI', label: 'Cohesion & Identity' },
    { key: 'subversion_friction', abbr: 'SF', label: 'Subversion & Friction' }
  ] as const;

  let probeId = $derived(page.params.probeId ?? '');
  let activeFunctionKey = $derived(page.params.functionKey ?? '');
  let isDossier = $derived(page.url.pathname.endsWith('/dossier'));

  const url = $derived(urlState());
  let sourceId = $derived(url.sourceId);

  function isLaneActive(key: string): boolean {
    return activeFunctionKey === key;
  }

  function clearSourceScope() {
    setUrl({ sourceId: null });
  }

  // Shorten probe ID for display (strip common prefix to save space)
  let probeShort = $derived(probeId.length > 28 ? probeId.slice(0, 26) + '…' : probeId);
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- all lane links are internal dynamic routes served by SPA fallback -->
<ScopeBar label="Surface II — Function Lanes navigation">
  <a
    href="/lanes/{probeId}/dossier"
    class="probe-label"
    class:active={isDossier}
    aria-label="Probe Dossier: {probeId}"
    title={probeId}
    data-sveltekit-preload-data="off"
  >
    <span class="probe-glyph" aria-hidden="true">◎</span>
    <span class="probe-id">{probeShort}</span>
  </a>

  <span class="divider" aria-hidden="true">|</span>

  <nav class="lane-tabs" aria-label="Function lanes">
    {#each FUNCTION_LANES as lane (lane.key)}
      <a
        href="/lanes/{probeId}/{lane.key}"
        class="lane-tab"
        class:active={isLaneActive(lane.key)}
        aria-label="{lane.label} lane"
        aria-current={isLaneActive(lane.key) ? 'page' : undefined}
        title={lane.label}
        data-sveltekit-preload-data="off"
      >
        <span class="lane-abbr">{lane.abbr}</span>
        <span class="lane-label">{lane.label}</span>
      </a>
    {/each}
  </nav>

  {#if sourceId}
    <span class="scope-badge" aria-label="Source scope: {sourceId}">
      <span class="scope-icon" aria-hidden="true">⊂</span>
      <span class="scope-name">{sourceId}</span>
      <button
        type="button"
        class="scope-clear"
        aria-label="Clear source scope, return to probe scope"
        onclick={clearSourceScope}>×</button
      >
    </span>
  {/if}

  {#if !isDossier && activeFunctionKey}
    <ViewModeSwitcher />
  {/if}

  <SilverLayerToggle />
</ScopeBar>

{#if children}{@render children()}{/if}

<style>
  .probe-label {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    text-decoration: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid transparent;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }

  .probe-label:hover,
  .probe-label:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border);
    background: var(--color-surface);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .probe-label.active {
    color: var(--color-fg);
    border-color: var(--color-border);
    background: var(--color-surface);
  }

  .probe-glyph {
    color: var(--color-accent);
    font-size: 0.9em;
  }

  .probe-id {
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .divider {
    color: var(--color-border-strong);
    font-size: var(--font-size-xs);
    flex-shrink: 0;
  }

  .lane-tabs {
    display: flex;
    align-items: center;
    gap: 2px;
    flex-wrap: nowrap;
  }

  .lane-tab {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: 3px var(--space-2);
    border-radius: var(--radius-sm);
    text-decoration: none;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    border: 1px solid transparent;
    white-space: nowrap;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .lane-tab:hover,
  .lane-tab:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border);
    background: var(--color-surface);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .lane-tab.active {
    color: var(--color-fg);
    background: var(--color-surface);
    border-color: var(--color-accent-muted);
  }

  .lane-abbr {
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    font-size: 10px;
    letter-spacing: 0.04em;
    opacity: 0.7;
  }

  .lane-tab.active .lane-abbr {
    opacity: 1;
    color: var(--color-accent);
  }

  .lane-label {
    font-size: var(--font-size-xs);
  }

  /* Hide long labels on small viewports */
  @media (max-width: 800px) {
    .lane-label {
      display: none;
    }
  }

  .scope-badge {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    background: rgba(82, 131, 184, 0.15);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    flex-shrink: 0;
  }

  .scope-icon {
    font-size: 0.8em;
  }

  .scope-name {
    font-family: var(--font-mono);
    max-width: 100px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .scope-clear {
    background: transparent;
    border: none;
    color: var(--color-accent-muted);
    cursor: pointer;
    padding: 0 2px;
    font-size: var(--font-size-sm);
    line-height: 1;
    border-radius: var(--radius-sm);
  }

  .scope-clear:hover,
  .scope-clear:focus-visible {
    color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  @media (prefers-reduced-motion: reduce) {
    .probe-label,
    .lane-tab {
      transition: none;
    }
  }
</style>

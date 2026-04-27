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
  import { setUrl, urlState } from '$lib/state/url.svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  let probeId = $derived(page.params.probeId ?? '');

  const url = $derived(urlState());
  let sourceIds = $derived(url.sourceIds);

  function clearSourceScope() {
    setUrl({ sourceIds: [] });
  }

  // Shorten probe ID for display (strip common prefix to save space)
  let probeShort = $derived(probeId.length > 28 ? probeId.slice(0, 26) + '…' : probeId);
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- all lane links are internal dynamic routes served by SPA fallback -->
<ScopeBar label="Surface II — Function Lanes navigation">
  <a
    href="/lanes/{probeId}/dossier"
    class="probe-label"
    aria-label="Probe Dossier: {probeId}"
    title={probeId}
    data-sveltekit-preload-data="off"
  >
    <span class="probe-glyph" aria-hidden="true">◎</span>
    <span class="probe-id">{probeShort}</span>
  </a>

  <!-- Function lane tabs removed: navigation is via function tiles on L2
       (Probe Dossier) and via the LensBar Function group on L3 (Phase 113d). -->

  {#if sourceIds.length > 0}
    <span class="scope-badge" aria-label="Source scope: {sourceIds.join(', ')}">
      <span class="scope-icon" aria-hidden="true">⊂</span>
      <span class="scope-name"
        >{sourceIds.length === 1 ? sourceIds[0] : `${sourceIds.length} sources`}</span
      >
      <button
        type="button"
        class="scope-clear"
        aria-label="Clear source scope, return to probe scope"
        onclick={clearSourceScope}>×</button
      >
    </span>
  {/if}
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
    .probe-label {
      transition: none;
    }
  }
</style>

<script lang="ts">
  // Silver-layer toggle for the Surface II ScopeBar (Phase 111).
  // Switches between Gold (default) and Silver data layers. URL state
  // carries the selection (?layer=silver); `gold` is the default and is
  // omitted from the URL so existing deep-links are unaffected.
  //
  // The toggle is always rendered when a probe is active. Its Silver
  // option is visually enabled regardless of eligibility — eligibility
  // is evaluated per-source inside the lane/dossier and surfaces a
  // methodological refusal panel rather than disabling the toggle.
  import { setUrl, urlState } from '$lib/state/url.svelte';

  const url = $derived(urlState());
  let layer = $derived(url.layer ?? 'gold');

  function selectGold() {
    setUrl({ layer: null });
  }

  function selectSilver() {
    setUrl({ layer: 'silver' });
  }
</script>

<div class="layer-toggle" role="group" aria-label="Data layer">
  <button
    type="button"
    class="layer-btn"
    class:active={layer === 'gold'}
    aria-pressed={layer === 'gold'}
    aria-label="Gold layer — aggregated metrics"
    onclick={selectGold}
  >
    Au Gold
  </button>
  <button
    type="button"
    class="layer-btn silver"
    class:active={layer === 'silver'}
    aria-pressed={layer === 'silver'}
    aria-label="Silver layer — document-level data (WP-006 §5.2)"
    onclick={selectSilver}
  >
    Ag Silver
  </button>
</div>

<style>
  .layer-toggle {
    display: flex;
    align-items: center;
    gap: 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    overflow: hidden;
    flex-shrink: 0;
  }

  .layer-btn {
    display: inline-flex;
    align-items: center;
    padding: 3px var(--space-3);
    background: transparent;
    border: none;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    cursor: pointer;
    white-space: nowrap;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .layer-btn:not(:last-child) {
    border-right: 1px solid var(--color-border);
  }

  .layer-btn:hover,
  .layer-btn:focus-visible {
    color: var(--color-fg);
    background: var(--color-bg-elevated);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .layer-btn.active {
    color: var(--color-fg);
    background: var(--color-surface);
  }

  .layer-btn.silver.active {
    color: #7ec4a0;
    background: rgba(126, 196, 160, 0.1);
  }

  @media (prefers-reduced-motion: reduce) {
    .layer-btn {
      transition: none;
    }
  }
</style>

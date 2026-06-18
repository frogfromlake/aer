<script lang="ts">
  // Phase 122k K2 — Metadata Coverage Modal.
  //
  // The Phase-122f MetadataCoveragePanel is a dense per-field × per-source
  // matrix. Inline in the ProbeCard it dominates the layout and split-
  // attention reading is unhelpful. The K2 design moves it into a modal:
  // one click from the ProbeCard header, focused reading, Esc to return.
  //
  // Wraps `MetadataCoveragePanel` verbatim — the matrix's per-cell
  // semantics (publisher choice, extractor success, structural absence)
  // are unchanged. K2 grouping by DF inside the matrix is a Phase 122k.2
  // follow-up — the modal lands first; DF-grouped sections inside follow.
  import { onMount, onDestroy } from 'svelte';
  import type { FetchContext } from '$lib/api/queries';
  import MetadataCoveragePanel from '$lib/components/source/MetadataCoveragePanel.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    probeId: string;
    ctx?: FetchContext;
    onClose: () => void;
  }

  let { probeId, ctx = { baseUrl: '/api/v1' }, onClose }: Props = $props();

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
  });
  onDestroy(() => {
    window.removeEventListener('keydown', onKeydown);
  });
</script>

<div class="modal-backdrop" role="presentation">
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-labelledby="mdc-modal-heading"
    tabindex="-1"
  >
    <header class="modal-header">
      <div class="header-titles">
        <h2 id="mdc-modal-heading">{m.dossier_coverage_modal_title({ probeId })}</h2>
        <p class="header-hint">
          {m.dossier_coverage_modal_hint()}
        </p>
      </div>
      <button
        type="button"
        class="close-btn"
        onclick={onClose}
        aria-label={m.dossier_coverage_modal_close()}
      >
        ×
      </button>
    </header>
    <div class="modal-body">
      <MetadataCoveragePanel {probeId} {ctx} />
    </div>
  </section>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--color-bg) 75%, transparent);
    backdrop-filter: blur(2px);
    z-index: 50;
    display: grid;
    place-items: center;
    padding: var(--space-4);
  }

  .modal {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    width: min(72rem, 100%);
    max-height: 90vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding: var(--space-5);
    gap: var(--space-3);
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.35);
  }

  .modal-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-3);
  }

  .header-titles h2 {
    margin: 0 0 var(--space-1) 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
  }

  .header-hint {
    margin: 0;
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    max-width: 48rem;
    line-height: 1.45;
  }

  .close-btn {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    width: 2rem;
    height: 2rem;
    font-size: 1.25rem;
    cursor: pointer;
    flex-shrink: 0;
  }
  .close-btn:hover,
  .close-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .modal-body {
    flex: 1 1 auto;
  }
</style>

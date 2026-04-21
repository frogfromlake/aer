<script lang="ts">
  // Right-anchored side panel (drawer). Used on the Atmosphere route to
  // surface probe detail at L3 without obscuring the globe — Design Brief
  // §4.1 rule 2 ("no layer replaces"). The globe stays visible and
  // interactable behind the panel; there is no backdrop, and closing
  // the panel does not reset camera state.
  //
  // A11y mirrors `Dialog.svelte` (focus trap, Escape, previously-focused
  // restore, `role=dialog` + `aria-modal=false` since the surface remains
  // live). Structure is deliberately separate — a panel is not a modal
  // and should not inherit the centered-overlay visual.
  import { onMount, type Snippet } from 'svelte';

  interface Props {
    open: boolean;
    title: string;
    describedBy?: string;
    onClose?: () => void;
    children?: Snippet;
    footer?: Snippet;
    /** Override the default "Close" button aria-label (e.g. for i18n). */
    closeLabel?: string;
  }

  let {
    open = $bindable(),
    title,
    describedBy,
    onClose,
    children,
    footer,
    closeLabel = 'Close panel'
  }: Props = $props();

  let panelEl: HTMLElement | undefined = $state();
  let previouslyFocused: HTMLElement | null = null;
  const titleId = `sidepanel-title-${Math.random().toString(36).slice(2, 10)}`;

  function close() {
    open = false;
    onClose?.();
  }

  function onKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      close();
      return;
    }
    if (e.key === 'Tab' && panelEl) {
      trapFocus(e, panelEl);
    }
  }

  function trapFocus(e: KeyboardEvent, root: HTMLElement) {
    const focusables = root.querySelectorAll<HTMLElement>(
      'a[href],button:not([disabled]),textarea:not([disabled]),input:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])'
    );
    if (focusables.length === 0) return;
    const first = focusables[0] as HTMLElement;
    const last = focusables[focusables.length - 1] as HTMLElement;
    const active = document.activeElement as HTMLElement | null;
    if (e.shiftKey && active === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }

  $effect(() => {
    if (open) {
      previouslyFocused = document.activeElement as HTMLElement | null;
      queueMicrotask(() => panelEl?.focus());
    } else if (previouslyFocused) {
      previouslyFocused.focus();
      previouslyFocused = null;
    }
  });

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
    return () => window.removeEventListener('keydown', onKeydown);
  });
</script>

{#if open}
  <div
    bind:this={panelEl}
    class="sidepanel"
    role="dialog"
    aria-modal="false"
    aria-labelledby={titleId}
    aria-describedby={describedBy}
    tabindex="-1"
  >
    <header>
      <h2 id={titleId}>{title}</h2>
      <button type="button" class="close" aria-label={closeLabel} onclick={close}>×</button>
    </header>
    <div class="body">
      {#if children}{@render children()}{/if}
    </div>
    {#if footer}
      <footer>
        {@render footer()}
      </footer>
    {/if}
  </div>
{/if}

<style>
  .sidepanel {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    width: min(92vw, 28rem);
    display: flex;
    flex-direction: column;
    background: var(--color-surface);
    color: var(--color-fg);
    border-left: 1px solid var(--color-border);
    box-shadow: var(--elevation-3);
    z-index: 1000;
    /* No backdrop: the Atmosphere must remain visible and legible. */
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--color-border);
  }

  h2 {
    font-size: var(--font-size-lg);
    margin: 0;
  }

  .close {
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xl);
    line-height: 1;
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }

  .close:hover,
  .close:focus-visible {
    color: var(--color-fg);
    background: var(--color-surface-hover);
  }

  .body {
    padding: var(--space-5);
    overflow: auto;
    flex: 1;
  }

  footer {
    padding: var(--space-4) var(--space-5);
    border-top: 1px solid var(--color-border);
    display: flex;
    gap: var(--space-3);
    justify-content: flex-end;
  }
</style>

<script lang="ts">
  import { onMount, type Snippet } from 'svelte';

  interface Props {
    open: boolean;
    title: string;
    describedBy?: string;
    onClose?: () => void;
    children?: Snippet;
    footer?: Snippet;
  }

  let { open = $bindable(), title, describedBy, onClose, children, footer }: Props = $props();

  let dialogEl: HTMLDivElement | undefined = $state();
  let previouslyFocused: HTMLElement | null = null;
  const titleId = `dialog-title-${Math.random().toString(36).slice(2, 10)}`;

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
    if (e.key === 'Tab' && dialogEl) {
      trapFocus(e, dialogEl);
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
      queueMicrotask(() => dialogEl?.focus());
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
    class="backdrop"
    role="presentation"
    onclick={close}
    onkeydown={(e) => e.key === 'Enter' && close()}
  ></div>
  <div
    bind:this={dialogEl}
    class="dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby={titleId}
    aria-describedby={describedBy}
    tabindex="-1"
  >
    <header>
      <h2 id={titleId}>{title}</h2>
      <button type="button" class="close" aria-label="Close dialog" onclick={close}>×</button>
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
  .backdrop {
    position: fixed;
    inset: 0;
    background: var(--color-bg-overlay);
    z-index: 900;
  }

  .dialog {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    min-width: 320px;
    max-width: min(90vw, 560px);
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    background: var(--color-surface);
    color: var(--color-fg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--elevation-3);
    z-index: 1000;
    overflow: hidden;
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

  .close:hover {
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

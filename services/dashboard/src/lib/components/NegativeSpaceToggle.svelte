<script lang="ts">
  // Negative Space overlay toggle (Design Brief §3.4, §4.4 — "what is
  // not observed is a first-class rendering, not an absence"). Phase
  // 112 delivers the full per-surface behavior: Surface I shows absence
  // regions prominently; Surface II highlights empty lanes and adds
  // demographic-skew annotations; Surface III renders absence-prose in
  // the margin; the methodology tray switches to known-limitations-first
  // mode. URL state (`?negSpace=1`) persists the overlay across surfaces.
  //
  // Keyboard: Shift+N toggles the overlay from any surface.
  import { onMount } from 'svelte';

  interface Props {
    active: boolean;
    onToggle: (next: boolean) => void;
  }

  let { active, onToggle }: Props = $props();

  function onKeydown(e: KeyboardEvent) {
    // Shift+N — do not collide with browser `Ctrl/Cmd+N` (new window)
    // or single-letter search shortcuts.
    if (e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey && e.key === 'N') {
      // Don't intercept Shift+N while the user is typing in an input.
      const t = e.target as HTMLElement | null;
      const tag = t?.tagName?.toLowerCase();
      if (tag === 'input' || tag === 'textarea' || (t?.isContentEditable ?? false)) return;
      e.preventDefault();
      onToggle(!active);
    }
  }

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
    return () => window.removeEventListener('keydown', onKeydown);
  });
</script>

<button
  type="button"
  class="ns"
  class:active
  aria-pressed={active}
  aria-label="Toggle negative space overlay (Shift+N)"
  title="Negative Space overlay (Shift+N) — foregrounds what AĒR does not observe across all surfaces"
  onclick={() => onToggle(!active)}
>
  <span class="glyph" aria-hidden="true">∅</span>
  <span class="label" aria-hidden="true">NS</span>
</button>

<style>
  .ns {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    width: 28px;
    height: 34px;
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .ns:hover,
  .ns:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-fg-subtle);
    border-style: solid;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .ns.active {
    color: var(--color-fg);
    border-style: solid;
    border-color: #5283b8;
    background: rgba(82, 131, 184, 0.2);
  }
  .glyph {
    font-family: var(--font-mono);
    font-size: 1rem;
    line-height: 1;
  }
  .label {
    font-size: 8px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: var(--font-weight-semibold);
    line-height: 1;
  }
  @media (prefers-reduced-motion: reduce) {
    .ns {
      transition: none;
    }
  }
</style>

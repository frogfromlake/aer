<script lang="ts">
  // Negative Space overlay toggle (Design Brief §3.4, §4.4 — "what is
  // not observed is a first-class rendering, not an absence"). Phase
  // 100a delivers only the structural toggle: a keyboard shortcut
  // (Shift+N), a quiet UI affordance, and a `$state` rune wired through
  // the parent. Visual reweighting of the globe (dimming the "seen"
  // probes, lighting the "unseen" regions) and WP-003 §6 demographic-
  // skew annotations land in a later phase — the toggle is idempotent
  // until then and the aria-pressed state reflects that honestly.
  //
  // Keeping the affordance present even when it doesn't render yet is
  // deliberate: hiding it would violate §4.1 rule 1 (each concern
  // reachable in one interaction) and re-introduce a discovery problem
  // when the visual payload lands.
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
  title="Negative Space overlay (Shift+N) — structural only, visual reweighting lands in a later phase"
  onclick={() => onToggle(!active)}
>
  <span class="glyph" aria-hidden="true">∅</span>
  <span class="label">Neg. Space</span>
</button>

<style>
  .ns {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    cursor: pointer;
  }
  .ns:hover,
  .ns:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-fg-subtle);
  }
  .ns.active {
    color: var(--color-fg);
    border-style: solid;
    border-color: #5283b8;
    background: rgba(82, 131, 184, 0.12);
  }
  .glyph {
    font-family: var(--font-family-mono);
  }
</style>

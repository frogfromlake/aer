<script lang="ts">
  // Appearance menu (Color Themes feature) — a contrast-disc button in the scope
  // bar, beside the UI-language globe, that opens a small popover to switch the
  // colour theme. Self-contained; calls `setTheme`, which persists the choice to
  // localStorage and re-applies `<html data-theme>` live (same engine pattern as
  // LocaleMenu / setLocale). Five choices: System · Dark · Light · two
  // high-contrast polarities for low-vision readers.
  import { theme, setTheme, THEMES, type Theme } from '$lib/state/theme.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const LABEL: Record<Theme, () => string> = {
    system: m.chrome_theme_system,
    dark: m.chrome_theme_dark,
    light: m.chrome_theme_light,
    'contrast-dark': m.chrome_theme_contrast_dark,
    'contrast-light': m.chrome_theme_contrast_light
  };

  const active = $derived(theme());

  let open = $state(false);
  let root = $state<HTMLElement | null>(null);

  function toggle() {
    open = !open;
  }
  function choose(t: Theme) {
    setTheme(t);
    open = false;
  }
  function onWindowKeydown(event: KeyboardEvent) {
    if (open && event.key === 'Escape') open = false;
  }
  function onWindowPointerdown(event: PointerEvent) {
    if (open && root && !root.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window onkeydown={onWindowKeydown} onpointerdown={onWindowPointerdown} />

<div class="theme-menu" bind:this={root}>
  <button
    type="button"
    class="theme-btn"
    class:open
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={m.chrome_theme_group_label()}
    title={m.chrome_theme_group_label()}
    onclick={toggle}
  >
    <!-- Contrast disc: the universal appearance/theme glyph (half-filled circle). -->
    <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
      <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="1.6" />
      <path d="M12 3a9 9 0 0 1 0 18z" fill="currentColor" />
    </svg>
  </button>

  {#if open}
    <div class="menu" role="menu" aria-label={m.chrome_theme_group_label()}>
      {#each THEMES as t (t)}
        <button
          type="button"
          role="menuitemradio"
          class="menu-item"
          class:active={active === t}
          aria-checked={active === t}
          onclick={() => choose(t)}
        >
          <span class="check" aria-hidden="true">{active === t ? '✓' : ''}</span>
          <span>{LABEL[t]()}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .theme-menu {
    position: relative;
    display: inline-flex;
    flex-shrink: 0;
  }
  .theme-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    appearance: none;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    padding: var(--space-1);
    cursor: pointer;
    transition:
      color var(--motion-duration-fast) var(--motion-ease-standard),
      background var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .theme-btn:hover,
  .theme-btn:focus-visible,
  .theme-btn.open {
    color: var(--color-accent);
    background: var(--color-surface-hover);
    outline: none;
  }
  .theme-btn:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .menu {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 460;
    min-width: 12rem;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-3);
    padding: var(--space-1);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    appearance: none;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    text-align: left;
    padding: var(--space-2) var(--space-2);
    cursor: pointer;
    white-space: nowrap;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .menu-item:hover,
  .menu-item:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: none;
  }
  .menu-item.active {
    color: var(--color-accent);
  }
  .check {
    width: 1em;
    flex-shrink: 0;
    color: var(--color-accent);
  }
  @media (prefers-reduced-motion: reduce) {
    .theme-btn,
    .menu-item {
      transition: none;
    }
  }
</style>

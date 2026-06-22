<script lang="ts">
  // UI-language menu (Phase 148e) — a globe button in the scope bar that opens a
  // small popover to switch the interface language. Replaces the account
  // overlay's language section: the selector now lives in the persistent chrome,
  // reachable from every surface. Self-contained; calls `setLocale`, which
  // persists + deep-links the choice (same engine as LocaleSwitch).
  //
  // Language names are shown as ENDONYMS ("English" / "Deutsch") with a matching
  // `lang` attribute so a screen reader voices each in its own tongue.
  import { locale, setLocale, LOCALES, type Locale } from '$lib/state/locale.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ENDONYM: Record<Locale, string> = { en: 'English', de: 'Deutsch' };
  const active = $derived(locale());

  let open = $state(false);
  let root = $state<HTMLElement | null>(null);

  function toggle() {
    open = !open;
  }
  function choose(l: Locale) {
    setLocale(l);
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

<div class="locale-menu" bind:this={root}>
  <button
    type="button"
    class="globe-btn"
    class:open
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={m.chrome_locale_group_label()}
    title={m.chrome_locale_group_label()}
    onclick={toggle}
  >
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18" />
      <path d="M12 3c2.6 2.7 4 5.7 4 9s-1.4 6.3-4 9c-2.6-2.7-4-5.7-4-9s1.4-6.3 4-9z" />
    </svg>
  </button>

  {#if open}
    <div class="menu" role="menu" aria-label={m.chrome_locale_group_label()}>
      {#each LOCALES as l (l)}
        <button
          type="button"
          role="menuitemradio"
          class="menu-item"
          class:active={active === l}
          aria-checked={active === l}
          lang={l}
          onclick={() => choose(l)}
        >
          <span class="check" aria-hidden="true">{active === l ? '✓' : ''}</span>
          <span>{ENDONYM[l]}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .locale-menu {
    position: relative;
    display: inline-flex;
    flex-shrink: 0;
  }
  .globe-btn {
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
  .globe-btn:hover,
  .globe-btn:focus-visible,
  .globe-btn.open {
    color: var(--color-accent);
    background: var(--color-surface-hover);
    outline: none;
  }
  .globe-btn:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .globe-btn svg {
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
  }

  .menu {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 460;
    min-width: 9rem;
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
    .globe-btn,
    .menu-item {
      transition: none;
    }
  }
</style>

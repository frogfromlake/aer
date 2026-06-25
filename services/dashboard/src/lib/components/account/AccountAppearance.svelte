<script lang="ts">
  // Appearance section for the account overlay (Color Themes feature) — the
  // same theme choice as the scope-bar ThemeMenu, surfaced in the user area as a
  // labelled radio group. Both read/write the one `$lib/state/theme.svelte` rune,
  // so a switch in either place updates the other live.
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
</script>

<section class="block">
  <h3>{m.chrome_theme_group_label()}</h3>
  <div class="options" role="radiogroup" aria-label={m.chrome_theme_group_label()}>
    {#each THEMES as t (t)}
      <button
        type="button"
        role="radio"
        class="opt"
        class:active={active === t}
        aria-checked={active === t}
        onclick={() => setTheme(t)}
      >
        <span class="dot" aria-hidden="true"></span>
        {LABEL[t]()}
      </button>
    {/each}
  </div>
</section>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }
  .block h3 {
    margin: 0;
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }
  .options {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .opt {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    padding: var(--space-2) var(--space-3);
    cursor: pointer;
    transition:
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .opt:hover,
  .opt:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: none;
  }
  .opt:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .opt .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    border: 1px solid var(--color-border-strong);
    flex-shrink: 0;
  }
  .opt.active {
    color: var(--color-accent);
    border-color: var(--color-accent);
  }
  .opt.active .dot {
    background: var(--color-accent);
    border-color: var(--color-accent);
  }
  @media (prefers-reduced-motion: reduce) {
    .opt {
      transition: none;
    }
  }
</style>

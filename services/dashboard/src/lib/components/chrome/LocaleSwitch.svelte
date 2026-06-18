<script lang="ts">
  // UI-language selector (Phase 144 / ADR-042). A two-option segmented toggle
  // for the SideRail user section. Self-contained (own scoped CSS) so it can
  // live anywhere; calls `setLocale`, which persists + deep-links the choice.
  //
  // Language names are shown as ENDONYMS ("English" / "Deutsch") and carry a
  // matching `lang` attribute so a screen reader voices each in its own tongue —
  // they are not routed through messages (a language's own name is invariant).
  import { locale, setLocale, LOCALES, type Locale } from '$lib/state/locale.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ENDONYM: Record<Locale, string> = { en: 'English', de: 'Deutsch' };
  const SHORT: Record<Locale, string> = { en: 'EN', de: 'DE' };
  const active = $derived(locale());
</script>

<div class="locale-switch" role="group" aria-label={m.chrome_locale_group_label()}>
  {#each LOCALES as l (l)}
    <button
      type="button"
      class="locale-opt"
      class:active={active === l}
      aria-pressed={active === l}
      lang={l}
      title={ENDONYM[l]}
      aria-label={m.chrome_locale_switch_to({ language: ENDONYM[l] })}
      onclick={() => setLocale(l)}
    >
      {SHORT[l]}
    </button>
  {/each}
</div>

<style>
  .locale-switch {
    display: flex;
    gap: 2px;
    padding: var(--space-1) var(--space-2) var(--space-2);
  }
  .locale-opt {
    flex: 1 1 0;
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    padding: var(--space-1) var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.04em;
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .locale-opt:hover,
  .locale-opt:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .locale-opt.active {
    color: var(--color-accent);
    border-color: var(--color-accent-muted);
    background: var(--color-surface);
  }
  @media (prefers-reduced-motion: reduce) {
    .locale-opt {
      transition: none;
    }
  }
</style>

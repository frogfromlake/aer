<script lang="ts">
  // Left side rail — Phase 122h / ADR-033 §3; restyled in Phase 151 to the
  // claude.ai/design "AĒR Design System" chrome (operator-led design pass).
  //
  // Structure (top → bottom):
  //   1. Brand          — the AĒR wordmark, filling the scope-bar height zone.
  //   2. Surface group  — the three surface anchors (Atmosphere / Workbench /
  //                       Reflection). Each carries glyph + full word and a
  //                       boxed active state.
  //   3. Scope card     — "Where am I": the current probe selection + the
  //                       active Pillar (analytical stance). Replaces the
  //                       per-anchor pillar sub-item; the same information,
  //                       gathered into one quiet card.
  //   4. Bottom group   — muted mini-buttons: Open Dossier (with selection
  //                       count), Saved analyses, Your account.
  //
  // Phase 151 — the user identity block, the interface-language selector,
  // Administration, and Sign out moved OFF the rail and INTO the account
  // overlay (now tabbed). The rail's "Your account" button is the single
  // entry point to all of them.
  //
  // Keyboard: every interactive target is a native <a> or <button> with
  // browser-default focus handling. `prefers-reduced-motion` suppresses
  // all transitions.
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { urlState, toggleOverlay } from '$lib/state/url.svelte';
  import { getPillar } from '$lib/presentations';
  import { buildSelectionWorkbenchUrl } from '$lib/workbench/panel-queries';
  import { m } from '$lib/paraglide/messages.js';
  import type { PillarId } from '$lib/state/url-internals';

  const url = $derived(urlState());
  const activePillarId = $derived<PillarId>(url.activePillar ?? 'aleph');
  const activePillar = $derived(getPillar(activePillarId));
  const selectedCount = $derived(url.selectedProbes.length);

  // Phase 123a — the Dossier is a global overlay (ADR-033 amendment), not a
  // surface route; this button opens the search/catalogue overlay over any
  // surface and surfaces the current selection count.
  function openDossier() {
    toggleOverlay('dossier');
  }
  // Phase 135 — saved analyses are a global overlay (`?analyses=open`).
  function openAnalyses() {
    toggleOverlay('analyses');
  }
  // Phase 151 — the account overlay is now tabbed (account + admin); the rail
  // opens it on its default (Account) tab.
  function openAccount() {
    toggleOverlay('account');
  }

  // Phase 127 — in-app AĒR back/forward, the unified navigation the operator
  // asked for (Reflection in particular had no in-page way back and forth). The
  // app mutates history directly via `history.pushState` (url.svelte `pushUrl`),
  // so a reliable enabled/disabled state is not derivable from SvelteKit's
  // navigation hooks; the arrows therefore always defer to the browser's own
  // back/forward stack (which DOES account for those pushState entries) and the
  // browser no-ops at either end — exactly like its native toolbar buttons.
  function goBack() {
    if (typeof history !== 'undefined') history.back();
  }
  function goForward() {
    if (typeof history !== 'undefined') history.forward();
  }

  interface SurfaceEntry {
    href: string;
    label: string;
    glyph: string;
    hint: string;
    disabled: boolean;
  }

  const SURFACES = $derived<SurfaceEntry[]>([
    {
      href: '/',
      label: m.chrome_surface_atmosphere(),
      glyph: '◉',
      hint: m.chrome_surface_atmosphere_hint(),
      disabled: false
    },
    {
      // Phase 122k — Workbench is always reachable. When the user has a
      // Selection-State the anchor carries ONLY `?selectedProbes=` (no pillar
      // state), so the Workbench auto-opens the ScopeEditor seeded from the
      // selection rather than silently seeding a whole-probe panel.
      href:
        selectedCount > 0
          ? `/workbench${buildSelectionWorkbenchUrl(url.selectedProbes)}`
          : '/workbench',
      label: m.chrome_surface_workbench(),
      glyph: '⚙',
      hint:
        selectedCount > 0
          ? selectedCount === 1
            ? m.chrome_surface_workbench_hint_selected_one({ count: selectedCount })
            : m.chrome_surface_workbench_hint_selected_other({ count: selectedCount })
          : m.chrome_surface_workbench_hint(),
      disabled: false
    },
    {
      href: '/reflection',
      label: m.chrome_surface_reflection(),
      glyph: '¶',
      hint: m.chrome_surface_reflection_hint(),
      disabled: false
    }
  ]);

  // Phase 135 — surface buttons toggle: clicking the surface you are already on
  // returns to the Atmosphere (the persistent globe). Atmosphere itself is the
  // globe, so it has nothing to toggle back to.
  function onSurfaceClick(event: MouseEvent, href: string): void {
    if (href !== '/' && isActiveSurface(href)) {
      event.preventDefault();
      // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal back-to-globe
      void goto('/');
    }
  }

  function isActiveSurface(href: string): boolean {
    const p = page.url.pathname;
    if (href === '/') return p === '/';
    if (href.startsWith('/workbench') || href.startsWith('/lanes')) {
      // /workbench and /lanes/* (legacy redirected) map to the Workbench anchor.
      return p.startsWith('/lanes') || p.startsWith('/workbench');
    }
    return p.startsWith(href);
  }
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- all rail links are internal surface routes -->
<nav class="rail" aria-label={m.chrome_nav_primary()}>
  <div class="rail-brand">
    <span class="rail-wordmark">AĒR</span>
  </div>

  <!-- Surface anchors -->
  <ul class="rail-group" role="list">
    {#each SURFACES as s (s.label)}
      <li>
        {#if s.disabled}
          <span
            class="rail-anchor disabled"
            role="link"
            aria-disabled="true"
            aria-label={m.chrome_surface_disabled_aria({ label: s.label })}
            title={s.hint}
          >
            <span class="rail-icon" aria-hidden="true">{s.glyph}</span>
            <span class="rail-label">{s.label}</span>
          </span>
        {:else}
          <a
            href={s.href}
            class="rail-anchor"
            class:is-active={isActiveSurface(s.href)}
            aria-label={s.label}
            aria-current={isActiveSurface(s.href) ? 'page' : undefined}
            title={s.hint}
            data-sveltekit-preload-data="hover"
            onclick={(e) => onSurfaceClick(e, s.href)}
          >
            <span class="rail-icon" aria-hidden="true">{s.glyph}</span>
            <span class="rail-label">{s.label}</span>
          </a>
        {/if}
      </li>
    {/each}
  </ul>

  <!-- History back/forward — Phase 127. Always-available AĒR arrows mirroring
       the browser's own back/forward (see goBack/goForward for why a reliable
       disabled state is not derivable). -->
  <div class="rail-history" role="group" aria-label={m.chrome_nav_history_label()}>
    <button
      type="button"
      class="rail-history-btn"
      onclick={goBack}
      aria-label={m.chrome_nav_back()}
      title={m.chrome_nav_back_title()}
    >
      <span aria-hidden="true">←</span>
    </button>
    <button
      type="button"
      class="rail-history-btn"
      onclick={goForward}
      aria-label={m.chrome_nav_forward()}
      title={m.chrome_nav_forward_title()}
    >
      <span aria-hidden="true">→</span>
    </button>
  </div>

  <!-- Scope card — "Where am I": current selection + active Pillar. -->
  <div class="rail-scope">
    <span class="rail-scope-h">{m.chrome_section_where_am_i()}</span>
    <span class="rail-scope-line">
      {#if selectedCount === 0}
        {m.chrome_scope_no_probe()}
      {:else if selectedCount === 1}
        {m.chrome_scope_probe_one({ count: selectedCount })}
      {:else}
        {m.chrome_scope_probe_other({ count: selectedCount })}
      {/if}
    </span>
    <span class="rail-pillar" title={m.chrome_pillar_subitem_title()}>
      <span class="rail-pillar-dot" aria-hidden="true"></span>
      <span class="rail-pillar-label">{activePillar.label}</span>
    </span>
  </div>

  <div class="rail-spacer" aria-hidden="true"></div>

  <!-- Bottom group — overlay triggers + account. -->
  <div class="rail-bottom">
    <button
      type="button"
      class="rail-mini"
      class:has-selection={selectedCount > 0}
      onclick={openDossier}
      title={m.chrome_dossier_open_title()}
    >
      <span class="rail-mini-glyph" aria-hidden="true">❒</span>
      <span class="rail-mini-label">
        {selectedCount > 0
          ? m.chrome_dossier_selected_label({ count: selectedCount })
          : m.chrome_dossier_open_label()}
      </span>
    </button>
    <button
      type="button"
      class="rail-mini"
      onclick={openAnalyses}
      title={m.chrome_library_open_title()}
    >
      <span class="rail-mini-glyph" aria-hidden="true">★</span>
      <span class="rail-mini-label">{m.chrome_library_label()}</span>
    </button>
    <button type="button" class="rail-mini" onclick={openAccount} title={m.chrome_user_account()}>
      <span class="rail-mini-glyph" aria-hidden="true">◐</span>
      <span class="rail-mini-label">{m.chrome_user_account()}</span>
    </button>
  </div>
</nav>

<!-- eslint-enable svelte/no-navigation-without-resolve -->

<style>
  .rail {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    width: var(--rail-width);
    z-index: 450;
    background: var(--color-bg-elevated);
    border-right: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    padding: 0 var(--space-3) var(--space-4);
    gap: var(--space-4);
  }

  /* AĒR brand bar — fills the scope-bar height zone at the top of the rail. */
  .rail-brand {
    display: flex;
    align-items: center;
    width: 100%;
    min-height: var(--scope-bar-height);
    flex-shrink: 0;
    margin: 4 calc(-1 * var(--space-3));
    padding: 16px var(--space-3);
  }
  .rail-wordmark {
    font-family: var(--font-inter);
    font-weight: var(--font-weight-semibold);
    font-size: var(--font-size-xl);
    color: var(--color-wordmark);
    letter-spacing: 0.18em;
  }

  /* Surface anchors. */
  .rail-group {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .rail-group li {
    display: flex;
  }
  .rail-anchor {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    text-decoration: none;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .rail-anchor:hover,
  .rail-anchor:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .rail-anchor.is-active {
    background: color-mix(in srgb, var(--color-accent) 14%, transparent);
    color: var(--color-fg);
    border-color: color-mix(in srgb, var(--color-accent) 40%, transparent);
  }
  .rail-anchor.is-active .rail-icon {
    color: var(--color-accent);
  }
  .rail-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.2rem;
    font-size: 1.1rem;
    line-height: 1;
    flex-shrink: 0;
    color: var(--color-accent);
  }
  .rail-label {
    letter-spacing: 0.01em;
    line-height: 1.2;
  }
  .rail-anchor.disabled {
    opacity: 0.35;
    cursor: not-allowed;
    pointer-events: auto; /* keep tooltip + focus working */
  }
  .rail-anchor.disabled:hover,
  .rail-anchor.disabled:focus-visible {
    background: transparent;
    color: var(--color-fg-muted);
    outline: none;
  }

  /* History back/forward — a quiet two-button row under the surface anchors. */
  .rail-history {
    display: flex;
    gap: 6px;
  }
  .rail-history-btn {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-1) 0;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    font-size: 1rem;
    line-height: 1;
    cursor: pointer;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .rail-history-btn:hover,
  .rail-history-btn:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  @media (prefers-reduced-motion: reduce) {
    .rail-history-btn {
      transition: none;
    }
  }

  /* Scope card — "Where am I". */
  .rail-scope {
    margin-top: var(--space-1);
    padding: var(--space-3);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .rail-scope-h {
    font-size: 9.5px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
  }
  .rail-scope-line {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }
  .rail-pillar {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-top: 2px;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }
  .rail-pillar-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--color-accent);
    flex-shrink: 0;
  }
  .rail-pillar-label {
    color: var(--color-fg-subtle);
    letter-spacing: 0.02em;
  }

  .rail-spacer {
    flex: 1;
  }

  /* Bottom group — muted mini-buttons. */
  .rail-bottom {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .rail-mini {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    appearance: none;
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    color: var(--color-fg-subtle);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    padding: var(--space-2) var(--space-3);
    text-align: left;
    cursor: pointer;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .rail-mini:hover,
  .rail-mini:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .rail-mini.has-selection {
    color: var(--color-accent);
  }
  .rail-mini-glyph {
    font-size: var(--font-size-sm);
    flex-shrink: 0;
  }
  .rail-mini-label {
    flex: 1 1 auto;
  }

  @media (prefers-reduced-motion: reduce) {
    .rail-anchor,
    .rail-mini {
      transition: none;
    }
  }
</style>

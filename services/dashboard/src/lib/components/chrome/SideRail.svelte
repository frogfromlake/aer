<script lang="ts">
  // Left side rail — Phase 122h / ADR-033 §3.
  //
  // Three labelled sections, each with a section eyebrow that names its role:
  //
  //   1. Where am I?    — Surface anchors (Atmosphere / Workbench / Reflection).
  //                       Each anchor carries icon + full word. Workbench
  //                       additionally exposes the active Pillar as a small
  //                       sub-item (`↳ Aleph` / `↳ Episteme` / `↳ Rhizome`)
  //                       so the user always sees both the surface AND the
  //                       analytical stance at a glance.
  //
  //   2. Dossier        — Phase 123a: the Dossier is a global overlay
  //                       (search/catalogue + probe selection), not a
  //                       surface route. This button opens it over any
  //                       surface and shows the current selection count.
  //                       Replaces the Phase-122k Probe-Filter trigger.
  //
  //   3. View           — Persistent global toggles (Negative Space overlay).
  //
  // Keyboard: every interactive target is a native <a> or <button> with
  // browser-default focus handling. `prefers-reduced-motion` suppresses
  // all transitions.
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { urlState, toggleOverlay } from '$lib/state/url.svelte';
  import { user, isAdmin, doLogout } from '$lib/state/auth.svelte';
  import { getPillar } from '$lib/presentations';
  import { buildSelectionWorkbenchUrl } from '$lib/workbench/panel-queries';
  import type { PillarId } from '$lib/state/url-internals';

  const url = $derived(urlState());
  const activePillarId = $derived<PillarId>(url.activePillar ?? 'aleph');
  const activePillar = $derived(getPillar(activePillarId));

  // Phase 123a — the Dossier is a global overlay (ADR-033 amendment), not
  // a surface route. The rail exposes it as a button that opens the
  // search/catalogue overlay (`?dossier=open`) over any surface and
  // surfaces the current selection count. Replaces the Phase-122k
  // Probe-Filter Modal trigger; probe selection now lives in the overlay.
  function openDossier() {
    toggleOverlay('dossier');
  }

  // Phase 135 — saved analyses are a global overlay (`?analyses=open`), the
  // same model as the Dossier. Reachable here and from the account submenu.
  // Saving the *current* Workbench view lives in the Workbench header (where the
  // analysis is built), not here — this entry is the library (browse/load).
  function openAnalyses() {
    toggleOverlay('analyses');
  }

  // Phase 135 — the account submenu is retired: its items live as standalone,
  // bottom-anchored rail buttons. Your account / Administration stay toggles.
  function openAccount() {
    toggleOverlay('account');
  }
  function openAdmin() {
    toggleOverlay('admin');
  }
  const currentUser = $derived(user());
  const userInitial = $derived((currentUser?.email ?? '?').trim().charAt(0).toUpperCase());

  // Active probe — from the route path param (legacy per-probe routes) OR
  // from the multi-probe selection cart. Drives the Workbench rail anchor's
  // active-Pillar sub-item display.
  const activeProbe = $derived<string | null>(
    (page.params as Record<string, string | undefined>).probeId ??
      (url.selectedProbes.length > 0 ? url.selectedProbes[0]! : null)
  );

  interface SurfaceEntry {
    href: string;
    label: string;
    glyph: string;
    hint: string;
    disabled: boolean;
    pillarSubItem?: { glyph: string; label: string; color: string };
  }

  const SURFACES = $derived<SurfaceEntry[]>([
    {
      href: '/',
      label: 'Atmosphere',
      glyph: '◉',
      hint: '3D globe and probe selection',
      disabled: false
    },
    {
      // Phase 122k — Workbench is always reachable. Issue 3 — when the user
      // has a Selection-State the anchor carries ONLY `?selectedProbes=` (no
      // pillar state), so the Workbench auto-opens the ScopeEditor seeded
      // from the selection rather than silently seeding a whole-probe panel.
      href:
        url.selectedProbes.length > 0
          ? `/workbench${buildSelectionWorkbenchUrl(url.selectedProbes)}`
          : '/workbench',
      label: 'Workbench',
      glyph: '⚙',
      hint:
        url.selectedProbes.length > 0
          ? `Open Workbench with ${url.selectedProbes.length} selected probe${url.selectedProbes.length === 1 ? '' : 's'}`
          : 'Analysis — Aleph · Episteme · Rhizome',
      disabled: false,
      ...(activeProbe
        ? {
            pillarSubItem: {
              glyph: activePillar.glyph,
              label: activePillar.label,
              color: activePillar.color
            }
          }
        : {})
    },
    {
      href: '/reflection',
      label: 'Reflection',
      glyph: '¶',
      hint: 'Working Papers · Primers · Open research questions',
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
<nav class="rail" aria-label="Primary navigation">
  <div class="logo">
    <span class="logo-text">AĒR</span>
  </div>

  <!-- Section 1: Surface anchors -->
  <div class="section">
    <span class="section-eyebrow">Where am I?</span>
    <ul class="surface-list" role="list">
      {#each SURFACES as s (s.label)}
        <li>
          {#if s.disabled}
            <span
              class="surface-link disabled"
              role="link"
              aria-disabled="true"
              aria-label="{s.label} (disabled — pick a probe on the Atmosphere first)"
              title={s.hint}
            >
              <span class="glyph" aria-hidden="true">{s.glyph}</span>
              <span class="rail-label">{s.label}</span>
            </span>
          {:else}
            <a
              href={s.href}
              class="surface-link"
              class:active={isActiveSurface(s.href)}
              aria-label={s.label}
              aria-current={isActiveSurface(s.href) ? 'page' : undefined}
              title={s.hint}
              data-sveltekit-preload-data="hover"
              onclick={(e) => onSurfaceClick(e, s.href)}
            >
              <span class="glyph" aria-hidden="true">{s.glyph}</span>
              <span class="rail-label">{s.label}</span>
            </a>
          {/if}
          {#if s.pillarSubItem}
            <div
              class="pillar-sub-item"
              style:--pillar-color={s.pillarSubItem.color}
              aria-label="Active Pillar: {s.pillarSubItem.label}"
              title="Active Pillar — switch via the PillarSwitch tiles at the top of the Workbench"
            >
              <span class="pillar-sub-arrow" aria-hidden="true">↳</span>
              <span class="pillar-sub-glyph" aria-hidden="true">{s.pillarSubItem.glyph}</span>
              <span class="pillar-sub-label">{s.pillarSubItem.label}</span>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  </div>

  <!-- Section 2: Phase 123a — Dossier overlay trigger. The Dossier is a
       global overlay (search/catalogue + probe selection), not a surface
       route; this button opens it over any surface and shows the current
       selection count. -->
  <div class="section section-probe">
    <span class="section-eyebrow">Dossier</span>
    <button
      type="button"
      class="probe-filter-btn"
      onclick={openDossier}
      class:has-selection={url.selectedProbes.length > 0}
      title="Open the Dossier — search the catalogue & build your probe selection"
    >
      <span class="probe-filter-glyph" aria-hidden="true">❒</span>
      <span class="probe-filter-label">
        {url.selectedProbes.length > 0
          ? `Dossier · ${url.selectedProbes.length} selected`
          : 'Open Dossier'}
      </span>
    </button>
  </div>

  <!-- Phase 135 — Saved analyses overlay trigger (re-openable Workbench
       deep links, shareable by identity). Same overlay model as the Dossier. -->
  <div class="section section-probe">
    <span class="section-eyebrow">Library</span>
    <button
      type="button"
      class="probe-filter-btn"
      onclick={openAnalyses}
      title="Open your saved analyses — re-openable & shareable Workbench views"
    >
      <span class="probe-filter-glyph" aria-hidden="true">★</span>
      <span class="probe-filter-label">Saved analyses</span>
    </button>
  </div>

  <div class="flex-spacer" aria-hidden="true"></div>

  <!-- User menu (Phase 135) — the former account submenu, flattened into
       standalone bottom-anchored rail buttons. The avatar + email is a static
       display (not a button); Your account / Administration stay overlay
       toggles; Sign out ends the session. -->
  {#if currentUser}
    <div class="section user-section">
      <span class="section-eyebrow">User menu</span>
      <div class="user-display">
        <span class="user-avatar" aria-hidden="true">{userInitial}</span>
        <span class="user-email" title={currentUser.email}>{currentUser.email}</span>
      </div>
      <button type="button" class="rail-menu-btn" onclick={openAccount}>
        <span class="rail-menu-glyph" aria-hidden="true">◐</span>
        <span class="rail-menu-label">Your account</span>
      </button>
      {#if isAdmin()}
        <button type="button" class="rail-menu-btn" onclick={openAdmin}>
          <span class="rail-menu-glyph" aria-hidden="true">⛨</span>
          <span class="rail-menu-label">Administration</span>
        </button>
      {/if}
      <button type="button" class="rail-menu-btn signout" onclick={() => doLogout()}>
        <span class="rail-menu-glyph" aria-hidden="true">⎋</span>
        <span class="rail-menu-label">Sign out</span>
      </button>
    </div>
  {/if}
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
    background: var(--color-bg-overlay);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    border-right: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    padding: 0 0 var(--space-4);
  }

  /* AĒR brand bar — fills the scope-bar height zone at the top of the rail */
  .logo {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    min-height: var(--scope-bar-height);
    flex-shrink: 0;
    border-bottom: 1px solid var(--color-border);
  }

  .logo-text {
    font-family: var(--font-mono);
    font-size: 15px;
    font-weight: var(--font-weight-bold);
    color: var(--color-accent);
    letter-spacing: 0.18em;
  }

  /* Sections have generous vertical breathing room per Finding 2.1 —
     SideRail felt cramped vertically in the first manual test. */
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--color-border);
  }

  .section:last-of-type {
    border-bottom: none;
  }

  .section-eyebrow {
    font-size: 9.5px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    padding: 0 var(--space-2) var(--space-1);
    line-height: 1;
  }

  .flex-spacer {
    flex: 1;
  }

  .surface-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .surface-list li {
    display: flex;
    flex-direction: column;
  }

  .surface-link {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    /* slightly taller per Finding 2.1 — easier hit-target + more visual rhythm */
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    text-decoration: none;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .surface-link .glyph {
    font-size: 1.3rem;
    line-height: 1;
    flex-shrink: 0;
    color: var(--color-accent);
  }

  .rail-label {
    font-size: 13px;
    font-weight: var(--font-weight-medium);
    font-family: var(--font-ui);
    letter-spacing: 0.01em;
    line-height: 1.2;
  }

  .surface-link:hover,
  .surface-link:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .surface-link.active {
    color: var(--color-fg);
    background: var(--color-surface);
  }

  .surface-link.disabled {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-md);
    opacity: 0.35;
    cursor: not-allowed;
    pointer-events: auto; /* keep tooltip + focus working */
    color: var(--color-fg-muted);
  }
  .surface-link.disabled .glyph {
    font-size: 1.3rem;
    line-height: 1;
    flex-shrink: 0;
    color: var(--color-fg-muted);
  }
  .surface-link.disabled:hover,
  .surface-link.disabled:focus-visible {
    background: transparent;
    color: var(--color-fg-muted);
    outline: none;
  }

  /* Active-Pillar sub-item under the Workbench surface anchor.
     Visually subordinate (smaller, indented, tinted with pillar color)
     so it does not compete with the surface label for primary attention.
     It is purely informational here — the actual switching happens on
     the PillarSwitch tiles inside the Workbench surface. */
  .pillar-sub-item {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px var(--space-3) 4px calc(var(--space-3) + 1.3rem + var(--space-3));
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--pillar-color);
    line-height: 1.2;
  }

  .pillar-sub-arrow {
    color: var(--color-fg-subtle);
  }

  .pillar-sub-glyph {
    font-size: 13px;
    line-height: 1;
  }

  .pillar-sub-label {
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.02em;
  }

  /* Probe-section gets a touch more horizontal padding so the
     Probe-Filter button has full width without bumping the rail edges. */
  .section-probe {
    padding-left: var(--space-2);
    padding-right: var(--space-2);
  }

  .probe-filter-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    padding: var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .probe-filter-btn:hover,
  .probe-filter-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .probe-filter-btn.has-selection {
    color: var(--color-accent);
    border-color: var(--color-accent-muted);
  }
  .probe-filter-glyph {
    font-size: var(--font-size-sm);
  }
  .probe-filter-label {
    flex: 1 1 auto;
    text-align: left;
  }

  /* Unified bottom rail buttons (Negative Space + user-menu items). Same
     look as the Library/Dossier buttons so the bottom reads as one menu. */
  .rail-menu-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    padding: var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
    text-align: left;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .rail-menu-btn:hover,
  .rail-menu-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .rail-menu-btn.signout {
    color: var(--color-status-expired);
  }
  .rail-menu-btn.signout:hover,
  .rail-menu-btn.signout:focus-visible {
    color: var(--color-status-expired);
    border-color: var(--color-status-expired);
  }
  .rail-menu-glyph {
    font-size: var(--font-size-sm);
    flex-shrink: 0;
  }
  .rail-menu-label {
    flex: 1 1 auto;
    text-align: left;
  }

  /* User identity — static display, NOT interactive. */
  .user-section {
    gap: var(--space-2);
  }
  .user-display {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-1) var(--space-2) var(--space-2);
  }
  .user-avatar {
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border-strong);
    background: var(--color-surface);
    color: var(--color-fg);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    display: grid;
    place-items: center;
  }
  .user-email {
    font-size: 12px;
    font-family: var(--font-ui);
    color: var(--color-fg-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @media (prefers-reduced-motion: reduce) {
    .surface-link {
      transition: none;
    }
  }
</style>

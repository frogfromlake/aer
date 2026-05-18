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
  //   2. Select Probe   — The ProbePicker as a prominent central control —
  //                       inline expandable list (no popover). The dashboard
  //                       is useless without a probe; the picker is foregrounded
  //                       above the bottom toggles, not pushed to the floor.
  //                       Highlighted state pulses the trigger when the user
  //                       lands on the Workbench without a probe in scope.
  //
  //   3. View           — Persistent global toggles (Negative Space overlay).
  //
  // Keyboard: every interactive target is a native <a> or <button> with
  // browser-default focus handling. `prefers-reduced-motion` suppresses
  // all transitions.
  import { page } from '$app/state';
  import { urlState } from '$lib/state/url.svelte';
  import { negativeSpaceActive, setNegativeSpaceActive } from '$lib/state/tray.svelte';
  import NegativeSpaceToggle from '$lib/components/NegativeSpaceToggle.svelte';
  import ProbeFilterModal from './ProbeFilterModal.svelte';
  import { getPillar } from '$lib/viewmodes';
  import { buildFreeComposeUrl } from '$lib/workbench/panel-queries';
  import type { ViewingMode } from '$lib/state/url-internals';

  const url = $derived(urlState());
  const activePillarId = $derived<ViewingMode>(url.activePillar ?? 'aleph');
  const activePillar = $derived(getPillar(activePillarId));
  const negSpace = $derived(negativeSpaceActive());

  // Phase 122k K3 — Probe-Filter Modal trigger. Visible only on
  // Atmosphere + Dossier (the selection-driven contexts); hidden on
  // Workbench (the Workbench's own ScopeEditor owns scope) and on
  // Reflection (no scope semantic). The Modal itself is mounted at the
  // bottom of SideRail's template.
  let probeFilterOpen = $state(false);
  const showProbeFilter = $derived(
    page.url.pathname === '/' || page.url.pathname.startsWith('/dossier')
  );

  // Active probe — either from the route path param (Surface II legacy
  // routes + the new /dossier/{probeId}) OR from the multi-probe scope
  // state (when on /workbench with ?probeId=... selected). The Workbench
  // rail anchor is only meaningful when at least one probe is in scope.
  const activeProbe = $derived<string | null>(
    (page.params as Record<string, string | undefined>).probeId ??
      (url.selectedProbes.length > 0 ? url.selectedProbes[0]! : null)
  );

  // Phase 122k K3: the inline ProbePicker is retired in favour of the
  // ProbeFilterModal opened from the Probe-Filter button. The activeProbe
  // derivation stays — it still drives the legacy `?probeId=` SideRail
  // pillar-sub-item display.

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
      // Phase 122i revision (R5). Dossier is a first-class top-level
      // surface (ADR-033 amendment) hosting the probe catalogue + the
      // General Free-Compose entry path.
      href: '/dossier',
      label: 'Dossier',
      glyph: '❒',
      hint: 'Probe catalogue · General Free-Compose · per-probe inspection',
      disabled: false
    },
    {
      // Phase 122k — Workbench is always reachable. When the user has a
      // Selection-State the anchor seeds a pillar Workbench from those
      // probes; when not, it lands on /workbench plain (empty state, to
      // be replaced by an auto-opening ScopeEditor in K3).
      href:
        url.selectedProbes.length > 0
          ? `/workbench${buildFreeComposeUrl({
              pillar: 'aleph',
              probeIds: [...url.selectedProbes],
              sourceIds: []
            })}`
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

  function isActiveSurface(href: string): boolean {
    const p = page.url.pathname;
    if (href === '/') return p === '/';
    if (href === '/dossier') {
      // Phase 122i revision (R5). The Dossier anchor lights up for any
      // /dossier* path. Legacy `/dossier/{id}` was retired but the
      // 308-redirect from /lanes/{id}/dossier still passes through this
      // namespace.
      return p.startsWith('/dossier');
    }
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

  <!-- Section 2: Phase 122k K3 — Probe-Filter affordance. Opens the
       ProbeFilterModal with region-grouped probes + search. Selection
       feeds the Dossier filter + the Workbench ScopeEditor seed. Visible
       only on Atmosphere and Dossier surfaces (the Workbench owns its
       own ScopeEditor; Reflection is read-only). -->
  {#if showProbeFilter}
    <div class="section section-probe">
      <span class="section-eyebrow">Probes</span>
      <button
        type="button"
        class="probe-filter-btn"
        onclick={() => (probeFilterOpen = true)}
        class:has-selection={url.selectedProbes.length > 0}
        title="Filter the catalogue / seed the Workbench"
      >
        <span class="probe-filter-glyph" aria-hidden="true">▤</span>
        <span class="probe-filter-label">
          {url.selectedProbes.length > 0
            ? `${url.selectedProbes.length} selected`
            : 'Filter probes'}
        </span>
      </button>
    </div>
  {/if}

  <div class="flex-spacer" aria-hidden="true"></div>

  <!-- Section 3: Persistent view toggles -->
  <div class="section">
    <span class="section-eyebrow">View</span>
    <div class="ns-wrap">
      <NegativeSpaceToggle active={negSpace} onToggle={setNegativeSpaceActive} />
    </div>
  </div>
</nav>

{#if probeFilterOpen}
  <ProbeFilterModal onClose={() => (probeFilterOpen = false)} />
{/if}

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

  .ns-wrap {
    display: flex;
    justify-content: flex-start;
    padding: 0 var(--space-2);
  }

  @media (prefers-reduced-motion: reduce) {
    .surface-link {
      transition: none;
    }
  }
</style>

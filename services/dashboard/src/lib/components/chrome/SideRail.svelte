<script lang="ts">
  // Left side rail — Design Brief §3.2.
  // Persistent primary navigation: three surface anchors, return-to-Atmosphere
  // planet glyph, compact scope indicator, and pillar-mode toggle.
  //
  // Phase 105: Function Lanes (/lanes) navigated to a stub page — live surface followed in Phase 106.
  // Phase 109: Reflection (/reflection) is the live Surface III (Working Papers, primers, open questions).
  //
  // Keyboard: Tab/Shift+Tab cycles through interactive elements. All targets
  // are standard <a> or <button> elements, so browser focus handling is native.
  // Reduced-motion: no CSS transitions when prefers-reduced-motion: reduce.
  import { page } from '$app/state';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import type { ViewingMode } from '$lib/state/url-internals';
  import { negativeSpaceActive, setNegativeSpaceActive } from '$lib/state/tray.svelte';
  import NegativeSpaceToggle from '$lib/components/NegativeSpaceToggle.svelte';

  // ROADMAP §1773: pillar modes are URL-tracked but visually identical
  // today — Episteme/Aleph differ only in framing once L2/L3 panels read
  // them, and Rhizome has no data source. Tooltips reflect that reality
  // so the buttons are not silently misleading.
  const PILLARS: readonly {
    id: ViewingMode;
    abbr: string;
    label: string;
    hint: string;
    disabled?: boolean;
  }[] = [
    {
      id: 'aleph',
      abbr: 'A',
      label: 'Aleph',
      hint: 'Aleph — totality (default). Every observed probe, no filter.'
    },
    {
      id: 'episteme',
      abbr: 'E',
      label: 'Episteme',
      hint: 'Episteme — knowledge register. Pillar lever wired, dedicated rendering arrives in a later phase.'
    },
    {
      id: 'rhizome',
      abbr: 'R',
      label: 'Rhizome',
      hint: 'Rhizome — relational propagation. No data source yet; selectable but inert.',
      disabled: true
    }
  ];

  const url = $derived(urlState());
  let activePillar = $derived<ViewingMode>(url.viewingMode ?? 'aleph');
  const negSpace = $derived(negativeSpaceActive());

  // Active probe: prefer path param (dossier/lane pages) over URL query param (Atmosphere).
  let activeProbe = $derived<string | null>(
    (page.params as Record<string, string | undefined>).probeId ?? url.probe ?? null
  );

  // Function Lanes link → dossier when a probe is active, otherwise stub.
  let lanesHref = $derived(activeProbe ? `/lanes/${activeProbe}/dossier` : '/lanes');

  const SURFACES = $derived([
    {
      href: lanesHref,
      label: 'Lanes',
      glyph: '≡',
      hint: 'Surface II — Function Lanes'
    },
    {
      href: '/reflection',
      label: 'Reflect',
      glyph: '¶',
      hint: 'Surface III — Reflection (Working Papers, Primers, Open Questions)'
    }
  ]);

  function isActiveSurface(href: string): boolean {
    const p = page.url.pathname;
    if (href === '/') return p === '/';
    // /lanes/* matches Function Lanes regardless of probe in path
    if (href.startsWith('/lanes')) return p.startsWith('/lanes');
    return p.startsWith(href);
  }
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- all rail links are internal surface routes -->
<nav class="rail" aria-label="Primary navigation">
  <!-- Planet glyph: Surface I — Atmosphere -->
  <a
    href="/"
    class="planet"
    class:active={page.url.pathname === '/'}
    aria-label="Atmosphere"
    aria-current={page.url.pathname === '/' ? 'page' : undefined}
    title="Surface I — Atmosphere (3D globe + probe overview)"
    data-sveltekit-preload-data="hover"
  >
    <span class="glyph" aria-hidden="true">◉</span>
    <span class="rail-label">Atmos</span>
  </a>

  <div class="divider" role="separator" aria-hidden="true"></div>

  <!-- Surface anchors -->
  <ul class="surfaces" role="list" aria-label="Surfaces">
    {#each SURFACES as s (s.label)}
      <li>
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
      </li>
    {/each}
  </ul>

  <!-- Scope indicator: active probe (compact) -->
  <div class="scope-indicator" aria-label="Active scope: {activeProbe ?? 'no probe selected'}">
    <span
      class="probe-tag"
      class:dim={!activeProbe}
      title={activeProbe ? `Active probe: ${activeProbe}` : 'No probe selected'}
    >
      {#if activeProbe}
        {activeProbe.slice(0, 7)}
      {:else}
        —
      {/if}
    </span>
  </div>

  <div class="divider" role="separator" aria-hidden="true"></div>

  <!-- Pillar-mode toggle -->
  <div class="pillar-section">
    <span class="rail-eyebrow" aria-hidden="true">Pillar</span>
    <div class="pillar-group" role="radiogroup" aria-label="Pillar mode">
      {#each PILLARS as p (p.id)}
        <button
          type="button"
          role="radio"
          aria-checked={activePillar === p.id}
          aria-disabled={p.disabled ? 'true' : undefined}
          class="pillar-btn"
          class:active={activePillar === p.id}
          class:inert={p.disabled}
          title={p.hint}
          onclick={() => {
            if (p.disabled) return;
            setUrl({ viewingMode: p.id });
          }}
        >
          <span aria-hidden="true">{p.abbr}</span>
          <span class="sr-only">{p.label}</span>
        </button>
      {/each}
    </div>
  </div>

  <div class="divider" role="separator" aria-hidden="true"></div>

  <!-- Negative Space overlay toggle — persistent across all surfaces -->
  <div class="ns-wrap">
    <NegativeSpaceToggle active={negSpace} onToggle={setNegativeSpaceActive} />
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
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-3) 0 var(--space-3);
  }

  .planet {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    width: 44px;
    padding: 4px 0;
    border-radius: var(--radius-md);
    color: var(--color-accent);
    text-decoration: none;
    flex-shrink: 0;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .planet .glyph {
    font-size: 1.2rem;
    line-height: 1;
  }

  .planet:hover,
  .planet:focus-visible {
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .planet.active {
    background: var(--color-surface);
  }

  .divider {
    width: 28px;
    height: 1px;
    background: var(--color-border);
    flex-shrink: 0;
    margin: var(--space-1) 0;
  }

  .surfaces {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    flex: 1;
  }

  .surface-link {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    width: 44px;
    padding: 4px 0;
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    text-decoration: none;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .surface-link .glyph {
    font-size: 1.1rem;
    line-height: 1;
  }

  .rail-label {
    font-size: 9px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-weight: var(--font-weight-medium);
    font-family: var(--font-ui);
    line-height: 1;
  }

  .pillar-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }
  .rail-eyebrow {
    font-size: 8px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    line-height: 1;
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

  .scope-indicator {
    padding: var(--space-1) 0;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .probe-tag {
    font-size: 9px;
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    text-align: center;
    max-width: 44px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.4;
  }

  .probe-tag.dim {
    opacity: 0.35;
  }

  .pillar-group {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .ns-wrap {
    padding: var(--space-1) var(--space-2) var(--space-2);
    display: flex;
    justify-content: center;
  }

  .pillar-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 26px;
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: 11px;
    font-family: var(--font-ui);
    font-weight: var(--font-weight-medium);
    letter-spacing: 0.04em;
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .pillar-btn:hover {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .pillar-btn.active {
    color: var(--color-fg);
    background: rgba(82, 131, 184, 0.2);
    border-color: #5283b8;
  }

  .pillar-btn.inert {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .pillar-btn.inert:hover {
    color: var(--color-fg-muted);
    border-color: var(--color-border);
  }

  .pillar-btn:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .planet,
    .surface-link,
    .pillar-btn {
      transition: none;
    }
  }
</style>

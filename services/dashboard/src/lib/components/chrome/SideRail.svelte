<script lang="ts">
  // Left side rail — Design Brief §3.2.
  // Persistent primary navigation: three surface anchors, return-to-Atmosphere
  // planet glyph, compact scope indicator, and pillar-mode toggle.
  //
  // Phase 105: Function Lanes (/lanes) and Reflection (/reflection) navigate to
  // stub pages — live surfaces follow in Phases 106 and 109 respectively.
  //
  // Keyboard: Tab/Shift+Tab cycles through interactive elements. All targets
  // are standard <a> or <button> elements, so browser focus handling is native.
  // Reduced-motion: no CSS transitions when prefers-reduced-motion: reduce.
  import { page } from '$app/state';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import type { ViewingMode } from '$lib/state/url-internals';

  const PILLARS: readonly { id: ViewingMode; abbr: string; label: string; hint: string }[] = [
    {
      id: 'aleph',
      abbr: 'A',
      label: 'Aleph',
      hint: 'Totality — every observed probe, no filter'
    },
    {
      id: 'episteme',
      abbr: 'E',
      label: 'Episteme',
      hint: 'Knowledge register — epistemic-authority probes'
    },
    {
      id: 'rhizome',
      abbr: 'R',
      label: 'Rhizome',
      hint: 'Relational propagation — no data yet'
    }
  ];

  const url = $derived(urlState());
  let activePillar = $derived<ViewingMode>(url.viewingMode ?? 'aleph');

  // Active probe: prefer path param (dossier/lane pages) over URL query param (Atmosphere).
  let activeProbe = $derived<string | null>(
    (page.params as Record<string, string | undefined>).probeId ?? url.probe ?? null
  );

  // Function Lanes link → dossier when a probe is active, otherwise stub.
  let lanesHref = $derived(activeProbe ? `/lanes/${activeProbe}/dossier` : '/lanes');

  const SURFACES = $derived([
    {
      href: '/',
      label: 'Atmosphere',
      glyph: '●',
      hint: 'Surface I — Atmosphere (3D globe + probe overview)'
    },
    {
      href: lanesHref,
      label: 'Function Lanes',
      glyph: '≡',
      hint: 'Surface II — Function Lanes'
    },
    {
      href: '/reflection',
      label: 'Reflection',
      glyph: '¶',
      hint: 'Surface III — Reflection (Phase 109)'
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
  <!-- Planet glyph: return to Atmosphere from any surface -->
  <a
    href="/"
    class="planet"
    aria-label="Return to Atmosphere"
    title="Return to Atmosphere"
    data-sveltekit-preload-data="hover"
  >
    <span aria-hidden="true">◉</span>
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
          <span aria-hidden="true">{s.glyph}</span>
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
  <div class="pillar-group" role="radiogroup" aria-label="Pillar mode">
    {#each PILLARS as p (p.id)}
      <button
        type="button"
        role="radio"
        aria-checked={activePillar === p.id}
        class="pillar-btn"
        class:active={activePillar === p.id}
        title="{p.label}: {p.hint}"
        onclick={() => setUrl({ viewingMode: p.id })}
      >
        <span aria-hidden="true">{p.abbr}</span>
        <span class="sr-only">{p.label}</span>
      </button>
    {/each}
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
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: var(--radius-md);
    color: var(--color-accent);
    text-decoration: none;
    font-size: 1.2rem;
    flex-shrink: 0;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .planet:hover,
  .planet:focus-visible {
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
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
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    text-decoration: none;
    font-size: 1.1rem;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
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
    padding-bottom: var(--space-1);
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

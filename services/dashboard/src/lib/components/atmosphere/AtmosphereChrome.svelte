<script lang="ts">
  // Atmosphere globe chrome (Phase 151 design pass) — the three overlay cards
  // that frame the globe on Surface I:
  //   • Coverage disclosure — top-right titled card (AĒR makes NO geographic
  //     coverage claim; reach is unmeasurable — Negative Space, WP-006 §4.2).
  //   • Dataset quick-stats — bottom-left readout window (Design Brief §4.1).
  //   • "How to read the globe" — bottom-right primer link.
  //
  // Presentational: it receives already-formatted stat strings and renders
  // them. The data derivation (probe + metric counts) stays in the owning
  // AtmosphereSurface; this child only carries layout + scoped CSS, keeping
  // the surface file within its length budget.
  import StatReadout from '$lib/components/base/StatReadout.svelte';
  import { openOverlay } from '$lib/state/url.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    /** Formatted (locale-aware) stat values. `documents` is "—" until metrics resolve. */
    activeProbes: string;
    sources: string;
    documents: string;
  }
  let { activeProbes, sources, documents }: Props = $props();
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface III routes -->

<aside class="coverage-card" aria-label={m.atmosphere_absence_label()}>
  <div class="coverage-h">{m.atmosphere_absence_label()}</div>
  <p class="coverage-text">
    {m.atmosphere_absence_text()}
    <a class="coverage-link" href="/reflection/wp/wp-006?section=4.2"
      >{m.atmosphere_absence_link()}</a
    >
  </p>
</aside>

<aside class="atm-stats" aria-label={m.atmosphere_stats_label()}>
  <StatReadout
    size="sm"
    label={m.atmosphere_stats_active_probes()}
    value={activeProbes}
    accent="accent"
  />
  <StatReadout size="sm" label={m.atmosphere_stats_sources()} value={sources} />
  <StatReadout
    size="sm"
    label={m.atmosphere_stats_documents()}
    value={documents}
    caption={m.atmosphere_stats_documents_caption()}
  />
  <StatReadout
    size="sm"
    label={m.atmosphere_stats_dataset_age()}
    value="—"
    caption={m.atmosphere_stats_provisional()}
  />
</aside>

<div class="atm-corner">
  <!-- Re-openable "About AĒR" intro (Phase 149) — the readable home of the
       first-visit welcome. ⓘ keeps it discoverable without nagging. -->
  <button type="button" class="atm-about" onclick={() => openOverlay('about')}>
    <span class="atm-about-mark" aria-hidden="true">ⓘ</span>
    {m.about_open()}
  </button>
  <a class="atm-primer" href="/reflection/primer/globe">{m.atmosphere_primer_link()}</a>
</div>

<!-- eslint-enable svelte/no-navigation-without-resolve -->

<style>
  /* Coverage disclosure — top-right titled card. Offset below the fixed
     ScopeBar so it is never occluded. */
  .coverage-card {
    position: absolute;
    right: var(--space-5);
    top: calc(var(--scope-bar-height) + var(--space-5));
    width: 260px;
    z-index: 350;
    padding: var(--space-4);
    background: color-mix(in srgb, var(--color-bg-elevated) 70%, transparent);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    pointer-events: auto;
  }
  .coverage-h {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-status-refused);
    margin-bottom: var(--space-2);
  }
  .coverage-text {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: 1.55;
  }
  .coverage-link {
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-border);
    white-space: nowrap;
  }
  .coverage-link:hover,
  .coverage-link:focus-visible {
    color: var(--color-accent);
    border-bottom-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  /* Bottom-left dataset quick-stats window. Offset right of the fixed side rail. */
  .atm-stats {
    position: absolute;
    left: calc(var(--rail-width) + var(--space-6));
    bottom: var(--space-6);
    z-index: 350;
    display: flex;
    gap: var(--space-6);
    padding: var(--space-4) var(--space-5);
    background: color-mix(in srgb, var(--color-bg-elevated) 70%, transparent);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    pointer-events: auto;
  }

  /* Bottom-right corner cluster: "About AĒR" trigger + "How to read" primer,
     right-aligned and stacked. */
  .atm-corner {
    position: absolute;
    right: var(--space-6);
    bottom: var(--space-6);
    z-index: 350;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: var(--space-2);
    pointer-events: none;
  }

  .atm-about {
    pointer-events: auto;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 0;
    border: 0;
    background: transparent;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    cursor: pointer;
  }
  .atm-about-mark {
    font-size: 1em;
    color: var(--color-fg-subtle);
  }
  .atm-about:hover,
  .atm-about:focus-visible {
    color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  /* "How to read the globe" — bottom-right primer link. */
  .atm-primer {
    pointer-events: auto;
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }
  .atm-primer:hover,
  .atm-primer:focus-visible {
    text-decoration: underline;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
</style>

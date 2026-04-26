<script lang="ts">
  // Surface I — L3 landing teaser (Phase 108 cleanup pass).
  //
  // Pre-Phase-108 this panel duplicated Surface II's analysis: a metric
  // selector, a per-probe time-series, and a full Dual-Register block.
  // Surface II's Probe Dossier + Function Lanes (Phases 106 / 107) now
  // own analysis, and the methodology tray (Phase 108) owns the
  // methodological register. Keeping the chart + metric selector + a
  // second Dual-Register surface here violated the reframing-note rule
  // "no two surfaces do the same job" and produced the user-facing
  // confusion that motivated this rewrite.
  //
  // The new contract is narrow: this panel anchors the user inside
  // Surface I (emic frame + structural meta + reach-disclaimer) and
  // lifts them into Surface II for actual analysis. Concretely:
  //
  //   - One inline semantic-register paragraph (emic anchoring frame).
  //     The methodological register is *only* in the tray now.
  //   - Structural meta: probe id, language, source count, pub rate.
  //   - Quick-jump tiles: one per WP-001 discourse function plus a
  //     "Probe Dossier" tile, each routing into Surface II.
  //   - "Open methodology tray" affordance — explicit so first-time
  //     readers discover the right-edge surface; the tray was always
  //     reachable via the right-edge tab as well.
  //   - Reach-disclaimer (Brief §5.7) remains.
  //
  // No chart, no metric selector, no Dual-Register flip. Surface II
  // owns the chart-bearing analysis; the tray owns provenance.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import { trayOpen } from '$lib/state/tray.svelte';

  interface Props {
    probe: ProbeDto;
    ctx: FetchContext;
    onOpenProvenance: () => void;
    publicationRate: number | null;
  }

  let { probe, ctx, onOpenProvenance, publicationRate }: Props = $props();

  const isTrayOpen = $derived(trayOpen());

  // Probe content — used only for the inline emic semantic paragraph.
  // The methodological register lives in the tray; do not render it
  // here, otherwise we re-introduce the duplicate Dual-Register block.
  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'probe', probe.probeId, 'en');
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  // WP-001 discourse functions — same canonical order as the lane
  // layout (`/lanes/[probeId]/+layout.svelte`). Labels duplicate the
  // lane chrome rather than fetching from `discourse_function/<key>`
  // because this is navigation chrome, not science-bearing copy.
  const FUNCTIONS: readonly { key: string; label: string; abbr: string }[] = [
    { key: 'epistemic_authority', label: 'Epistemic Authority', abbr: 'EA' },
    { key: 'power_legitimation', label: 'Power Legitimation', abbr: 'PL' },
    { key: 'cohesion_identity', label: 'Cohesion & Identity', abbr: 'CI' },
    { key: 'subversion_friction', label: 'Subversion & Friction', abbr: 'SF' }
  ];
</script>

<section class="l3" aria-label="Probe landing">
  {#if contentQ.data?.kind === 'success'}
    <p class="emic-frame">{contentQ.data.data.registers.semantic.long}</p>
  {:else if contentQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={contentQ.data} {ctx} />
  {:else if contentQ.isPending}
    <p class="muted" aria-busy="true">…</p>
  {/if}

  <dl class="meta">
    <div>
      <dt>Probe</dt>
      <dd><code>{probe.probeId}</code></dd>
    </div>
    <div>
      <dt>Language</dt>
      <dd>{probe.language.toUpperCase()}</dd>
    </div>
    <div>
      <dt>Sources</dt>
      <dd>{probe.sources.length}</dd>
    </div>
    <div>
      <dt>Publication rate</dt>
      <dd>{publicationRate !== null ? `${publicationRate.toFixed(1)} docs/h` : '—'}</dd>
    </div>
  </dl>

  <nav class="jump" aria-label="Open in Surface II">
    <p class="jump-eyebrow">Analyse this probe</p>
    <!-- eslint-disable svelte/no-navigation-without-resolve -- internal Surface II routes -->
    <a class="jump-tile primary" href="/lanes/{probe.probeId}/dossier">
      <span class="tile-title">Probe Dossier</span>
      <span class="tile-sub">Source cards · per-source articles · Silver eligibility</span>
    </a>
    <div class="jump-grid">
      {#each FUNCTIONS as fn (fn.key)}
        <a class="jump-tile" href="/lanes/{probe.probeId}/{fn.key}">
          <span class="tile-abbr" aria-hidden="true">{fn.abbr}</span>
          <span class="tile-title">{fn.label}</span>
          <span class="tile-sub">View-mode matrix · time-series · distribution · network</span>
        </a>
      {/each}
    </div>
    <!-- eslint-enable svelte/no-navigation-without-resolve -->
  </nav>

  <button
    type="button"
    class="meth-link"
    aria-pressed={isTrayOpen}
    aria-label="{isTrayOpen ? 'Close' : 'Open'} methodology tray for this probe"
    onclick={onOpenProvenance}
  >
    {isTrayOpen ? 'Close methodology tray ←' : 'Open methodology tray →'}
  </button>

  <p class="reach-note">
    Reach is not rendered. This probe's emission points mark where its bound publishers emit — not
    where their content is consumed or influential. No reach claim is made by AĒR (Design Brief
    §5.7).
  </p>
</section>

<style>
  .l3 {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    min-height: 100%;
  }

  .emic-frame {
    margin: 0;
    padding: var(--space-3) 0;
    color: var(--color-fg);
    font-size: var(--font-size-base);
    line-height: 1.55;
    border-left: 2px solid var(--color-accent);
    padding-left: var(--space-3);
  }

  dl.meta {
    display: grid;
    grid-template-columns: auto 1fr auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
    font-size: var(--font-size-xs);
  }
  dl.meta > div {
    display: contents;
  }
  dt {
    color: var(--color-fg-muted);
  }
  dd {
    margin: 0;
    color: var(--color-fg);
  }

  .jump {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .jump-eyebrow {
    margin: 0;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .jump-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
    gap: var(--space-3);
  }

  .jump-tile {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3) var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    text-decoration: none;
    color: var(--color-fg);
    transition:
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      background var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .jump-tile:hover,
  .jump-tile:focus-visible {
    border-color: var(--color-accent-muted);
    background: var(--color-bg-elevated);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .jump-tile.primary {
    background: rgba(82, 131, 184, 0.08);
    border-color: var(--color-accent-muted);
  }

  .tile-abbr {
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
    letter-spacing: 0.05em;
  }

  .tile-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
  }

  .tile-sub {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  .meth-link {
    align-self: flex-start;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 4px var(--space-3);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    cursor: pointer;
  }

  .meth-link:hover,
  .meth-link:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .reach-note {
    margin: 0;
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    line-height: 1.55;
  }

  @media (max-width: 720px) {
    dl.meta {
      grid-template-columns: auto 1fr;
    }
  }
</style>

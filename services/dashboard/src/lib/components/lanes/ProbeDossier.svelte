<script lang="ts">
  // Probe Dossier — Surface II's default landing (Design Brief §4.2.1, Phase 106).
  // Renders: function-coverage indicator, per-source cards, article preview.
  import type { ProbeDossierDto, FetchContext } from '$lib/api/queries';
  import SourceCard from './SourceCard.svelte';

  interface Props {
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
  }

  let { dossier, ctx, windowStart, windowEnd }: Props = $props();

  const FUNCTION_META: Record<string, { label: string; abbr: string; color: string }> = {
    epistemic_authority: { label: 'Epistemic Authority', abbr: 'EA', color: '#5283b8' },
    power_legitimation: { label: 'Power Legitimation', abbr: 'PL', color: '#7ec4a0' },
    cohesion_identity: { label: 'Cohesion & Identity', abbr: 'CI', color: '#c8a85a' },
    subversion_friction: { label: 'Subversion & Friction', abbr: 'SF', color: '#9a8fb8' }
  };

  const ALL_FUNCTIONS = Object.keys(FUNCTION_META);

  let coveragePercent = $derived(
    Math.round((dossier.functionCoverage.covered / dossier.functionCoverage.total) * 100)
  );
</script>

<article class="dossier" aria-labelledby="dossier-title">
  <!-- Header: probe identity -->
  <header class="dossier-header">
    <div class="probe-identity">
      <h1 id="dossier-title" class="probe-title">
        <span class="probe-glyph" aria-hidden="true">◎</span>
        {dossier.probeId}
      </h1>
      <span class="lang-badge" aria-label="Primary language: {dossier.language}">
        {dossier.language}
      </span>
    </div>
    {#if dossier.windowStart && dossier.windowEnd}
      <p
        class="window-note"
        aria-label="Article counts cover the window from {dossier.windowStart} to {dossier.windowEnd}"
      >
        Article counts: <time datetime={dossier.windowStart}
          >{new Date(dossier.windowStart).toLocaleDateString('en-CA')}</time
        >
        →
        <time datetime={dossier.windowEnd}
          >{new Date(dossier.windowEnd).toLocaleDateString('en-CA')}</time
        >
      </p>
    {/if}
  </header>

  <!-- Function coverage indicator -->
  <section class="coverage-section" aria-labelledby="coverage-heading">
    <h2 id="coverage-heading" class="section-title">
      Function Coverage
      <span
        class="coverage-fraction"
        aria-label="{dossier.functionCoverage.covered} of {dossier.functionCoverage.total} covered"
      >
        {dossier.functionCoverage.covered}/{dossier.functionCoverage.total}
      </span>
    </h2>
    <div class="coverage-bar" aria-hidden="true">
      <div class="coverage-fill" style:width="{coveragePercent}%"></div>
    </div>
    <ul class="function-pills" role="list" aria-label="Discourse function coverage">
      {#each ALL_FUNCTIONS as fn (fn)}
        {@const meta = FUNCTION_META[fn]}
        {@const covered = dossier.functionCoverage.functions.includes(
          fn as (typeof dossier.functionCoverage.functions)[number]
        )}
        <li>
          <span
            class="fn-pill"
            class:fn-covered={covered}
            style:--fn-color={meta?.color ?? 'var(--color-border-strong)'}
            aria-label="{meta?.label ?? fn}: {covered ? 'covered' : 'not covered'}"
            title={meta?.label ?? fn}
          >
            {meta?.abbr ?? fn}
          </span>
        </li>
      {/each}
    </ul>
  </section>

  <!-- Source cards -->
  <section class="sources-section" aria-labelledby="sources-heading">
    <h2 id="sources-heading" class="section-title">
      Sources
      <span class="source-count">({dossier.sources.length})</span>
    </h2>
    {#if dossier.sources.length === 0}
      <p class="muted">No sources configured for this probe.</p>
    {:else}
      <ul class="source-grid" role="list" aria-label="Sources in this probe">
        {#each dossier.sources as source (source.name)}
          <li>
            <SourceCard {source} {ctx} {windowStart} {windowEnd} />
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</article>

<style>
  .dossier {
    max-width: 72rem;
    display: flex;
    flex-direction: column;
    gap: var(--space-7);
  }

  .dossier-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .probe-identity {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .probe-title {
    font-size: var(--font-size-xl);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-medium);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .probe-glyph {
    color: var(--color-accent);
  }

  .lang-badge {
    display: inline-block;
    padding: 2px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .window-note {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    margin: 0;
  }

  .section-title {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-3) 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .coverage-fraction,
  .source-count {
    font-family: var(--font-mono);
    font-weight: var(--font-weight-regular);
    color: var(--color-fg-muted);
    letter-spacing: 0;
  }

  .coverage-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .coverage-bar {
    height: 3px;
    background: var(--color-border);
    border-radius: var(--radius-pill);
    overflow: hidden;
    width: 100%;
    max-width: 20rem;
  }

  .coverage-fill {
    height: 100%;
    background: var(--color-accent-muted);
    border-radius: var(--radius-pill);
    transition: width var(--motion-duration-base) var(--motion-ease-emphasized);
  }

  .function-pills {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .fn-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 3px var(--space-3);
    border-radius: var(--radius-pill);
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.04em;
    border: 1px solid var(--color-border);
    color: var(--color-fg-subtle);
    background: transparent;
  }

  .fn-pill.fn-covered {
    border-color: var(--fn-color);
    color: var(--fn-color);
    background: color-mix(in srgb, var(--fn-color) 12%, transparent);
  }

  .sources-section {
    display: flex;
    flex-direction: column;
  }

  .source-grid {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(22rem, 1fr));
    gap: var(--space-4);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .coverage-fill {
      transition: none;
    }
  }
</style>

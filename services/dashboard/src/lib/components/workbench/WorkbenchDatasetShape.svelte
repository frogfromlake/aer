<script lang="ts">
  // Phase 122i revision (A4 generalised) — pillar-agnostic dataset-shape
  // strip shown above any Multi-Panel Workbench window. Reports the
  // active panel's resolved scope: probe count, source count, articles
  // in window, language, function coverage.
  //
  // Originally lived inside AlephShell. Hoisted into WindowHost so
  // Aleph / Episteme / Rhizome all surface the same status row — the
  // strip is purely informational and applies regardless of which
  // pillar's analytical lens is active.
  //
  // The probes counter is suppressed on DF-entry locked panels (single
  // probe is implicit; the "Probes: 0" / "Probes: 1" readout would only
  // confuse).
  import type { PillarState } from '$lib/state/url-internals';
  import type { ProbeDossierDto } from '$lib/api/queries';

  interface Props {
    pillarState: PillarState;
    dossier: ProbeDossierDto;
  }

  let { pillarState, dossier }: Props = $props();

  const shape = $derived.by(() => {
    const win = pillarState.windows[pillarState.activeWindowIndex] ?? pillarState.windows[0];
    const focused = win?.panels[win.focusedPanelIndex] ?? win?.panels[0] ?? null;
    if (!focused) return null;
    const probeList: string[] = [];
    const sourceList: string[] = [];
    let anyEmptySourceList = false;
    for (const g of focused.scopes) {
      for (const p of g.probeIds) if (!probeList.includes(p)) probeList.push(p);
      if (g.sourceIds.length === 0) anyEmptySourceList = true;
      else for (const s of g.sourceIds) if (!sourceList.includes(s)) sourceList.push(s);
    }
    const resolvedSources = anyEmptySourceList
      ? dossier.sources
      : dossier.sources.filter((s) => sourceList.includes(s.name));
    const articleCount = resolvedSources.reduce((sum, s) => sum + (s.articlesInWindow ?? 0), 0);
    return {
      probes: focused.locked === true ? null : probeList.length,
      sources: resolvedSources.length,
      articlesInWindow: articleCount,
      language: dossier.language,
      coverage: dossier.functionCoverage
    };
  });
</script>

{#if shape}
  <header class="dataset-shape" aria-label="What AĒR sees right now">
    {#if shape.probes !== null}
      <div class="shape-item">
        <span class="shape-label">Probes</span>
        <span class="shape-value">{shape.probes}</span>
      </div>
    {/if}
    <div class="shape-item">
      <span class="shape-label">Sources</span>
      <span class="shape-value">{shape.sources}</span>
    </div>
    <div class="shape-item">
      <span class="shape-label">Articles in window</span>
      <span class="shape-value">{shape.articlesInWindow.toLocaleString('en-US')}</span>
    </div>
    <div class="shape-item">
      <span class="shape-label">Language</span>
      <span class="shape-value">{shape.language.toUpperCase()}</span>
    </div>
    <div class="shape-item">
      <span class="shape-label">Function coverage</span>
      <span class="shape-value">
        {shape.coverage.covered}/{shape.coverage.total}
      </span>
    </div>
  </header>
{/if}

<style>
  .dataset-shape {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
    padding: var(--space-2) var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .shape-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 5rem;
  }

  .shape-label {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .shape-value {
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    font-family: var(--font-mono);
  }
</style>

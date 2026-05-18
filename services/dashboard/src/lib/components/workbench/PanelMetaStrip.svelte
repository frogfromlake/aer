<script lang="ts">
  // PanelMetaStrip — Phase 122k §14b finding 2.
  //
  // Per-panel compact metadata strip mounted ABOVE PanelControls. Shows
  // the panel's resolved scope: probes / sources / articles-in-window /
  // language / function coverage / DF lock. Replaces the previous
  // panel-wide WorkbenchScopeBar + WorkbenchDatasetShape that hovered
  // OUTSIDE the panel raster — those carried only the focused-panel's
  // numbers but rendered globally, which the user noted is confusing
  // when working with multiple panels.
  //
  // Collapsed (default): one-line summary strip.
  // Expanded: shows probe chips + source chips + DF lock chip with
  // remove affordances. The strip itself is collapsible via header click
  // so vertical real estate stays tight inside narrow panels.
  import type { Panel } from '$lib/state/url-internals';
  import type { ProbeDossierDto } from '$lib/api/queries';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { getFunctionDef } from '$lib/discourse-function';
  import FunctionBadge from '$lib/components/base/FunctionBadge.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    panelPath: PanelPath;
  }

  let { panel, dossier, panelPath }: Props = $props();

  let expanded = $state(false);

  const shape = $derived.by(() => {
    const probeList: string[] = [];
    const sourceList: string[] = [];
    let anyEmptySourceList = false;
    for (const g of panel.scopes) {
      for (const p of g.probeIds) if (!probeList.includes(p)) probeList.push(p);
      if (g.sourceIds.length === 0) anyEmptySourceList = true;
      else for (const s of g.sourceIds) if (!sourceList.includes(s)) sourceList.push(s);
    }
    const resolvedSources = anyEmptySourceList
      ? dossier.sources
      : dossier.sources.filter((s) => sourceList.includes(s.name));
    const articleCount = resolvedSources.reduce((sum, s) => sum + (s.articlesInWindow ?? 0), 0);
    return {
      probes: probeList,
      sources: resolvedSources.map((s) => s.name),
      articlesInWindow: articleCount,
      language: dossier.language,
      coverage: dossier.functionCoverage,
      lockedFunction: panel.lockedFunction ?? null
    };
  });

  const lockedFnMeta = $derived(shape.lockedFunction ? getFunctionDef(shape.lockedFunction) : null);

  function toggleExpanded() {
    expanded = !expanded;
  }

  function removeProbe(probeId: string) {
    const remainingScopes = panel.scopes
      .map((g) => ({
        probeIds: g.probeIds.filter((p) => p !== probeId),
        sourceIds: g.sourceIds
      }))
      .filter((g) => g.probeIds.length > 0);
    if (remainingScopes.length === 0) {
      // Removing the last probe would leave the panel scopeless. Treat
      // it as a group-removal: keep the panel structurally but reset
      // the scope to a single empty group so the user can refill via
      // the ScopeEditor.
      updatePanel(panelPath, (p) => ({
        ...p,
        scopes: [{ probeIds: [], sourceIds: [] }]
      }));
      return;
    }
    updatePanel(panelPath, (p) => ({ ...p, scopes: remainingScopes }));
  }

  function removeSource(sourceName: string) {
    updatePanel(panelPath, (p) => ({
      ...p,
      scopes: p.scopes.map((g) => ({
        probeIds: g.probeIds,
        sourceIds: g.sourceIds.filter((s) => s !== sourceName)
      }))
    }));
  }

  function clearLockedFunction() {
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      delete out.locked;
      delete out.lockedFunction;
      delete out.lockedReason;
      return out;
    });
  }
</script>

<section class="meta-strip" class:expanded aria-label="Panel metadata">
  <button
    type="button"
    class="meta-toggle"
    aria-expanded={expanded}
    aria-controls={`meta-body-${panelPath.windowIndex}-${panelPath.panelIndex}`}
    onclick={toggleExpanded}
    onkeydown={(e) => e.stopPropagation()}
  >
    <span class="meta-chevron" class:expanded aria-hidden="true">›</span>
    <span class="meta-item">
      <span class="meta-label">Probes</span>
      <span class="meta-value">{shape.probes.length}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">Sources</span>
      <span class="meta-value">{shape.sources.length}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">Articles</span>
      <span class="meta-value">{shape.articlesInWindow.toLocaleString('en-US')}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">Lang</span>
      <span class="meta-value">{shape.language.toUpperCase()}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">DF coverage</span>
      <span class="meta-value">{shape.coverage.covered}/{shape.coverage.total}</span>
    </span>
    {#if lockedFnMeta}
      <span class="meta-locked">
        🔒 <FunctionBadge function={lockedFnMeta.key} size="sm" showLabel />
      </span>
    {/if}
  </button>

  {#if expanded}
    <div
      class="meta-body"
      id={`meta-body-${panelPath.windowIndex}-${panelPath.panelIndex}`}
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      role="presentation"
    >
      <div class="chip-row">
        <span class="chip-eyebrow">Probes</span>
        {#if shape.probes.length === 0}
          <span class="muted">none</span>
        {:else}
          <ul class="chips" role="list">
            {#each shape.probes as id (id)}
              <li>
                <button
                  type="button"
                  class="chip"
                  onclick={() => removeProbe(id)}
                  aria-label="Remove probe {id}"
                  title="Remove probe from this panel"
                >
                  {id}
                  <span class="chip-x" aria-hidden="true">×</span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <div class="chip-row">
        <span class="chip-eyebrow">Sources</span>
        {#if shape.sources.length === 0}
          <span class="muted">whole probe (all sources)</span>
        {:else}
          <ul class="chips" role="list">
            {#each shape.sources as name (name)}
              <li>
                <button
                  type="button"
                  class="chip"
                  onclick={() => removeSource(name)}
                  aria-label="Remove source {name}"
                  title="Remove source from this panel"
                >
                  {name}
                  <span class="chip-x" aria-hidden="true">×</span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      {#if lockedFnMeta}
        <div class="chip-row">
          <span class="chip-eyebrow">DF lock</span>
          <button
            type="button"
            class="chip chip-lock"
            onclick={clearLockedFunction}
            aria-label="Clear discourse function lock"
            title="Clear the discourse-function lock for this panel"
          >
            🔒 {lockedFnMeta.label}
            <span class="chip-x" aria-hidden="true">×</span>
          </button>
        </div>
      {/if}
    </div>
  {/if}
</section>

<style>
  .meta-strip {
    display: flex;
    flex-direction: column;
    border: 1px solid color-mix(in srgb, var(--color-border) 50%, transparent);
    border-radius: var(--radius-sm);
    background: var(--color-bg);
  }

  .meta-toggle {
    appearance: none;
    background: transparent;
    border: none;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-1) var(--space-3);
    text-align: left;
    flex-wrap: wrap;
  }
  .meta-toggle:hover,
  .meta-toggle:focus-visible {
    background: color-mix(in srgb, var(--color-fg) 4%, transparent);
  }

  .meta-chevron {
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }
  .meta-chevron.expanded {
    transform: rotate(90deg);
  }

  .meta-item {
    display: flex;
    align-items: baseline;
    gap: var(--space-1);
    font-family: var(--font-mono);
  }

  .meta-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .meta-value {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    font-weight: var(--font-weight-semibold);
  }

  .meta-locked {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--font-size-xs);
  }

  .meta-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-top: 1px dashed color-mix(in srgb, var(--color-border) 50%, transparent);
  }

  .chip-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .chip-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    min-width: 5rem;
  }

  .chips {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .chip {
    appearance: none;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px var(--space-2);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .chip:hover,
  .chip:focus-visible {
    border-color: #d97a7a;
    color: #d97a7a;
  }
  .chip-x {
    font-weight: 700;
    color: var(--color-fg-subtle);
  }
  .chip:hover .chip-x,
  .chip:focus-visible .chip-x {
    color: #d97a7a;
  }

  .chip-lock {
    border-color: color-mix(in srgb, var(--color-accent) 40%, var(--color-border));
  }

  .muted {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-style: italic;
  }
</style>

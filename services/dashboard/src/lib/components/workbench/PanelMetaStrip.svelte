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
  import type { ProbeDossierDto, ProbeDto, FetchContext, QueryOutcome } from '$lib/api/queries';
  import { probesQuery } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';
  import { formatNumber } from '$lib/localization/format';
  import { createQuery } from '@tanstack/svelte-query';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { getFunctionDef } from '$lib/discourse-function';
  import FunctionBadge from '$lib/components/base/FunctionBadge.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    panelPath: PanelPath;
    /** TanStack fetch context — used to resolve probe display names so the
     *  scope chips never show the raw machine id (Phase 123c). */
    ctx: FetchContext;
    /** Re-open the ScopeEditor — invoked when a quick-remove empties the scope
     *  (all probes or all sources gone), so the panel never ends up scopeless. */
    onEditScope?: () => void;
  }

  let { panel, dossier, panelPath, ctx, onEditScope }: Props = $props();

  // Phase 123c — expanded by default (TESTING.md Issue 4.2): a freshly composed
  // panel should reveal its probes/sources scope chips, not hide them behind a
  // collapsed strip. The user can still collapse it to reclaim vertical space.
  let expanded = $state(true);

  // Phase 123c — probe display-name resolution for the scope chips. Falls back
  // to the raw id while the list loads or for an unknown probe.
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);
  function probeLabel(probeId: string): string {
    return probeList.find((p) => p.probeId === probeId)?.displayName ?? probeId;
  }

  // Phase 123c (B) parity — the threaded `dossier` covers ONLY one probe, so its
  // `sources` list is just that probe's. A multi-probe panel must resolve every
  // in-scope probe's sources from the probe registry (already cached), mirroring
  // PanelHost.sourceNamesForProbe — otherwise the strip shows only the first
  // probe's sources while the cells correctly fan out all of them.
  function probeSources(probeId: string): string[] {
    if (probeId === dossier.probeId) return dossier.sources.map((s) => s.name);
    return probeList.find((p) => p.probeId === probeId)?.sources ?? [];
  }

  const shape = $derived.by(() => {
    const probeIds: string[] = [];
    const sourceNames: string[] = [];
    const addSource = (s: string) => {
      if (!sourceNames.includes(s)) sourceNames.push(s);
    };
    for (const g of panel.scopes) {
      for (const p of g.probeIds) if (!probeIds.includes(p)) probeIds.push(p);
      // Empty sourceIds = "whole probe" → expand to every source of the group's
      // probes; an explicit list narrows to exactly those.
      if (g.sourceIds.length === 0) {
        for (const p of g.probeIds) for (const s of probeSources(p)) addSource(s);
      } else {
        for (const s of g.sourceIds) addSource(s);
      }
    }
    // Article counts are only known for the threaded dossier's probe — sum where
    // available (single-probe panels are unaffected; multi-probe is best-effort).
    const articleCount = dossier.sources
      .filter((s) => sourceNames.includes(s.name))
      .reduce((sum, s) => sum + (s.articlesInWindow ?? 0), 0);
    return {
      probes: probeIds,
      sources: sourceNames,
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
      // Removing the last probe leaves the panel scopeless — re-open the
      // ScopeEditor instead of committing an empty scope (the panel keeps its
      // current scope until the user applies a new one).
      onEditScope?.();
      return;
    }
    updatePanel(panelPath, (p) => ({ ...p, scopes: remainingScopes }));
  }

  function removeSource(sourceName: string) {
    // The chips show the RESOLVED sources: a group with an empty `sourceIds`
    // means "whole probe (all sources)", so removing a chip must first
    // materialise that group to its probes' full source set — across ALL the
    // group's probes, not just the dossier probe — then drop the source.
    let anyRemaining = false;
    const nextScopes = panel.scopes.map((g) => {
      const effective =
        g.sourceIds.length > 0 ? g.sourceIds : [...new Set(g.probeIds.flatMap(probeSources))];
      if (!effective.includes(sourceName)) {
        if (effective.length > 0) anyRemaining = true;
        return g;
      }
      const sourceIds = effective.filter((s) => s !== sourceName);
      if (sourceIds.length > 0) anyRemaining = true;
      return { probeIds: g.probeIds, sourceIds };
    });
    if (!anyRemaining) {
      // All sources removed → re-open the ScopeEditor (keep current scope).
      onEditScope?.();
      return;
    }
    updatePanel(panelPath, (p) => ({ ...p, scopes: nextScopes }));
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

<section class="meta-strip" class:expanded aria-label={m.workbench_meta_aria_label()}>
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
      <span class="meta-label">{m.workbench_meta_probes()}</span>
      <span class="meta-value">{shape.probes.length}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">{m.workbench_meta_sources()}</span>
      <span class="meta-value">{shape.sources.length}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">{m.workbench_meta_articles()}</span>
      <span class="meta-value">{formatNumber(shape.articlesInWindow)}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">{m.workbench_meta_lang()}</span>
      <span class="meta-value">{shape.language.toUpperCase()}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">{m.workbench_meta_df_coverage()}</span>
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
        <span class="chip-eyebrow">{m.workbench_meta_chip_probes()}</span>
        {#if shape.probes.length === 0}
          <span class="muted">{m.workbench_meta_chip_probes_none()}</span>
        {:else}
          <ul class="chips" role="list">
            {#each shape.probes as id (id)}
              <li>
                <button
                  type="button"
                  class="chip"
                  onclick={() => removeProbe(id)}
                  aria-label={m.workbench_meta_remove_probe_label({ probe: probeLabel(id) })}
                  title={m.workbench_meta_remove_probe_title()}
                >
                  {probeLabel(id)}
                  <span class="chip-x" aria-hidden="true">×</span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <div class="chip-row">
        <span class="chip-eyebrow">{m.workbench_meta_chip_sources()}</span>
        {#if shape.sources.length === 0}
          <span class="muted">{m.workbench_meta_chip_sources_whole_probe()}</span>
        {:else}
          <ul class="chips" role="list">
            {#each shape.sources as name (name)}
              <li>
                <button
                  type="button"
                  class="chip"
                  onclick={() => removeSource(name)}
                  aria-label={m.workbench_meta_remove_source_label({ name })}
                  title={m.workbench_meta_remove_source_title()}
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
          <span class="chip-eyebrow">{m.workbench_meta_chip_df_lock()}</span>
          <button
            type="button"
            class="chip chip-lock"
            onclick={clearLockedFunction}
            aria-label={m.workbench_meta_clear_df_lock_label()}
            title={m.workbench_meta_clear_df_lock_title()}
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

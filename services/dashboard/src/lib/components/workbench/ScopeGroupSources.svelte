<script lang="ts">
  // ScopeGroupSources — Phase 141 decomposition of ScopeEditor (step 3).
  //
  // The source-selection sub-surface of one ScopeGroup card: per selected
  // probe, the Select-all / Clear-all actions and the DF-aware source rows
  // (matching rows selectable, non-matching rows dimmed + disabled). Split
  // out of ScopeGroupCard so each stays under the file-length cap. Controlled
  // presentation child — the ScopeEditor owns the draft state; this reports
  // intent through callbacks.
  import { m } from '$lib/paraglide/messages.js';
  import type { ScopeGroup } from '$lib/state/url-internals';
  import type { ProbeDto } from '$lib/api/queries';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import {
    DISCOURSE_FUNCTIONS,
    sourceMatchesDf,
    type DossierSource
  } from '$lib/workbench/scope-editor-internals';

  interface Props {
    group: ScopeGroup;
    /** Effective DF lock for the group (null = all functions). */
    lock: DiscourseFunction | null;
    probeList: ProbeDto[];
    /** Resolves a probe's cached sources (`[]` while its dossier loads). */
    sourcesForProbe: (probeId: string) => DossierSource[];
    sourcesLoading: (probeId: string) => boolean;
    onToggleSource: (sourceName: string) => void;
    onSelectAll: (probeId: string) => void;
    onClearAll: (probeId: string) => void;
  }

  let {
    group,
    lock,
    probeList,
    sourcesForProbe,
    sourcesLoading,
    onToggleSource,
    onSelectAll,
    onClearAll
  }: Props = $props();

  const lockMeta = $derived(lock ? DISCOURSE_FUNCTIONS.find((d) => d.id === lock) : null);

  // Phase 148e — per-probe source sections are EXPANDED by default; each is
  // independently collapsible, with a group-wide expand-all / collapse-all, and
  // the whole step folds away too. Controlled open-state per probe (only the
  // explicit overrides are stored; an absent entry means "expanded") so the
  // group toggle can drive them all.
  let expanded = $state<Record<string, boolean>>({});
  let stepOpen = $state(true);

  function isExpanded(probeId: string): boolean {
    return expanded[probeId] !== false; // expanded unless explicitly collapsed
  }
  function setExpanded(probeId: string, open: boolean) {
    if (isExpanded(probeId) === open) return; // guard a redundant write / loop
    expanded = { ...expanded, [probeId]: open };
  }
  // "Fully expanded" means the section is open AND every probe is expanded, so
  // the button reads/acts correctly from a fully-collapsed section: one click
  // opens the section AND expands all its probes.
  const allExpanded = $derived(
    stepOpen && group.probeIds.length > 0 && group.probeIds.every((p) => isExpanded(p))
  );
  function toggleAll() {
    if (allExpanded) {
      // Collapse EVERYTHING: every probe (explicit false) + the whole section.
      const next: Record<string, boolean> = {};
      for (const p of group.probeIds) next[p] = false;
      expanded = next;
      stepOpen = false;
    } else {
      // Expand everything: open the section, clear the overrides (absent =
      // expanded), so every probe is open.
      expanded = {};
      stepOpen = true;
    }
  }
</script>

<details class="step" data-step="3" bind:open={stepOpen}>
  <summary class="step-header" title={m.workbench_scope_sources_section_toggle()}>
    <span class="step-num" aria-hidden="true">3</span>
    <h3 class="step-title">{m.workbench_scope_sources_step_title()}</h3>
    <span class="step-hint">{m.workbench_scope_sources_step_hint()}</span>
    {#if group.probeIds.length > 0}
      <button
        type="button"
        class="expand-all"
        onclick={(e) => {
          e.preventDefault();
          toggleAll();
        }}
      >
        {allExpanded
          ? m.workbench_scope_sources_collapse_all()
          : m.workbench_scope_sources_expand_all()}
      </button>
    {/if}
    <span class="step-chevron" aria-hidden="true">›</span>
  </summary>
  <div class="step-body">
    {#if group.probeIds.length === 0}
      <p class="muted-large">{m.workbench_scope_sources_pick_probe()}</p>
    {:else}
      {#each group.probeIds as probeId (probeId)}
        {@const probeSources = sourcesForProbe(probeId)}
        {@const probeLabel = probeList.find((p) => p.probeId === probeId)?.displayName ?? probeId}
        {@const selectedCount = probeSources.filter((s) => group.sourceIds.includes(s.name)).length}
        <details
          class="source-section"
          open={isExpanded(probeId)}
          ontoggle={(e) => setExpanded(probeId, (e.currentTarget as HTMLDetailsElement).open)}
        >
          <summary class="source-summary">
            <span class="probe-section-label">{probeLabel}</span>
            {#if !isExpanded(probeId)}
              <span class="expand-hint">{m.workbench_scope_sources_expand_hint()}</span>
            {/if}
            <span class="summary-right">
              {#if probeSources.length > 0}
                <span class="source-count">{selectedCount}/{probeSources.length}</span>
                <span class="source-actions">
                  <button
                    type="button"
                    class="source-action"
                    onclick={(e) => {
                      e.preventDefault();
                      onSelectAll(probeId);
                    }}
                    title={m.workbench_scope_sources_select_all_title()}
                  >
                    {m.workbench_scope_sources_select_all()}
                  </button>
                  <button
                    type="button"
                    class="source-action"
                    onclick={(e) => {
                      e.preventDefault();
                      onClearAll(probeId);
                    }}
                    title={m.workbench_scope_sources_clear_all_title()}
                  >
                    {m.workbench_scope_sources_clear_all()}
                  </button>
                </span>
              {/if}
              <span class="summary-chevron" aria-hidden="true">›</span>
            </span>
          </summary>
          {#if probeSources.length === 0}
            {#if sourcesLoading(probeId)}
              <p class="muted" aria-busy="true">
                {m.workbench_scope_sources_loading({ probe: probeLabel })}
              </p>
            {:else}
              <p class="muted">{m.workbench_scope_sources_empty({ probe: probeLabel })}</p>
            {/if}
          {:else}
            <ul class="source-list" role="list">
              {#each probeSources as source (source.name)}
                {@const checked = group.sourceIds.includes(source.name)}
                {@const dim = !sourceMatchesDf(source, lock)}
                <li>
                  <label class="source-row" class:checked class:dim>
                    <input
                      type="checkbox"
                      {checked}
                      disabled={dim}
                      onchange={() => onToggleSource(source.name)}
                      aria-label={m.workbench_scope_sources_include({
                        name: source.emicDesignation ?? source.name
                      })}
                    />
                    <span class="source-label">
                      <span class="source-name">
                        {source.emicDesignation ?? source.name}
                      </span>
                      {#if source.emicDesignation && source.emicDesignation !== source.name}
                        <code class="source-id">{source.name}</code>
                      {/if}
                      {#if source.primaryFunction}
                        {@const fnMeta = DISCOURSE_FUNCTIONS.find(
                          (d) => d.id === source.primaryFunction
                        )}
                        <span
                          class="source-df-tag"
                          style:--tag-color={fnMeta?.color ?? 'var(--color-fg-subtle)'}
                          title={m.workbench_scope_sources_df_tag_title()}
                        >
                          {fnMeta?.label() ?? ''}
                        </span>
                      {/if}
                    </span>
                    {#if dim}
                      <span class="source-dim-hint"
                        >{m.workbench_scope_sources_not_matching({
                          function: lockMeta?.label() ?? ''
                        })}</span
                      >
                    {/if}
                  </label>
                </li>
              {/each}
            </ul>
          {/if}
        </details>
      {/each}
    {/if}
  </div>
</details>

<style>
  /* Step shell — duplicated from ScopeGroupCard so this section stands alone
     (Svelte scopes <style> per-component). */
  .step {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    border-left: 2px solid color-mix(in srgb, #a3c984 50%, var(--color-border));
  }

  /* The step header is now the <summary> that folds the whole sources section. */
  .step-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
    cursor: pointer;
    list-style: none;
  }
  .step-header::-webkit-details-marker {
    display: none;
  }
  .step-header:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-radius: var(--radius-sm);
  }
  /* Expand-all / collapse-all of the per-probe sections + the section chevron,
     pushed to the right edge of the header. */
  .expand-all {
    margin-left: auto;
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 3px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .expand-all:hover,
  .expand-all:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  .step-chevron {
    margin-left: auto;
    color: var(--color-fg-subtle);
    font-size: 1.15rem;
    line-height: 1;
    align-self: center;
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .step[open] > .step-header .step-chevron {
    transform: rotate(90deg);
  }
  .step-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  @media (prefers-reduced-motion: reduce) {
    .step-chevron {
      transition: none;
    }
  }

  /* "click to expand" affordance after a collapsed probe's name. */
  .expand-hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-style: italic;
  }
  /* The count + per-probe actions + chevron cluster, pushed right. */
  .summary-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .step-num {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    height: 1.5rem;
    border-radius: 999px;
    background: var(--color-bg-elevated);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-weight: 700;
    font-size: var(--font-size-sm);
    border: 1px solid var(--color-border);
    flex-shrink: 0;
  }

  .step-title {
    margin: 0;
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }

  .step-hint {
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  .source-section {
    padding-top: var(--space-2);
  }
  .source-section + .source-section {
    border-top: 1px dashed var(--color-border);
    margin-top: var(--space-2);
    padding-top: var(--space-3);
  }

  /* Collapsible per-probe summary: probe label (left) + count + actions +
     chevron (right). */
  .source-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    cursor: pointer;
    list-style: none;
    padding: 2px 0;
  }
  .source-summary::-webkit-details-marker {
    display: none;
  }
  .source-summary:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-radius: var(--radius-sm);
  }
  .source-section[open] .source-list {
    margin-top: var(--space-2);
  }

  .probe-section-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    background: color-mix(in srgb, #a3c984 14%, var(--color-bg-elevated));
    border: 1px solid color-mix(in srgb, #a3c984 40%, transparent);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
  }

  .source-count {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .summary-chevron {
    color: var(--color-fg-subtle);
    font-size: 1.1rem;
    line-height: 1;
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .source-section[open] .summary-chevron {
    transform: rotate(90deg);
  }
  @media (prefers-reduced-motion: reduce) {
    .summary-chevron {
      transition: none;
    }
  }

  .source-actions {
    display: flex;
    gap: var(--space-2);
  }

  .source-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 3px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .source-action:hover,
  .source-action:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .source-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .source-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .source-row:hover {
    background: var(--color-bg-elevated);
  }
  .source-row.checked {
    background: color-mix(in srgb, #a3c984 12%, var(--color-surface));
  }
  .source-row.dim {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .source-row.dim:hover {
    background: transparent;
  }
  .source-row input {
    accent-color: #a3c984;
  }

  .source-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1 1 auto;
    flex-wrap: wrap;
  }

  .source-name {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .source-df-tag {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 6px;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--tag-color, var(--color-fg-subtle)) 14%, transparent);
    color: var(--tag-color, var(--color-fg-subtle));
    border: 1px solid color-mix(in srgb, var(--tag-color, var(--color-border)) 40%, transparent);
  }

  .source-dim-hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  .muted {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
  }
  .muted-large {
    margin: 0;
    font-size: var(--font-size-md);
    color: var(--color-fg-subtle);
    font-style: italic;
    padding: var(--space-2) 0;
  }
</style>

<script lang="ts">
  // Phase 141 — multi-group scope chips, extracted from PanelHost.svelte. Lists
  // a Panel's ScopeGroups (probes · sources) when it carries more than one, with
  // a per-group remove affordance. Panel-bound: it owns its remove handler via
  // the resolved PanelPath. Renders nothing for a single-group panel.
  import { m } from '$lib/paraglide/messages.js';
  import type { Panel } from '$lib/state/url-internals';
  import { removeScopeGroup, type PanelPath } from '$lib/workbench/panel-mutators';

  interface Props {
    panel: Panel;
    panelPath: PanelPath;
  }

  let { panel, panelPath }: Props = $props();

  function onRemoveGroup(groupIndex: number) {
    removeScopeGroup(panelPath, groupIndex);
  }
</script>

{#if panel.scopes.length > 1}
  <ul class="scope-groups" role="list" aria-label={m.workbench_scope_chips_aria_label()}>
    {#each panel.scopes as group, i (i)}
      <li class="scope-group-chip">
        <span class="scope-group-eyebrow">{m.workbench_scope_chip_group({ index: i + 1 })}</span>
        <span class="scope-group-detail">
          {group.probeIds.join(', ') || '—'}
          {#if group.sourceIds.length > 0}
            · {group.sourceIds.join(', ')}
          {/if}
        </span>
        {#if !panel.locked}
          <button
            type="button"
            class="scope-group-remove"
            onclick={(e) => {
              e.stopPropagation();
              onRemoveGroup(i);
            }}
            title={m.workbench_scope_chip_remove_title()}
            aria-label={m.workbench_scope_chip_remove_label({ index: i + 1 })}
          >
            ×
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .scope-groups {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .scope-group-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 2px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
  }

  .scope-group-eyebrow {
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .scope-group-detail {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }

  .scope-group-remove {
    appearance: none;
    background: transparent;
    border: none;
    color: var(--color-fg-subtle);
    cursor: pointer;
    font-size: var(--font-size-sm);
    line-height: 1;
    padding: 0 4px;
  }

  .scope-group-remove:hover,
  .scope-group-remove:focus-visible {
    color: var(--color-status-expired);
  }
</style>

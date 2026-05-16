<script lang="ts">
  // Phase 122i revision (D3) — ScopeEditor popover.
  //
  // Opens from the `+ Compare` button in PanelHost (or from a per-group
  // "edit" affordance once we wire that up). Lets the user define the
  // ScopeGroups of a Panel: which probes and which sources each group
  // covers. Today's production runs a single probe, so the probe-picker
  // is effectively a no-op preset; the source-multiselect is the live
  // affordance. When Phase-123 lands the multi-probe corpus, the probe-
  // picker becomes interactive without any further UI work.
  //
  // The Editor is informational about lock state: when the focused
  // panel is locked, all edits are disabled and a footer note explains
  // the user must return to the Dossier to recombine.
  import type { Panel, ScopeGroup } from '$lib/state/url-internals';
  import type { ProbeDossierDto } from '$lib/api/queries';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';

  interface Props {
    panelPath: PanelPath;
    panel: Panel;
    dossier: ProbeDossierDto;
    onClose: () => void;
  }

  let { panelPath, panel, dossier, onClose }: Props = $props();

  const locked = $derived(panel.locked === true);

  function patchScopeGroup(index: number, next: ScopeGroup) {
    updatePanel(panelPath, (p) => {
      const scopes = p.scopes.slice();
      scopes[index] = next;
      return { ...p, scopes };
    });
  }

  function addEmptyGroup() {
    updatePanel(panelPath, (p) => ({
      ...p,
      scopes: [...p.scopes, { probeIds: [dossier.probeId], sourceIds: [] }]
    }));
  }

  function removeGroup(index: number) {
    updatePanel(panelPath, (p) => {
      if (p.scopes.length <= 1) return p;
      const scopes = p.scopes.slice();
      scopes.splice(index, 1);
      return { ...p, scopes };
    });
  }

  function toggleSource(groupIndex: number, sourceName: string) {
    const group = panel.scopes[groupIndex];
    if (!group) return;
    const next: ScopeGroup = {
      probeIds: [...group.probeIds],
      sourceIds: group.sourceIds.includes(sourceName)
        ? group.sourceIds.filter((s) => s !== sourceName)
        : [...group.sourceIds, sourceName]
    };
    patchScopeGroup(groupIndex, next);
  }

  function clearGroupSources(groupIndex: number) {
    const group = panel.scopes[groupIndex];
    if (!group) return;
    patchScopeGroup(groupIndex, { probeIds: [...group.probeIds], sourceIds: [] });
  }

  function selectAllSources(groupIndex: number) {
    const group = panel.scopes[groupIndex];
    if (!group) return;
    patchScopeGroup(groupIndex, {
      probeIds: [...group.probeIds],
      sourceIds: dossier.sources.map((s) => s.name)
    });
  }
</script>

<div
  class="scope-editor-backdrop"
  role="presentation"
  onclick={onClose}
  onkeydown={(e) => {
    if (e.key === 'Escape') onClose();
  }}
>
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section
    class="scope-editor"
    role="dialog"
    aria-modal="true"
    aria-label="Edit panel scope"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => e.stopPropagation()}
  >
    <header class="editor-header">
      <h2>Edit scope</h2>
      <button type="button" class="close-btn" onclick={onClose} aria-label="Close scope editor">
        ×
      </button>
    </header>

    {#if locked}
      <p class="locked-note" role="status">
        🔒 This panel's scope is locked to <strong
          >{panel.lockedFunction ?? 'a discourse function'}</strong
        >. Return to the Probe Dossier to compose freely.
      </p>
    {/if}

    <div class="groups">
      {#each panel.scopes as group, groupIndex (groupIndex)}
        <article class="group" aria-label="Scope group {groupIndex + 1}">
          <header class="group-header">
            <span class="group-eyebrow">Group {groupIndex + 1}</span>
            <span class="group-probes">{group.probeIds.join(', ') || '—'}</span>
            <div class="group-actions">
              <button
                type="button"
                class="group-action"
                onclick={() => selectAllSources(groupIndex)}
                disabled={locked || group.sourceIds.length === dossier.sources.length}
                title="Include all sources of this probe"
              >
                All
              </button>
              <button
                type="button"
                class="group-action"
                onclick={() => clearGroupSources(groupIndex)}
                disabled={locked || group.sourceIds.length === 0}
                title="Whole-probe scope (no source narrowing)"
              >
                None
              </button>
              {#if panel.scopes.length > 1}
                <button
                  type="button"
                  class="group-action group-action-remove"
                  onclick={() => removeGroup(groupIndex)}
                  disabled={locked}
                  title="Remove this scope group"
                >
                  ×
                </button>
              {/if}
            </div>
          </header>

          <ul class="source-list" role="list">
            {#each dossier.sources as source (source.name)}
              {@const checked = group.sourceIds.includes(source.name)}
              <li>
                <label class="source-row" class:checked class:disabled={locked}>
                  <input
                    type="checkbox"
                    {checked}
                    disabled={locked}
                    onchange={() => toggleSource(groupIndex, source.name)}
                    aria-label="Include {source.emicDesignation ?? source.name}"
                  />
                  <span class="source-label">
                    <span class="source-name">{source.emicDesignation ?? source.name}</span>
                    {#if source.emicDesignation && source.emicDesignation !== source.name}
                      <code class="source-id">{source.name}</code>
                    {/if}
                  </span>
                </label>
              </li>
            {/each}
          </ul>

          {#if group.sourceIds.length === 0}
            <p class="group-hint">Whole probe (all sources unioned).</p>
          {:else}
            <p class="group-hint">
              {group.sourceIds.length} source{group.sourceIds.length === 1 ? '' : 's'} selected.
            </p>
          {/if}
        </article>
      {/each}
    </div>

    <footer class="editor-footer">
      <button type="button" class="add-group" onclick={addEmptyGroup} disabled={locked}>
        ＋ Add scope group
      </button>
      <button type="button" class="done" onclick={onClose}>Done</button>
    </footer>
  </section>
</div>

<style>
  .scope-editor-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--color-bg) 75%, transparent);
    backdrop-filter: blur(2px);
    z-index: 50;
    display: grid;
    place-items: center;
    padding: var(--space-4);
  }

  .scope-editor {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    width: min(48rem, 100%);
    max-height: 90vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
  }

  .editor-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }

  .editor-header h2 {
    margin: 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
  }

  .close-btn {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    width: 2rem;
    height: 2rem;
    color: var(--color-fg);
    cursor: pointer;
    font-size: var(--font-size-md);
  }

  .close-btn:hover,
  .close-btn:focus-visible {
    background: color-mix(in srgb, var(--color-status-expired) 14%, var(--color-surface));
    border-color: var(--color-status-expired);
    color: var(--color-status-expired);
    outline: none;
  }

  .locked-note {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface);
    border-left: 3px solid var(--color-accent);
    border-radius: var(--radius-sm);
    margin: 0;
  }

  .groups {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .group {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-3);
    background: var(--color-surface);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .group-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .group-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .group-probes {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .group-actions {
    margin-left: auto;
    display: flex;
    gap: var(--space-1);
  }

  .group-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .group-action:hover:not(:disabled),
  .group-action:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .group-action:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .group-action-remove {
    color: var(--color-status-expired);
  }

  .source-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: var(--space-1);
  }

  .source-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    background: var(--color-bg-elevated);
  }

  .source-row.checked {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .source-row.disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .source-label {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .source-name {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }

  .group-hint {
    margin: 0;
    font-size: 10px;
    font-style: italic;
    color: var(--color-fg-subtle);
  }

  .editor-footer {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-3);
  }

  .add-group {
    appearance: none;
    background: transparent;
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-3);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }

  .add-group:hover:not(:disabled),
  .add-group:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    border-style: solid;
    border-color: var(--color-accent);
  }

  .add-group:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .done {
    appearance: none;
    background: var(--color-accent);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-4);
    color: var(--color-bg);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    cursor: pointer;
  }

  .done:hover,
  .done:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 80%, var(--color-fg));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
</style>

<script lang="ts">
  // ScopeGroupCard — Phase 141 decomposition of ScopeEditor.
  //
  // One first-class ScopeGroup card: the three numbered steps (1 Probes ·
  // 2 Discourse function · 3 Sources) for a single group. Purely a
  // controlled presentation child — the ScopeEditor owns the draft `$state`
  // and the mutators; this component renders the group and reports user
  // intent back through callback props. It holds no draft state of its own.
  import { m } from '$lib/paraglide/messages.js';
  import type { ScopeGroup } from '$lib/state/url-internals';
  import type { ProbeDto } from '$lib/api/queries';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import { DISCOURSE_FUNCTIONS, type DossierSource } from '$lib/workbench/scope-editor-internals';
  import ScopeGroupSources from './ScopeGroupSources.svelte';

  interface Props {
    group: ScopeGroup;
    groupIndex: number;
    /** Effective DF lock for this group (null = all functions). */
    lock: DiscourseFunction | null;
    /** Whether the "× Remove group" affordance shows (>1 group on the panel). */
    canRemove: boolean;
    probeList: ProbeDto[];
    probesPending: boolean;
    /** Resolves a probe's cached sources (`[]` while its dossier loads). */
    sourcesForProbe: (probeId: string) => DossierSource[];
    sourcesLoading: (probeId: string) => boolean;
    onToggleProbe: (probeId: string) => void;
    onToggleSource: (sourceName: string) => void;
    onSetLock: (df: DiscourseFunction | null) => void;
    onSelectAll: (probeId: string) => void;
    onClearAll: () => void;
    onRemove: () => void;
  }

  let {
    group,
    groupIndex,
    lock,
    canRemove,
    probeList,
    probesPending,
    sourcesForProbe,
    sourcesLoading,
    onToggleProbe,
    onToggleSource,
    onSetLock,
    onSelectAll,
    onClearAll,
    onRemove
  }: Props = $props();

  const lockMeta = $derived(lock ? DISCOURSE_FUNCTIONS.find((d) => d.id === lock) : null);
</script>

<article
  class="group"
  aria-label={m.workbench_scope_group_aria_label({ index: groupIndex + 1 })}
  style:--lock-color={lockMeta?.color ?? 'var(--color-accent)'}
>
  <header class="group-header">
    <div class="group-title-line">
      <span class="group-eyebrow">{m.workbench_scope_group_eyebrow({ index: groupIndex + 1 })}</span
      >
      <span class="group-summary">
        {group.probeIds.length === 1
          ? m.workbench_scope_group_summary_probes_one({ count: group.probeIds.length })
          : m.workbench_scope_group_summary_probes_other({ count: group.probeIds.length })} ·
        {group.sourceIds.length === 1
          ? m.workbench_scope_group_summary_sources_one({ count: group.sourceIds.length })
          : m.workbench_scope_group_summary_sources_other({ count: group.sourceIds.length })}
        {#if lockMeta}{m.workbench_scope_group_summary_locked_to()}
          <strong>{lockMeta.label}</strong>{/if}
      </span>
      {#if canRemove}
        <button
          type="button"
          class="group-remove-btn"
          onclick={onRemove}
          aria-label={m.workbench_scope_group_remove_label()}
          title={m.workbench_scope_group_remove_label()}
        >
          {m.workbench_scope_group_remove()}
        </button>
      {/if}
    </div>
  </header>

  <!-- 1. Probes -->
  <section class="step" data-step="1">
    <header class="step-header">
      <span class="step-num" aria-hidden="true">1</span>
      <h3 class="step-title">{m.workbench_scope_group_step_probes_title()}</h3>
      <span class="step-hint">{m.workbench_scope_group_step_probes_hint()}</span>
    </header>
    <div class="probe-grid">
      {#if probesPending}
        <p class="muted" aria-busy="true">{m.workbench_scope_group_probes_loading()}</p>
      {:else if probeList.length === 0}
        <p class="muted">{m.workbench_scope_group_probes_empty()}</p>
      {:else}
        {#each probeList as probe (probe.probeId)}
          {@const checked = group.probeIds.includes(probe.probeId)}
          <label class="probe-chip" class:checked>
            <input
              type="checkbox"
              {checked}
              onchange={() => onToggleProbe(probe.probeId)}
              aria-label={m.workbench_scope_group_probe_include({ name: probe.displayName })}
            />
            <span class="probe-name">{probe.displayName}</span>
            <span class="probe-lang">{probe.language.toUpperCase()}</span>
          </label>
        {/each}
      {/if}
    </div>
  </section>

  <!-- 2. DF restriction -->
  <section class="step" data-step="2">
    <header class="step-header">
      <span class="step-num" aria-hidden="true">2</span>
      <h3 class="step-title">{m.workbench_scope_group_step_df_title()}</h3>
      <span class="step-hint">{m.workbench_scope_group_step_df_hint()}</span>
    </header>
    <div class="df-row">
      <button
        type="button"
        class="df-chip df-chip-none"
        class:active={lock === null}
        onclick={() => onSetLock(null)}
      >
        {m.workbench_scope_group_df_none()}
      </button>
      {#each DISCOURSE_FUNCTIONS as df (df.id)}
        <button
          type="button"
          class="df-chip"
          class:active={lock === df.id}
          style:--chip-color={df.color}
          onclick={() => onSetLock(df.id)}
        >
          {df.label}
        </button>
      {/each}
    </div>
  </section>

  <!-- 3. Sources -->
  <ScopeGroupSources
    {group}
    {lock}
    {probeList}
    {sourcesForProbe}
    {sourcesLoading}
    {onToggleSource}
    {onSelectAll}
    {onClearAll}
  />
</article>

<style>
  .group {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4) var(--space-5);
    background: var(--color-surface);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    box-shadow:
      0 1px 2px rgba(0, 0, 0, 0.05),
      inset 3px 0 0 var(--lock-color);
  }

  .group-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .group-title-line {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .group-eyebrow {
    text-transform: uppercase;
    font-size: 11px;
    letter-spacing: 0.1em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .group-summary {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .group-remove-btn {
    margin-left: auto;
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 4px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .group-remove-btn:hover,
  .group-remove-btn:focus-visible {
    color: #d97a7a;
    border-color: #d97a7a;
  }

  /* ---------- Steps (numbered sections) ---------- */
  .step {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
  }
  /* Phase 122k §10 finding — accent dimmed and pulled to a single
     consistent border-left tint. The dimmed accents read as visual
     organisation without competing with the DF chips and source rows. */
  .step[data-step='1'] {
    border-left: 2px solid color-mix(in srgb, #7dc7e5 50%, var(--color-border));
  }
  .step[data-step='2'] {
    border-left: 2px solid color-mix(in srgb, #e8a25c 50%, var(--color-border));
  }

  .step-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
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
  /* Number badges stay neutral — the dim border-left does the
     section-tinting work. */

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

  /* ---------- Step 1 — probe grid ---------- */
  .probe-grid {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .probe-chip {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    cursor: pointer;
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .probe-chip:hover {
    border-color: #7dc7e5;
  }
  .probe-chip.checked {
    background: color-mix(in srgb, #7dc7e5 18%, var(--color-bg-elevated));
    border-color: #7dc7e5;
  }
  .probe-chip input {
    accent-color: #7dc7e5;
  }

  .probe-name {
    font-family: var(--font-mono);
  }

  .probe-lang {
    font-family: var(--font-mono);
    font-size: 10.5px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 5px;
    background: var(--color-bg);
    border-radius: var(--radius-sm);
    color: var(--color-fg-subtle);
  }

  /* ---------- Step 2 — DF chips ---------- */
  .df-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .df-chip {
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .df-chip:hover,
  .df-chip:focus-visible {
    color: var(--color-fg);
    border-color: var(--chip-color, var(--color-border-strong));
  }
  .df-chip.active {
    background: color-mix(
      in srgb,
      var(--chip-color, var(--color-accent)) 20%,
      var(--color-bg-elevated)
    );
    border-color: var(--chip-color, var(--color-accent));
    color: var(--color-fg);
    font-weight: 600;
  }
  .df-chip-none.active {
    background: color-mix(in srgb, var(--color-fg-subtle) 12%, var(--color-bg-elevated));
    border-color: var(--color-fg-subtle);
  }

  /* Step 1 "Loading probes…" / "No probes available." live in this card; the
     Step 3 source styles moved to ScopeGroupSources with the markup. */
  .muted {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
  }
</style>

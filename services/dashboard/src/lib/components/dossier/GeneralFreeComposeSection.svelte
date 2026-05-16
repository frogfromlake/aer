<script lang="ts">
  // Phase 122i revision (R5) — General Free-Compose section.
  //
  // Top-level entry path on `/dossier`. AĒR's most powerful tool:
  // compose **across the whole catalog** — any combination of probes
  // and any combination of those probes' sources — into a single
  // editable Workbench.
  //
  // Today's production runs Probe-0 only; the probe-multiselect
  // therefore lists one entry. The component is built for multi-probe
  // ahead of Phase 123, so the moment Probe-1 lands the picker has the
  // right shape automatically.
  //
  // Distinct from the per-probe FreeComposeSection that lives inside
  // each ProbeCard: that one is scoped to one probe's sources; this
  // one spans the catalog.
  import { createQuery } from '@tanstack/svelte-query';
  import { goto } from '$app/navigation';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { buildFreeComposeUrl } from '$lib/workbench/panel-queries';
  import ProbeSourcePicker from './ProbeSourcePicker.svelte';

  interface Props {
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
  }

  let { ctx, windowStart, windowEnd }: Props = $props();

  // Local component state for selection. Probes-state is the cross-
  // probe checkbox set; per-probe source selection is keyed by probeId.
  let selectedProbes = $state<readonly string[]>([]);
  let selectedSourcesByProbe = $state<Record<string, readonly string[]>>({});

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  function toggleProbe(probeId: string) {
    if (selectedProbes.includes(probeId)) {
      selectedProbes = selectedProbes.filter((p) => p !== probeId);
      // Drop the source selection for the deselected probe.
      const next = { ...selectedSourcesByProbe };
      delete next[probeId];
      selectedSourcesByProbe = next;
      return;
    }
    selectedProbes = [...selectedProbes, probeId];
  }

  function toggleSource(probeId: string, sourceName: string) {
    const current = selectedSourcesByProbe[probeId] ?? [];
    const next = current.includes(sourceName)
      ? current.filter((s) => s !== sourceName)
      : [...current, sourceName];
    selectedSourcesByProbe = { ...selectedSourcesByProbe, [probeId]: next };
  }

  const totalSourcesSelected = $derived(
    Object.values(selectedSourcesByProbe).reduce((sum, arr) => sum + arr.length, 0)
  );
  const canOpen = $derived(selectedProbes.length > 0);

  function open() {
    if (!canOpen) return;
    // Collect sourceIds across all selected probes. Today the
    // buildFreeComposeUrl helper takes flat probeIds + sourceIds and
    // produces a single Panel with one ScopeGroup unioning everything.
    // Multi-probe Cross-source-Group structure would require a richer
    // builder; deferring to the ScopeEditor inside the Workbench (the
    // user can refine after landing).
    const sourceIds: string[] = [];
    for (const probeId of selectedProbes) {
      const sourcesForProbe = selectedSourcesByProbe[probeId] ?? [];
      sourceIds.push(...sourcesForProbe);
    }
    const qs = buildFreeComposeUrl({
      pillar: 'aleph',
      probeIds: [...selectedProbes],
      sourceIds
    });
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Workbench route
    void goto(`/workbench${qs}`);
  }
</script>

<section class="general-free-compose" aria-labelledby="gfc-heading">
  <header class="header">
    <h2 id="gfc-heading">Compose across probes</h2>
    <p class="lede">
      AĒR's most powerful tool. Compose ANY combination of probes and ANY combination of those
      probes' sources into a single editable Workbench. Use the per-probe Free-Compose section
      inside a Probe Card below for probe-internal composition.
    </p>
  </header>

  {#if probesQ.isPending}
    <p class="muted" aria-busy="true">Loading probe catalog…</p>
  {:else if probeList.length === 0}
    <p class="muted">No probes in the catalog.</p>
  {:else}
    <ul class="probe-list" role="list">
      {#each probeList as probe (probe.probeId)}
        {@const probeSelected = selectedProbes.includes(probe.probeId)}
        <li class="probe-row" class:selected={probeSelected}>
          <label class="probe-head">
            <input
              type="checkbox"
              checked={probeSelected}
              onchange={() => toggleProbe(probe.probeId)}
              aria-label="Include {probe.probeId}"
            />
            <span class="probe-name">{probe.probeId}</span>
            <span class="probe-lang">[{probe.language}]</span>
          </label>
          {#if probeSelected}
            <ProbeSourcePicker
              {ctx}
              probeId={probe.probeId}
              {windowStart}
              {windowEnd}
              selected={selectedSourcesByProbe[probe.probeId] ?? []}
              onToggle={(name) => toggleSource(probe.probeId, name)}
            />
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  <div class="cta-row">
    <span class="cta-count">
      {#if !canOpen}
        Pick at least one probe to open the Workbench.
      {:else}
        {selectedProbes.length} probe{selectedProbes.length === 1 ? '' : 's'} · {totalSourcesSelected}
        source{totalSourcesSelected === 1 ? '' : 's'} selected
      {/if}
    </span>
    <button
      type="button"
      class="cta"
      onclick={open}
      disabled={!canOpen}
      title={canOpen
        ? 'Open Workbench composed across the selected probes and sources'
        : 'Pick at least one probe first'}
    >
      Open Workbench (general compose) →
    </button>
  </div>
</section>

<style>
  .general-free-compose {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 2px solid var(--color-accent);
    border-radius: var(--radius-md);
  }

  .header h2 {
    margin: 0 0 var(--space-1) 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
  }

  .lede {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: 1.45;
  }

  .probe-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .probe-row {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface);
  }

  .probe-row.selected {
    background: color-mix(in srgb, var(--color-accent) 8%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .probe-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
  }

  .probe-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .probe-lang {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }

  .cta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
    margin-top: var(--space-2);
  }

  .cta-count {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
  }

  .cta {
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

  .cta:hover:not(:disabled),
  .cta:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 80%, var(--color-fg));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .cta:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

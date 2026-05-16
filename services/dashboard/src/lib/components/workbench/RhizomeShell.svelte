<script lang="ts">
  // RhizomeShell — Phase 122h / ADR-033 §2 (Rhizome paragraph).
  //
  // The Rhizome Pillar renders the relational substrate. Layout:
  //   - Opinionated default view: *Actors & Topics* (entity co-occurrence)
  //     when no sub-view is set. No empty canvas.
  //   - Entry-question switcher: a compact tile row that lets the user
  //     pick one of four sub-views (Actors & Topics / Source Resonance /
  //     Concept Migration / Free Composition). Switches encode in
  //     `url.view` as a `RhizomeView` enum.
  //   - Body: the active sub-view's Cell or a feature-gate refusal
  //     (Concept Migration needs Phase 124; Free Composition needs the
  //     full Phase 125 card/edge canvas which is absorbed in a follow-up).
  import { createQuery } from '@tanstack/svelte-query';
  import type { Component } from 'svelte';
  import {
    probeDossierQuery,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS, type RhizomeView } from '$lib/state/url-internals';
  import { DEFAULT_METRIC_NAME, getPresentation, type ViewModeCellProps } from '$lib/viewmodes';
  import CellControls from './CellControls.svelte';
  import CellMethodology from './CellMethodology.svelte';

  interface Props {
    probeIds: string[];
  }

  let { probeIds }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());
  const activeProbeId = $derived(probeIds[0] ?? '');

  const windowMs = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      start: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS).toISOString(),
      end: new Date(Number.isFinite(toMs) ? toMs : now).toISOString()
    };
  });

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, activeProbeId, {
        windowStart: windowMs.start,
        windowEnd: windowMs.end
      });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: activeProbeId !== ''
      };
    }
  );

  const dossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );

  // Entry-question state. `null` resolves to the default (`actors-topics`).
  const activeView = $derived<RhizomeView>(url.view ?? 'actors-topics');

  interface EntryQuestion {
    id: RhizomeView;
    label: string;
    question: string;
    cellViewMode: 'cooccurrence_network' | 'topic_distribution' | null;
    requires?: 'cross-probe' | 'composition-workspace';
  }
  const ENTRY_QUESTIONS: readonly EntryQuestion[] = [
    {
      id: 'actors-topics',
      label: 'Actors & Topics',
      question: 'Who is mentioned alongside what?',
      cellViewMode: 'cooccurrence_network'
    },
    {
      id: 'source-resonance',
      label: 'Source Resonance',
      question: 'Which sources talk about the same topics?',
      cellViewMode: 'topic_distribution'
    },
    {
      id: 'concept-migration',
      label: 'Concept Migration',
      question: 'Where does a concept first appear, and who picks it up?',
      cellViewMode: null,
      requires: 'cross-probe'
    },
    {
      id: 'free-composition',
      label: 'Free Composition',
      question: 'Compose your own nodes and edges.',
      cellViewMode: null,
      requires: 'composition-workspace'
    }
  ];

  const activeQuestion = $derived(
    ENTRY_QUESTIONS.find((q) => q.id === activeView) ?? ENTRY_QUESTIONS[0]!
  );

  function pickEntry(id: RhizomeView) {
    if (id === activeView) return;
    setUrl({ view: id });
  }

  const metricName = $derived(url.metric ?? DEFAULT_METRIC_NAME);
  const dataLayer = $derived<'gold' | 'silver'>(url.layer === 'silver' ? 'silver' : 'gold');
  const cellSources = $derived(
    dossier
      ? dossier.sources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }))
      : []
  );
  const scope = $derived<'probe' | 'source'>('probe');
  const scopeId = $derived<string>(activeProbeId);

  // Cell selection — derived from the entry-question's `cellViewMode`.
  // The entry-question grammar is the Rhizome-native API; we map to the
  // underlying view-mode catalog under the hood. `null` means "feature
  // gate", not "load a Cell".
  const presentation = $derived(
    activeQuestion.cellViewMode ? getPresentation(activeQuestion.cellViewMode) : null
  );

  let CellComponent = $state<Component<ViewModeCellProps> | null>(null);
  let loadError = $state<string | null>(null);
  let loadToken = 0;

  $effect(() => {
    const t = ++loadToken;
    loadError = null;
    CellComponent = null;
    if (!presentation) return;
    presentation
      .loadComponent()
      .then((Comp) => {
        if (t !== loadToken) return;
        CellComponent = Comp;
      })
      .catch((err: unknown) => {
        if (t !== loadToken) return;
        loadError = err instanceof Error ? err.message : 'Cell failed to load';
      });
  });

  // Cross-probe gate for Concept-Migration — requires two probes plus
  // a Phase-124 equivalence grant (Level 1 temporal). The first ship
  // surfaces a refusal note explaining the dependency.
  const hasMultiprobe = $derived(probeIds.length >= 2);
</script>

<section class="rhizome-shell" aria-label="Rhizome — connections in the discourse">
  <header class="entry-row" aria-label="Which connection do you want to investigate?">
    {#each ENTRY_QUESTIONS as q (q.id)}
      {@const isActive = q.id === activeView}
      <button
        type="button"
        role="tab"
        aria-selected={isActive}
        class="entry-tile"
        class:active={isActive}
        title={q.question}
        onclick={() => pickEntry(q.id)}
      >
        <span class="entry-label">{q.label}</span>
        <span class="entry-question">{q.question}</span>
      </button>
    {/each}
  </header>

  {#if activeQuestion.cellViewMode}
    <CellControls pillar="rhizome" lockedView={activeQuestion.cellViewMode} />
  {/if}

  <div class="rhizome-body" aria-live="polite">
    {#if dossierQ.isPending}
      <p class="muted" aria-busy="true">Loading dataset…</p>
    {:else if !dossier}
      <p class="muted">Dossier failed to load.</p>
    {:else if activeQuestion.requires === 'cross-probe' && !hasMultiprobe}
      <div class="refusal" role="status">
        <h2>Concept Migration needs two probes</h2>
        <p>
          This view shows how concepts migrate between cultural contexts. It needs at least two
          probes with validated temporal comparability. Only Probe 0 is in scope right now.
        </p>
        <p class="refusal-meta">
          Becomes available with <strong>Phase 123</strong> (Probe 1 — French institutional) and
          <strong>Phase 124</strong> (first cross-probe equivalence grant).
        </p>
      </div>
    {:else if activeQuestion.requires === 'composition-workspace'}
      <div class="refusal" role="status">
        <h2>Free Composition — coming soon</h2>
        <p>
          Compose your own nodes and edges into a graph — the full card-/edge-palette from
          <strong>Phase 125</strong>. Will be absorbed into Rhizome in a follow-up slice.
        </p>
      </div>
    {:else if loadError}
      <p class="muted">Cell failed to load: {loadError}</p>
    {:else if !CellComponent}
      <p class="muted" aria-busy="true">Loading {activeQuestion.label}…</p>
    {:else}
      {@const Cell = CellComponent}
      <Cell
        {ctx}
        scopeProbeId={dossier.probeId}
        {scope}
        {scopeId}
        windowStart={windowMs.start}
        windowEnd={windowMs.end}
        {metricName}
        sources={cellSources}
        {dataLayer}
        probeIds={probeIds.length > 1 ? probeIds : []}
      />
    {/if}
  </div>

  {#if activeQuestion.cellViewMode && presentation}
    <CellMethodology
      {metricName}
      viewMode={activeQuestion.cellViewMode}
      viewLabel={activeQuestion.label}
    />
  {/if}
</section>

<style>
  .rhizome-shell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: 1;
  }

  .entry-row {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-2);
  }

  .entry-tile {
    appearance: none;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-3);
    text-align: left;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 4px;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .entry-tile:hover,
  .entry-tile:focus-visible {
    color: var(--color-fg);
    border-color: color-mix(in srgb, #9a8fb8 50%, var(--color-border));
    background: color-mix(in srgb, #9a8fb8 6%, var(--color-surface));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .entry-tile.active {
    background: color-mix(in srgb, #9a8fb8 18%, var(--color-surface));
    border-color: #9a8fb8;
    color: var(--color-fg);
    box-shadow: inset 0 -3px 0 #9a8fb8;
  }

  .entry-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: #9a8fb8;
    font-weight: var(--font-weight-semibold);
  }

  .entry-question {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
    line-height: 1.4;
  }

  .rhizome-body {
    flex: 1;
    min-height: 24rem;
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: auto;
  }

  .refusal {
    max-width: 38rem;
    margin: var(--space-5) auto;
    padding: var(--space-5);
    background: color-mix(in srgb, #9a8fb8 6%, var(--color-surface));
    border: 1px solid color-mix(in srgb, #9a8fb8 40%, transparent);
    border-left: 3px solid #9a8fb8;
    border-radius: var(--radius-md);
  }

  .refusal h2 {
    margin: 0 0 var(--space-2) 0;
    font-size: var(--font-size-md);
    color: var(--color-fg);
    font-weight: var(--font-weight-semibold);
  }

  .refusal p {
    margin: 0 0 var(--space-2) 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
  }

  .refusal-meta {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  @media (max-width: 900px) {
    .entry-row {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>

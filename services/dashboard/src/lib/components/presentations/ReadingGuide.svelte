<script lang="ts">
  // Panel-level Reading Guide — Phase 148f.
  //
  // The viridis "epistemic ladder": the panel's shared configuration explained as
  // six perceptually-ordered steps (purple → yellow = reading depth), each a
  // scientific question a researcher asks of a chart:
  //   what · measure · sample · encoding · compare · limits
  // Composition logic is the pure `composeReadingGuide`; this component resolves
  // the panel's live config + labels, fetches the presentation template, filters
  // to the panel-level notes, and renders them grouped per question. Notes that
  // describe a visual channel are tinted to mirror the chart's encoding. Collapsed
  // by default (mirrors HowToRead) — the per-cell delta carries the rest.
  import { createQuery } from '@tanstack/svelte-query';
  import { m } from '$lib/paraglide/messages.js';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import {
    composeReadingGuide,
    type ReadingGuideInput,
    type ReadingNote,
    type ReadingQuestion
  } from '$lib/presentations/reading-guide';
  import { panelSubjectKind } from '$lib/presentations/metric-presentation';
  import MeasureDetail from '$lib/components/workbench/MeasureDetail.svelte';
  import type { PresentationDefinition } from '$lib/presentations';
  import type { Panel } from '$lib/state/url-internals';
  import { metricLabel, fieldLabel, fieldDescription } from '$lib/state/labels.svelte';
  import { locale } from '$lib/state/locale.svelte';

  interface Props {
    panel: Panel;
    presentation: PresentationDefinition;
    ctx: FetchContext;
    /** The probe dossier — its source list resolves the "all sources" scope count. */
    dossier: ProbeDossierDto;
    /** Effective (inherited) window bounds for the SAMPLE note. */
    windowStart: string | undefined;
    windowEnd: string | undefined;
    /** 'panel' = the panel-wide guide; 'cell' = an overridden cell's own guide. */
    variant?: 'panel' | 'cell';
    /** Panel variant only — appends "(overridden cells excepted)" when true. */
    hasOverriddenCells?: boolean;
  }
  let {
    panel,
    presentation,
    ctx,
    dossier,
    windowStart,
    windowEnd,
    variant = 'panel',
    hasOverriddenCells = false
  }: Props = $props();

  let expanded = $state(false);

  const titleText = $derived(
    variant === 'cell'
      ? m.rg_title_cell()
      : hasOverriddenCells
        ? m.rg_title_panel_overrides()
        : m.rg_title_panel()
  );

  // The presentation template line (WHAT) — same source as HowToRead.
  const templateQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', `howto_${presentation.id}`, locale());
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const templateBase = $derived(
    templateQ.data?.kind === 'success' ? templateQ.data.data.registers.semantic.short : null
  );

  const subjectKind = $derived(panelSubjectKind(presentation));

  // Scope counts (distinct probes / sources across the panel's scope-groups).
  // Array dedup (scope-groups are tiny) — avoids a reactive-Set lint flag.
  const scopeCounts = $derived.by(() => {
    const probes: string[] = [];
    const sources: string[] = [];
    const allSourceNames = dossier.sources.map((s) => s.name);
    for (const sg of panel.scopes) {
      for (const p of sg.probeIds ?? []) if (!probes.includes(p)) probes.push(p);
      // An empty `sourceIds` means "all sources of the probe" (ScopeGroup
      // contract) — resolve it to the dossier's source list so the count is the
      // sources actually rendered, not 0.
      const effective = sg.sourceIds && sg.sourceIds.length > 0 ? sg.sourceIds : allSourceNames;
      for (const s of effective) if (!sources.includes(s)) sources.push(s);
    }
    return { probeCount: probes.length, sourceCount: sources.length };
  });

  const windowLabel = $derived.by(() => {
    const start = panel.windowStart ?? windowStart;
    const end = panel.windowEnd ?? windowEnd;
    if (!start || !end) return undefined;
    return `${start.slice(0, 10)} – ${end.slice(0, 10)}`;
  });

  // The shared/free axis note is meaningful only on a value-axis view rendered as
  // >1 cell (the axis MODE is panel-wide). Surface it once, here, instead of
  // duplicating it under every cell.
  const VALUE_AXIS_VIEWS = new Set(['distribution', 'time_series', 'metric_scatter']);
  const multiCell = $derived(
    panel.scopes.length > 1 ||
      scopeCounts.sourceCount > 1 ||
      !!panel.facetField ||
      panel.composition === 'overlay'
  );
  const scalesForGuide = $derived<'shared' | 'free' | undefined>(
    VALUE_AXIS_VIEWS.has(presentation.id) && multiCell ? (panel.scales ?? 'shared') : undefined
  );

  const input = $derived<ReadingGuideInput>({
    presentation: presentation.id,
    presentationLabel: presentation.label,
    subjectKind,
    metricLabel: subjectKind === 'metric' ? metricLabel(panel.metric) : undefined,
    fieldLabel: subjectKind === 'field' ? fieldLabel(panel.metric) : undefined,
    fieldDescription: subjectKind === 'field' ? fieldDescription(panel.metric) : undefined,
    composition: panel.composition,
    normalization: panel.normalization,
    resolution: panel.resolution,
    windowLabel,
    facetFieldLabel: panel.facetField ? fieldLabel(panel.facetField) : undefined,
    probeCount: scopeCounts.probeCount,
    sourceCount: scopeCounts.sourceCount,
    // Panel-level config encoding (shared across the panel's cells) so the
    // ENCODING step is complete — the per-cell delta then carries only runtime
    // data (this cell's r / counts / override).
    bins: panel.bins,
    topN: panel.topN,
    showBand: panel.showBand,
    x: panel.channels?.x,
    y: panel.channels?.y,
    size: panel.channels?.size,
    color: panel.channels?.color,
    netSize: panel.channels?.netSize,
    netColor: panel.channels?.netColor,
    scales: scalesForGuide
  });

  // The six questions, in viridis (perceptual) order. The anchor colours are
  // DESATURATED versions of the viridis ramp (legible on the dark theme as a calm
  // accent, not a loud full-saturation hue) — they keep the perceptual ordering
  // lavender→olive while reading as serious, not festive.
  const STEPS: { q: ReadingQuestion; heading: () => string; color: string }[] = [
    { q: 'what', heading: () => m.rg_q_what(), color: '#9a8fb0' },
    { q: 'measure', heading: () => m.rg_q_measure(), color: '#7e93b3' },
    { q: 'sample', heading: () => m.rg_q_sample(), color: '#6f9bac' },
    { q: 'encoding', heading: () => m.rg_q_encoding(), color: '#69a596' },
    { q: 'compare', heading: () => m.rg_q_compare(), color: '#82ab84' },
    { q: 'limits', heading: () => m.rg_q_limits(), color: '#a3a978' }
  ];

  const panelNotes = $derived(
    composeReadingGuide(input, templateBase).notes.filter((n) => n.level === 'panel')
  );
  function notesFor(q: ReadingQuestion): ReadingNote[] {
    return panelNotes.filter((n) => n.question === q);
  }
</script>

<aside class="reading-guide" aria-label={m.cells_howto_aria()}>
  <button
    type="button"
    class="rg-toggle"
    aria-expanded={expanded}
    onclick={() => (expanded = !expanded)}
  >
    <span class="rg-chevron" class:expanded aria-hidden="true">›</span>
    <span class="rg-title">{titleText}</span>
  </button>

  {#if expanded}
    <div class="rg-content">
      <ol class="rg-ladder">
        {#each STEPS as step (step.q)}
          {@const notes = notesFor(step.q)}
          {#if notes.length > 0}
            <li class="rg-step" style="--step-color: {step.color}">
              <span class="rg-step-eyebrow">{step.heading()}</span>
              <ul class="rg-notes">
                {#each notes as note (note.text)}
                  <li class="rg-note">
                    <span class="rg-text">{note.text}</span>
                    {#if note.anchor}
                      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
                      <a class="rg-anchor" href={note.anchor.href}>{note.anchor.label}</a>
                    {/if}
                  </li>
                {/each}
              </ul>
            </li>
          {/if}
        {/each}
      </ol>
      <!-- Phase 148f Step 7 — the deep methodology, folded in under the ladder as
           the MEASURE detail (METHODIK header + default-collapsed blocks + links). -->
      <MeasureDetail
        metricName={panel.metric}
        viewMode={presentation.id}
        viewLabel={presentation.label}
      />
    </div>
  {/if}
</aside>

<style>
  .reading-guide {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-bg-elevated);
  }

  .rg-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-fg-muted);
    text-align: left;
  }
  .rg-toggle:hover,
  .rg-toggle:focus-visible {
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .rg-chevron {
    display: inline-flex;
    width: 0.9rem;
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    color: var(--color-fg-subtle);
  }
  .rg-chevron.expanded {
    transform: rotate(90deg);
  }
  .rg-title {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-muted);
  }

  /* Unified breathing room for the expanded content (ladder + measure detail). */
  .rg-content {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    padding: var(--space-2) var(--space-5) var(--space-5);
  }

  .rg-ladder {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  /* The ONLY colour in the ladder: one soft (desaturated) viridis rail per step.
     The ladder reads lavender→olive top to bottom (perceptual reading depth). All
     text stays calm grey — no coloured eyebrows, dots, chips or dashed lines. */
  .rg-step {
    border-left: 2px solid color-mix(in srgb, var(--step-color) 60%, transparent);
    padding-left: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .rg-step-eyebrow {
    font-family: var(--font-mono);
    font-size: 10.5px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-muted);
  }

  .rg-notes {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .rg-note {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--space-2);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
  }
  .rg-text {
    flex: 1 1 12rem;
    min-width: 0;
  }
  .rg-anchor {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    text-decoration: none;
    white-space: nowrap;
  }
  .rg-anchor:hover,
  .rg-anchor:focus-visible {
    color: var(--color-accent);
    text-decoration: underline;
  }

  @media (prefers-reduced-motion: reduce) {
    .rg-chevron {
      transition: none;
    }
  }
</style>

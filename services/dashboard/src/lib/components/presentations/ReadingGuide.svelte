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
  import { cellSubjects } from '$lib/presentations';
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

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && expanded) expanded = false;
  }}
/>

<aside class="reading-guide" aria-label={m.cells_howto_aria()}>
  {#if !expanded}
    <!-- Phase 149 — the launcher is ONE labeled pull-tab pinned to the right edge,
         where the drawer emerges (book glyph + "How to read"). The panel guide
         centres it on the panel's right edge; an overridden cell's own guide rides
         the UPPER-right of that cell — the same button, clear of the cell's export
         row (top) and the override note (bottom). -->
    <button
      type="button"
      class="rg-tab"
      class:rg-tab--cell={variant === 'cell'}
      data-tutorial-id={variant === 'panel' ? 'wb-reading-guide' : undefined}
      aria-expanded={expanded}
      title={titleText}
      onclick={(e) => {
        e.stopPropagation();
        expanded = true;
      }}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
        <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
      </svg>
      <span class="rg-tab-label">{m.rg_tab_label()}</span>
    </button>
  {/if}

  {#if expanded}
    <!-- Phase 149 — the guide is a glassy drawer that slides in from the RIGHT,
         overlaying the panel/cell at full height (anchored to the positioned
         .panel-host / .panel-cell ancestor) so the reader sees the chart while
         reading. Close via ×, Esc, or the toggle. No backdrop → the chart under
         the left portion stays interactive. -->
    <div class="rg-drawer" role="region" aria-label={titleText}>
      <div class="rg-drawer-head">
        <span class="rg-title">{titleText}</span>
        <button
          type="button"
          class="rg-close"
          aria-label={m.rg_close()}
          title={m.rg_close()}
          onclick={() => (expanded = false)}>×</button
        >
      </div>
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
          subjects={cellSubjects(presentation, panel)}
          viewMode={presentation.id}
          viewLabel={presentation.label}
        />
      </div>
    </div>
  {/if}
</aside>

<style>
  /* Phase 149 — the wrapper is transparent: the launcher (tab / cell icon) and the
     drawer are absolutely / inline positioned, so the wrapper itself takes no
     layout box (no bottom strip). `display: contents` lets the panel-tab anchor to
     .panel-host and the cell-icon flow inline in the override-notice row. */
  .reading-guide {
    display: contents;
  }

  /* Panel launcher — a vertical pull-tab pinned to the panel's right edge, where
     the drawer emerges. Hidden while the drawer is open (the drawer covers it). */
  .rg-tab {
    position: absolute;
    top: 60%;
    right: 0;
    transform: translateY(-50%);
    z-index: 20;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    writing-mode: vertical-rl;
    padding: var(--space-3) 5px;
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-bg-elevated));
    border: 1px solid color-mix(in srgb, var(--color-accent) 32%, var(--color-border));
    border-right: none;
    border-radius: var(--radius-sm) 0 0 var(--radius-sm);
    color: var(--color-accent);
    cursor: pointer;
    transition: background-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .rg-tab:hover,
  .rg-tab:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 22%, var(--color-bg-elevated));
    outline: none;
  }
  .rg-tab svg {
    width: 15px;
    height: 15px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .rg-tab-label {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--font-weight-semibold);
  }

  /* Cell variant — the same labeled tab, but anchored to the UPPER third of the
     cell's right edge (anchors to .panel-cell): high enough to clear the panel
     guide's centred tab, low enough to clear the cell's own export header. */
  .rg-tab--cell {
    top: 40%;
  }

  .rg-title {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-muted);
  }

  /* Phase 149 — glassy drawer: slides in from the RIGHT, anchored to the nearest
     positioned ancestor (.panel-host for the panel guide, .panel-cell for a cell
     guide), full height, scrollable. Overlays only the right portion so the chart
     stays visible (and interactive — there is no backdrop). */
  .rg-drawer {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: 30;
    width: clamp(19rem, 38%, 27rem);
    max-width: 100%;
    display: flex;
    flex-direction: column;
    background: var(--color-bg-overlay);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    border-left: 1px solid color-mix(in srgb, var(--color-accent) 35%, var(--color-border));
    border-radius: 0 var(--radius-md) var(--radius-md) 0;
    box-shadow: -8px 0 24px -12px rgba(0, 0, 0, 0.55);
    overflow-y: auto;
    animation: rg-slide-in var(--motion-duration-base, 180ms) var(--motion-ease-standard, ease-out);
  }
  @keyframes rg-slide-in {
    from {
      transform: translateX(8%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }
  .rg-drawer-head {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-3) var(--space-2) var(--space-5);
    background: linear-gradient(
      to bottom,
      var(--color-bg-overlay),
      color-mix(in srgb, var(--color-bg-overlay) 70%, transparent)
    );
  }
  .rg-close {
    appearance: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 26px;
    min-height: 26px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-md);
    line-height: 1;
    cursor: pointer;
  }
  .rg-close:hover,
  .rg-close:focus-visible {
    color: var(--color-fg);
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: color-mix(in srgb, var(--color-accent) 40%, transparent);
    outline: none;
  }

  /* Unified breathing room for the expanded content (ladder + measure detail). */
  .rg-content {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    padding: var(--space-2) var(--space-5) var(--space-5);
  }

  @media (prefers-reduced-motion: reduce) {
    .rg-drawer {
      animation: none;
    }
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
</style>

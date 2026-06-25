<script lang="ts">
  // MetricHints — Phase 151 (extracted from MetricControls so the metric/field
  // PICKER can become a compact dropdown that sits beside View on one row,
  // while these wider "withheld / show anyway" notes render as their own rows
  // below).
  //
  // ADR-038 honesty guard: a metric/field present on only SOME scoped sources
  // is withheld by default (no cell renders silently empty); "show anyway"
  // opts in and the panel then renders only the sources that carry it. Turning
  // it OFF snaps a now-unofferable metric back to a scope-valid default.
  import type { Panel, Presentation } from '$lib/state/url-internals';
  import {
    missingSourcesFor as missingSourcesForOf,
    resetMetricForScope as resetMetricForScopeOf,
    formatConstantValue,
    dominantSharePct
  } from '$lib/workbench/panel-controls-derive';
  import type { ScopeAvailableMetricsDto, ScopeAvailableMetadataDto } from '$lib/api/queries';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { metricLabel, fieldLabel, dimensionLabel, sourceLabel } from '$lib/state/labels.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    view: Presentation;
    viewUsesMetric: boolean;
    viewUsesMetadataField: boolean;
    availableMetricNames: string[];
    partialMetrics: ScopeAvailableMetricsDto['partial'];
    partialMetadataFields: ScopeAvailableMetadataDto['partial'];
    /** Task A — metrics/fields constant across the scope (dropped, disclosed). */
    degenerateMetrics?: ScopeAvailableMetricsDto['degenerate'];
    degenerateMetadata?: ScopeAvailableMetadataDto['degenerate'];
    /** Task A — near-constant metrics + fields (kept offerable, disclosed). */
    lowSignalMetrics?: ScopeAvailableMetricsDto['lowSignal'];
    lowSignalMetadata?: ScopeAvailableMetadataDto['lowSignal'];
    /** Phase 148g — fields constant for an individual source (Élysée's author). */
    perSourceConstantMetadata?: ScopeAvailableMetadataDto['perSourceConstant'];
    scopedSourceNames: readonly string[];
    scopedSourceCount: number;
    scopeAvailableSet: Set<string> | null;
    /** scatter binds metrics too — gates the partial-metric hint. */
    configParams: readonly string[];
  }

  let {
    panelPath,
    boundPanel,
    view,
    viewUsesMetric,
    viewUsesMetadataField,
    availableMetricNames,
    partialMetrics,
    partialMetadataFields,
    degenerateMetrics,
    degenerateMetadata,
    lowSignalMetrics,
    lowSignalMetadata,
    perSourceConstantMetadata,
    scopedSourceNames,
    scopedSourceCount,
    scopeAvailableSet,
    configParams
  }: Props = $props();

  // Task A — disclosure rows for constant ("no signal") dimensions. Metric
  // degeneracy is relevant only where the view binds metrics; metadata
  // degeneracy only where it groups by a field — so each is gated at the source.
  const metricBinds = $derived(viewUsesMetric || configParams.includes('scatterAxes'));
  // Phase 149 — a view can bind a categorical field either as its primary
  // dimension (`usesMetadataField` — categorical_distribution / cross_tab) OR
  // via the Sankey field-chain lever (`sankeyFields`). Both must surface the
  // withheld / no-signal / per-source-constant field disclosures, else a Sankey
  // over a cross-probe scope whose categorical fields don't intersect shows an
  // empty field picker with NO explanation (ADR-039 DISCLOSE-NEVER-COERCE).
  const fieldBinds = $derived(viewUsesMetadataField || configParams.includes('sankeyFields'));
  const degenerateRows = $derived<{ name: string; value: string }[]>([
    ...(metricBinds
      ? (degenerateMetrics ?? []).map((d) => ({
          name: d.metricName,
          value: formatConstantValue(d.value)
        }))
      : []),
    ...(fieldBinds
      ? (degenerateMetadata ?? []).map((d) => ({ name: d.field, value: d.value }))
      : [])
  ]);
  // Task A — near-constant ("effectively constant") rows: metrics where the view
  // binds metrics, fields where it groups by a field. Pre-resolved to a label +
  // formatted dominant value so the template stays metric/field-agnostic.
  const lowSignalRows = $derived<
    { key: string; label: string; share: number; valueText: string; distinct: number }[]
  >([
    ...(metricBinds
      ? (lowSignalMetrics ?? []).map((mm) => ({
          key: `m:${mm.metricName}`,
          label: metricLabel(mm.metricName),
          share: mm.dominantShare,
          valueText: formatConstantValue(mm.dominantValue),
          distinct: mm.distinctValues
        }))
      : []),
    ...(fieldBinds
      ? (lowSignalMetadata ?? []).map((f) => ({
          key: `f:${f.field}`,
          label: fieldLabel(f.field),
          share: f.dominantShare,
          valueText: f.dominantValue,
          distinct: f.distinctValues
        }))
      : [])
  ]);

  const activeShowWithheld = $derived(boundPanel.showWithheld === true);

  // Phase 148g — per-source-constant disclosure rows (field-driven views only:
  // grouping by a field is where a source's constant value matters). One row per
  // (field, source): "<field> — <source> = <value>".
  const perSourceConstantRows = $derived<
    { key: string; label: string; source: string; value: string }[]
  >(
    fieldBinds
      ? (perSourceConstantMetadata ?? []).map((p) => ({
          key: `${p.field}:${p.source}`,
          label: fieldLabel(p.field),
          source: sourceLabel(p.source),
          value: p.value
        }))
      : []
  );

  function missingSourcesFor(have: readonly string[]): string[] {
    return missingSourcesForOf(have, scopedSourceNames);
  }
  function resetMetricForScope(): string {
    return resetMetricForScopeOf({
      view,
      scopeAvailableSet,
      scopes: boundPanel.scopes,
      availableMetricNames
    });
  }
  function toggleShowWithheld() {
    const next = !activeShowWithheld;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next) {
        o.showWithheld = true;
      } else {
        delete o.showWithheld;
        if (scopeAvailableSet && !scopeAvailableSet.has(o.metric)) {
          o.metric = resetMetricForScope();
        }
      }
      return o;
    });
  }
</script>

<!-- ADR-038 — metadata withholding. Phase 148e: a collapsed disclosure — the
     eyebrow + count stay visible (never silently filtered), the per-field detail
     and "show anyway" toggle expand on demand so the strip stays shallow. -->
{#if fieldBinds && partialMetadataFields.length > 0}
  <details class="hint-disclosure">
    <summary class="hint-summary">
      <span class="ctrl-eyebrow">{m.levers_withheld_eyebrow()}</span>
      <span class="hint-count">{partialMetadataFields.length}</span>
      <span class="hint-chevron" aria-hidden="true">›</span>
    </summary>
    <div class="partial-hint-body">
      <p class="partial-hint-lead">
        {partialMetadataFields.length === 1
          ? m.levers_withheld_fields_lead_one({
              count: partialMetadataFields.length,
              sources: scopedSourceCount,
              sourcePlural:
                scopedSourceCount === 1
                  ? m.levers_withheld_source_plural_one()
                  : m.levers_withheld_source_plural_other()
            })
          : m.levers_withheld_fields_lead_other({
              count: partialMetadataFields.length,
              sources: scopedSourceCount,
              sourcePlural:
                scopedSourceCount === 1
                  ? m.levers_withheld_source_plural_one()
                  : m.levers_withheld_source_plural_other()
            })}
      </p>
      <ul class="partial-hint-list" role="list">
        {#each partialMetadataFields as pf (pf.field)}
          {@const missing = missingSourcesFor(pf.sources)}
          <li class="partial-metric-row">
            <code class="partial-metric">{fieldLabel(pf.field)}</code>
            <span class="partial-metric-detail">
              {m.levers_withheld_has({
                present: pf.sources.length,
                total: scopedSourceCount
              })}{#if missing.length > 0}
                {m.levers_withheld_missing_on()} <strong>{missing.join(', ')}</strong>{/if}
            </span>
          </li>
        {/each}
      </ul>
      <LeverButton
        variant="partial-toggle"
        role="switch"
        active={activeShowWithheld}
        onclick={toggleShowWithheld}
        title={m.levers_withheld_fields_toggle_title()}
      >
        {activeShowWithheld ? m.levers_withheld_showing_fields() : m.levers_withheld_show_anyway()}
      </LeverButton>
    </div>
  </details>
{/if}

<!-- Phase 123c (C1) + ADR-038 — partial-metric hint (metrics on only SOME
     scoped sources). -->
{#if partialMetrics.length > 0 && (viewUsesMetric || configParams.includes('scatterAxes'))}
  <details class="hint-disclosure">
    <summary class="hint-summary">
      <span class="ctrl-eyebrow">{m.levers_withheld_eyebrow()}</span>
      <span class="hint-count">{partialMetrics.length}</span>
      <span class="hint-chevron" aria-hidden="true">›</span>
    </summary>
    <div class="partial-hint-body">
      <p class="partial-hint-lead">
        {partialMetrics.length === 1
          ? m.levers_withheld_metrics_lead_one({
              count: partialMetrics.length,
              sources: scopedSourceCount,
              sourcePlural:
                scopedSourceCount === 1
                  ? m.levers_withheld_source_plural_one()
                  : m.levers_withheld_source_plural_other()
            })
          : m.levers_withheld_metrics_lead_other({
              count: partialMetrics.length,
              sources: scopedSourceCount,
              sourcePlural:
                scopedSourceCount === 1
                  ? m.levers_withheld_source_plural_one()
                  : m.levers_withheld_source_plural_other()
            })}
      </p>
      <ul class="partial-hint-list" role="list">
        {#each partialMetrics as pm (pm.metricName)}
          {@const missing = missingSourcesFor(pm.sources)}
          <li class="partial-metric-row">
            <code class="partial-metric">{metricLabel(pm.metricName)}</code>
            <span class="partial-metric-detail">
              {m.levers_withheld_has({
                present: pm.sources.length,
                total: scopedSourceCount
              })}{#if missing.length > 0}
                {m.levers_withheld_missing_on()} <strong>{missing.join(', ')}</strong>{/if}
            </span>
          </li>
        {/each}
      </ul>
      <LeverButton
        variant="partial-toggle"
        role="switch"
        active={activeShowWithheld}
        onclick={toggleShowWithheld}
        title={m.levers_withheld_metrics_toggle_title()}
      >
        {activeShowWithheld ? m.levers_withheld_showing_metrics() : m.levers_withheld_show_anyway()}
      </LeverButton>
    </div>
  </details>
{/if}

<!-- Phase 148g — ONE "No signal" disclosure. Constant (degenerate) and
     near-constant (≤2 distinct / dominant) dimensions are now treated identically
     (both carry no usable signal and are dropped from the picker), so the former
     two separate "No signal" + "Effectively constant" expandables are merged into
     a single list — each row keeps its own detail (constant value vs dominant
     share). Disclosed, never silently filtered (ADR-039 DISCLOSE-NEVER-COERCE);
     neutral hue — methodological, not a warning (METHODOLOGICAL-NOT-WARNING). -->
{#if degenerateRows.length + lowSignalRows.length > 0}
  {@const noSignalCount = degenerateRows.length + lowSignalRows.length}
  <details class="hint-disclosure signal-note">
    <summary class="hint-summary">
      <span class="ctrl-eyebrow">{m.levers_degenerate_eyebrow()}</span>
      <span class="hint-count">{noSignalCount}</span>
      <span class="hint-chevron" aria-hidden="true">›</span>
    </summary>
    <div class="partial-hint-body">
      <p class="partial-hint-lead">
        {noSignalCount === 1
          ? m.levers_lowsignal_lead_one({ count: noSignalCount })
          : m.levers_lowsignal_lead_other({ count: noSignalCount })}
      </p>
      <ul class="partial-hint-list" role="list">
        {#each degenerateRows as row (row.name)}
          <li class="partial-metric-row">
            <code class="partial-metric signal-chip">{dimensionLabel(row.name)}</code>
            <span class="partial-metric-detail">
              {m.levers_degenerate_value({ value: row.value })}
            </span>
          </li>
        {/each}
        {#each lowSignalRows as row (row.key)}
          <li class="partial-metric-row">
            <code class="partial-metric signal-chip">{row.label}</code>
            <span class="partial-metric-detail">
              {m.levers_lowsignal_detail({
                share: dominantSharePct(row.share),
                value: row.valueText,
                distinct: row.distinct
              })}
            </span>
          </li>
        {/each}
      </ul>
    </div>
  </details>
{/if}

<!-- Phase 148g — per-source constancy. A field that carries signal across the
     scope but is CONSTANT for one source (e.g. Élysée's institutional `author`,
     always its own name, while editorial sources vary) is disclosed here so the
     reader sees that source contributes no WITHIN-source signal — symmetric with
     the structural-absence note for a source that emits the field not at all
     (ADR-039 DISCLOSE-NEVER-COERCE). The field stays fully offerable. -->
{#if perSourceConstantRows.length > 0}
  <details class="hint-disclosure signal-note">
    <summary class="hint-summary">
      <span class="ctrl-eyebrow">{m.levers_persourceconst_eyebrow()}</span>
      <span class="hint-count">{perSourceConstantRows.length}</span>
      <span class="hint-chevron" aria-hidden="true">›</span>
    </summary>
    <div class="partial-hint-body">
      <p class="partial-hint-lead">
        {perSourceConstantRows.length === 1
          ? m.levers_persourceconst_lead_one({ count: perSourceConstantRows.length })
          : m.levers_persourceconst_lead_other({ count: perSourceConstantRows.length })}
      </p>
      <ul class="partial-hint-list" role="list">
        {#each perSourceConstantRows as row (row.key)}
          <li class="partial-metric-row">
            <code class="partial-metric signal-chip">{row.label}</code>
            <span class="partial-metric-detail">
              {m.levers_persourceconst_detail({ source: row.source, value: row.value })}
            </span>
          </li>
        {/each}
      </ul>
    </div>
  </details>
{/if}

<style>
  /* Phase 148e — each withheld / no-signal note is a collapsed <details>: the
     eyebrow + count are always visible (disclosed, never silently filtered —
     ADR-039), the per-dimension detail expands on demand so the control strip
     stays shallow. Calm and low-emphasis; never an error. */
  .hint-disclosure {
    font-size: var(--font-size-xs);
  }
  /* Phase 148g — the disclosure must READ as expandable: the chevron leads the
     row (before the label, not glued to the far edge), and the whole summary
     gets a hover surface + a "tap target" border so the user sees there is a
     "show anyway" affordance behind it. */
  .hint-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
    list-style: none;
    padding: var(--space-1) var(--space-2);
    border: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
  }
  .hint-summary:hover {
    background: color-mix(in srgb, var(--color-fg) 5%, transparent);
    border-color: var(--color-border);
    color: var(--color-fg);
  }
  .hint-summary::-webkit-details-marker {
    display: none;
  }
  .hint-summary:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-radius: var(--radius-sm);
  }
  .hint-count {
    /* Pushed to the right edge; the chevron + label lead the row. */
    margin-left: 0.5rem;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    background: color-mix(in srgb, var(--color-status-expired) 12%, transparent);
    border-radius: var(--radius-pill);
    padding: 0 6px;
    min-width: 1.2em;
    text-align: center;
  }
  /* Degenerate / low-signal counts are methodological, never a warning hue. */
  .signal-note .hint-count {
    background: color-mix(in srgb, var(--color-fg-subtle) 14%, transparent);
  }
  .hint-chevron {
    /* Leads the row (flex order) so the disclosure clearly reads as expandable;
       full-strength so it is not missed (was a far-right, dim arrow). */
    order: -1;
    flex-shrink: 0;
    color: var(--color-fg);
    font-size: 1.1rem;
    line-height: 1;
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .hint-disclosure[open] .hint-chevron {
    transform: rotate(90deg);
  }
  @media (prefers-reduced-motion: reduce) {
    .hint-chevron {
      transition: none;
    }
  }
  .hint-disclosure .partial-hint-body {
    margin-top: var(--space-1);
  }
  .partial-hint-body {
    margin: 0;
    font-size: var(--font-size-xs);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1 1 auto;
    min-width: 0;
  }
  .partial-hint-lead {
    margin: 0;
  }
  .partial-hint-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .partial-metric-row {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .partial-metric {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    background: color-mix(in srgb, var(--color-status-expired) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-status-expired) 24%, var(--color-border));
    border-radius: var(--radius-sm);
    padding: 0 4px;
    white-space: nowrap;
  }
  .partial-metric-detail {
    color: var(--color-fg-subtle);
  }
  .partial-metric-detail strong {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-semibold);
  }

  /* Task A — degenerate / low-signal disclosure. A constant dimension is a
     methodological observation, never a defect, so this note uses a
     perceptually-neutral dim accent — never the warning hue (ADR-039
     METHODOLOGICAL-NOT-WARNING). */
  .signal-note .signal-chip {
    color: var(--color-fg-muted);
    background: color-mix(in srgb, var(--color-fg-subtle) 8%, transparent);
    border-color: color-mix(in srgb, var(--color-fg-subtle) 24%, var(--color-border));
  }
</style>

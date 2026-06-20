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
  import { metricLabel, fieldLabel, dimensionLabel } from '$lib/state/labels.svelte';
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
    scopedSourceNames,
    scopedSourceCount,
    scopeAvailableSet,
    configParams
  }: Props = $props();

  // Task A — disclosure rows for constant ("no signal") dimensions. Metric
  // degeneracy is relevant only where the view binds metrics; metadata
  // degeneracy only where it groups by a field — so each is gated at the source.
  const metricBinds = $derived(viewUsesMetric || configParams.includes('scatterAxes'));
  const degenerateRows = $derived<{ name: string; value: string }[]>([
    ...(metricBinds
      ? (degenerateMetrics ?? []).map((d) => ({
          name: d.metricName,
          value: formatConstantValue(d.value)
        }))
      : []),
    ...(viewUsesMetadataField
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
    ...(viewUsesMetadataField
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

<!-- ADR-038 — metadata withholding (mirror of the metric hint). -->
{#if viewUsesMetadataField && partialMetadataFields.length > 0}
  <div class="ctrl-row partial-hint" role="note">
    <span class="ctrl-eyebrow">{m.levers_withheld_eyebrow()}</span>
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
  </div>
{/if}

<!-- Phase 123c (C1) + ADR-038 — partial-metric hint (metrics on only SOME
     scoped sources). -->
{#if partialMetrics.length > 0 && (viewUsesMetric || configParams.includes('scatterAxes'))}
  <div class="ctrl-row partial-hint" role="note">
    <span class="ctrl-eyebrow">{m.levers_withheld_eyebrow()}</span>
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
  </div>
{/if}

<!-- Task A — degenerate (constant, "no signal") dimensions. Present but
     signal-free, so dropped from the picker; disclosed here with the constant
     value (ADR-039 DISCLOSE-NEVER-COERCE). Neutral hue — methodological, not a
     warning (METHODOLOGICAL-NOT-WARNING). -->
{#if degenerateRows.length > 0}
  <div class="ctrl-row partial-hint signal-note" role="note">
    <span class="ctrl-eyebrow">{m.levers_degenerate_eyebrow()}</span>
    <div class="partial-hint-body">
      <p class="partial-hint-lead">
        {degenerateRows.length === 1
          ? m.levers_degenerate_lead_one({ count: degenerateRows.length })
          : m.levers_degenerate_lead_other({ count: degenerateRows.length })}
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
      </ul>
    </div>
  </div>
{/if}

<!-- Task A — low-signal (near-constant) metrics + fields. NOT dropped (a
     rare-but-real minority is still real signal); disclosed so the reader knows
     the dimension is effectively constant in THIS scope before binding it. -->
{#if lowSignalRows.length > 0}
  <div class="ctrl-row partial-hint signal-note" role="note">
    <span class="ctrl-eyebrow">{m.levers_lowsignal_eyebrow()}</span>
    <div class="partial-hint-body">
      <p class="partial-hint-lead">
        {lowSignalRows.length === 1
          ? m.levers_lowsignal_lead_one({ count: lowSignalRows.length })
          : m.levers_lowsignal_lead_other({ count: lowSignalRows.length })}
      </p>
      <ul class="partial-hint-list" role="list">
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
  </div>
{/if}

<style>
  /* Phase 123c (C1) — partial-metric (withheld) hint. A calm, low-emphasis note
     in the warning hue; never an error — the withholding is a deliberate honesty
     guard. The row itself overrides the base alignment. */
  .partial-hint {
    align-items: flex-start;
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

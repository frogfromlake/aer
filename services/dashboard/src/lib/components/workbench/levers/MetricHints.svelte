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
    resetMetricForScope as resetMetricForScopeOf
  } from '$lib/workbench/panel-controls-derive';
  import type { ScopeAvailableMetricsDto, ScopeAvailableMetadataDto } from '$lib/api/queries';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
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
    scopedSourceNames,
    scopedSourceCount,
    scopeAvailableSet,
    configParams
  }: Props = $props();

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
            <code class="partial-metric">{pf.field}</code>
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
            <code class="partial-metric">{pm.metricName}</code>
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
</style>

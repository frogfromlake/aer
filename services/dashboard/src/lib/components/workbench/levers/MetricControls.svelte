<script lang="ts">
  // MetricControls — Phase 141 (extracted from PanelControls).
  //
  // The "what is measured / grouped by" levers: the analytical + metadata metric
  // picker (scalar views), the categorical metadata FIELD picker (field-driven
  // views), and the partial-metric / partial-field "withheld" hints with their
  // "show anyway" toggle (ADR-038 — the default is always the intersection so no
  // cell renders silently empty; partials are offered only on opt-in). Owns the
  // two capability-driven reconcile effects (snap a scope-invalid metric/field
  // back to a valid one).
  //
  // The parent passes the already-computed availability data (the queries live
  // there, shared across levers); this child derives the picker lists from it.
  import type { Panel, Presentation } from '$lib/state/url-internals';
  import { isMetadataMetric } from '$lib/presentations';
  import {
    buildMetricList,
    buildMetadataFields,
    firstMetadataField,
    isScopeAvailable,
    missingSourcesFor as missingSourcesForOf,
    resetMetricForScope as resetMetricForScopeOf,
    type ScopeGate
  } from '$lib/workbench/panel-controls-derive';
  import type { ScopeAvailableMetricsDto, ScopeAvailableMetadataDto } from '$lib/api/queries';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    view: Presentation;
    viewUsesMetric: boolean;
    viewUsesMetadataField: boolean;
    availableMetricNames: string[];
    availableMetadataFields: readonly string[];
    partialMetrics: ScopeAvailableMetricsDto['partial'];
    partialMetadataFields: ScopeAvailableMetadataDto['partial'];
    scopedSourceNames: readonly string[];
    scopedSourceCount: number;
    scopeGate: ScopeGate;
    scopeAvailableSet: Set<string> | null;
    /** Offerable categorical fields (shared — computed once in the parent). */
    offerableFields: string[];
    /** metadataAvail query resolved (gates the field reconcile effect). */
    metadataResolved: boolean;
    /** For the partial-metric hint gate (scatter binds metrics too). */
    configParams: readonly string[];
  }

  let {
    panelPath,
    boundPanel,
    view,
    viewUsesMetric,
    viewUsesMetadataField,
    availableMetricNames,
    availableMetadataFields,
    partialMetrics,
    partialMetadataFields,
    scopedSourceNames,
    scopedSourceCount,
    scopeGate,
    scopeAvailableSet,
    offerableFields,
    metadataResolved,
    configParams
  }: Props = $props();

  const activeMetric = $derived(boundPanel.metric);
  const activeShowWithheld = $derived(boundPanel.showWithheld === true);

  // Metric list (DEFAULT first, then API order, filtered through the view's
  // metric→presentation compatibility + the scope gate), split into analytical
  // measures and publisher-declared metadata metrics.
  const metrics = $derived.by<string[]>(() =>
    buildMetricList({ view, availableMetricNames, gate: scopeGate, activeMetric })
  );
  const analyticalMetrics = $derived<string[]>(metrics.filter((mn) => !isMetadataMetric(mn)));
  const metadataMetrics = $derived<string[]>(metrics.filter((mn) => isMetadataMetric(mn)));

  // Field picker list: offerable fields + the active field surfaced.
  const metadataFields = $derived.by<string[]>(() =>
    buildMetadataFields({ viewUsesMetadataField, offerable: offerableFields, activeMetric })
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

  function pickMetric(name: string) {
    if (name === activeMetric) return;
    updatePanel(panelPath, (p) => ({ ...p, metric: name }));
  }

  // Issue 6 — "show anyway": offer the withheld (partial) metrics. Turning it
  // OFF snaps a now-unofferable metric back to a scope-valid default.
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

  // Capability-driven default reconcile: when the active SCALAR metric is not
  // available for the panel's scope, snap it to a scope-valid metric. Guards:
  // only metric-consuming views; only real /metrics/available scalar metrics;
  // skipped while "show anyway" is on. Converges (reset target is scope-valid).
  $effect(() => {
    if (!viewUsesMetric) return;
    if (scopeAvailableSet === null || activeShowWithheld) return;
    if (isScopeAvailable(activeMetric, scopeGate)) return;
    if (!availableMetricNames.includes(activeMetric)) return;
    const next = resetMetricForScope();
    if (next && next !== activeMetric) {
      updatePanel(panelPath, (p) => ({ ...p, metric: next }));
    }
  });

  // Categorical-field reconcile (Phase 133 Issue 6 + ADR-038): once availability
  // is in hand, if the active field is not offerable, snap to the first offerable
  // field — or to '' (Negative Space) when the scope shares no field.
  $effect(() => {
    if (!viewUsesMetadataField) return;
    if (!metadataResolved) return;
    if (activeMetric && offerableFields.includes(activeMetric)) return;
    const next = firstMetadataField(availableMetadataFields);
    if (next !== activeMetric) {
      updatePanel(panelPath, (p) => ({ ...p, metric: next }));
    }
  });
</script>

{#if viewUsesMetric}
  <LeverRow
    eyebrow={m.levers_metric_eyebrow()}
    role="radiogroup"
    ariaLabel={m.levers_metric_aria()}
  >
    {#snippet options()}
      {#each analyticalMetrics as mn (mn)}
        <LeverButton
          role="radio"
          active={activeMetric === mn}
          variant="metric-btn"
          onclick={() => pickMetric(mn)}
        >
          <code>{mn}</code>
        </LeverButton>
      {/each}
      {#if metadataMetrics.length > 0}
        <span class="metric-group-label" aria-hidden="true">{m.levers_metric_group_metadata()}</span
        >
        {#each metadataMetrics as mn (mn)}
          <LeverButton
            role="radio"
            active={activeMetric === mn}
            variant="metric-btn metadata-metric"
            title={m.levers_metric_metadata_title()}
            onclick={() => pickMetric(mn)}
          >
            <code>{mn}</code>
          </LeverButton>
        {/each}
      {/if}
    {/snippet}
  </LeverRow>
{/if}

<!-- Phase 133 — categorical metadata FIELD picker (field-driven views). The
     field is the GROUPING dimension and rides in Panel.metric. -->
{#if viewUsesMetadataField}
  <LeverRow
    eyebrow={m.levers_groupby_eyebrow()}
    role="group"
    ariaLabel={m.levers_groupby_aria()}
    rowClass="config-row"
  >
    {#if metadataFields.length > 0}
      <select
        class="config-select"
        value={activeMetric}
        onchange={(e) => pickMetric((e.currentTarget as HTMLSelectElement).value)}
        onclick={(e) => e.stopPropagation()}
        aria-label={m.levers_groupby_select_aria()}
      >
        {#each metadataFields as f (f)}
          <option value={f}>{f}</option>
        {/each}
      </select>
    {:else}
      <span class="field-empty">{m.levers_groupby_empty()}</span>
    {/if}
  </LeverRow>

  <!-- ADR-038 — metadata withholding (mirror of the metric hint). -->
  {#if partialMetadataFields.length > 0}
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
          {activeShowWithheld
            ? m.levers_withheld_showing_fields()
            : m.levers_withheld_show_anyway()}
        </LeverButton>
      </div>
    </div>
  {/if}
{/if}

<!-- Phase 123c (C1) + ADR-038 — partial-metric hint (metrics on only SOME
     scoped sources). "Show anyway" offers them; the panel then renders only the
     sources that carry the chosen metric. -->
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
  /* Phase 133 (Issue 4) — metadata-metric group inline label. */
  .metric-group-label {
    align-self: center;
    margin-left: var(--space-2);
    padding-left: var(--space-2);
    border-left: 1px solid var(--color-border);
    font-size: var(--font-size-2xs, 10px);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-fg-subtle, var(--color-fg-muted));
  }

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

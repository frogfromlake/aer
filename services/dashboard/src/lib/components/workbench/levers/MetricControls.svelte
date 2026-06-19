<script lang="ts">
  // MetricControls — Phase 141 (extracted from PanelControls); Phase 151 reduced
  // to the metric/field PICKER (now a compact dropdown that sits beside View on
  // one row). The "withheld / show anyway" hints moved to sibling MetricHints.
  //
  // Renders ONE of: the analytical + metadata metric dropdown (scalar views), or
  // the categorical metadata FIELD dropdown (field-driven views — the field is
  // the grouping dimension and rides in Panel.metric). Owns the two
  // capability-driven reconcile effects (snap a scope-invalid metric/field back
  // to a valid one). The parent passes already-computed availability data.
  import type { Panel, Presentation } from '$lib/state/url-internals';
  import { isMetadataMetric } from '$lib/presentations';
  import {
    buildMetricList,
    buildMetadataFields,
    firstMetadataField,
    isScopeAvailable,
    resetMetricForScope as resetMetricForScopeOf,
    type ScopeGate
  } from '$lib/workbench/panel-controls-derive';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    view: Presentation;
    viewUsesMetric: boolean;
    viewUsesMetadataField: boolean;
    availableMetricNames: string[];
    scopeGate: ScopeGate;
    scopeAvailableSet: Set<string> | null;
    /** Offerable categorical fields (shared — computed once in the parent). */
    offerableFields: string[];
    /** metadataAvail query resolved (gates the field reconcile effect). */
    metadataResolved: boolean;
  }

  let {
    panelPath,
    boundPanel,
    view,
    viewUsesMetric,
    viewUsesMetadataField,
    availableMetricNames,
    scopeGate,
    scopeAvailableSet,
    offerableFields,
    metadataResolved
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
  //
  // Default from `offerableFields` (the exact dropdown list), NOT the scope's
  // intersection set: under "show anyway" the picker offers the withheld
  // categorical fields even though the scope shares none, so seeding from the
  // intersection (which is []) would leave the Group-by picker empty. Without
  // "show anyway" offerableFields == the intersection, preserving the empty state.
  $effect(() => {
    if (!viewUsesMetadataField) return;
    if (!metadataResolved) return;
    if (activeMetric && offerableFields.includes(activeMetric)) return;
    const next = firstMetadataField(offerableFields);
    if (next !== activeMetric) {
      updatePanel(panelPath, (p) => ({ ...p, metric: next }));
    }
  });
</script>

<!-- Phase 151 — the metric/field picker is a dropdown so it sits beside View on
     one row. Scalar views show the metric select; field-driven views show the
     categorical FIELD select (the grouping dimension, riding in Panel.metric). -->
{#if viewUsesMetric}
  <div class="ctrl-group">
    <span class="ctrl-eyebrow">{m.levers_metric_eyebrow()}</span>
    <select
      class="config-select"
      value={activeMetric}
      onchange={(e) => pickMetric((e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_metric_aria()}
    >
      {#each analyticalMetrics as mn (mn)}
        <option value={mn}>{mn}</option>
      {/each}
      {#if metadataMetrics.length > 0}
        <optgroup label={m.levers_metric_group_metadata()}>
          {#each metadataMetrics as mn (mn)}
            <option value={mn}>{mn}</option>
          {/each}
        </optgroup>
      {/if}
    </select>
  </div>
{:else if viewUsesMetadataField}
  <div class="ctrl-group">
    <span class="ctrl-eyebrow">{m.levers_groupby_eyebrow()}</span>
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
  </div>
{/if}

<style>
  .ctrl-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
</style>

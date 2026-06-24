<script lang="ts">
  // ViewControls — Phase 141 (extracted from PanelControls).
  //
  // The View (Darstellung) lever: the presentations the active Pillar owns
  // (ADR-035 — the pillar is fixed by the presentation set). Owns the full
  // view-switch reconciliation (`pickView` → reconcilePanelForView): discard
  // presentation-specific overrides, reconcile metric↔field, seed channels /
  // metric-set / field-chain. The reactive reconcile inputs are passed in as
  // props (the parent computes them once and shares them across levers).
  import type { PresentationDefinition } from '$lib/presentations';
  import type { Presentation, ScopeGroup } from '$lib/state/url-internals';
  import { reconcilePanelForView } from '$lib/workbench/panel-controls-derive';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    panelPath: PanelPath;
    presentations: PresentationDefinition[];
    activePresentation: PresentationDefinition;
    scalarMetricOptions: string[];
    offerableFields: string[];
    availableMetricNames: string[];
    availableMetadataFields: readonly string[];
    /** Phase 148g — the scope's all-source metric intersection + scope groups +
     *  show-anyway flag, so the view-switch metric reset is SCOPE-AWARE (a
     *  cross-probe panel defaults to the multilingual backbone, never a
     *  scope-withheld German-only metric). */
    scopeAvailableSet: Set<string> | null;
    scopes: readonly ScopeGroup[];
    showWithheld: boolean;
  }

  let {
    panelPath,
    presentations,
    activePresentation,
    scalarMetricOptions,
    offerableFields,
    availableMetricNames,
    availableMetadataFields,
    scopeAvailableSet,
    scopes,
    showWithheld
  }: Props = $props();

  function pickView(id: Presentation) {
    if (id === activePresentation.id) return;
    updatePanel(panelPath, (p) =>
      reconcilePanelForView(p, id, {
        presentations,
        prevUsesMetadataField: activePresentation.usesMetadataField ?? false,
        scalarMetricOptions,
        offerableFields,
        availableMetricNames,
        availableMetadataFields,
        scopeAvailableSet,
        scopes,
        showWithheld
      })
    );
  }
</script>

<!-- Phase 151 — View is a dropdown (was a radio cluster) so it can sit beside the
     Metric dropdown on one row, saving vertical space. -->
<div class="ctrl-group">
  <span class="ctrl-eyebrow">{m.levers_view_eyebrow()}</span>
  <select
    class="config-select"
    value={activePresentation.id}
    onchange={(e) => pickView((e.currentTarget as HTMLSelectElement).value as Presentation)}
    onclick={(e) => e.stopPropagation()}
    aria-label={m.levers_view_aria()}
  >
    {#each presentations as p (p.id)}
      <option value={p.id} title={p.description}>{p.label}</option>
    {/each}
  </select>
</div>

<style>
  .ctrl-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
</style>

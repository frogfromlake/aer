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
  import type { Presentation } from '$lib/state/url-internals';
  import { reconcilePanelForView } from '$lib/workbench/panel-controls-derive';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    presentations: PresentationDefinition[];
    activePresentation: PresentationDefinition;
    scalarMetricOptions: string[];
    offerableFields: string[];
    availableMetricNames: string[];
    availableMetadataFields: readonly string[];
  }

  let {
    panelPath,
    presentations,
    activePresentation,
    scalarMetricOptions,
    offerableFields,
    availableMetricNames,
    availableMetadataFields
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
        availableMetadataFields
      })
    );
  }
</script>

<LeverRow eyebrow="View" role="radiogroup" ariaLabel="View">
  {#snippet options()}
    {#each presentations as p (p.id)}
      <LeverButton
        role="radio"
        active={activePresentation.id === p.id}
        title={p.description}
        onclick={() => pickView(p.id)}
      >
        {p.label}
      </LeverButton>
    {/each}
  {/snippet}
</LeverRow>

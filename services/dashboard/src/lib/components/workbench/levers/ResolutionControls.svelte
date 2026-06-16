<script lang="ts">
  // ResolutionControls — Phase 141 (extracted from PanelControls).
  //
  // The time-resolution lever (Hourly · Daily · Weekly · Monthly). Rendered by
  // the parent only when the active view bins along a time axis
  // (`usesResolution`). Self-contained, panel-bound.
  import type { Panel, Resolution } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
  }

  let { panelPath, boundPanel }: Props = $props();

  const RESOLUTIONS: ReadonlyArray<{ id: Resolution; label: string }> = [
    { id: 'hourly', label: 'Hourly' },
    { id: 'daily', label: 'Daily' },
    { id: 'weekly', label: 'Weekly' },
    { id: 'monthly', label: 'Monthly' }
  ];

  const activeResolution = $derived<Resolution>(boundPanel.resolution ?? 'daily');

  function pickResolution(next: Resolution) {
    if (next === activeResolution) return;
    updatePanel(panelPath, (p) => ({ ...p, resolution: next }));
  }
</script>

<LeverRow eyebrow="Resolution" role="radiogroup" ariaLabel="Time resolution">
  {#snippet options()}
    {#each RESOLUTIONS as r (r.id)}
      <LeverButton
        role="radio"
        active={activeResolution === r.id}
        onclick={() => pickResolution(r.id)}
      >
        {r.label}
      </LeverButton>
    {/each}
  {/snippet}
</LeverRow>

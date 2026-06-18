<script lang="ts">
  // ResolutionControls — Phase 141 (extracted from PanelControls).
  //
  // The time-resolution lever (Hourly · Daily · Weekly · Monthly). Rendered by
  // the parent only when the active view bins along a time axis
  // (`usesResolution`). Self-contained, panel-bound.
  import type { Panel, Resolution } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
  }

  let { panelPath, boundPanel }: Props = $props();

  const RESOLUTIONS: readonly Resolution[] = ['hourly', 'daily', 'weekly', 'monthly'];
  const resolutionLabel = (id: Resolution): string =>
    id === 'hourly'
      ? m.levers_resolution_hourly()
      : id === 'weekly'
        ? m.levers_resolution_weekly()
        : id === 'monthly'
          ? m.levers_resolution_monthly()
          : m.levers_resolution_daily();

  const activeResolution = $derived<Resolution>(boundPanel.resolution ?? 'daily');

  function pickResolution(next: Resolution) {
    if (next === activeResolution) return;
    updatePanel(panelPath, (p) => ({ ...p, resolution: next }));
  }
</script>

<LeverRow
  eyebrow={m.levers_resolution_eyebrow()}
  role="radiogroup"
  ariaLabel={m.levers_resolution_aria()}
>
  {#snippet options()}
    {#each RESOLUTIONS as r (r)}
      <LeverButton role="radio" active={activeResolution === r} onclick={() => pickResolution(r)}>
        {resolutionLabel(r)}
      </LeverButton>
    {/each}
  {/snippet}
</LeverRow>

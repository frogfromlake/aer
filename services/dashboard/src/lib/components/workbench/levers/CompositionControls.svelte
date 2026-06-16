<script lang="ts">
  // CompositionControls — Phase 141 (extracted from PanelControls).
  //
  // The Composition lever: Merged · Split (· Overlay where supported), plus the
  // Split-direction sub-lever (↔ Horizontal / ↕ Vertical) shown only while Split
  // is active. Self-contained — owns its state reads, handlers, markup, and the
  // two-group composition-row CSS. Behaviour-preserving move of the original
  // composition row.
  import type { Panel } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    /** Phase 131 — Overlay is meaningful only for the time-series cell. */
    supportsOverlay: boolean;
  }

  let { panelPath, boundPanel, supportsOverlay }: Props = $props();

  const activeSplitDirection = $derived<'horizontal' | 'vertical'>(
    boundPanel.splitDirection ?? 'horizontal'
  );

  function pickComposition(next: 'merged' | 'split' | 'overlay') {
    if (boundPanel.composition === next) return;
    updatePanel(panelPath, (p) => ({ ...p, composition: next }));
  }
  function pickSplitDirection(next: 'horizontal' | 'vertical') {
    if ((boundPanel.splitDirection ?? 'horizontal') === next) return;
    updatePanel(panelPath, (p) => ({ ...p, splitDirection: next }));
  }
</script>

<!-- Phase 122k §14c finding 1 — two labelled control groups (Composition /
     Direction) separated by a thin vertical divider; Direction appears only
     when Split is active. Merged is its own button, never glued to Vertical. -->
<div class="ctrl-row composition-row">
  <div class="comp-group" role="radiogroup" aria-label="Composition">
    <span class="ctrl-eyebrow">Composition</span>
    <div class="ctrl-options">
      <LeverButton
        role="radio"
        active={boundPanel.composition === 'split'}
        title="One Cell per source or per ScopeGroup (small-multiples)"
        onclick={() => pickComposition('split')}
      >
        Split
      </LeverButton>
      {#if supportsOverlay}
        <LeverButton
          role="radio"
          active={boundPanel.composition === 'overlay'}
          title="One Cell — sources rendered as separate viridis-coloured lines on a shared canvas"
          onclick={() => pickComposition('overlay')}
        >
          Overlay
        </LeverButton>
      {/if}
      <LeverButton
        role="radio"
        active={boundPanel.composition === 'merged'}
        title="One Cell — sources aggregated into a single joint-corpus chart"
        onclick={() => pickComposition('merged')}
      >
        Merged
      </LeverButton>
    </div>
  </div>

  {#if boundPanel.composition === 'split'}
    <div class="comp-divider" aria-hidden="true"></div>
    <div class="comp-group" role="radiogroup" aria-label="Split direction">
      <span class="ctrl-eyebrow">Direction</span>
      <div class="ctrl-options">
        <LeverButton
          role="radio"
          active={activeSplitDirection === 'horizontal'}
          title="Arrange split cells side-by-side"
          ariaLabel="Split direction: horizontal"
          onclick={() => pickSplitDirection('horizontal')}
        >
          ↔ Horizontal
        </LeverButton>
        <LeverButton
          role="radio"
          active={activeSplitDirection === 'vertical'}
          title="Stack split cells vertically"
          ariaLabel="Split direction: vertical"
          onclick={() => pickSplitDirection('vertical')}
        >
          ↕ Vertical
        </LeverButton>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Phase 122k §14c finding 1 — composition-row layout. */
  .composition-row {
    align-items: flex-start;
  }
  .comp-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .comp-group > .ctrl-options {
    display: flex;
    gap: 2px;
  }
  .comp-divider {
    align-self: stretch;
    width: 1px;
    background: color-mix(in srgb, var(--color-border) 60%, transparent);
    margin: 0 var(--space-2);
  }
</style>

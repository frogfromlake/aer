<script lang="ts">
  // Phase 122i / ADR-034 — Multi-Panel Workbench: Window renderer.
  //
  // A Window holds 1..8 Panels arranged side-by-side. The first ship
  // renders the active window's panel grid. Multi-window tab UI lands
  // in a Slice 4 follow-up once Panel editing affordances are wired —
  // until then, a Pillar holds at most one Window in practice (entry
  // paths always seed a single Window with a single Panel).
  //
  // Layout: CSS grid with `auto-fit, minmax(28rem, 1fr)`. At default
  // viewport widths (~1440px and the SideRail consuming ~184px) this
  // fits 3-4 panels before horizontal scrolling kicks in. The cap is
  // enforced at the URL-state layer (MAX_PANELS_PER_WINDOW = 8).
  import type { FetchContext, ProbeDossierDto } from '$lib/api/queries';
  import type { PillarState } from '$lib/state/url-internals';
  import PanelHost from './PanelHost.svelte';

  interface Props {
    pillarState: PillarState;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
  }

  let { pillarState, dossier, ctx, windowStart, windowEnd }: Props = $props();

  // Resolve the active window from the PillarState. Out-of-range
  // activeWindowIndex (e.g. after URL surgery) falls back to the first
  // window so the user is never stranded.
  const activeWindow = $derived(
    pillarState.windows[pillarState.activeWindowIndex] ?? pillarState.windows[0] ?? null
  );

  const panelCount = $derived(activeWindow?.panels.length ?? 0);
</script>

<section class="window-host" aria-label="Workbench window" data-panel-count={panelCount}>
  {#if !activeWindow || panelCount === 0}
    <p class="muted">This window has no panels.</p>
  {:else}
    <div class="panel-grid" data-cols={Math.min(panelCount, 4)}>
      {#each activeWindow.panels as panel, i (i)}
        <PanelHost {panel} {dossier} {ctx} {windowStart} {windowEnd} />
      {/each}
    </div>
  {/if}
</section>

<style>
  .window-host {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: 1;
    min-height: 0;
  }

  .panel-grid {
    display: grid;
    /* 1..4 panels: equal columns; 5..8 panels: horizontal scroll with
       fixed per-panel min width. The grid-template-columns rules below
       branch on data-cols which mirrors the panel count clamped to 4. */
    grid-auto-flow: column;
    grid-auto-columns: minmax(28rem, 1fr);
    gap: var(--space-4);
    overflow-x: auto;
    align-items: stretch;
  }

  .panel-grid[data-cols='1'] {
    grid-auto-columns: minmax(28rem, 1fr);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

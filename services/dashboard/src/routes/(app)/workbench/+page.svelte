<script lang="ts">
  // Workbench — Phase 122h / ADR-033.
  //
  // The Workbench is AĒR's analytical surface. It carries:
  //   - The PillarSwitch at the top — three tiles selecting Aleph,
  //     Episteme, or Rhizome.
  //   - One of three Pillar Shells in the body, chosen by url.viewingMode.
  //   - The WorkbenchScopeBar at the bottom — Probes / Sources / Functions
  //     / Window chips.
  //
  // When the user lands without a probe in scope, an empty-scope surface
  // invites them either to return to the Atmosphere globe or to pick a
  // probe from the in-rail ProbePicker (highlighted in that state).
  import { urlState } from '$lib/state/url.svelte';
  import { getPillar } from '$lib/viewmodes';
  import PillarSwitch from '$lib/components/chrome/PillarSwitch.svelte';
  import WorkbenchScopeBar from '$lib/components/chrome/WorkbenchScopeBar.svelte';
  import AlephShell from '$lib/components/workbench/AlephShell.svelte';
  import EpistemeShell from '$lib/components/workbench/EpistemeShell.svelte';
  import RhizomeShell from '$lib/components/workbench/RhizomeShell.svelte';

  const url = $derived(urlState());

  // Scope is sourced exclusively from the rune store (`urlState`):
  // - The (app)/+layout afterNavigate hook re-hydrates the store on every
  //   SvelteKit nav.
  // - The store is eagerly hydrated at module load for first paint.
  // - In-page mutations (e.g. removing a probe chip in WorkbenchScopeBar)
  //   write via setUrl → history.replaceState, which intentionally bypasses
  //   the router and therefore does NOT update `$app/state.page.url`.
  // Reading `page.url.searchParams` here would shadow the cleared store
  // with a stale URL and keep charts rendered after the user clears scope.
  const probes = $derived(url.probeIds);

  const activePillar = $derived(getPillar(url.viewingMode));
  const hasScope = $derived(probes.length > 0);
</script>

<svelte:head>
  <title>AĒR — Workbench · {activePillar.label}</title>
</svelte:head>

<main class="workbench-main" id="main-workbench">
  {#if !hasScope}
    <div class="empty-scope">
      <h1>Workbench</h1>
      <p class="muted">
        Pick a probe first — click a probe glyph on the Atmosphere, or use the highlighted Probe
        Picker in the side rail.
      </p>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Atmosphere route -->
      <a class="back-to-atmos" href="/">→ Back to Atmosphere</a>
    </div>
  {:else}
    <PillarSwitch />
    <!-- ScopeBar sits ABOVE the pillar body (Finding round 2 §D) — the
         user has to know which probes/sources/functions/window are in
         scope BEFORE reading the cell. Full viewport-width (the bar's
         own .scope-bar styling does not constrain margin). -->
    <WorkbenchScopeBar />
    <div class="pillar-body">
      {#if activePillar.id === 'aleph'}
        <AlephShell probeIds={probes} />
      {:else if activePillar.id === 'episteme'}
        <EpistemeShell probeIds={probes} />
      {:else}
        <RhizomeShell probeIds={probes} />
      {/if}
    </div>
  {/if}
</main>

<style>
  .workbench-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: 0;
    right: 0;
    overflow-y: auto;
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    padding: var(--space-5);
    gap: var(--space-4);
  }

  .empty-scope {
    margin: auto;
    text-align: center;
    max-width: 32rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
  }

  .empty-scope h1 {
    font-size: var(--font-size-3xl);
    font-weight: var(--font-weight-semibold);
    margin: 0;
    color: var(--color-fg);
  }

  .back-to-atmos {
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }

  .back-to-atmos:hover,
  .back-to-atmos:focus-visible {
    color: var(--color-fg);
    border-bottom-color: var(--color-fg);
    outline: none;
  }

  /* Pillar-body slot — hosts AlephShell / EpistemeShell / RhizomeShell. */
  .pillar-body {
    flex: 1;
    min-height: 24rem;
    display: flex;
    flex-direction: column;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

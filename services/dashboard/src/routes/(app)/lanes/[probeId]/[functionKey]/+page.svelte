<script lang="ts">
  // Surface II — Function Lane page (Phase 106).
  // Renders the baseline time-series view for a single WP-001 discourse function.
  // The view-mode matrix (Phase 107) lands on top of this shell.
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { probeDossierQuery, type ProbeDossierDto, type QueryOutcome } from '$lib/api/queries';
  import FunctionLaneShell from '$lib/components/lanes/FunctionLaneShell.svelte';
  import { urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';

  const VALID_FUNCTION_KEYS = new Set([
    'epistemic_authority',
    'power_legitimation',
    'cohesion_identity',
    'subversion_friction'
  ]);

  const ctx = { baseUrl: '/api/v1' };

  let probeId = $derived(page.params.probeId ?? '');
  let functionKey = $derived(page.params.functionKey ?? '');
  let validFunctionKey = $derived(VALID_FUNCTION_KEYS.has(functionKey) ? functionKey : null);

  const url = $derived(urlState());

  let windowMs = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      start: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS).toISOString(),
      end: new Date(Number.isFinite(toMs) ? toMs : now).toISOString()
    };
  });

  // Re-use dossier query (served from TanStack cache — same key as the dossier page).
  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probeId, {
        windowStart: windowMs.start,
        windowEnd: windowMs.end
      });
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    }
  );

  let dossier = $derived(dossierQ.data?.kind === 'success' ? dossierQ.data.data : null);

  const FUNCTION_LABELS: Record<string, string> = {
    epistemic_authority: 'Epistemic Authority',
    power_legitimation: 'Power Legitimation',
    cohesion_identity: 'Cohesion & Identity',
    subversion_friction: 'Subversion & Friction'
  };

  let functionLabel = $derived(FUNCTION_LABELS[functionKey] ?? functionKey);
</script>

<svelte:head>
  <title>AĒR — {functionLabel} · {probeId}</title>
</svelte:head>

<main class="lane-main" id="main-lane">
  {#if validFunctionKey === null}
    <div class="state-slot">
      <p class="error">Unknown discourse function: <code>{functionKey}</code></p>
    </div>
  {:else if dossierQ.isPending}
    <div class="state-slot">
      <p class="muted" aria-busy="true">Loading…</p>
    </div>
  {:else}
    <FunctionLaneShell
      {functionKey}
      {dossier}
      {ctx}
      windowStart={windowMs.start}
      windowEnd={windowMs.end}
      sourceId={url.sourceId}
    />
  {/if}
</main>

<style>
  .lane-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    overflow-y: auto;
    background: var(--color-bg);
  }

  .state-slot {
    display: grid;
    place-items: center;
    min-height: 12rem;
    padding: var(--space-6);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .error {
    font-size: var(--font-size-sm);
    color: var(--color-status-expired);
    margin: 0;
  }
</style>

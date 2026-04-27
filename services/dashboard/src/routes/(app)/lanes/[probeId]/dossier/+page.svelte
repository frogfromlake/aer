<script lang="ts">
  // Surface II — Probe Dossier landing (Phase 106).
  // Default landing when a probe is selected from the globe.
  // Consumes /api/v1/probes/{id}/dossier and renders source cards,
  // function-coverage indicator, and per-source article preview lists.
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { probeDossierQuery, type ProbeDossierDto, type QueryOutcome } from '$lib/api/queries';
  import ProbeDossier from '$lib/components/lanes/ProbeDossier.svelte';
  import { untrack } from 'svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';

  const ctx = { baseUrl: '/api/v1' };

  let probeId = $derived(page.params.probeId ?? '');
  const url = $derived(urlState());

  // Sync ?sourceId=… URL param → urlState.sourceIds for deep-link and
  // backward-compat (old bookmarks, satellite-click URL from pre-113d).
  //
  // The url.sourceIds comparison is wrapped in untrack so this effect only
  // re-runs on actual SvelteKit navigations (page.url changes), not when
  // setUrl() mutates internalState via history.replaceState. Without
  // untrack, clicking "Clear scope" would trigger the effect, which would
  // read the stale page.url (bypassed by replaceState) and immediately
  // re-apply the old sourceId, making the button appear broken.
  $effect(() => {
    const fromUrl = page.url.searchParams.get('sourceId');
    if (!fromUrl) return;
    const ids = fromUrl.split(',').filter(Boolean);
    if (ids.length === 0) return;
    untrack(() => {
      if (url.sourceIds.join(',') !== ids.join(',')) {
        setUrl({ sourceIds: ids });
      }
    });
  });

  let windowMs = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      start: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS).toISOString(),
      end: new Date(Number.isFinite(toMs) ? toMs : now).toISOString()
    };
  });

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probeId, {
        windowStart: windowMs.start,
        windowEnd: windowMs.end
      });
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    }
  );
</script>

<svelte:head>
  <title>AĒR — Dossier · {probeId}</title>
</svelte:head>

<main class="dossier-main" id="main-dossier">
  {#if dossierQ.isPending}
    <div class="state-slot">
      <p class="muted" aria-busy="true">Loading probe dossier…</p>
    </div>
  {:else if dossierQ.isError}
    <div class="state-slot">
      <p class="error">Failed to load dossier. Check network connectivity.</p>
    </div>
  {:else if dossierQ.data?.kind === 'success'}
    <ProbeDossier
      dossier={dossierQ.data.data}
      {ctx}
      windowStart={windowMs.start}
      windowEnd={windowMs.end}
    />
  {:else if dossierQ.data?.kind === 'refusal'}
    <div class="state-slot">
      <p class="muted">Probe not found or access refused: {dossierQ.data.message}</p>
    </div>
  {:else if dossierQ.data?.kind === 'network-error'}
    <div class="state-slot">
      <p class="error">Network error: {dossierQ.data.message}</p>
    </div>
  {/if}
</main>

<style>
  .dossier-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    overflow-y: auto;
    background: var(--color-bg);
    padding: var(--space-6);
  }

  .state-slot {
    display: grid;
    place-items: center;
    min-height: 12rem;
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

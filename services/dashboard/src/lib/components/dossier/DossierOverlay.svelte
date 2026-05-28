<script lang="ts">
  // Phase 123a — Dossier as a global overlay (ADR-033 amendment).
  //
  // The Dossier is no longer a top-level route. It opens as a global
  // overlay over ANY surface, driven entirely by URL state so it stays
  // deep-linkable:
  //   ?probe=<id>    → mini overlay: one probe, focused + expanded
  //   ?dossier=open  → large catalogue overlay (search/facets land in Slice 2)
  //
  // Hosts the unchanged `ProbeCard`. Pure DOM — fully usable in the
  // no-WebGL2 fallback (independent of the globe engine). Mounted once in
  // the (app) layout so it is available on Atmosphäre, Workbench, Reflexion.
  import { onMount, onDestroy, tick } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import ProbeCard from './ProbeCard.svelte';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  const mode = $derived<'large' | 'mini' | null>(
    url.dossier === 'open' ? 'large' : url.probe ? 'mini' : null
  );
  const isOpen = $derived(mode !== null);

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  // Phase 131a — `undefined` ⇒ whole dataset (BFF treats absent bounds as
  // no filter). The Slice-5 date-range picker will drive `url.from`/`url.to`.
  const windowMs = $derived.by<{ start: string | undefined; end: string | undefined }>(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : NaN;
    const toMs = url.to ? Date.parse(url.to) : NaN;
    return {
      start: Number.isFinite(fromMs) ? new Date(fromMs).toISOString() : undefined,
      end: Number.isFinite(toMs)
        ? new Date(toMs).toISOString()
        : url.from
          ? new Date(now).toISOString()
          : undefined
    };
  });

  // Mini = the one focused probe; Large = the selection shopping-cart
  // (or the whole catalogue when nothing is selected). Slice 2 adds the
  // search/facet surface on top of the large mode.
  const visibleProbes = $derived<ProbeDto[]>(
    mode === 'mini'
      ? probeList.filter((p) => p.probeId === url.probe)
      : url.selectedProbes.length > 0
        ? probeList.filter((p) => url.selectedProbes.includes(p.probeId))
        : probeList
  );

  function startCollapsedFor(probeId: string): boolean {
    if (mode === 'mini') return false; // the single focused probe is expanded
    if (url.selectedProbes.length > 0) return !url.selectedProbes.includes(probeId);
    return true; // plain catalogue browsing starts collapsed
  }

  function close() {
    setUrl({ probe: null, dossier: null });
  }

  // ---- a11y: Esc + focus restore + Tab trap ----------------------------
  let dialogEl = $state<HTMLElement | null>(null);
  let lastFocused: HTMLElement | null = null;

  function hasNestedModal(): boolean {
    // A ProbeCard inside the overlay can open its MetadataCoverageModal
    // (also a [role=dialog]). Defer Esc/trap to that nested modal so a
    // single Esc closes the inner modal first, not the whole overlay.
    return !!dialogEl?.querySelector('[role="dialog"][aria-modal="true"]');
  }

  function onKeydown(e: KeyboardEvent) {
    if (!isOpen) return;
    if (e.key === 'Escape') {
      if (e.defaultPrevented || hasNestedModal()) return;
      e.preventDefault();
      close();
      return;
    }
    if (e.key === 'Tab' && dialogEl && !hasNestedModal()) {
      const focusable = dialogEl.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  $effect(() => {
    if (isOpen) {
      if (!lastFocused) lastFocused = document.activeElement as HTMLElement | null;
      void tick().then(() => dialogEl?.focus());
    } else if (lastFocused) {
      lastFocused.focus();
      lastFocused = null;
    }
  });

  onMount(() => window.addEventListener('keydown', onKeydown));
  onDestroy(() => window.removeEventListener('keydown', onKeydown));
</script>

{#if isOpen}
  <div class="dossier-overlay-backdrop" role="presentation">
    <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
    <section
      class="dossier-overlay"
      class:mini={mode === 'mini'}
      role="dialog"
      aria-modal="true"
      aria-label={mode === 'mini' ? `Dossier · ${url.probe}` : 'Probe catalogue'}
      tabindex="-1"
      bind:this={dialogEl}
    >
      <header class="overlay-header">
        <div class="overlay-titles">
          <p class="eyebrow">Dossier</p>
          <h2>{mode === 'mini' ? url.probe : 'Atmospheric record of AĒR’s probes'}</h2>
        </div>
        <button type="button" class="close-btn" onclick={close} aria-label="Close dossier">×</button
        >
      </header>

      <div class="overlay-body">
        {#if probesQ.isPending}
          <p class="muted" aria-busy="true">Loading probe catalogue…</p>
        {:else if probesQ.isError || probesQ.data?.kind === 'network-error'}
          <p class="error">Could not load the probe catalogue. Check network connectivity.</p>
        {:else if visibleProbes.length === 0}
          <p class="muted">No probes match the current selection.</p>
        {:else}
          <div class="probe-cards">
            {#each visibleProbes as probe (probe.probeId)}
              <ProbeCard
                {probe}
                {ctx}
                windowStart={windowMs.start}
                windowEnd={windowMs.end}
                startCollapsed={startCollapsedFor(probe.probeId)}
              />
            {/each}
          </div>
        {/if}
      </div>
    </section>
  </div>
{/if}

<style>
  .dossier-overlay-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--color-bg) 70%, transparent);
    backdrop-filter: blur(3px);
    /* Below MetadataCoverageModal (z-index 50) so a ProbeCard's metadata
       modal layers above this overlay. */
    z-index: 40;
    display: grid;
    place-items: start center;
    padding: var(--space-5);
    overflow-y: auto;
  }

  .dossier-overlay {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    width: min(90rem, 90%);
    max-height: 90vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-5);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
  }
  .dossier-overlay.mini {
    width: min(60rem, 90%);
  }

  .overlay-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-3);
  }
  .overlay-titles .eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-1) 0;
  }
  .overlay-titles h2 {
    margin: 0;
    font-size: var(--font-size-xl);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    line-height: 1.2;
  }

  .close-btn {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    width: 2rem;
    height: 2rem;
    font-size: 1.25rem;
    cursor: pointer;
    flex-shrink: 0;
  }
  .close-btn:hover,
  .close-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .overlay-body {
    flex: 1 1 auto;
  }
  .probe-cards {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
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

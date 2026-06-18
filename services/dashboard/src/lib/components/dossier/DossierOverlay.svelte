<script lang="ts">
  // Phase 123a — Dossier as a global overlay (ADR-033 amendment).
  //
  // The Dossier is no longer a top-level route. It opens as a single global
  // search/catalogue overlay over ANY surface, driven by `?dossier=open`
  // (deep-linkable). Single-probe focus rides on `?selectedProbes=` — the
  // catalogue auto-expands selected probes; there is no separate mini mode.
  //
  // Hosts the unchanged `ProbeCard`. Pure DOM — fully usable in the
  // no-WebGL2 fallback (independent of the globe engine). Mounted once in
  // the (app) layout so it is available on Atmosphäre, Workbench, Reflexion.
  import { onMount, tick } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import ProbeCard from './ProbeCard.svelte';
  import DateRangePicker from '$lib/components/base/DateRangePicker.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  const isOpen = $derived(url.dossier === 'open');

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

  // Slice 2 — the large overlay IS the search/catalogue surface (it
  // replaces the old ProbeFilterModal). The list is driven by full-text
  // search + facets over UNIVERSAL attributes only (probe / source /
  // language / country — never capability/metric, to avoid a data-rich
  // discovery bias). Probe SELECTION (the `?selectedProbes=` cart) is a
  // separate concern surfaced as a per-row checkbox; toggling writes the
  // cart immediately (URL-native, no draft/Apply ceremony).
  let search = $state('');
  let langFilter = $state('');
  let countryFilter = $state('');

  const languages = $derived<string[]>([...new Set(probeList.map((p) => p.language))].sort());
  const countries = $derived<string[]>(
    [...new Set(probeList.map((p) => p.country).filter((c): c is string => !!c))].sort()
  );

  function matchesSearch(p: ProbeDto): boolean {
    const q = search.trim().toLowerCase();
    if (!q) return true;
    const hay = [
      p.probeId,
      p.displayName,
      p.shortName,
      p.language,
      p.country ?? '',
      ...(p.sources ?? [])
    ]
      .join(' ')
      .toLowerCase();
    return hay.includes(q);
  }

  // The catalogue filtered by search + facets (NOT by the selection cart —
  // selection is shown as checked state so the user can keep browsing while
  // building it).
  const catalogue = $derived<ProbeDto[]>(
    probeList.filter(
      (p) =>
        matchesSearch(p) &&
        (!langFilter || p.language === langFilter) &&
        (!countryFilter || p.country === countryFilter)
    )
  );

  function startCollapsedFor(probeId: string): boolean {
    // Probes carried in the selection (e.g. a deep-link
    // `?dossier=open&selectedProbes=…`) start expanded; otherwise browse collapsed.
    if (url.selectedProbes.length > 0) return !url.selectedProbes.includes(probeId);
    return true;
  }

  function isSelected(probeId: string): boolean {
    return url.selectedProbes.includes(probeId);
  }
  function toggleSelect(probeId: string) {
    const next = isSelected(probeId)
      ? url.selectedProbes.filter((id) => id !== probeId)
      : [...url.selectedProbes, probeId];
    setUrl({ selectedProbes: next });
  }
  function clearSelection() {
    setUrl({ selectedProbes: [] });
  }

  function close() {
    setUrl({ dossier: null });
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

  // Register on mount and tear down via the returned cleanup — both run
  // client-only. (A bare `onDestroy` would also run during SSR, where
  // `window` is undefined, because this overlay is mounted unconditionally.)
  onMount(() => {
    window.addEventListener('keydown', onKeydown);
    return () => window.removeEventListener('keydown', onKeydown);
  });
</script>

{#if isOpen}
  <div class="dossier-overlay-backdrop" role="presentation">
    <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
    <section
      class="dossier-overlay"
      role="dialog"
      aria-modal="true"
      aria-label={m.dossier_overlay_aria_label()}
      tabindex="-1"
      bind:this={dialogEl}
    >
      <header class="overlay-header">
        <div class="overlay-titles">
          <p class="eyebrow">{m.dossier_overlay_eyebrow()}</p>
          <h2>{m.dossier_overlay_title()}</h2>
        </div>
        <button
          type="button"
          class="close-btn"
          onclick={close}
          aria-label={m.dossier_overlay_close()}>×</button
        >
      </header>

      <div class="window-row">
        <span class="window-label">{m.dossier_window_label()}</span>
        <DateRangePicker
          from={url.from}
          to={url.to}
          onChange={(f, t) => setUrl({ from: f, to: t })}
        />
      </div>

      <div class="catalogue-controls">
        <input
          type="search"
          class="catalogue-search"
          bind:value={search}
          placeholder={m.dossier_search_placeholder()}
          aria-label={m.dossier_search_aria_label()}
        />
        {#if languages.length > 1}
          <select bind:value={langFilter} aria-label={m.dossier_facet_language_aria_label()}>
            <option value="">{m.dossier_facet_language_all()}</option>
            {#each languages as l (l)}<option value={l}>{l.toUpperCase()}</option>{/each}
          </select>
        {/if}
        {#if countries.length > 1}
          <select bind:value={countryFilter} aria-label={m.dossier_facet_country_aria_label()}>
            <option value="">{m.dossier_facet_country_all()}</option>
            {#each countries as c (c)}<option value={c}>{c}</option>{/each}
          </select>
        {/if}
        <span class="selection-count"
          >{m.dossier_selection_count({ count: url.selectedProbes.length })}</span
        >
        {#if url.selectedProbes.length > 0}
          <button type="button" class="clear-sel" onclick={clearSelection}
            >{m.dossier_selection_clear()}</button
          >
        {/if}
      </div>

      <div class="overlay-body">
        {#if probesQ.isPending}
          <p class="muted" aria-busy="true">{m.dossier_list_loading()}</p>
        {:else if probesQ.isError || probesQ.data?.kind === 'network-error'}
          <p class="error">{m.dossier_list_error()}</p>
        {:else if catalogue.length === 0}
          <p class="muted">{m.dossier_list_empty()}</p>
        {:else}
          <div class="probe-cards">
            {#each catalogue as probe (probe.probeId)}
              <div class="catalogue-entry" class:selected={isSelected(probe.probeId)}>
                <label class="select-toggle">
                  <input
                    type="checkbox"
                    checked={isSelected(probe.probeId)}
                    onchange={() => toggleSelect(probe.probeId)}
                    aria-label={m.dossier_selection_add({ name: probe.displayName })}
                  />
                  <span
                    >{isSelected(probe.probeId)
                      ? m.dossier_selection_selected()
                      : m.dossier_selection_select()}</span
                  >
                </label>
                <ProbeCard
                  {probe}
                  {ctx}
                  windowStart={windowMs.start}
                  windowEnd={windowMs.end}
                  startCollapsed={startCollapsedFor(probe.probeId)}
                />
              </div>
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
    /* Phase 123 — start the backdrop AFTER the fixed SideRail (width
       `--rail-width`) so `place-items: center` centres the panel in the
       visible area to the rail's right, not across the whole viewport
       (which pushed the panel's left edge behind the rail). Falls back to
       the token's 184px if the var is unavailable. On narrow viewports the
       rail var still applies; see the media query below. */
    inset: 0 0 0 var(--rail-width, 184px);
    background: color-mix(in srgb, var(--color-bg) 70%, transparent);
    backdrop-filter: blur(3px);
    /* Below MetadataCoverageModal (z-index 50) so a ProbeCard's metadata
       modal layers above this overlay. */
    z-index: 40;
    display: grid;
    place-items: center;
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

  .window-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .window-label {
    font-family: var(--font-mono);
    font-size: 9.5px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .catalogue-controls {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-3);
  }
  .catalogue-search {
    flex: 1 1 18rem;
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
  }
  .catalogue-search:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-color: var(--color-accent);
  }
  .catalogue-controls select {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .selection-count {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .clear-sel {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    padding: 4px var(--space-3);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .clear-sel:hover,
  .clear-sel:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .catalogue-entry {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    padding: var(--space-2);
  }
  .catalogue-entry.selected {
    border-color: var(--color-accent-muted);
    background: color-mix(in srgb, var(--color-accent) 8%, transparent);
  }
  .select-toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    cursor: pointer;
    align-self: flex-start;
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

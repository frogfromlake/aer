<script lang="ts">
  // ProbeFilterModal — Phase 122k K3.
  //
  // The guided, calm probe-selection surface. Replaces the Phase-122i
  // sidebar-embedded ProbePicker (cramped, popover-style) with a modal
  // that gives the user generous whitespace, search, and explicit Apply
  // / Cancel semantics. Two entry points: a SideRail button (from any
  // surface), and a button in the Dossier top-banner.
  //
  // The modal's selection is staged locally; only Apply commits to
  // `url.selectedProbes`. Esc / Cancel discard.
  //
  // Region grouping: today every probe is German (probe-0); the modal
  // is structured to host region sections when Phase 123 lands the
  // French probe and beyond. Each probe row exposes language + a
  // discourse-function abbrev preview so the user can pick by coverage
  // at a glance.
  import { createQuery } from '@tanstack/svelte-query';
  import { onMount, onDestroy } from 'svelte';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';

  interface Props {
    ctx?: FetchContext;
    onClose: () => void;
  }

  let { ctx = { baseUrl: '/api/v1' }, onClose }: Props = $props();

  const url = $derived(urlState());

  // Draft selection — snapshotted from URL state at mount; Apply commits.
  // svelte-ignore state_referenced_locally
  let draft = $state<readonly string[]>([...url.selectedProbes]);
  let search = $state('');

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  // Group probes by language as a region proxy until a proper region
  // field lands on the Probe schema (Phase 123).
  const grouped = $derived.by<{ region: string; probes: ProbeDto[] }[]>(() => {
    const buckets: Record<string, ProbeDto[]> = {};
    const q = search.trim().toLowerCase();
    for (const p of probeList) {
      if (q && !p.probeId.toLowerCase().includes(q) && !p.language.toLowerCase().includes(q))
        continue;
      const region = languageToRegion(p.language);
      const arr = buckets[region] ?? [];
      arr.push(p);
      buckets[region] = arr;
    }
    return Object.entries(buckets)
      .map(([region, probes]) => ({ region, probes }))
      .sort((a, b) => a.region.localeCompare(b.region));
  });

  function languageToRegion(lang: string): string {
    const l = lang.toLowerCase();
    if (l === 'de' || l === 'fr' || l === 'en') return 'Western Europe';
    if (l === 'ru') return 'Russophone Eurasia';
    if (l === 'zh' || l === 'ja') return 'East Asia';
    if (l === 'ar') return 'Arabophone';
    if (l === 'hi') return 'South Asia';
    if (l === 'es' || l === 'pt') return 'Iberophone';
    return 'Other';
  }

  function toggle(probeId: string) {
    draft = draft.includes(probeId) ? draft.filter((p) => p !== probeId) : [...draft, probeId];
  }

  function clearAll() {
    draft = [];
  }

  function applyAndClose() {
    setUrl({ selectedProbes: [...draft] });
    onClose();
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }
    if (e.key === 'Enter' && !(e.target instanceof HTMLInputElement)) {
      e.preventDefault();
      applyAndClose();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
  });
  onDestroy(() => {
    window.removeEventListener('keydown', onKeydown);
  });

  const draftCount = $derived(draft.length);
  const totalCount = $derived(probeList.length);
</script>

<div class="modal-backdrop" role="presentation">
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-labelledby="pfm-heading"
    tabindex="-1"
  >
    <header class="modal-header">
      <div class="header-titles">
        <h2 id="pfm-heading">Filter probes</h2>
        <p class="header-hint">
          Pick the probes you want to focus on. The selection feeds the Dossier filter and seeds the
          Workbench's ScopeEditor when you open the Workbench next.
        </p>
      </div>
      <button type="button" class="close-btn" onclick={onClose} aria-label="Cancel and close">
        ×
      </button>
    </header>

    <div class="search-row">
      <input
        type="search"
        bind:value={search}
        placeholder="Search by probe id or language…"
        aria-label="Search probes"
      />
      <span class="search-count">
        {draftCount} of {totalCount} selected
      </span>
    </div>

    <div class="groups">
      {#if probesQ.isPending}
        <p class="muted" aria-busy="true">Loading probe catalogue…</p>
      {:else if grouped.length === 0}
        <p class="muted">No probes match the search.</p>
      {:else}
        {#each grouped as group (group.region)}
          <section class="region-group" aria-label="Region: {group.region}">
            <header class="region-header">
              <h3>{group.region}</h3>
              <span class="region-count">
                {group.probes.length} probe{group.probes.length === 1 ? '' : 's'}
              </span>
            </header>
            <ul class="probe-list" role="list">
              {#each group.probes as probe (probe.probeId)}
                {@const checked = draft.includes(probe.probeId)}
                <li>
                  <label class="probe-row" class:checked>
                    <input
                      type="checkbox"
                      {checked}
                      onchange={() => toggle(probe.probeId)}
                      aria-label="Include {probe.probeId}"
                    />
                    <span class="probe-label">
                      <span class="probe-id">{probe.probeId}</span>
                      <span class="probe-lang">[{probe.language.toUpperCase()}]</span>
                    </span>
                  </label>
                </li>
              {/each}
            </ul>
          </section>
        {/each}
      {/if}
    </div>

    <footer class="modal-footer">
      <button
        type="button"
        class="clear-btn"
        onclick={clearAll}
        disabled={draftCount === 0}
        title="Clear the selection"
      >
        Clear all
      </button>
      <div class="footer-spacer"></div>
      <button type="button" class="cancel-btn" onclick={onClose}>Cancel</button>
      <button type="button" class="apply-btn" onclick={applyAndClose}>Apply Selection</button>
    </footer>
  </section>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--color-bg) 75%, transparent);
    backdrop-filter: blur(2px);
    z-index: 50;
    display: grid;
    place-items: center;
    padding: var(--space-4);
  }

  .modal {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    width: min(48rem, 100%);
    max-height: 90vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding: var(--space-5);
    gap: var(--space-4);
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.35);
  }

  .modal-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-3);
  }

  .header-titles h2 {
    margin: 0 0 var(--space-1) 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
  }

  .header-hint {
    margin: 0;
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    max-width: 36rem;
    line-height: 1.45;
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

  .search-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .search-row input {
    flex: 1 1 auto;
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
  }
  .search-row input:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-color: var(--color-accent);
  }

  .search-count {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    flex-shrink: 0;
  }

  .groups {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .region-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .region-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-2);
    padding-bottom: var(--space-1);
    border-bottom: 1px dashed var(--color-border);
  }

  .region-header h3 {
    margin: 0;
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-muted);
  }

  .region-count {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .probe-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .probe-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .probe-row:hover {
    background: var(--color-surface);
  }
  .probe-row.checked {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
  }

  .probe-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1 1 auto;
  }

  .probe-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .probe-lang {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .modal-footer {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-3);
  }

  .footer-spacer {
    flex: 1 1 auto;
  }

  .clear-btn,
  .cancel-btn,
  .apply-btn {
    appearance: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }

  .clear-btn,
  .cancel-btn {
    background: transparent;
    color: var(--color-fg-muted);
  }
  .clear-btn:hover:not(:disabled),
  .clear-btn:focus-visible,
  .cancel-btn:hover,
  .cancel-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  .clear-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .apply-btn {
    background: var(--color-accent);
    color: var(--color-bg);
    border-color: var(--color-accent);
    font-weight: 600;
  }
  .apply-btn:hover,
  .apply-btn:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, var(--color-fg));
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>

<script lang="ts">
  // Probe Picker — inline expansion inside the SideRail (Phase 122h
  // Findings round 1, §1.1 ProbePicker revision).
  //
  // Replaces the prior popover/dialog with an inline expandable list
  // that lives directly in the rail. Rationale: the popover felt like
  // a secondary affordance — too easy to overlook. The inline list
  // foregrounds probe selection as the central control surface it is
  // (the dashboard is useless without a probe in scope).
  //
  // Behaviour:
  //   - Trigger button "Select probes" toggles the list open / closed.
  //   - When open, probe rows render inline (no floating layer).
  //   - Per row: click anywhere = switch to that probe's dossier;
  //     the `+` / `✓` button toggles the probe's membership in the
  //     composition set (`url.probeIds`), without navigating.
  //   - Default-open when the user has no probe in scope (helps the
  //     first-time-visitor flow) and the trigger pulses with the
  //     accent colour to attract attention.
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';

  interface Props {
    /** When true, the picker starts open and the trigger pulses with the
     *  accent colour — used when the user lands on the Workbench without
     *  a probe in scope. */
    highlighted?: boolean;
  }

  let { highlighted = false }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let probes = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  const url = $derived(urlState());

  // Active probe from path param (Workbench / Dossier routes carry it) or
  // first composed probe — same logic the SideRail uses.
  let activeProbe = $derived<string | null>(
    (page.params as Record<string, string | undefined>).probeId ??
      (url.probeIds.length > 0 ? url.probeIds[0]! : null)
  );

  let composedSet = $derived(new Set(url.probeIds));
  let composedCount = $derived(url.probeIds.length);

  // Default-open when highlighted (no probe in scope on Workbench).
  let open = $state(false);
  $effect(() => {
    if (highlighted) open = true;
  });

  function toggleOpen() {
    open = !open;
  }

  function switchTo(probeId: string) {
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Probe-Dossier route
    void goto(`/dossier/${encodeURIComponent(probeId)}`);
  }

  function toggleComposition(probeId: string) {
    const current = url.probeIds;
    if (current.includes(probeId)) {
      setUrl({ probeIds: current.filter((p) => p !== probeId) });
    } else {
      setUrl({ probeIds: [...current, probeId] });
    }
  }

  function clearComposition() {
    setUrl({ probeIds: [] });
  }

  // Trigger label — concise + informative.
  let triggerLabel = $derived(
    activeProbe ? `${activeProbe.replace('probe-', '')}` : 'No probe selected'
  );
</script>

<div class="probe-picker" class:open class:highlighted>
  <button
    type="button"
    class="probe-trigger"
    class:has-probe={activeProbe !== null}
    class:has-composition={composedCount > 0}
    aria-expanded={open}
    aria-controls="probe-picker-body"
    aria-label={open ? 'Collapse probe list' : 'Expand probe list'}
    onclick={toggleOpen}
  >
    <span class="probe-trigger-cta">Select probes</span>
    <span class="probe-trigger-state">
      <span class="probe-trigger-label" title={activeProbe ?? 'No probe selected'}>
        {triggerLabel}
      </span>
      {#if composedCount > 1}
        <span class="probe-trigger-badge" aria-label="{composedCount} probes in composition">
          ⊗ {composedCount}
        </span>
      {/if}
    </span>
    <span class="probe-trigger-chevron" class:open aria-hidden="true">›</span>
  </button>

  {#if open}
    <div class="probe-picker-body" id="probe-picker-body" role="region" aria-label="Probes">
      {#if probesQ.isPending}
        <p class="probe-picker-msg" aria-busy="true">Loading probes…</p>
      {:else if probes.length === 0}
        <p class="probe-picker-msg">No probes available.</p>
      {:else}
        <ul class="probe-list" role="list">
          {#each probes as p (p.probeId)}
            {@const isActive = p.probeId === activeProbe}
            {@const isComposed = composedSet.has(p.probeId)}
            <li class="probe-row" class:probe-row-active={isActive}>
              <button
                type="button"
                class="probe-row-switch"
                aria-current={isActive ? 'page' : undefined}
                onclick={() => switchTo(p.probeId)}
                title={isActive
                  ? `${p.probeId} (active) — click to open its Dossier again`
                  : `Switch to ${p.probeId} — opens its Dossier`}
              >
                <span class="probe-state" aria-hidden="true">{isActive ? '●' : '○'}</span>
                <span class="probe-id">{p.probeId.replace('probe-', '')}</span>
                <span class="probe-lang">{p.language.toUpperCase()}</span>
              </button>
              <button
                type="button"
                class="probe-compose-btn"
                class:active={isComposed}
                aria-pressed={isComposed}
                aria-label="{isComposed ? 'Remove from' : 'Add to'} composition: {p.probeId}"
                title={isComposed
                  ? 'Remove from composition set'
                  : 'Add to composition set (multi-probe analysis)'}
                onclick={() => toggleComposition(p.probeId)}
              >
                {isComposed ? '✓' : '+'}
              </button>
            </li>
          {/each}
        </ul>

        {#if composedCount > 0}
          <div class="compose-foot">
            <span class="compose-summary">
              <span class="compose-glyph" aria-hidden="true">⊗</span>
              {composedCount} composed
            </span>
            <button type="button" class="compose-clear" onclick={clearComposition}>Clear</button>
          </div>
        {:else}
          <p class="compose-hint">Use ＋ to compose a multi-probe set.</p>
        {/if}
      {/if}
    </div>
  {/if}
</div>

<style>
  .probe-picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    width: 100%;
  }

  /* Trigger — primary control. Always visible, always informative. */
  .probe-trigger {
    appearance: none;
    display: grid;
    grid-template-columns: 1fr auto;
    grid-template-rows: auto auto;
    gap: 2px 6px;
    align-items: center;
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    cursor: pointer;
    font-family: var(--font-ui);
    text-align: left;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .probe-trigger-cta {
    font-size: 10.5px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-subtle);
    grid-column: 1;
    grid-row: 1;
  }

  .probe-trigger-chevron {
    grid-column: 2;
    grid-row: 1 / span 2;
    align-self: center;
    color: var(--color-fg-subtle);
    font-size: 1rem;
    line-height: 1;
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .probe-trigger-chevron.open {
    transform: rotate(90deg);
  }

  .probe-trigger-state {
    grid-column: 1;
    grid-row: 2;
    display: flex;
    align-items: center;
    gap: var(--space-1);
    min-width: 0;
  }

  .probe-trigger-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }

  .probe-trigger-badge {
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 1px 6px;
    background: var(--color-accent);
    color: var(--color-bg);
    border-radius: var(--radius-pill);
    font-weight: var(--font-weight-semibold);
    flex-shrink: 0;
  }

  .probe-trigger:hover,
  .probe-trigger:focus-visible {
    background: var(--color-surface-hover);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .probe-trigger.has-probe {
    border-color: var(--color-border);
  }

  .probe-trigger.has-composition {
    border-color: var(--color-accent-muted);
  }

  /* Highlighted state — Workbench-without-probe pulse. The trigger
     and its border breathe with the accent colour until a probe is
     picked. */
  .probe-picker.highlighted .probe-trigger {
    border-color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 8%, var(--color-surface));
    animation: probe-trigger-pulse 1.6s ease-in-out infinite;
  }

  .probe-picker.highlighted .probe-trigger-cta {
    color: var(--color-accent);
  }

  @keyframes probe-trigger-pulse {
    0%,
    100% {
      box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-accent) 50%, transparent);
    }
    50% {
      box-shadow: 0 0 0 6px color-mix(in srgb, var(--color-accent) 0%, transparent);
    }
  }

  /* Body — inline expansion within the rail. */
  .probe-picker-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    max-height: 40vh;
    overflow-y: auto;
  }

  .probe-picker-msg {
    margin: 0;
    padding: var(--space-2);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .probe-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .probe-row {
    display: flex;
    align-items: stretch;
    gap: 4px;
    padding: 0;
  }

  .probe-row-switch {
    flex: 1;
    display: grid;
    grid-template-columns: 1rem 1fr auto;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-2);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    cursor: pointer;
    text-align: left;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    min-width: 0;
  }

  .probe-row-switch:hover,
  .probe-row-switch:focus-visible {
    background: var(--color-surface-hover);
    border-color: var(--color-border);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .probe-row-active .probe-row-switch {
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
    border-color: var(--color-accent-muted);
  }

  .probe-state {
    text-align: center;
    color: var(--color-accent);
  }

  .probe-id {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .probe-lang {
    font-size: 9.5px;
    color: var(--color-fg-subtle);
    text-transform: uppercase;
  }

  .probe-compose-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .probe-compose-btn:hover,
  .probe-compose-btn:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .probe-compose-btn.active {
    background: color-mix(in srgb, var(--color-accent) 18%, transparent);
    color: var(--color-accent);
    border-color: var(--color-accent);
  }

  .compose-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-1);
    padding-top: var(--space-1);
    border-top: 1px solid var(--color-border);
  }

  .compose-summary {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    font-weight: var(--font-weight-medium);
  }

  .compose-clear {
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    padding: 1px var(--space-2);
    font-size: 10.5px;
    font-family: var(--font-mono);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }

  .compose-clear:hover,
  .compose-clear:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .compose-hint {
    margin: 0;
    padding-top: var(--space-1);
    border-top: 1px dashed var(--color-border);
    font-size: 10.5px;
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  @media (prefers-reduced-motion: reduce) {
    .probe-trigger,
    .probe-trigger-chevron,
    .probe-compose-btn {
      transition: none;
    }
    .probe-picker.highlighted .probe-trigger {
      animation: none;
    }
  }
</style>

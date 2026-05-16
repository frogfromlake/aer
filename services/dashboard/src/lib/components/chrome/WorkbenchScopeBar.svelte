<script lang="ts">
  // WorkbenchScopeBar — Phase 122h / ADR-033 §3.
  //
  // Bottom-anchored unified scope-editing surface for the Workbench.
  // Renders the four facets of analysis scope as removable chip rows:
  //
  //   - Probes        — comma-separated `probeId[]` URL param. The Probe
  //                     Picker in the SideRail is the canonical add path;
  //                     this bar lets the user *remove* a probe quickly
  //                     without leaving the Workbench.
  //   - Sources       — comma-separated `sourceId[]`. The Probe Dossier
  //                     is the canonical add path; this bar removes.
  //   - Functions     — discourse-function filter chips. Click a chip to
  //                     toggle; functions live in `url.viewingMode`-agnostic
  //                     scope state. Phase 122h Slice 4 ships read-only
  //                     chips for the active filter; the multi-select
  //                     editor lands with Slice 8's URL-state migration.
  //   - Window        — from / to (ISO timestamps). Inline editable inputs.
  //
  // Distinct from the legacy `ScopeBar.svelte` (Phase 105) which carries
  // Surface I/II/III breadcrumb logic and hardcoded `/lanes/...` references.
  // Resolution + Normalization are deliberately *not* in this bar — they
  // live per-Cell or per-Stratum per ADR-033 §6.
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS, type ViewingMode } from '$lib/state/url-internals';
  import FunctionBadge from '$lib/components/base/FunctionBadge.svelte';
  import { DISCOURSE_FUNCTIONS, type DiscourseFunction } from '$lib/discourse-function';
  import { removePanel as removePanelMutator } from '$lib/workbench/panel-mutators';
  import { updatePanelPure } from '$lib/workbench/panel-mutators-pure';

  const url = $derived(urlState());

  // Phase 122i revision (A3). The bar's chip data must reflect the
  // ACTIVE PANEL's scope when the URL is in pillar-state form. The
  // Phase-122h implementation read `url.probeIds`/`url.sourceIds` which
  // are empty in pillar-state URLs (the reader nulls them when any
  // pillar key is present), so the bar showed the "no probe selected"
  // empty state inside a populated Workbench.
  const activePillar = $derived<ViewingMode>(url.activePillar ?? url.viewingMode ?? 'aleph');
  const activePanelInfo = $derived.by(() => {
    const pillar = url.pillars?.[activePillar];
    if (!pillar) return null;
    const win = pillar.windows[pillar.activeWindowIndex] ?? pillar.windows[0];
    if (!win) return null;
    const panel = win.panels[win.focusedPanelIndex] ?? win.panels[0];
    if (!panel) return null;
    return {
      pillar,
      windowIndex: pillar.activeWindowIndex,
      win,
      panelIndex: win.focusedPanelIndex,
      panel
    };
  });

  const probeIds = $derived.by<string[]>(() => {
    if (activePanelInfo) {
      const out: string[] = [];
      for (const g of activePanelInfo.panel.scopes)
        for (const p of g.probeIds) if (!out.includes(p)) out.push(p);
      return out;
    }
    return url.probeIds;
  });
  const sourceIds = $derived.by<string[]>(() => {
    if (activePanelInfo) {
      const out: string[] = [];
      for (const g of activePanelInfo.panel.scopes)
        for (const s of g.sourceIds) if (!out.includes(s)) out.push(s);
      return out;
    }
    return url.sourceIds;
  });

  // Locked-DF chip: emitted only when the active panel was opened via
  // the DF tile and carries `lockedFunction`. Shown as a single highlighted
  // FunctionBadge alongside the standard function-row chips.
  const lockedFunction = $derived<DiscourseFunction | null>(
    activePanelInfo?.panel.locked === true && activePanelInfo.panel.lockedFunction
      ? (activePanelInfo.panel.lockedFunction as DiscourseFunction)
      : null
  );

  // Time window — fall back to default lookback when unset so the inputs
  // always show a meaningful value the user can edit.
  const windowDates = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      from: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS),
      to: new Date(Number.isFinite(toMs) ? toMs : now)
    };
  });

  function toIsoDate(d: Date): string {
    return d.toISOString().slice(0, 10);
  }

  function removeProbe(id: string) {
    if (activePanelInfo) {
      // Phase 122i revision (A3). When in pillar-state form, removing a
      // probe chip removes it from EVERY ScopeGroup of the focused
      // panel. If the panel ends up with no probes, remove the panel
      // entirely (consistent with the legacy "clear-all" semantic).
      const info = activePanelInfo;
      const stillHasProbes = info.panel.scopes.some((g) => g.probeIds.some((p) => p !== id));
      if (!stillHasProbes) {
        removePanelMutator({
          pillar: activePillar,
          windowIndex: info.windowIndex,
          panelIndex: info.panelIndex
        });
        return;
      }
      const nextPillars = updatePanelPure(
        url.pillars,
        {
          pillar: activePillar,
          windowIndex: info.windowIndex,
          panelIndex: info.panelIndex
        },
        (p) => ({
          ...p,
          scopes: p.scopes.map((g) => ({
            probeIds: g.probeIds.filter((pid) => pid !== id),
            sourceIds: g.sourceIds
          }))
        })
      );
      if (nextPillars) setUrl({ pillars: nextPillars });
      return;
    }
    setUrl({ probeIds: probeIds.filter((p) => p !== id) });
  }
  function removeSource(id: string) {
    if (activePanelInfo) {
      const info = activePanelInfo;
      const nextPillars = updatePanelPure(
        url.pillars,
        {
          pillar: activePillar,
          windowIndex: info.windowIndex,
          panelIndex: info.panelIndex
        },
        (p) => ({
          ...p,
          scopes: p.scopes.map((g) => ({
            probeIds: g.probeIds,
            sourceIds: g.sourceIds.filter((sid) => sid !== id)
          }))
        })
      );
      if (nextPillars) setUrl({ pillars: nextPillars });
      return;
    }
    setUrl({ sourceIds: sourceIds.filter((s) => s !== id) });
  }
  function onFromChange(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.value) return;
    const d = new Date(target.value);
    if (Number.isNaN(d.getTime())) return;
    setUrl({ from: d.toISOString() });
  }
  function onToChange(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.value) return;
    const d = new Date(target.value);
    if (Number.isNaN(d.getTime())) return;
    setUrl({ to: d.toISOString() });
  }

  // Functions: in Slice 4 we surface ALL four as filter chips; the
  // discourse-function URL param lands in Slice 8 when the Workbench fully
  // takes over. For now the chips are informational + WP-001 §3 anchors
  // via the FunctionBadge's ⓘ-affordance.
  const FUNCTION_KEYS: readonly DiscourseFunction[] = DISCOURSE_FUNCTIONS;
</script>

<aside class="scope-bar" aria-label="Workbench scope">
  <div class="row" data-facet="probes">
    <span class="row-label">Probes</span>
    {#if probeIds.length === 0}
      <span class="row-empty">no probe selected — pick one on the Atmosphere</span>
    {:else}
      <ul class="chip-list" role="list">
        {#each probeIds as id (id)}
          <li>
            <button
              type="button"
              class="chip chip-probe"
              aria-label="Remove probe {id}"
              title="Remove probe"
              onclick={() => removeProbe(id)}
            >
              <span class="chip-glyph" aria-hidden="true">⊙</span>
              <span class="chip-label">{id}</span>
              <span class="chip-remove" aria-hidden="true">×</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  {#if sourceIds.length > 0}
    <div class="row" data-facet="sources">
      <span class="row-label">Sources</span>
      <ul class="chip-list" role="list">
        {#each sourceIds as id (id)}
          <li>
            <button
              type="button"
              class="chip chip-source"
              aria-label="Remove source {id}"
              title="Remove source"
              onclick={() => removeSource(id)}
            >
              <span class="chip-glyph" aria-hidden="true">⊂</span>
              <span class="chip-label">{id}</span>
              <span class="chip-remove" aria-hidden="true">×</span>
            </button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  <div class="row" data-facet="functions">
    <span class="row-label">Functions</span>
    {#if lockedFunction}
      <!-- Phase 122i revision (A3). When the active panel is locked to
           a discourse function, surface that ONE function prominently
           instead of the full four-function strip. -->
      <ul class="chip-list" role="list">
        <li>
          <FunctionBadge function={lockedFunction} size="sm" showLabel showInfo selected />
        </li>
        <li class="locked-fn-hint">🔒 scope locked</li>
      </ul>
    {:else}
      <ul class="chip-list" role="list">
        {#each FUNCTION_KEYS as fn (fn)}
          <li>
            <FunctionBadge function={fn} size="sm" showLabel showInfo />
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  <div class="row" data-facet="window">
    <span class="row-label">Window</span>
    <label class="date-input">
      <span class="date-input-eyebrow">from</span>
      <input type="date" value={toIsoDate(windowDates.from)} onchange={onFromChange} />
    </label>
    <span class="date-sep" aria-hidden="true">→</span>
    <label class="date-input">
      <span class="date-input-eyebrow">to</span>
      <input type="date" value={toIsoDate(windowDates.to)} onchange={onToChange} />
    </label>
  </div>
</aside>

<style>
  .scope-bar {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    min-height: 28px;
  }

  .row-label {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
    min-width: 5rem;
    flex-shrink: 0;
  }

  .row-empty {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  .locked-fn-hint {
    font-size: 10px;
    color: var(--color-fg-subtle);
    font-style: italic;
    align-self: center;
  }

  .chip-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .chip {
    appearance: none;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    cursor: pointer;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .chip:hover,
  .chip:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .chip-glyph {
    color: var(--color-accent);
  }

  .chip-remove {
    color: var(--color-fg-subtle);
    margin-left: 2px;
  }

  .chip:hover .chip-remove,
  .chip:focus-visible .chip-remove {
    color: var(--color-status-expired);
  }

  .date-input {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .date-input-eyebrow {
    color: var(--color-fg-subtle);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .date-input input {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .date-input input:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-color: var(--color-accent);
  }

  .date-sep {
    color: var(--color-fg-subtle);
  }

  @media (prefers-reduced-motion: reduce) {
    .chip {
      transition: none;
    }
  }
</style>

<script lang="ts">
  // Right-edge methodology tray — Design Brief §3.3 / §5.7.
  //
  // Phase 108 wires the Phase 105 shell to live content. The tray is
  // the single L4 Provenance surface, reachable from every metric on
  // every surface; the legacy `L4ProvenanceFlyout` was retired in this
  // phase per the reframing-note rework guidance.
  //
  // Subscriptions:
  //   focusedMetric() — transient, set by chart / lane / dossier
  //                     interactions (setFocusedMetric).
  //   url.metric      — fallback metric carried in the URL (L3 selector,
  //                     view-mode controls). Either source updates the
  //                     panel in place — no separate interaction needed.
  //   negativeSpaceActive() — when on, the open-state body re-orders to
  //                     known-limitations-first (Brief §4.4 / Phase 113).
  //
  // Closed-state binding: the tier badge reflects the live provenance
  // tierClassification, and a small dot appears when the metric carries
  // any known limitations applicable to the current view.
  //
  // Open-state binding: parallel fetches of /content/metric/{name} and
  // /metrics/{name}/provenance, rendered as Dual-Register content
  // (methodological register primary per §7.7) plus a deep link to
  // /reflection/wp/{id}?section=… (Phase 109 target — link works as a
  // stub before then).
  //
  // A11y: role="complementary" with an accessible label; not a dialog —
  // the tray does not trap focus, because the surface behind it remains
  // live (Brief §4.1 rule 2 "no layer replaces").
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { urlState } from '$lib/state/url.svelte';
  import { focusedMetric } from '$lib/state/metric.svelte';
  import { negativeSpaceActive, setTrayOpen, trayOpen } from '$lib/state/tray.svelte';
  import {
    contentQuery,
    provenanceQuery,
    type ContentResponseDto,
    type FetchContext,
    type MetricProvenanceDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import Badge from '$lib/components/base/Badge.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import {
    pickBadgeTier,
    workingPaperHref as parseWorkingPaperHref
  } from './methodology-tray-internals';

  const PUSH_BREAKPOINT_PX = 900;

  // The dashboard is single-tenant against /api/v1; the per-page ctx
  // pattern elsewhere (e.g. lanes/+page.svelte) just hard-codes this
  // baseUrl, and the tray sits in the global (app) layout where there
  // is no surface-level ctx to inherit.
  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const url = $derived(urlState());
  const transientFocus = $derived(focusedMetric());
  const probeSelected = $derived(url.probe !== null);
  const open = $derived(trayOpen());
  const negSpace = $derived(negativeSpaceActive());

  // The methodology tray is only relevant on Surface II L3 Function Lanes
  // where metric selection happens. Hide it everywhere else (Surface I,
  // L2 Probe Dossier, Surface III) to avoid clutter (L1 item 1, L2 item 1).
  const isFunctionLane = $derived(
    /^\/lanes\/[^/]+\/(?!dossier(?:\/|$))[^/]+/.test(page.url.pathname)
  );

  // Effective focused metric: the explicit transient focus wins,
  // falling back to the URL-carried metric so URL deep-links and the
  // L3 metric selector both surface the right tray content.
  const effectiveMetric = $derived<string | null>(transientFocus?.metricName ?? url.metric ?? null);
  const chartContext = $derived(transientFocus?.chartContext ?? null);

  function isOverlayMode(): boolean {
    if (typeof window === 'undefined') return false;
    return window.innerWidth < PUSH_BREAKPOINT_PX;
  }

  $effect(() => {
    if (!isFunctionLane) {
      document.documentElement.style.setProperty('--tray-right-edge', '0px');
      return () => {
        document.documentElement.style.setProperty('--tray-right-edge', '0px');
      };
    }
    const inset = open && !isOverlayMode() ? 'var(--tray-open-width)' : 'var(--tray-closed-width)';
    document.documentElement.style.setProperty('--tray-right-edge', inset);
    return () => {
      document.documentElement.style.setProperty('--tray-right-edge', 'var(--tray-closed-width)');
    };
  });

  function toggle() {
    setTrayOpen(!open);
  }

  // -------------------------------------------------------------------
  // Parallel fetches — content + provenance for the focused metric.
  // Both queries are gated on `effectiveMetric` to avoid spurious
  // 404s on the bare Atmosphere where no metric is selected yet.
  // -------------------------------------------------------------------
  const provQ = createQuery<
    QueryOutcome<MetricProvenanceDto>,
    Error,
    QueryOutcome<MetricProvenanceDto>
  >(() => {
    const name = effectiveMetric ?? '';
    const o = provenanceQuery(ctx, name);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: name.length > 0
    };
  });

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const name = effectiveMetric ?? '';
    const o = contentQuery(ctx, 'metric', name, 'en');
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: name.length > 0
    };
  });

  const provenance = $derived<MetricProvenanceDto | null>(
    provQ.data?.kind === 'success' ? provQ.data.data : null
  );
  const contentRecord = $derived<ContentResponseDto | null>(
    contentQ.data?.kind === 'success' ? contentQ.data.data : null
  );

  const badgeTier = $derived(pickBadgeTier(provenance));
  const hasLimitations = $derived((provenance?.knownLimitations.length ?? 0) > 0);

  // Working Paper anchor for the deep link. Catalog entries ship
  // 0..n anchors; we surface the first one as the canonical "Read the
  // full Working Paper" target. Phase 109 materialises the route; the
  // link works as a stub URL until then.
  const workingPaperHref = $derived(
    parseWorkingPaperHref(contentRecord?.workingPaperAnchors?.[0] ?? null)
  );
</script>

{#if isFunctionLane}
  <aside class="tray" class:tray-open={open} aria-label="Methodology">
    <!-- Tab strip — always visible -->
    <button
      type="button"
      class="tab"
      class:tab-dim={!probeSelected && !effectiveMetric}
      aria-label="{open ? 'Close' : 'Open'} methodology tray"
      aria-expanded={open}
      onclick={toggle}
    >
      {#if effectiveMetric}
        <!-- Collapsed tab is narrow; render only a small tier pip + a known-
           limitations dot here. The full text Badge renders inside the
           open panel header so it never overflows the closed tab. -->
        <span
          class="tier-pip tier-pip-{badgeTier}"
          aria-label={badgeTier === 'expired'
            ? 'Validation expired'
            : badgeTier === 'refused'
              ? 'Methodological refusal'
              : `Tier badge: ${badgeTier}`}
          title={badgeTier === 'expired'
            ? 'Validation expired — open tray for details'
            : badgeTier === 'refused'
              ? 'Methodological refusal — open tray for details'
              : 'Open tray for tier and provenance details'}
        ></span>
        {#if hasLimitations}
          <span
            class="limitations-dot"
            aria-label="Known limitations apply to this metric"
            title="Known limitations apply"
          ></span>
        {/if}
      {/if}
      <span class="tab-label">Methodology</span>
      <span class="tab-chevron" aria-hidden="true">{open ? '›' : '‹'}</span>
    </button>

    <!-- Open-state panel -->
    {#if open}
      <div class="panel" role="region" aria-label="Methodology content">
        <header class="panel-header">
          <span class="panel-title">Methodology</span>
          <button
            type="button"
            class="close-btn"
            aria-label="Close methodology tray"
            onclick={toggle}
          >
            ×
          </button>
        </header>

        <div class="panel-body" class:limitations-first={negSpace}>
          {#if !effectiveMetric}
            <p class="hint">
              Focus a metric on any chart, lane, or dossier to see its provenance, validation tier,
              and known limitations here.
            </p>
          {:else}
            <header class="metric-head">
              <p class="metric-eyebrow">Focused metric</p>
              <code class="metric-name">{effectiveMetric}</code>
              <div class="metric-badges">
                <Badge tier={badgeTier} />
                {#if hasLimitations}
                  <span class="limitations-pill" title="Known limitations apply">
                    Known limitations
                  </span>
                {/if}
              </div>
              {#if chartContext}
                <p class="chart-context" aria-label="Chart-level focus">
                  <span class="ctx-eyebrow">Selection</span>
                  <span class="ctx-value">{chartContext}</span>
                </p>
              {/if}
            </header>

            {#if provQ.isPending || contentQ.isPending}
              <p class="muted" aria-busy="true">Loading methodology…</p>
            {:else}
              <!-- Body order: under Negative Space, known-limitations
                 leads. Otherwise the canonical order is dual-register
                 → tier/algorithm → limitations → cultural context. -->
              {#if hasLimitations && provenance}
                <section class="limits epistemic-weight" data-section="limitations">
                  <h4>Known limitations</h4>
                  <ul>
                    {#each provenance.knownLimitations as lim (lim)}
                      <li>{lim}</li>
                    {/each}
                  </ul>
                </section>
              {/if}

              {#if contentRecord}
                <section class="registers epistemic-weight" data-section="dual-register">
                  <ProgressiveSemantics
                    registers={contentRecord.registers}
                    emphasis="methodological"
                  />
                </section>
              {/if}

              {#if provenance}
                <section class="prov epistemic-weight" data-section="provenance">
                  <h4>Provenance</h4>
                  <dl>
                    <dt>Tier</dt>
                    <dd>
                      <Badge tier={badgeTier} />
                    </dd>
                    <dt>Validation</dt>
                    <dd class="status status-{provenance.validationStatus}">
                      {provenance.validationStatus}
                    </dd>
                    <dt>Algorithm</dt>
                    <dd>{provenance.algorithmDescription}</dd>
                    <dt>Extractor</dt>
                    <dd><code>{provenance.extractorVersionHash}</code></dd>
                  </dl>
                  {#if provenance.culturalContextNotes}
                    <p class="ctx-notes">{provenance.culturalContextNotes}</p>
                  {/if}
                </section>
              {/if}

              {#if workingPaperHref}
                <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection-surface route, materialised in Phase 109 -->
                <a class="wp-link" href={workingPaperHref}> Read the full Working Paper → </a>
              {/if}
            {/if}
          {/if}
        </div>
      </div>
    {/if}
  </aside>
{/if}

<style>
  .tray {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    /* Above the L3 SidePanel (z-index 1000) so the tray remains
       reachable when L3 is open in dashboard size — and the only
       layer that can render above L3 is the L4 Provenance surface,
       which is exactly what this tray is. */
    z-index: 1100;
    display: flex;
    flex-direction: row-reverse;
    pointer-events: none;
  }

  .tab {
    pointer-events: auto;
    width: var(--tray-closed-width);
    background: var(--color-bg-elevated);
    border: none;
    border-left: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    cursor: pointer;
    color: var(--color-fg-muted);
    position: relative;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .tab:hover,
  .tab:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .tab-dim {
    opacity: 0.35;
  }

  .badge-slot {
    writing-mode: horizontal-tb;
  }

  /* Compact tier indicator for the collapsed tab — replaces the full
     text Badge so "Validation expired" never overflows the narrow tab. */
  .tier-pip {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-status-unvalidated);
    box-shadow: 0 0 0 2px var(--color-bg-elevated);
    writing-mode: horizontal-tb;
  }
  .tier-pip-tier1-validated,
  .tier-pip-tier2-validated {
    background: var(--color-status-validated);
  }
  .tier-pip-expired {
    background: var(--color-status-expired);
  }
  .tier-pip-refused {
    background: var(--color-status-refused);
  }

  .metric-badges {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }

  .limitations-pill {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 1px var(--space-2);
    border: 1px solid var(--color-status-expired);
    color: var(--color-status-expired);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
  }

  .limitations-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-status-expired, #c06060);
    box-shadow: 0 0 0 2px var(--color-bg-elevated);
  }

  .tab-label {
    writing-mode: vertical-lr;
    text-orientation: mixed;
    transform: rotate(180deg);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    user-select: none;
  }

  .tab-chevron {
    font-size: var(--font-size-base);
    writing-mode: horizontal-tb;
  }

  .panel {
    pointer-events: auto;
    width: calc(var(--tray-open-width) - var(--tray-closed-width));
    background: var(--color-surface);
    border-left: 1px solid var(--color-border);
    box-shadow: var(--elevation-3);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slide-in var(--motion-duration-base) var(--motion-ease-emphasized);
  }

  @keyframes slide-in {
    from {
      transform: translateX(100%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-3) var(--space-4) var(--space-4);
    border-bottom: 1px solid var(--color-border);
    flex-shrink: 0;
  }

  .panel-title {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-muted);
  }

  .close-btn {
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xl);
    line-height: 1;
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }

  .close-btn:hover,
  .close-btn:focus-visible {
    color: var(--color-fg);
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .panel-body {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* Negative Space mode — limitations bubble to the top of the panel
     body via flex order so the methodological register no longer leads. */
  .panel-body.limitations-first :global([data-section='limitations']) {
    order: -1;
  }

  .metric-head {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .metric-eyebrow {
    margin: 0;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .metric-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .chart-context {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: var(--space-2) 0 0;
  }

  .ctx-eyebrow {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .ctx-value {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .hint {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    margin: 0;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  h4 {
    margin: 0 0 var(--space-2);
    font-size: var(--font-size-sm);
  }

  .limits ul {
    margin: 0;
    padding-left: var(--space-5);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
  }

  .prov dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
    font-size: var(--font-size-sm);
  }

  .prov dt {
    color: var(--color-fg-muted);
  }

  .prov dd {
    margin: 0;
    color: var(--color-fg);
  }

  .status {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 10px;
    font-size: 11px;
    text-transform: uppercase;
  }

  .status-unvalidated {
    color: #caa04a;
    background: rgba(202, 160, 74, 0.12);
  }
  .status-validated {
    color: #4ca84c;
    background: rgba(76, 168, 76, 0.12);
  }
  .status-expired {
    color: #c06060;
    background: rgba(192, 96, 96, 0.12);
  }

  .ctx-notes {
    margin: var(--space-3) 0 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  .wp-link {
    align-self: flex-start;
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px solid currentColor;
    padding-bottom: 1px;
  }

  .wp-link:hover,
  .wp-link:focus-visible {
    color: var(--color-fg);
  }

  @media (prefers-reduced-motion: reduce) {
    .tab {
      transition: none;
    }
    .panel {
      animation: none;
    }
  }
</style>

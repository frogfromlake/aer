<script lang="ts">
  // PanelControls — Phase 122h Findings round 2 §F1.
  //
  // Per-cell control strip used by every Pillar Shell. Exposes the four
  // levers that define the active cell of the View-Mode Matrix:
  //
  //   - Metric         (`url.metric`)         — dynamic list from `/metrics/available`
  //   - Darstellung    (`url.viewMode`)       — pillar-filtered presentations
  //   - Layer          (`url.layer`)          — gold | silver
  //   - Vergleich      (`url.normalization`)  — raw | zscore | percentile
  //
  // Replaces the lever surface that lived in the retired `LensBar`. The
  // controls write through `setUrl` so deep-links restore byte-identically.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    metricsAvailableQuery,
    scopeAvailableMetricsQuery,
    type AvailableMetricDto,
    type FetchContext,
    type QueryOutcome,
    type ScopeAvailableMetricsDto
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import {
    CROSS_PROBE_DEFAULT_METRIC,
    DEFAULT_METRIC_NAME,
    metricSupportsPresentation,
    presentationsForPillar,
    resolvePresentation
  } from '$lib/viewmodes';
  import {
    DEFAULT_LOOKBACK_MS,
    type CellChannelBinding,
    type Normalization,
    type NetworkColorChannel,
    type NetworkSizeChannel,
    type Resolution,
    type ViewMode,
    type ViewingMode
  } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { defaultMetricForScopes } from '$lib/workbench/panel-queries';
  import { viewerLabelLanguage } from '$lib/viewmodes/viewer-language';

  // Phase 131 — default per-cell config values. Mirrors the cell-side and
  // BFF-side defaults so a freshly-added Panel renders identically whether
  // or not the user has touched the config levers.
  const DEFAULT_BINS = 30;
  const DEFAULT_TOPN = 60;
  const DEFAULT_FORCE_STRENGTH = 50;

  interface Props {
    pillar: ViewingMode;
    /** Phase 122i — when set, the controls bind to the addressed Panel
     *  in the new Pillar→Window→Panel state instead of the legacy flat
     *  URL params. The pillar prop must match `panelPath.pillar`. */
    panelPath?: PanelPath;
  }

  let { pillar, panelPath }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  // Phase 122i — resolve the addressed Panel when bound. Returns `null`
  // when the path is stale (e.g. the user just removed the panel) so the
  // controls fall back to legacy flat-URL behaviour for one frame
  // instead of crashing.
  const boundPanel = $derived.by(() => {
    if (!panelPath) return null;
    return (
      url.pillars?.[panelPath.pillar]?.windows[panelPath.windowIndex]?.panels[
        panelPath.panelIndex
      ] ?? null
    );
  });
  const isPanelBound = $derived(boundPanel !== null);
  const isPanelLocked = $derived(boundPanel?.locked === true);

  // Phase 122k F5 — per-Panel window. The dateWindow derives from the
  // bound panel's own windowStart/windowEnd when set; otherwise falls
  // back to the global url.from/url.to. The Window date inputs below
  // mutate panel.windowStart/windowEnd via updatePanel.
  const windowBounds = $derived.by(() => {
    const panelStart = boundPanel?.windowStart;
    const panelEnd = boundPanel?.windowEnd;
    const fromSrc = panelStart ?? url.from;
    const toSrc = panelEnd ?? url.to;
    const fromMs = fromSrc ? Date.parse(fromSrc) : NaN;
    const toMs = toSrc ? Date.parse(toSrc) : NaN;
    // Episteme (diachronic) defaults to a disclosed recent window (mirrors
    // EpistemeShell) so the date inputs + availability query reflect the same
    // effective window the cells use — otherwise the window is invisible.
    // Aleph/Rhizome stay unbounded (undefined ⇒ whole dataset, the optional
    // time-limit is off by default there).
    const epistemeDefault = pillar === 'episteme';
    const now = Date.now();
    return {
      startMs: Number.isFinite(fromMs)
        ? fromMs
        : epistemeDefault
          ? now - DEFAULT_LOOKBACK_MS
          : undefined,
      endMs: Number.isFinite(toMs) ? toMs : epistemeDefault ? now : undefined,
      isPanelOverride: panelStart !== undefined || panelEnd !== undefined
    };
  });
  // Date-only form for the `/metrics/available` window + the native date
  // inputs (YYYY-MM-DD). Undefined when that side is unbounded — the input
  // renders empty and the query omits the bound.
  const dateWindow = $derived({
    startDate:
      windowBounds.startMs !== undefined
        ? new Date(windowBounds.startMs).toISOString().slice(0, 10)
        : undefined,
    endDate:
      windowBounds.endMs !== undefined
        ? new Date(windowBounds.endMs).toISOString().slice(0, 10)
        : undefined,
    isPanelOverride: windowBounds.isPanelOverride
  });
  // Phase 123c (C1) — full RFC 3339 form for `/scope/available-metrics`,
  // whose `start`/`end` parameters are `format: date-time` (both optional).
  const windowIso = $derived({
    start:
      windowBounds.startMs !== undefined ? new Date(windowBounds.startMs).toISOString() : undefined,
    end: windowBounds.endMs !== undefined ? new Date(windowBounds.endMs).toISOString() : undefined
  });
  // Today (YYYY-MM-DD) — the native date inputs use it to forbid future dates
  // and to keep TO ≥ FROM (no inverted/future windows can be picked at all).
  const todayStr = new Date().toISOString().slice(0, 10);

  const availQ = createQuery<
    QueryOutcome<AvailableMetricDto[]>,
    Error,
    QueryOutcome<AvailableMetricDto[]>
  >(() => {
    const o = metricsAvailableQuery(ctx, dateWindow);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  // -----------------------------------------------------------------------
  // Phase 123c (C1) — cross-probe metric-availability guard.
  //
  // The FULL panel scope is the union of every ScopeGroup's probes +
  // sources. `/scope/available-metrics` reports which metrics have data for
  // EVERY scoped source (`available`, the intersection) versus only SOME
  // (`partial`). The metric picker and the scatter axis/size/colour
  // selectors offer only `available` metrics so a panel spanning probes with
  // asymmetric capability can never bind a metric that silently yields empty
  // cells; `partial` is surfaced as an explanatory hint.
  // -----------------------------------------------------------------------
  const panelScope = $derived.by(() => {
    const seenP: Record<string, true> = {};
    const seenS: Record<string, true> = {};
    const probeIds: string[] = [];
    const sourceIds: string[] = [];
    for (const g of boundPanel?.scopes ?? []) {
      for (const p of g.probeIds)
        if (!seenP[p]) {
          seenP[p] = true;
          probeIds.push(p);
        }
      for (const s of g.sourceIds)
        if (!seenS[s]) {
          seenS[s] = true;
          sourceIds.push(s);
        }
    }
    return { probeIds, sourceIds };
  });
  const hasScope = $derived(panelScope.probeIds.length > 0 || panelScope.sourceIds.length > 0);
  // The metric-availability WITHHOLDING is a CROSS-PROBE discipline only
  // (backbone strategy: cross-probe runs on the shared multilingual backbone;
  // within a single frame all tiers are allowed). Within one probe — even with
  // several sources — we never withhold: a metric missing on one source simply
  // renders an empty cell there, which is honest absence-as-data, not a reason
  // to hide the metric from the picker.
  const isCrossProbe = $derived(panelScope.probeIds.length > 1);

  const scopeAvailQ = createQuery<
    QueryOutcome<ScopeAvailableMetricsDto>,
    Error,
    QueryOutcome<ScopeAvailableMetricsDto>
  >(() => {
    const o = scopeAvailableMetricsQuery(ctx, {
      probeIds: panelScope.probeIds,
      sourceIds: panelScope.sourceIds,
      start: windowIso.start,
      end: windowIso.end
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: hasScope
    };
  });

  // The authoritative availability payload, or `null` whenever we have none
  // (no scope, query pending, refused, or errored). Single guard so the
  // three derivations below don't each re-assert the success shape.
  const scopeAvail = $derived<ScopeAvailableMetricsDto | null>(
    hasScope && scopeAvailQ.data?.kind === 'success' ? scopeAvailQ.data.data : null
  );
  // Set of metric names available across the WHOLE scope. `null` falls back
  // to the unconstrained `/metrics/available` list so the picker is never
  // wrongly emptied.
  const scopeAvailableSet = $derived<Set<string> | null>(
    scopeAvail ? new Set(scopeAvail.available) : null
  );
  // Metrics present for SOME but not all scoped sources — rendered as a hint.
  const partialMetrics = $derived<ScopeAvailableMetricsDto['partial']>(scopeAvail?.partial ?? []);
  // The full resolved scoped-source list — denominator + "missing on" set.
  const scopedSourceNames = $derived<readonly string[]>(scopeAvail?.scopedSources ?? []);
  const scopedSourceCount = $derived(scopedSourceNames.length);
  // Issue 6 — "show anyway": when on, the picker also offers partial metrics.
  const activeShowWithheld = $derived(boundPanel?.showWithheld === true);
  // A metric is offerable when there is no scope constraint yet OR it is in
  // the all-source intersection OR the user opted to "show anyway". Filters
  // both the metric picker and the scatter selectors.
  function isScopeAvailable(name: string): boolean {
    // Within a single probe, never withhold (show all; missing-on-a-source
    // renders an empty cell). Withholding is a cross-probe-only discipline.
    if (scopeAvailableSet === null || activeShowWithheld || !isCrossProbe) return true;
    return scopeAvailableSet.has(name);
  }
  // Issue 6 — the scoped sources that LACK a given partial metric (the "cause"
  // of the withholding), so the hint can name them instead of leaving the
  // user to trial-and-error.
  function missingSourcesFor(have: readonly string[]): string[] {
    const haveSet = new Set(have);
    return scopedSourceNames.filter((s) => !haveSet.has(s));
  }

  const activeMetric = $derived(boundPanel ? boundPanel.metric : DEFAULT_METRIC_NAME);
  const activeLayer = $derived<'gold' | 'silver'>(boundPanel ? boundPanel.layer : 'gold');
  const activeNormalization = $derived<Normalization>(
    boundPanel ? (boundPanel.normalization ?? 'raw') : (url.normalization ?? 'raw')
  );
  const activeResolution = $derived<Resolution>(
    boundPanel ? (boundPanel.resolution ?? 'daily') : (url.resolution ?? 'daily')
  );

  const RESOLUTIONS: ReadonlyArray<{ id: Resolution; label: string }> = [
    { id: 'hourly', label: 'Hourly' },
    { id: 'daily', label: 'Daily' },
    { id: 'weekly', label: 'Weekly' },
    { id: 'monthly', label: 'Monthly' }
  ];

  const presentations = $derived(presentationsForPillar(pillar as ViewingMode));
  const activePresentation = $derived(resolvePresentation(boundPanel?.view ?? null, pillar));

  // Per-view capability flags (Phase 122h Findings round 3). Cells that
  // don't consume the metric / resolution prop get the corresponding
  // control hidden so the UI never misleads the user about what changes
  // when they click.
  const viewUsesMetric = $derived(activePresentation.usesMetric ?? true);
  const viewUsesResolution = $derived(activePresentation.usesResolution ?? false);
  // Phase 131 (bugfix BUG4) — Compare (normalization) only does something on
  // the time-series cell; hide it elsewhere so it isn't a no-op lever.
  const viewUsesNormalization = $derived(activePresentation.usesNormalization ?? false);
  // Phase 131 (BUG1) — deviation/percentile (z-score / percentile) assert a
  // cross-context equivalence claim and the BFF refuses them unless the metric
  // has a deviation/absolute `metric_equivalence` entry (ADR-016 / Phase 115).
  // Read that availability from /metrics/available so we can DISABLE those two
  // buttons (instead of letting the user hit a refusal), with a "?" explainer.
  const metricEquivalenceLevel = $derived.by<string | null>(() => {
    if (availQ.data?.kind !== 'success') return null;
    const m = availQ.data.data.find((x) => x.metricName === activeMetric);
    return m?.equivalenceStatus?.level ?? m?.equivalenceLevel ?? null;
  });
  const canNormalize = $derived(
    metricEquivalenceLevel === 'deviation' || metricEquivalenceLevel === 'absolute'
  );
  let showCompareHelp = $state(false);

  // Raw metric names from the API, in API order. Defensive — `availQ` may
  // be pending or refusing.
  const availableMetricNames = $derived<string[]>(
    availQ.data?.kind === 'success' ? availQ.data.data.map((m) => m.metricName) : []
  );

  // Metric list: DEFAULT first, then API order; the default is always
  // surfaced so the picker is never empty.
  //
  // Phase 130 — the list is filtered through the metric→presentation map so
  // only metrics the ACTIVE view can sensibly render are offered (a
  // distribution view drops `temporal_distribution`; a time-series view
  // drops the cyclic `publication_hour`/`publication_weekday`). This is the
  // catalog (`metrics × presentations`) constrained by the SoT in
  // `metric-presentation.ts`. The UI never produces an incompatible
  // (metric × view) pair — `pickView` reconciles the metric on every view
  // change — so an incompatible active metric can only come from a
  // hand-crafted URL, in which case it is simply absent from the picker.
  const metrics = $derived.by<string[]>(() => {
    const view = activePresentation.id;
    const seen: Record<string, true> = {};
    const merged: string[] = [];
    for (const name of [DEFAULT_METRIC_NAME, ...availableMetricNames]) {
      if (!name || seen[name]) continue;
      if (!metricSupportsPresentation(name, view)) continue;
      // Phase 123c (C1) — withhold metrics absent from any scoped source.
      if (!isScopeAvailable(name)) continue;
      seen[name] = true;
      merged.push(name);
    }
    // The active metric is always surfaced (even if it has since become
    // partial across the scope) so the picker reflects the current
    // selection rather than dropping it silently.
    if (activeMetric && !seen[activeMetric] && metricSupportsPresentation(activeMetric, view)) {
      merged.push(activeMetric);
    }
    return merged;
  });

  function pickMetric(name: string) {
    if (name === activeMetric) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, metric: name }));
    }
    // Phase 122k: metric/view/layer are panel-state only. No-op when no
    // panel context (the empty-state path will be wired in K3 to open the
    // ScopeEditor instead of rendering PanelControls).
  }
  // Phase 130 — pick the first metric the target view can render, preferring
  // the canonical default. Used to reconcile a Panel whose current metric is
  // incompatible with a newly-chosen view (e.g. switching a `publication_hour`
  // distribution to a time-series, which the cyclic metric cannot render).
  function firstMetricSupporting(view: ViewMode): string {
    if (metricSupportsPresentation(DEFAULT_METRIC_NAME, view)) return DEFAULT_METRIC_NAME;
    return (
      availableMetricNames.find((m) => metricSupportsPresentation(m, view)) ?? DEFAULT_METRIC_NAME
    );
  }

  function pickView(id: ViewMode) {
    if (id === activePresentation.id) return;
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const next = { ...p, view: id };
      const pres = presentations.find((x) => x.id === id);
      const usesMetric = pres?.usesMetric ?? true;
      // Keep the Panel coherent: when the new view consumes a metric the
      // current one cannot satisfy, swap to a compatible default so the Cell
      // never renders a nonsensical (metric × presentation) pairing.
      if (usesMetric && !metricSupportsPresentation(next.metric, id)) {
        next.metric = firstMetricSupporting(id);
      }
      // Phase 131 (bugfix) — reconcile composition: if the panel was in
      // overlay but the new view cannot render overlay (only time-series
      // can), fall back to split — the per-scope equivalent of "keep the
      // sources distinct" — so the panel never sits in a no-op composition.
      if (next.composition === 'overlay' && !(pres?.supportsOverlay ?? false)) {
        next.composition = 'split';
      }
      // Phase 131 — seed scatter position channels on first switch so the
      // cell renders immediately rather than waiting for the user to pick
      // both axes. Distinct x/y when the corpus offers >1 metric.
      if (id === 'metric_scatter' && (!next.channels?.x || !next.channels?.y)) {
        const opts = scalarMetricOptions;
        // Issue 5 — a more interesting default than the alphabetical first
        // two (entity_count × language_confidence): put a sentiment metric on
        // X (prefer the multilingual backbone) and word_count on Y. Fall back
        // gracefully to whatever the scope actually offers.
        const sentiment =
          opts.find((m) => m === 'sentiment_score_bert_multilingual') ??
          opts.find((m) => m.startsWith('sentiment_score'));
        const x = next.channels?.x ?? sentiment ?? opts[0] ?? DEFAULT_METRIC_NAME;
        let y = next.channels?.y;
        if (!y) {
          y =
            opts.includes('word_count') && x !== 'word_count'
              ? 'word_count'
              : (opts.find((m) => m !== x) ?? x);
        }
        next.channels = { ...(next.channels ?? {}), x, y };
      }
      return next;
    });
  }
  function pickLayer(next: 'gold' | 'silver') {
    if (next === activeLayer) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, layer: next }));
    }
  }
  function pickNorm(next: Normalization) {
    if (next === activeNormalization) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => {
        const out = { ...p };
        if (next === 'raw') delete out.normalization;
        else out.normalization = next;
        return out;
      });
      return;
    }
    setUrl({ normalization: next === 'raw' ? null : next });
  }
  function pickResolution(next: Resolution) {
    if (next === activeResolution) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, resolution: next }));
      return;
    }
    setUrl({ resolution: next });
  }
  function pickComposition(next: 'merged' | 'split' | 'overlay') {
    if (!panelPath || !boundPanel) return;
    if (boundPanel.composition === next) return;
    updatePanel(panelPath, (p) => ({ ...p, composition: next }));
  }
  function pickSplitDirection(next: 'horizontal' | 'vertical') {
    if (!panelPath || !boundPanel) return;
    if ((boundPanel.splitDirection ?? 'horizontal') === next) return;
    updatePanel(panelPath, (p) => ({ ...p, splitDirection: next }));
  }
  function toggleCollapsed() {
    if (!panelPath || !boundPanel) return;
    const next = !(boundPanel.cellControlsCollapsed === true);
    updatePanel(panelPath, (p) => ({ ...p, cellControlsCollapsed: next }));
  }
  // A scope-valid metric to snap to when the active metric is not offerable
  // for the scope (e.g. the German-only SentiWS default on a French single-
  // probe panel, or after "show anyway" is turned off). Preference order:
  //   1. the scope's canonical default — SentiWS for a German single-probe
  //      panel — when the scope actually carries it (keeps Probe 0's tuned
  //      lexicon as its default);
  //   2. the multilingual sentiment backbone, the one sentiment EVERY probe
  //      carries, so a probe without a probe/language-specific model (e.g.
  //      Probe 1, French) defaults to backbone sentiment — not an arbitrary
  //      first metric;
  //   3. any available sentiment metric, then any available metric.
  function resetMetricForScope(): string {
    const view = activePresentation.id;
    const ok = (m: string) =>
      (scopeAvailableSet?.has(m) ?? true) && metricSupportsPresentation(m, view);
    const canonical = defaultMetricForScopes(boundPanel?.scopes ?? []);
    if (ok(canonical)) return canonical;
    if (ok(CROSS_PROBE_DEFAULT_METRIC)) return CROSS_PROBE_DEFAULT_METRIC;
    const firstSentiment = availableMetricNames.find(
      (m) => m.startsWith('sentiment_score') && ok(m)
    );
    if (firstSentiment) return firstSentiment;
    return availableMetricNames.find(ok) ?? canonical;
  }
  // Issue 6 — "show anyway": offer the withheld (partial) metrics in the
  // picker. PanelHost still renders only the sources that carry the chosen
  // metric, so the empty cells never appear. Turning it back OFF snaps a
  // now-unofferable metric back to a scope-valid default so every source's
  // cell reappears (instead of staying stuck on the withheld metric).
  function toggleShowWithheld() {
    if (!panelPath || !boundPanel) return;
    const next = !(boundPanel.showWithheld === true);
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next) {
        o.showWithheld = true;
      } else {
        delete o.showWithheld;
        if (scopeAvailableSet && !scopeAvailableSet.has(o.metric)) {
          o.metric = resetMetricForScope();
        }
      }
      return o;
    });
  }

  // Capability-driven default reconcile: when the active SCALAR metric is not
  // available for the panel's scope — e.g. the build-time SentiWS default on a
  // French single-probe panel (Probe 1 carries only the multilingual sentiment
  // backbone) — snap it to a scope-valid metric. This makes the *default*
  // probe-specific without `defaultMetricForScopes` needing async capability
  // data. Guards: only for views that consume the scalar metric (scatter /
  // cooccurrence drive their own binding); only for real /metrics/available
  // scalar metrics, never the cooccurrence pair pseudo-metric (absent from the
  // scope set by design); skipped while "show anyway" is on. Converges because
  // the reset target is always scope-available, so the next run early-returns.
  $effect(() => {
    if (!panelPath || !boundPanel || !viewUsesMetric) return;
    // Only cross-probe panels reconcile away an unavailable metric — within a
    // single probe nothing is withheld, so a metric stays selected even if a
    // source lacks it (the cell renders empty there).
    if (scopeAvailableSet === null || activeShowWithheld || !isCrossProbe) return;
    if (scopeAvailableSet.has(activeMetric)) return;
    if (!availableMetricNames.includes(activeMetric)) return;
    const next = resetMetricForScope();
    if (next && next !== activeMetric) {
      updatePanel(panelPath, (p) => ({ ...p, metric: next }));
    }
  });

  // Phase 122k F5 — Window date handlers. Both write to the bound panel's
  // own windowStart/windowEnd; when the panel has no override yet the
  // write installs one. Clearing the override (returning to global
  // default) is exposed via the "Reset to default window" button.
  // The native date input gives YYYY-MM-DD. Start anchors at the day's 00:00,
  // end at the day's 23:59:59.999 — so a single day (start==end picked) is a
  // valid non-empty window, never start==end (which the BFF rejects). And the
  // pair can never invert: if a pick would put end on/before start (e.g. a TO
  // in the past), the OTHER bound snaps to the same chosen day, collapsing to a
  // valid single-day window instead of erroring.
  function pickWindowStart(value: string) {
    if (!panelPath || !value) return;
    const start = new Date(`${value}T00:00:00.000Z`).toISOString();
    if (Number.isNaN(Date.parse(start))) return;
    updatePanel(panelPath, (p) => {
      const next = { ...p, windowStart: start };
      if (next.windowEnd && Date.parse(next.windowEnd) <= Date.parse(start)) {
        next.windowEnd = new Date(`${value}T23:59:59.999Z`).toISOString();
      }
      return next;
    });
  }
  function pickWindowEnd(value: string) {
    if (!panelPath || !value) return;
    const end = new Date(`${value}T23:59:59.999Z`).toISOString();
    if (Number.isNaN(Date.parse(end))) return;
    updatePanel(panelPath, (p) => {
      const next = { ...p, windowEnd: end };
      if (next.windowStart && Date.parse(next.windowStart) >= Date.parse(end)) {
        next.windowStart = new Date(`${value}T00:00:00.000Z`).toISOString();
      }
      return next;
    });
  }
  function resetWindowToGlobal() {
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      delete out.windowStart;
      delete out.windowEnd;
      return out;
    });
  }

  const activeSplitDirection = $derived<'horizontal' | 'vertical'>(
    boundPanel?.splitDirection ?? 'horizontal'
  );
  const isCollapsed = $derived(boundPanel?.cellControlsCollapsed === true);

  // -----------------------------------------------------------------------
  // Phase 131 — per-cell configuration. The active presentation declares the
  // levers it consumes (`configurableParams`); we render exactly those.
  // -----------------------------------------------------------------------
  const configParams = $derived(activePresentation.configurableParams ?? []);
  const activeBins = $derived(boundPanel?.bins ?? DEFAULT_BINS);
  const activeTopN = $derived(boundPanel?.topN ?? DEFAULT_TOPN);
  const activeShowBand = $derived(boundPanel?.showBand ?? true);
  const activeChannels = $derived<CellChannelBinding>(boundPanel?.channels ?? {});
  const activeForceStrength = $derived(boundPanel?.forceStrength ?? DEFAULT_FORCE_STRENGTH);

  // Scalar-metric options for the scatter axis/size/colour pickers. Every
  // real metric from /metrics/available, default-prepended so the picker is
  // never empty (cooccurrence's pair-shaped pseudo-metric never appears here
  // because it is not a /metrics/available entry).
  const scalarMetricOptions = $derived.by<string[]>(() => {
    const seen: Record<string, true> = {};
    const out: string[] = [];
    for (const name of [DEFAULT_METRIC_NAME, ...availableMetricNames]) {
      if (!name || seen[name]) continue;
      // Phase 123c (C1) — same all-source intersection guard as the metric
      // picker; the scatter axes must not bind a metric missing from a
      // scoped source.
      if (!isScopeAvailable(name)) continue;
      seen[name] = true;
      out.push(name);
    }
    // Keep any currently-bound channel metric visible even if it has since
    // become partial across the scope, so the selects reflect the binding.
    for (const bound of [
      activeChannels.x,
      activeChannels.y,
      activeChannels.size,
      activeChannels.color
    ]) {
      if (bound && !seen[bound]) {
        seen[bound] = true;
        out.push(bound);
      }
    }
    return out;
  });

  const NET_SIZE_CHANNELS: ReadonlyArray<{ id: NetworkSizeChannel; label: string }> = [
    { id: 'total_count', label: 'Weight' },
    { id: 'degree', label: 'Degree' }
  ];
  const NET_COLOR_CHANNELS: ReadonlyArray<{ id: NetworkColorChannel; label: string }> = [
    { id: 'label', label: 'Entity type' },
    { id: 'presence', label: 'Source presence' },
    { id: 'source_overlay', label: 'Source overlay' },
    { id: 'uniform', label: 'Uniform' }
  ];

  // Live slider read-outs. The range sliders update these on every `oninput`
  // tick for instant visual feedback, but COMMIT to Panel state (and thus the
  // URL + a BFF refetch) only on `onchange` (pointer release) — otherwise a
  // single drag would fire ~100 full-tree URL writes and superseded fetches.
  // `null` = not mid-drag, fall back to the committed value.
  let liveBins = $state<number | null>(null);
  let liveTopN = $state<number | null>(null);
  let liveForce = $state<number | null>(null);
  const displayBins = $derived(liveBins ?? activeBins);
  const displayTopN = $derived(liveTopN ?? activeTopN);
  const displayForce = $derived(liveForce ?? activeForceStrength);

  function setBins(n: number) {
    if (!panelPath || !Number.isFinite(n)) return;
    const clamped = Math.min(200, Math.max(1, Math.round(n)));
    if (clamped === activeBins) return;
    updatePanel(panelPath, (p) => ({ ...p, bins: clamped }));
  }
  function setTopN(n: number) {
    if (!panelPath || !Number.isFinite(n)) return;
    const clamped = Math.min(500, Math.max(1, Math.round(n)));
    if (clamped === activeTopN) return;
    updatePanel(panelPath, (p) => ({ ...p, topN: clamped }));
  }
  function setForceStrength(n: number) {
    if (!panelPath || !Number.isFinite(n)) return;
    const clamped = Math.min(100, Math.max(0, Math.round(n)));
    if (clamped === activeForceStrength) return;
    updatePanel(panelPath, (p) => ({ ...p, forceStrength: clamped }));
  }
  function setShowBand(next: boolean) {
    if (!panelPath || next === activeShowBand) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      // Shown is the default — omit the field to keep the URL clean.
      if (next) delete o.showBand;
      else o.showBand = false;
      return o;
    });
  }
  // Phase 124 — shared-axis scale mode. 'shared' (default) puts every cell of
  // the panel on one axis domain so values are directly comparable; 'free'
  // restores per-cell optimal domains.
  const activeScaleMode = $derived<'shared' | 'free'>(boundPanel?.scales ?? 'shared');
  function setScaleMode(next: 'shared' | 'free') {
    if (!panelPath || next === activeScaleMode) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      // Shared is the default — omit to keep the URL clean.
      if (next === 'shared') delete o.scales;
      else o.scales = 'free';
      return o;
    });
  }
  // Phase 123b — co-occurrence cross-lingual relabel toggle. 'source' (default)
  // keeps each node on its source surface form; 'viewer' swaps QID-linked nodes
  // to the app-language label. Default omitted from URL.
  const activeDisplayLanguage = $derived<'source' | 'viewer'>(
    boundPanel?.displayLanguage ?? 'source'
  );
  // The app content language (clamped to the index's label languages). Shown on
  // the toggle so the reader knows which language the relabel resolves to.
  const viewerLanguage = viewerLabelLanguage();
  function setDisplayLanguage(next: 'source' | 'viewer') {
    if (!panelPath || next === activeDisplayLanguage) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next === 'viewer') o.displayLanguage = 'viewer';
      else delete o.displayLanguage; // 'source' is the default
      return o;
    });
  }
  // Mutate one visual channel; empty string clears (unbinds) the channel.
  function setChannel(key: keyof CellChannelBinding, value: string) {
    if (!panelPath || (activeChannels[key] ?? '') === value) return;
    updatePanel(panelPath, (p) => {
      const ch: CellChannelBinding = { ...(p.channels ?? {}) };
      if (value === '') delete ch[key];
      else (ch[key] as string) = value;
      const o = { ...p };
      if (Object.keys(ch).length > 0) o.channels = ch;
      else delete o.channels;
      return o;
    });
  }
</script>

<section
  class="cell-controls"
  aria-label="Panel controls"
  class:locked={isPanelLocked}
  class:collapsed={isCollapsed}
>
  {#if isPanelBound && boundPanel}
    <!-- Phase 122k §14 — full-header click toggles collapse (not just
         the mini chevron). Header always rendered so a collapsed strip
         can be re-opened. -->
    <button
      type="button"
      class="cell-controls-header"
      aria-expanded={!isCollapsed}
      aria-label={isCollapsed ? 'Expand panel controls' : 'Collapse panel controls'}
      onclick={toggleCollapsed}
    >
      {#if isPanelLocked && boundPanel}
        <span class="locked-banner" role="status">
          🔒 Scope locked to
          <strong>{boundPanel.lockedFunction ?? 'discourse function'}</strong>'s sources
        </span>
      {:else}
        <span class="header-eyebrow">Panel controls</span>
      {/if}
      <span
        class="collapse-toggle"
        class:expanded={!isCollapsed}
        aria-hidden="true"
        title={isCollapsed ? 'Expand panel controls' : 'Collapse panel controls'}
      >
        {isCollapsed ? '▾' : '▴'}
      </span>
    </button>
  {/if}

  {#if isPanelBound && boundPanel && !isCollapsed}
    <!-- Phase 122i revision (B1): Composition row appears whenever
         PanelControls is bound to a Panel — including locked panels.
         `locked` is scope-only; the user can toggle Merged ↔ Split on
         a DF-entry Workbench freely. The legacy global-state path
         (boundPanel === null) has no composition concept and the row
         is hidden. -->
    <!-- Phase 122k §14c finding 1 — Composition row uses TWO parallel
         labeled control groups separated by a thin vertical divider.
         Both labels (Composition / Direction) sit on the same baseline
         using the standard `ctrl-eyebrow` style. Merged is its own
         button, never glued to Vertical. -->
    <div class="ctrl-row composition-row">
      <div class="comp-group" role="radiogroup" aria-label="Composition">
        <span class="ctrl-eyebrow">Composition</span>
        <div class="ctrl-options">
          <button
            type="button"
            role="radio"
            aria-checked={boundPanel.composition === 'split'}
            class="ctrl-btn"
            class:active={boundPanel.composition === 'split'}
            onclick={() => pickComposition('split')}
            title="One Cell per source or per ScopeGroup (small-multiples)"
          >
            Split
          </button>
          {#if activePresentation.supportsOverlay}
            <!-- Phase 131 (bugfix) — Overlay is only meaningful for the
                 time-series cell; per-scope cells render one artefact and
                 would show overlay identically to merged, so the option is
                 hidden there. -->
            <button
              type="button"
              role="radio"
              aria-checked={boundPanel.composition === 'overlay'}
              class="ctrl-btn"
              class:active={boundPanel.composition === 'overlay'}
              onclick={() => pickComposition('overlay')}
              title="One Cell — sources rendered as separate viridis-coloured lines on a shared canvas"
            >
              Overlay
            </button>
          {/if}
          <button
            type="button"
            role="radio"
            aria-checked={boundPanel.composition === 'merged'}
            class="ctrl-btn"
            class:active={boundPanel.composition === 'merged'}
            onclick={() => pickComposition('merged')}
            title="One Cell — sources aggregated into a single joint-corpus chart"
          >
            Merged
          </button>
        </div>
      </div>

      {#if boundPanel.composition === 'split'}
        <div class="comp-divider" aria-hidden="true"></div>
        <div class="comp-group" role="radiogroup" aria-label="Split direction">
          <span class="ctrl-eyebrow">Direction</span>
          <div class="ctrl-options">
            <button
              type="button"
              role="radio"
              aria-checked={activeSplitDirection === 'horizontal'}
              class="ctrl-btn"
              class:active={activeSplitDirection === 'horizontal'}
              onclick={() => pickSplitDirection('horizontal')}
              title="Arrange split cells side-by-side"
              aria-label="Split direction: horizontal"
            >
              ↔ Horizontal
            </button>
            <button
              type="button"
              role="radio"
              aria-checked={activeSplitDirection === 'vertical'}
              class="ctrl-btn"
              class:active={activeSplitDirection === 'vertical'}
              onclick={() => pickSplitDirection('vertical')}
              title="Stack split cells vertically"
              aria-label="Split direction: vertical"
            >
              ↕ Vertical
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  {#if !isCollapsed}
    <!-- View / Darstellung row — always visible. Lists the presentations
       the active Pillar owns (Phase 130 / ADR-035: the pillar is fixed by
       the presentation set). -->
    <div class="ctrl-row" role="radiogroup" aria-label="View">
      <span class="ctrl-eyebrow">View</span>
      <div class="ctrl-options">
        {#each presentations as p (p.id)}
          <button
            type="button"
            role="radio"
            aria-checked={activePresentation.id === p.id}
            class="ctrl-btn"
            class:active={activePresentation.id === p.id}
            title={p.description}
            onclick={() => pickView(p.id)}
          >
            {p.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- Metric row — only when the active view consumes a metric. BERTopic
       and co-occurrence cells ignore metricName, so the row is omitted
       entirely for those views (no misleading no-op selector). -->
    {#if viewUsesMetric}
      <div class="ctrl-row" role="radiogroup" aria-label="Metric">
        <span class="ctrl-eyebrow">Metric</span>
        <div class="ctrl-options">
          {#each metrics as m (m)}
            <button
              type="button"
              role="radio"
              aria-checked={activeMetric === m}
              class="ctrl-btn metric-btn"
              class:active={activeMetric === m}
              onclick={() => pickMetric(m)}
            >
              <code>{m}</code>
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Phase 123c (C1 + Issue 6) — partial-metric hint. Surfaces metrics
         that have data for only SOME scoped sources, and names the sources
         that LACK each one (the "cause") so the user doesn't have to trial-
         and-error. "Show anyway" offers them in the picker regardless; the
         panel then renders only the sources that carry the chosen metric. -->
    {#if isCrossProbe && partialMetrics.length > 0 && (viewUsesMetric || configParams.includes('scatterAxes'))}
      <div class="ctrl-row partial-hint" role="note">
        <span class="ctrl-eyebrow">Withheld</span>
        <div class="partial-hint-body">
          <p class="partial-hint-lead">
            {partialMetrics.length} metric{partialMetrics.length === 1 ? '' : 's'} not present on every
            one of the {scopedSourceCount} scoped source{scopedSourceCount === 1 ? '' : 's'} — withheld
            so a panel-wide binding cannot yield silently empty cells:
          </p>
          <ul class="partial-hint-list" role="list">
            {#each partialMetrics as pm (pm.metricName)}
              {@const missing = missingSourcesFor(pm.sources)}
              <li class="partial-metric-row">
                <code class="partial-metric">{pm.metricName}</code>
                <span class="partial-metric-detail">
                  has {pm.sources.length}/{scopedSourceCount}{#if missing.length > 0}
                    · missing on <strong>{missing.join(', ')}</strong>{/if}
                </span>
              </li>
            {/each}
          </ul>
          {#if isPanelBound}
            <button
              type="button"
              class="ctrl-btn partial-toggle"
              class:active={activeShowWithheld}
              role="switch"
              aria-checked={activeShowWithheld}
              onclick={toggleShowWithheld}
              title="Offer these metrics in the picker anyway. The panel renders only the sources that carry the chosen metric — no empty cells, no scope change needed."
            >
              {activeShowWithheld
                ? '✓ Showing withheld (only sources with data render)'
                : 'Show anyway'}
            </button>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Resolution row — only when the active view bins values along a
       time axis. Distribution / topic_* / cooccurrence cells aggregate
       differently and ignore resolution; the row stays hidden there. -->
    {#if viewUsesResolution}
      <div class="ctrl-row" role="radiogroup" aria-label="Time resolution">
        <span class="ctrl-eyebrow">Resolution</span>
        <div class="ctrl-options">
          {#each RESOLUTIONS as r (r.id)}
            <button
              type="button"
              role="radio"
              aria-checked={activeResolution === r.id}
              class="ctrl-btn"
              class:active={activeResolution === r.id}
              onclick={() => pickResolution(r.id)}
            >
              {r.label}
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Phase 131 — per-cell configuration. The active presentation declares
         which levers it consumes; we render exactly those. Panel-bound only
         (config is per-Panel state, absent from the legacy global path). -->
    {#if isPanelBound && configParams.length > 0}
      {#if configParams.includes('bins')}
        <div class="ctrl-row config-row" role="group" aria-label="Histogram bins">
          <span class="ctrl-eyebrow">Bins</span>
          <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
            <input
              type="range"
              min="5"
              max="120"
              step="1"
              value={activeBins}
              oninput={(e) => (liveBins = Number((e.currentTarget as HTMLInputElement).value))}
              onchange={(e) => {
                setBins(Number((e.currentTarget as HTMLInputElement).value));
                liveBins = null;
              }}
              aria-label="Histogram bin count slider"
            />
            <output class="config-value">{displayBins}</output>
          </div>
        </div>
      {/if}

      {#if configParams.includes('topN')}
        <div class="ctrl-row config-row" role="group" aria-label="Top edges">
          <span class="ctrl-eyebrow">Top N</span>
          <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
            <input
              type="range"
              min="5"
              max="500"
              step="5"
              value={activeTopN}
              oninput={(e) => (liveTopN = Number((e.currentTarget as HTMLInputElement).value))}
              onchange={(e) => {
                setTopN(Number((e.currentTarget as HTMLInputElement).value));
                liveTopN = null;
              }}
              aria-label="Top co-occurrence edge count slider"
            />
            <output class="config-value">{displayTopN}</output>
          </div>
        </div>
      {/if}

      {#if configParams.includes('forceStrength')}
        <div class="ctrl-row config-row" role="group" aria-label="Graph spread">
          <span class="ctrl-eyebrow">Spread</span>
          <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
            <input
              type="range"
              min="0"
              max="100"
              step="1"
              value={activeForceStrength}
              oninput={(e) => (liveForce = Number((e.currentTarget as HTMLInputElement).value))}
              onchange={(e) => {
                setForceStrength(Number((e.currentTarget as HTMLInputElement).value));
                liveForce = null;
              }}
              title="How strongly nodes repel each other — higher spreads a crowded graph apart"
              aria-label="Graph spread (node repulsion) slider"
            />
            <output class="config-value">{displayForce}</output>
          </div>
        </div>
      {/if}

      {#if configParams.includes('band')}
        <div class="ctrl-row config-row" role="group" aria-label="Uncertainty band">
          <span class="ctrl-eyebrow">Band</span>
          <button
            type="button"
            role="switch"
            aria-checked={activeShowBand}
            class="ctrl-btn"
            class:active={activeShowBand}
            onclick={() => setShowBand(!activeShowBand)}
            title="Toggle the ±1σ uncertainty band around each series"
          >
            {activeShowBand ? '±1σ shown' : '±1σ hidden'}
          </button>
        </div>
      {/if}

      {#if configParams.includes('scales')}
        <div class="ctrl-row config-row" role="group" aria-label="Axis scale">
          <span class="ctrl-eyebrow">Scale</span>
          <button
            type="button"
            role="switch"
            aria-checked={activeScaleMode === 'shared'}
            class="ctrl-btn"
            class:active={activeScaleMode === 'shared'}
            onclick={() => setScaleMode(activeScaleMode === 'shared' ? 'free' : 'shared')}
            title="Shared: every cell in this panel uses one axis domain, so identical values plot at identical positions (directly comparable). Free: each cell scales to its own data."
          >
            {activeScaleMode === 'shared' ? 'Shared axis' : 'Free axis'}
          </button>
        </div>
      {/if}

      {#if configParams.includes('networkChannels')}
        <div class="ctrl-row config-row" role="group" aria-label="Network visual channels">
          <span class="ctrl-eyebrow">Size</span>
          <select
            class="config-select"
            value={activeChannels.netSize ?? 'total_count'}
            onchange={(e) => setChannel('netSize', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Node size channel"
          >
            {#each NET_SIZE_CHANNELS as c (c.id)}
              <option value={c.id}>{c.label}</option>
            {/each}
          </select>
          <span class="ctrl-eyebrow">Colour</span>
          <select
            class="config-select"
            value={activeChannels.netColor ?? 'label'}
            onchange={(e) => setChannel('netColor', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Node colour channel"
          >
            {#each NET_COLOR_CHANNELS as c (c.id)}
              <option value={c.id}>{c.label}</option>
            {/each}
          </select>
        </div>
      {/if}

      {#if configParams.includes('displayLanguage')}
        <div class="ctrl-row config-row" role="group" aria-label="Display language">
          <span class="ctrl-eyebrow">Labels</span>
          <button
            type="button"
            role="switch"
            aria-checked={activeDisplayLanguage === 'viewer'}
            class="ctrl-btn"
            class:active={activeDisplayLanguage === 'viewer'}
            onclick={() =>
              setDisplayLanguage(activeDisplayLanguage === 'viewer' ? 'source' : 'viewer')}
            title="Source form keeps each entity in its original language; App language relabels Wikidata-linked nodes to the app language ({viewerLanguage}). Unlinked nodes always keep their source form."
          >
            {activeDisplayLanguage === 'viewer'
              ? `App language (${viewerLanguage})`
              : 'Source form'}
          </button>
        </div>
      {/if}

      {#if configParams.includes('scatterAxes')}
        <div class="ctrl-row config-row" role="group" aria-label="Scatter position channels">
          <span class="ctrl-eyebrow">X · Y</span>
          <select
            class="config-select"
            value={activeChannels.x ?? ''}
            onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Scatter X-axis metric"
          >
            {#each scalarMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
          <select
            class="config-select"
            value={activeChannels.y ?? ''}
            onchange={(e) => setChannel('y', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Scatter Y-axis metric"
          >
            {#each scalarMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>
        <div class="ctrl-row config-row" role="group" aria-label="Scatter size and colour channels">
          <span class="ctrl-eyebrow">Size</span>
          <select
            class="config-select"
            value={activeChannels.size ?? ''}
            onchange={(e) => setChannel('size', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Scatter point-size metric"
          >
            <option value="">— none —</option>
            {#each scalarMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
          <span class="ctrl-eyebrow">Colour</span>
          <select
            class="config-select"
            value={activeChannels.color ?? ''}
            onchange={(e) => setChannel('color', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Scatter point-colour metric"
          >
            <option value="">— none —</option>
            {#each scalarMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>
      {/if}
    {/if}

    <!-- Phase 122k §14 finding 5 — Window is more important than
         Layer/Compare so it sits above them in the row order. The date
         inputs have their click events stopped from bubbling so the
         article-level focus handler doesn't close the native date
         picker mid-interaction. -->
    {#if isPanelBound}
      <div class="ctrl-row" role="group" aria-label="Time window">
        <span class="ctrl-eyebrow">Window</span>
        <div class="window-inputs" onclick={(e) => e.stopPropagation()} role="presentation">
          <input
            type="date"
            value={dateWindow.startDate}
            max={dateWindow.endDate ?? todayStr}
            onchange={(e) => pickWindowStart((e.currentTarget as HTMLInputElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Window start"
          />
          <span class="window-sep" aria-hidden="true">→</span>
          <input
            type="date"
            value={dateWindow.endDate}
            min={dateWindow.startDate}
            max={todayStr}
            onchange={(e) => pickWindowEnd((e.currentTarget as HTMLInputElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Window end"
          />
          {#if dateWindow.isPanelOverride}
            <button
              type="button"
              class="ctrl-btn"
              onclick={resetWindowToGlobal}
              title="Drop this panel's window override and inherit the global default"
            >
              Reset
            </button>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Layer + Compare on one row — both are low-frequency controls;
         grouped to save vertical space. -->
    <div class="ctrl-row ctrl-row-split">
      <div class="ctrl-group" role="radiogroup" aria-label="Data layer">
        <span class="ctrl-eyebrow">Layer</span>
        <div class="ctrl-options">
          <button
            type="button"
            role="radio"
            aria-checked={activeLayer === 'gold'}
            class="ctrl-btn layer-btn"
            class:active={activeLayer === 'gold'}
            title="Au Gold — aggregated metrics"
            onclick={() => pickLayer('gold')}
          >
            Au Gold
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={activeLayer === 'silver'}
            class="ctrl-btn layer-btn silver"
            class:active={activeLayer === 'silver'}
            title="Ag Silver — document-level data (WP-006 §5.2)"
            onclick={() => pickLayer('silver')}
          >
            Ag Silver
          </button>
        </div>
      </div>

      {#if viewUsesNormalization}
        <div class="ctrl-group" role="radiogroup" aria-label="Normalization">
          <span class="ctrl-eyebrow">Compare</span>
          <div class="ctrl-options">
            <button
              type="button"
              role="radio"
              aria-checked={activeNormalization === 'raw'}
              class="ctrl-btn"
              class:active={activeNormalization === 'raw'}
              title="Raw values"
              onclick={() => pickNorm('raw')}
            >
              raw
            </button>
            <button
              type="button"
              role="radio"
              aria-checked={activeNormalization === 'zscore'}
              class="ctrl-btn"
              class:active={activeNormalization === 'zscore'}
              disabled={!canNormalize}
              title={canNormalize
                ? 'Z-score deviation from the baseline'
                : 'Needs a validated baseline + cross-context equivalence (Phase 115) — not available for this metric yet. Click ? to learn why.'}
              onclick={() => pickNorm('zscore')}
            >
              deviation
            </button>
            <button
              type="button"
              role="radio"
              aria-checked={activeNormalization === 'percentile'}
              class="ctrl-btn"
              class:active={activeNormalization === 'percentile'}
              disabled={!canNormalize}
              title={canNormalize
                ? 'Percentile rank within scope'
                : 'Needs a validated baseline + cross-context equivalence (Phase 115) — not available for this metric yet. Click ? to learn why.'}
              onclick={() => pickNorm('percentile')}
            >
              percentile
            </button>
            {#if !canNormalize}
              <button
                type="button"
                class="ctrl-help"
                aria-expanded={showCompareHelp}
                title="Why are deviation / percentile disabled?"
                onclick={(e) => {
                  e.stopPropagation();
                  showCompareHelp = !showCompareHelp;
                }}
              >
                ?
              </button>
            {/if}
          </div>
        </div>
      {/if}
    </div>
    {#if viewUsesNormalization && !canNormalize && showCompareHelp}
      <p class="compare-help" role="note">
        <strong>deviation</strong> (z-score) and <strong>percentile</strong> compare values
        <em>across contexts</em> — which asserts the metric measures the same thing in each. AĒR
        only allows that once a baseline + a cross-context equivalence study exist for the metric
        (ADR-016 / WP-004); <code>{activeMetric}</code> has none yet, so these stay disabled rather
        than show an unproven comparison. <strong>raw</strong> always works.
      </p>
    {/if}
  {/if}
</section>

<style>
  .cell-controls {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: linear-gradient(180deg, rgba(82, 131, 184, 0.08), rgba(82, 131, 184, 0.02));
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-md);
  }

  .cell-controls.locked {
    background: linear-gradient(180deg, rgba(150, 150, 150, 0.1), rgba(150, 150, 150, 0.04));
    border-color: var(--color-border);
  }

  .locked-banner {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    padding: var(--space-1) var(--space-2);
    background: var(--color-surface);
    border-radius: var(--radius-sm);
  }

  /* Phase 122k §14b finding 4 — clickable header gets a subtle resting
     background tint plus a stronger hover state so the user reads the
     full strip as an interactive surface, not just the chevron. */
  .cell-controls-header {
    appearance: none;
    background: color-mix(in srgb, var(--color-fg) 4%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-border) 50%, transparent);
    border-radius: var(--radius-sm);
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    text-align: left;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .cell-controls-header:hover,
  .cell-controls-header:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, transparent);
    border-color: var(--color-accent);
    color: var(--color-fg);
  }

  .header-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .collapse-toggle {
    margin-left: auto;
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: 1;
    padding: 0 var(--space-1);
  }

  .cell-controls.collapsed {
    padding-bottom: var(--space-2);
  }

  .ctrl-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    padding: var(--space-2) 0;
  }
  /* Phase 122k §14 finding 5 — subtle vertical separators between rows
     so the composition / view / metric / window blocks read as discrete
     control groups. The last row gets no bottom border. */
  .ctrl-row + .ctrl-row {
    border-top: 1px dashed color-mix(in srgb, var(--color-border) 50%, transparent);
  }

  .ctrl-row-split {
    gap: var(--space-4);
  }

  /* Phase 122k §14c finding 1 — Composition row layout. Two labeled
     control groups (Composition / Direction) separated by a thin
     vertical divider. Direction appears only when Split is active. */
  .composition-row {
    align-items: flex-start;
  }
  .comp-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .comp-group > .ctrl-options {
    display: flex;
    gap: 2px;
  }
  .comp-divider {
    align-self: stretch;
    width: 1px;
    background: color-mix(in srgb, var(--color-border) 60%, transparent);
    margin: 0 var(--space-2);
  }

  .ctrl-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }

  .ctrl-eyebrow {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
    min-width: 3.5rem;
    flex-shrink: 0;
  }

  .ctrl-options {
    display: inline-flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .ctrl-btn {
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    padding: 4px var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    font-family: var(--font-ui);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .ctrl-btn.metric-btn code {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: inherit;
  }

  .ctrl-btn:hover,
  .ctrl-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .ctrl-btn.active {
    color: var(--color-fg);
    background: rgba(82, 131, 184, 0.25);
    border-color: var(--color-accent);
  }

  .ctrl-btn.layer-btn.silver.active {
    color: #7ec4a0;
    background: rgba(126, 196, 160, 0.18);
    border-color: #7ec4a0;
  }

  .window-inputs {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .window-inputs input[type='date'] {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 3px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: text;
    color-scheme: dark;
  }
  .window-inputs input[type='date']:hover,
  .window-inputs input[type='date']:focus-visible {
    border-color: var(--color-accent);
  }

  .window-sep {
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  /* Phase 131 — per-cell config rows. */
  .config-row {
    align-items: center;
  }
  .config-inline {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1 1 auto;
    min-width: 0;
  }
  .config-inline input[type='range'] {
    flex: 1 1 auto;
    max-width: 16rem;
    accent-color: var(--color-accent);
    cursor: pointer;
  }
  .config-value {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    min-width: 2.5ch;
    text-align: right;
  }
  .config-select {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 3px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
    max-width: 14rem;
  }
  .config-select:hover,
  .config-select:focus-visible {
    border-color: var(--color-accent);
  }

  /* Phase 131 (BUG1) — Compare "?" explainer. */
  .ctrl-help {
    appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    border: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
    line-height: 1;
    cursor: pointer;
    padding: 0;
  }
  .ctrl-help:hover,
  .ctrl-help:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
  .compare-help {
    margin: var(--space-2) 0 0;
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-xs);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    background: color-mix(in srgb, var(--color-accent) 6%, transparent);
    border-left: 2px solid var(--color-accent-muted);
    border-radius: var(--radius-sm);
  }
  .compare-help code {
    font-family: var(--font-mono);
  }

  /* Phase 123c (C1) — partial-metric (withheld) hint. A calm, low-emphasis
     note in the warning hue; never an error — the withholding is a
     deliberate honesty guard, not a failure. */
  .partial-hint {
    align-items: flex-start;
  }
  .partial-hint-body {
    margin: 0;
    font-size: var(--font-size-xs);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1 1 auto;
    min-width: 0;
  }
  .partial-hint-lead {
    margin: 0;
  }
  .partial-hint-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .partial-metric-row {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .partial-metric {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    background: color-mix(in srgb, var(--color-status-expired) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-status-expired) 24%, var(--color-border));
    border-radius: var(--radius-sm);
    padding: 0 4px;
    white-space: nowrap;
  }
  .partial-metric-detail {
    color: var(--color-fg-subtle);
  }
  .partial-metric-detail strong {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-semibold);
  }
  .partial-toggle {
    align-self: flex-start;
    margin-top: var(--space-1);
  }
  .partial-toggle.active {
    color: var(--color-accent);
    border-color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
  }

  @media (prefers-reduced-motion: reduce) {
    .ctrl-btn {
      transition: none;
    }
  }
</style>

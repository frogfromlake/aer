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
    scopeAvailableMetadataQuery,
    type AvailableMetricDto,
    type FetchContext,
    type QueryOutcome,
    type ScopeAvailableMetricsDto,
    type ScopeAvailableMetadataDto
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import {
    CROSS_PROBE_DEFAULT_METRIC,
    DEFAULT_METRIC_NAME,
    isMetadataMetric,
    metricSupportsPresentation,
    presentationsForPillar,
    resolvePresentation
  } from '$lib/presentations';
  import {
    DEFAULT_LOOKBACK_MS,
    type CellChannelBinding,
    type Normalization,
    type Resolution,
    type Presentation,
    type PillarId
  } from '$lib/state/url-internals';
  import {
    resetAllCellOverrides,
    updatePanel,
    type PanelPath
  } from '$lib/workbench/panel-mutators';
  import { availabilityScope, defaultMetricForScopes } from '$lib/workbench/panel-queries';
  import { viewerLabelLanguage } from '$lib/presentations/viewer-language';
  // Phase 126 — shared lever constants (defaults + network channel tables) so
  // the panel controls and the per-cell override popover cannot drift.
  import {
    DEFAULT_BINS,
    DEFAULT_FORCE_STRENGTH,
    DEFAULT_TOPN,
    NET_COLOR_CHANNELS,
    NET_SIZE_CHANNELS
  } from '$lib/workbench/cell-levers';

  interface Props {
    pillar: PillarId;
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
  // ADR-038 — availability is computed with FILTER semantics (a group naming
  // specific sources scopes to THOSE only), mirroring the render fan-out, so a
  // single-source scope is never widened to its whole probe.
  const panelScope = $derived(availabilityScope(boundPanel?.scopes ?? []));
  const hasScope = $derived(panelScope.probeIds.length > 0 || panelScope.sourceIds.length > 0);
  // ADR-038 — availability WITHHOLDING is now uniform across scope shapes: a
  // partial dimension (present on some but not all scoped sources) is withheld by
  // default and offered only via "show anyway", whether the panel spans one probe
  // or several. The default is always the intersection, so every cell is filled;
  // there is no within-frame vs cross-probe distinction here any more.

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
  const partialMetricSet = $derived<Set<string>>(new Set(partialMetrics.map((p) => p.metricName)));
  // The full resolved scoped-source list — denominator + "missing on" set.
  const scopedSourceNames = $derived<readonly string[]>(scopeAvail?.scopedSources ?? []);
  const scopedSourceCount = $derived(scopedSourceNames.length);
  // Issue 6 — "show anyway": when on, the picker also offers partial metrics.
  const activeShowWithheld = $derived(boundPanel?.showWithheld === true);
  // ADR-038 — a metric is offerable when there is no scope constraint yet, OR it
  // is in the all-source intersection (`available`), OR it is a PARTIAL metric
  // (present on some scoped source) AND the user opted to "show anyway". Filters
  // both the metric picker and the scatter selectors.
  //
  // The catalog is the GLOBAL `/metrics/available` union; this gate constrains it
  // to the SCOPE's INTERSECTION by default (Tier 1) so every cell is populated and
  // the comparison is apples-to-apples. Partials sit behind the uniform
  // "show anyway" toggle (Tier 2) — within-frame and cross-probe alike, no silent
  // within-frame folding. A single-source scope has no partials, so it collapses
  // to exactly that source's dimensions — matching its metadata-coverage matrix.
  function isScopeAvailable(name: string): boolean {
    if (scopeAvailableSet === null) return true;
    if (scopeAvailableSet.has(name)) return true;
    if (partialMetricSet.has(name) && activeShowWithheld) return true;
    return false;
  }
  // Issue 6 — the scoped sources that LACK a given partial metric (the "cause"
  // of the withholding), so the hint can name them instead of leaving the
  // user to trial-and-error.
  function missingSourcesFor(have: readonly string[]): string[] {
    const haveSet = new Set(have);
    return scopedSourceNames.filter((s) => !haveSet.has(s));
  }

  // Phase 133 — categorical metadata field availability for the scope (the
  // categorical analog of scopeAvailQ). Only fetched when the active view is
  // field-driven, so scalar-view panels pay nothing.
  const metadataAvailQ = createQuery<
    QueryOutcome<ScopeAvailableMetadataDto>,
    Error,
    QueryOutcome<ScopeAvailableMetadataDto>
  >(() => {
    const o = scopeAvailableMetadataQuery(ctx, {
      probeIds: panelScope.probeIds,
      sourceIds: panelScope.sourceIds,
      start: windowIso.start,
      end: windowIso.end
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      // Enabled on scope alone (like the metric `scopeAvailQ`), NOT gated on the
      // current view: `firstMetadataField()` runs inside pickView at the instant
      // of switching INTO a field view, when `viewUsesMetadataField` is still
      // false for the OLD view — gating here would leave the offerable list empty
      // and always seed `section`, even where `section` is Negative Space.
      enabled: hasScope
    };
  });
  const metadataAvail = $derived<ScopeAvailableMetadataDto | null>(
    hasScope && metadataAvailQ.data?.kind === 'success' ? metadataAvailQ.data.data : null
  );
  const availableMetadataFields = $derived<readonly string[]>(metadataAvail?.available ?? []);
  const partialMetadataFields = $derived<ScopeAvailableMetadataDto['partial']>(
    metadataAvail?.partial ?? []
  );
  // ADR-038 — the offerable categorical fields (no carried-over metric). Same
  // discipline as metrics: the INTERSECTION (`available`) by default (Tier 1);
  // partials (some-source) only under "show anyway" (Tier 2), within-frame and
  // cross-probe alike — never folded in silently. View-independent so it is
  // correct even when read from pickView during a switch INTO a field view.
  function offerableMetadataFields(): string[] {
    const seen: Record<string, true> = {};
    const out: string[] = [];
    const add = (f: string) => {
      if (f && !seen[f]) {
        seen[f] = true;
        out.push(f);
      }
    };
    for (const f of availableMetadataFields) add(f);
    if (activeShowWithheld) {
      for (const p of partialMetadataFields) add(p.field);
    }
    out.sort();
    return out;
  }
  // The picker list: offerable fields + the active field surfaced so the picker
  // reflects the current selection. Gated on the active view (only consumed
  // under `{#if viewUsesMetadataField}`).
  const metadataFields = $derived.by<string[]>(() => {
    if (!viewUsesMetadataField) return [];
    const out = offerableMetadataFields();
    if (activeMetric && !out.includes(activeMetric)) {
      out.push(activeMetric);
      out.sort();
    }
    return out;
  });
  // ADR-038 — the field to seed when switching to a field-driven view: the first
  // INTERSECTION field (deterministic, sorted), NO hard-coded field-name bias and
  // NO partial (a partial would seed a field some scoped source lacks → an
  // off-comparison default). Returns '' when the scope shares no categorical field
  // at all — the cell then shows honest Negative Space instead of querying a
  // non-existent field. Reads `availableMetadataFields` directly so it is correct
  // at the instant pickView runs (the active view is still the old one); the
  // reconcile $effect below repairs the seed once the availability query resolves.
  function firstMetadataField(): string {
    return [...availableMetadataFields].sort()[0] ?? '';
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

  const presentations = $derived(presentationsForPillar(pillar as PillarId));
  const activePresentation = $derived(resolvePresentation(boundPanel?.view ?? null, pillar));

  // Per-view capability flags (Phase 122h Findings round 3). Cells that
  // don't consume the metric / resolution prop get the corresponding
  // control hidden so the UI never misleads the user about what changes
  // when they click.
  const viewUsesMetric = $derived(activePresentation.usesMetric ?? true);
  // Phase 133 — the active view consumes a categorical metadata FIELD (carried
  // in `Panel.metric`) instead of a Gold metric. Drives the field picker below.
  const viewUsesMetadataField = $derived(activePresentation.usesMetadataField ?? false);
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

  // Phase 133 (Issue 4) — split the offered metrics into the analytical
  // measures (sentiment, entity_count, …) and the publisher-declared metadata
  // metrics (image_count, paywall_status, …), so the picker can group them under
  // distinct labels rather than imply they carry equal analytical weight.
  const analyticalMetrics = $derived<string[]>(metrics.filter((m) => !isMetadataMetric(m)));
  const metadataMetrics = $derived<string[]>(metrics.filter((m) => isMetadataMetric(m)));

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
  function firstMetricSupporting(view: Presentation): string {
    if (metricSupportsPresentation(DEFAULT_METRIC_NAME, view)) return DEFAULT_METRIC_NAME;
    return (
      availableMetricNames.find((m) => metricSupportsPresentation(m, view)) ?? DEFAULT_METRIC_NAME
    );
  }

  function pickView(id: Presentation) {
    if (id === activePresentation.id) return;
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const next = { ...p, view: id };
      // Phase 126 — per-cell overrides are presentation-specific (a bins
      // override is meaningless for a network view; a scatter-axis override for
      // a distribution). A view change discards them so they neither apply
      // silently nor linger in the URL without an affordance to clear them.
      delete next.cellOverrides;
      const pres = presentations.find((x) => x.id === id);
      // Phase 125a — metricSet (metric names) and fieldChain (categorical
      // fields) are now distinct Panel fields and can never collide. Drop
      // whichever the new view does not consume so a stale list neither lingers
      // in the URL nor blocks the seed below. Same-kind switches (matrix↔
      // parallel) keep metricSet; switching into sankey keeps fieldChain.
      const nextParams = pres?.configurableParams ?? [];
      if (!nextParams.includes('metricSet')) delete next.metricSet;
      if (!nextParams.includes('sankeyFields')) delete next.fieldChain;
      // Phase 125a — drop a stale facet field when the new view cannot facet, so
      // it neither lingers in the URL nor silently reactivates on a later switch
      // back (symmetry with metricSet/fieldChain/cellOverrides above).
      if (!(pres?.supportsFaceting ?? false)) delete next.facetField;
      const usesMetric = pres?.usesMetric ?? true;
      const nextUsesMetadataField = pres?.usesMetadataField ?? false;
      const prevUsesMetadataField = activePresentation.usesMetadataField ?? false;
      // Phase 133 — `Panel.metric` carries a categorical FIELD for field-driven
      // views and a Gold metric otherwise. Reconcile across that boundary so the
      // cell never receives the wrong kind of identifier: switching INTO a
      // field-driven view (from a metric view) seeds a default field; switching
      // OUT of one (into a metric view) must reset, because a field name like
      // "section" spuriously passes metricSupportsPresentation (scalar default).
      if (nextUsesMetadataField) {
        if (!prevUsesMetadataField || !next.metric) {
          next.metric = firstMetadataField();
        }
      } else if (usesMetric) {
        // Keep the Panel coherent: swap to a compatible metric when the current
        // identifier is a field (came from a field view) or an incompatible
        // metric, so the cell never renders a nonsensical (metric × view) pair.
        if (prevUsesMetadataField || !metricSupportsPresentation(next.metric, id)) {
          next.metric = firstMetricSupporting(id);
        }
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
      // Phase 125 — seed the N-metric set on first switch into a metricSet-driven
      // view (correlation_matrix, parallel_coordinates) so it renders at once.
      // The first up-to-4 scope-available scalar metrics; needs ≥2.
      if (
        (pres?.configurableParams ?? []).includes('metricSet') &&
        (next.metricSet?.length ?? 0) < 2
      ) {
        next.metricSet = scalarMetricOptions.slice(0, 4);
      }
      // Phase 125 — seed the Sankey field chain (Phase 125a: in `fieldChain`)
      // with the first up-to-3 offerable categorical fields.
      if (
        (pres?.configurableParams ?? []).includes('sankeyFields') &&
        (next.fieldChain?.length ?? 0) < 2
      ) {
        next.fieldChain = offerableMetadataFields().slice(0, 3);
      }
      // Phase 125 — seed the cross-tab numeric metric (channels.x) on first
      // switch so it renders at once; prefer a sentiment metric.
      if ((pres?.configurableParams ?? []).includes('crossMetric') && !next.channels?.x) {
        const opts = scalarMetricOptions;
        const seed =
          opts.find((m) => m === 'sentiment_score_bert_multilingual') ??
          opts.find((m) => m.startsWith('sentiment_score')) ??
          opts.find((m) => m === 'word_count') ??
          opts[0];
        if (seed) next.channels = { ...(next.channels ?? {}), x: seed };
      }
      // Phase 125 — seed the lead-lag x/y metrics on first switch (two distinct
      // metrics so the cell renders at once).
      if (
        (pres?.configurableParams ?? []).includes('leadLagAxes') &&
        (!next.channels?.x || !next.channels?.y)
      ) {
        const opts = scalarMetricOptions;
        const x =
          next.channels?.x ??
          opts.find((m) => m.startsWith('sentiment_score')) ??
          opts[0] ??
          DEFAULT_METRIC_NAME;
        const y =
          next.channels?.y ??
          (opts.includes('word_count') && x !== 'word_count'
            ? 'word_count'
            : (opts.find((m) => m !== x) ?? x));
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
    // Reconcile away a metric the scope does not carry. `isScopeAvailable`
    // already encodes the within-frame rule (partials stay selected on a
    // multi-source single probe — the cell renders empty there, honest
    // absence), so this fires for a single-source scope too: a stale/URL-set
    // metadata metric the lone source never emits snaps back to a valid one.
    if (scopeAvailableSet === null || activeShowWithheld) return;
    if (isScopeAvailable(activeMetric)) return;
    if (!availableMetricNames.includes(activeMetric)) return;
    const next = resetMetricForScope();
    if (next && next !== activeMetric) {
      updatePanel(panelPath, (p) => ({ ...p, metric: next }));
    }
  });

  // Phase 133 (Issue 6) + ADR-038 — categorical-field reconcile. The seed in
  // pickView runs synchronously and can fall through to a stale field when the
  // availability query has not yet resolved (or when a panel is CREATED already
  // in a field-driven view, where pickView never ran), or when toggling
  // "show anyway" OFF leaves a now-unofferable partial selected. Once
  // `metadataAvail` is in hand, if the active field is not offerable, snap to the
  // first offerable field — or to '' (Negative Space) when the scope shares NO
  // categorical field (intersection empty + show-anyway off). Converges: the
  // reset target is always offerable, so the next run early-returns.
  $effect(() => {
    if (!panelPath || !boundPanel || !viewUsesMetadataField) return;
    if (metadataAvail === null) return; // query pending / refused / errored
    const fields = offerableMetadataFields();
    if (activeMetric && fields.includes(activeMetric)) return;
    const next = firstMetadataField();
    if (next !== activeMetric) {
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
  // Phase 126 — does any cell in this panel carry a per-cell override? Drives the
  // panel-level "Reset all cells" affordance.
  const cellOverrideCount = $derived(Object.keys(boundPanel?.cellOverrides ?? {}).length);
  // Phase 126 — does any cell override its own X/Y axis? Such a cell measures a
  // different metric than its siblings, so it always reads free regardless of
  // this panel-level Shared/Free toggle. Drives the clarifying note under Scale.
  const hasAxisOverride = $derived(
    Object.values(boundPanel?.cellOverrides ?? {}).some(
      (ov) => ov.channels?.x !== undefined || ov.channels?.y !== undefined
    )
  );
  const activeBins = $derived(boundPanel?.bins ?? DEFAULT_BINS);
  const activeTopN = $derived(boundPanel?.topN ?? DEFAULT_TOPN);
  // The Top N lever is shared and per-view: categorical-distribution clamps to
  // 200 server-side; co-occurrence accepts up to 6000 edges — raising it past
  // ~500 auto-switches the network to the large-scale WebGL renderer (no
  // Maximize needed). Other views cap at 500 so the slider never silently noops.
  const isCooccurrenceView = $derived((boundPanel?.view ?? '') === 'cooccurrence_network');
  const topNMax = $derived(isCooccurrenceView ? 6000 : viewUsesMetadataField ? 200 : 500);
  const activeShowBand = $derived(boundPanel?.showBand ?? true);
  const activeShowEdges = $derived(boundPanel?.showEdges ?? false);
  const activeChannels = $derived<CellChannelBinding>(boundPanel?.channels ?? {});
  const activeForceStrength = $derived(boundPanel?.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const activeSettle = $derived(boundPanel?.settleSeconds ?? 12);
  let liveSettle = $state<number | null>(null);
  const displaySettle = $derived(liveSettle ?? activeSettle);
  // Phase 125 — the N-metric set for multivariate cells (correlation_matrix,
  // parallel_coordinates), persisted in Panel.metricSet.
  const activeMetricSet = $derived<readonly string[]>(boundPanel?.metricSet ?? []);
  // Toggle a metric in/out of the set (no-discovery-bias: the option pool is the
  // scope-available scalar metrics, not a capability-ranked list).
  function toggleMetricSetMember(name: string) {
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const cur = p.metricSet ?? [];
      const next = cur.includes(name) ? cur.filter((m) => m !== name) : [...cur, name];
      const out = { ...p };
      if (next.length > 0) out.metricSet = next;
      else delete out.metricSet;
      return out;
    });
  }

  // Phase 125a — the ordered categorical field chain for the Sankey cell,
  // persisted in Panel.fieldChain (split out of the overloaded metricSet).
  const activeFieldChain = $derived<readonly string[]>(boundPanel?.fieldChain ?? []);
  // Toggle a field in/out of the chain. Append on add so the alluvial column
  // order follows the user's selection order (no-discovery-bias: the pool is
  // the scope's offerable categorical fields).
  function toggleFieldChainMember(name: string) {
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const cur = p.fieldChain ?? [];
      const next = cur.includes(name) ? cur.filter((m) => m !== name) : [...cur, name];
      const out = { ...p };
      if (next.length > 0) out.fieldChain = next;
      else delete out.fieldChain;
      return out;
    });
  }

  // Phase 125a — faceting / small-multiples. The facet field is a categorical
  // metadata field (no-discovery-bias: the pool is `offerableMetadataFields()`,
  // never a capability-ranked list); "None" clears it. Offered only for
  // presentations that support faceting (the per-article cells).
  const viewSupportsFaceting = $derived(activePresentation.supportsFaceting ?? false);
  const activeFacetField = $derived<string>(boundPanel?.facetField ?? '');
  // For a field-driven view (categorical_distribution / cross_tab) the panel's
  // own field rides in `metric`; faceting BY that same field is degenerate (each
  // sub-cell collapses to one bar), so exclude it from the facet picker.
  const facetFieldOptions = $derived<string[]>(
    viewUsesMetadataField
      ? offerableMetadataFields().filter((f) => f !== activeMetric)
      : offerableMetadataFields()
  );
  function setFacetField(field: string) {
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      if (field) out.facetField = field;
      else delete out.facetField;
      return out;
    });
  }

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
    const clamped = Math.min(topNMax, Math.max(1, Math.round(n)));
    if (clamped === activeTopN) return;
    updatePanel(panelPath, (p) => ({ ...p, topN: clamped }));
  }
  function setForceStrength(n: number) {
    if (!panelPath || !Number.isFinite(n)) return;
    const clamped = Math.min(100, Math.max(0, Math.round(n)));
    if (clamped === activeForceStrength) return;
    updatePanel(panelPath, (p) => ({ ...p, forceStrength: clamped }));
  }
  // Co-occurrence redesign — large-scale layout settle time (seconds).
  function setSettle(n: number) {
    if (!panelPath || !Number.isFinite(n)) return;
    const clamped = Math.min(60, Math.max(3, Math.round(n)));
    if (clamped === activeSettle) return;
    updatePanel(panelPath, (p) => ({ ...p, settleSeconds: clamped }));
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
  // Co-occurrence redesign — show/hide the edge (connection) lines. HIDDEN is the
  // default, so store only the explicit "shown" state and omit the default.
  function setShowEdges(next: boolean) {
    if (!panelPath || next === activeShowEdges) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next)
        o.showEdges = true; // shown is non-default → store
      else delete o.showEdges; // hidden is the default → keep URL clean
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
      // Phase 125 — selecting the network 'metric' channel needs a metric to
      // aggregate; seed one if none is bound yet so the cell renders at once.
      // ISSUE 7: size and colour seed independently (colour reuses the size
      // metric when none of its own is set, so it only seeds when neither is).
      const seedMetric = () =>
        scalarMetricOptions.find((m) => m.startsWith('sentiment_score')) ?? scalarMetricOptions[0];
      if (key === 'netSize' && value === 'metric' && !ch.netMetric) {
        const seed = seedMetric();
        if (seed) ch.netMetric = seed;
      }
      if (key === 'netColor' && value === 'metric' && !ch.netColorMetric && !ch.netMetric) {
        const seed = seedMetric();
        if (seed) ch.netColorMetric = seed;
      }
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
          {#each analyticalMetrics as m (m)}
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
          {#if metadataMetrics.length > 0}
            <span class="metric-group-label" aria-hidden="true">Metadata</span>
            {#each metadataMetrics as m (m)}
              <button
                type="button"
                role="radio"
                aria-checked={activeMetric === m}
                class="ctrl-btn metric-btn metadata-metric"
                class:active={activeMetric === m}
                onclick={() => pickMetric(m)}
                title="Publisher-declared metadata (structural; less analytical weight than the NLP measures)"
              >
                <code>{m}</code>
              </button>
            {/each}
          {/if}
        </div>
      </div>
    {/if}

    <!-- Phase 133 — categorical metadata FIELD picker. Shown only for
         field-driven views (categorical_distribution). The field is the
         GROUPING dimension (hence the "Group by" eyebrow, distinct from a
         measured "Metric") and rides in Panel.metric. A native <select>
         scales past the metric button-row as the field set grows. -->
    {#if viewUsesMetadataField}
      <div class="ctrl-row config-row" role="group" aria-label="Group-by field">
        <span class="ctrl-eyebrow">Group by</span>
        {#if metadataFields.length > 0}
          <select
            class="config-select"
            value={activeMetric}
            onchange={(e) => pickMetric((e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Categorical metadata field to group by"
          >
            {#each metadataFields as f (f)}
              <option value={f}>{f}</option>
            {/each}
          </select>
        {:else}
          <span class="field-empty">No categorical metadata for this scope (Negative Space).</span>
        {/if}
      </div>

      <!-- ADR-038 — metadata withholding (mirror of the metric hint). Fields
           absent on some scoped source are withheld unless "show anyway",
           uniformly within-frame AND cross-probe (never folded in silently). -->
      {#if partialMetadataFields.length > 0}
        <div class="ctrl-row partial-hint" role="note">
          <span class="ctrl-eyebrow">Withheld</span>
          <div class="partial-hint-body">
            <p class="partial-hint-lead">
              {partialMetadataFields.length} metadata field{partialMetadataFields.length === 1
                ? ''
                : 's'} not present on every one of the {scopedSourceCount} scoped source{scopedSourceCount ===
              1
                ? ''
                : 's'} (metadata asymmetry — a publisher choice, WP-003 §3.2):
            </p>
            <ul class="partial-hint-list" role="list">
              {#each partialMetadataFields as pf (pf.field)}
                {@const missing = missingSourcesFor(pf.sources)}
                <li class="partial-metric-row">
                  <code class="partial-metric">{pf.field}</code>
                  <span class="partial-metric-detail">
                    has {pf.sources.length}/{scopedSourceCount}{#if missing.length > 0}
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
                title="Offer these fields anyway. Sources lacking the chosen field render Negative Space, not a forced zero."
              >
                {activeShowWithheld ? '✓ Showing withheld' : 'Show anyway'}
              </button>
            {/if}
          </div>
        </div>
      {/if}
    {/if}

    <!-- Phase 123c (C1) + ADR-038 — partial-metric hint. Surfaces metrics
         that have data for only SOME scoped sources, and names the sources
         that LACK each one (the "cause") so the user doesn't have to trial-
         and-error. "Show anyway" offers them in the picker regardless; the
         panel then renders only the sources that carry the chosen metric.
         Shown uniformly within-frame AND cross-probe (never folded in silently). -->
    {#if partialMetrics.length > 0 && (viewUsesMetric || configParams.includes('scatterAxes'))}
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
              max={topNMax}
              step="5"
              value={activeTopN}
              oninput={(e) => (liveTopN = Number((e.currentTarget as HTMLInputElement).value))}
              onchange={(e) => {
                setTopN(Number((e.currentTarget as HTMLInputElement).value));
                liveTopN = null;
              }}
              aria-label="Top N slider"
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

      {#if configParams.includes('settleTime')}
        <div class="ctrl-row config-row" role="group" aria-label="Layout settle time">
          <span class="ctrl-eyebrow">Settle</span>
          <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
            <input
              type="range"
              min="3"
              max="60"
              step="1"
              value={activeSettle}
              oninput={(e) => (liveSettle = Number((e.currentTarget as HTMLInputElement).value))}
              onchange={(e) => {
                setSettle(Number((e.currentTarget as HTMLInputElement).value));
                liveSettle = null;
              }}
              title="Seconds the large-scale layout runs before it freezes. Raise it to give a big map more time to relax into clusters."
              aria-label="Layout settle time in seconds"
            />
            <output class="config-value">{displaySettle}s</output>
          </div>
        </div>
      {/if}

      {#if configParams.includes('showEdges')}
        <div class="ctrl-row config-row" role="group" aria-label="Connection lines">
          <span class="ctrl-eyebrow">Connections</span>
          <button
            type="button"
            role="switch"
            aria-checked={activeShowEdges}
            class="ctrl-btn"
            class:active={activeShowEdges}
            onclick={() => setShowEdges(!activeShowEdges)}
            title="Show or hide the edge lines between nodes. A nodes-only view is clearer for a dense map; clustering still shows the relationships."
          >
            {activeShowEdges ? 'lines shown' : 'lines hidden'}
          </button>
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
        {#if hasAxisOverride}
          <!-- Phase 126 — the panel Scale toggle is the default for cells that
               CAN share. A cell with its own X/Y axis measures a different
               metric and always reads free, independent of this. Disclose so the
               panel button never reads as a promise the custom cell can't keep. -->
          <p class="scale-note">
            ⓘ Cells with a custom X/Y axis always read free — independent of this.
          </p>
        {/if}
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
        <!-- Phase 125 — when a network channel is bound to 'metric', pick which
             per-article metric is aggregated onto the nodes. ISSUE 7: size and
             colour can bind to DIFFERENT metrics, so each gets its own picker. -->
        {#if activeChannels.netSize === 'metric'}
          <div class="ctrl-row config-row" role="group" aria-label="Size metric">
            <span class="ctrl-eyebrow">Size metric</span>
            <select
              class="config-select"
              value={activeChannels.netMetric ?? ''}
              onchange={(e) =>
                setChannel('netMetric', (e.currentTarget as HTMLSelectElement).value)}
              onclick={(e) => e.stopPropagation()}
              aria-label="Node size metric"
            >
              <option value="" disabled>— pick a metric —</option>
              {#each scalarMetricOptions as m (m)}
                <option value={m}>{m}</option>
              {/each}
            </select>
          </div>
        {/if}
        {#if activeChannels.netColor === 'metric'}
          <div class="ctrl-row config-row" role="group" aria-label="Colour metric">
            <span class="ctrl-eyebrow">Colour metric</span>
            <select
              class="config-select"
              value={activeChannels.netColorMetric ?? activeChannels.netMetric ?? ''}
              onchange={(e) =>
                setChannel('netColorMetric', (e.currentTarget as HTMLSelectElement).value)}
              onclick={(e) => e.stopPropagation()}
              aria-label="Node colour metric"
            >
              <option value="" disabled>— pick a metric —</option>
              {#each scalarMetricOptions as m (m)}
                <option value={m}>{m}</option>
              {/each}
            </select>
          </div>
        {/if}
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

      <!-- Phase 125 — N-metric set picker for multivariate cells (correlation
           matrix, parallel coordinates). A checkbox list of scope-available
           scalar metrics (no-discovery-bias); ≥2 needed to render. -->
      {#if configParams.includes('metricSet')}
        <div class="ctrl-row config-row" role="group" aria-label="Metric set">
          <span class="ctrl-eyebrow">Metric set</span>
          <div class="metric-set-options" onclick={(e) => e.stopPropagation()} role="presentation">
            {#each scalarMetricOptions as m (m)}
              <label class="metric-set-chip" class:active={activeMetricSet.includes(m)}>
                <input
                  type="checkbox"
                  checked={activeMetricSet.includes(m)}
                  onchange={() => toggleMetricSetMember(m)}
                />
                <code>{m}</code>
              </label>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Phase 125 — cross-tab numeric metric (a single metric select bound to
           channels.x). The categorical field comes from the Group-by picker. -->
      {#if configParams.includes('crossMetric')}
        <div class="ctrl-row config-row" role="group" aria-label="Cross-tab metric">
          <span class="ctrl-eyebrow">Metric</span>
          <select
            class="config-select"
            value={activeChannels.x ?? ''}
            onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Cross-tab numeric metric"
          >
            {#each scalarMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>
      {/if}

      <!-- Phase 125 — Sankey field chain: an ordered multi-select of categorical
           fields (Phase 125a: persisted in Panel.fieldChain). -->
      {#if configParams.includes('sankeyFields')}
        <div class="ctrl-row config-row" role="group" aria-label="Sankey fields">
          <span class="ctrl-eyebrow">Fields</span>
          <div class="metric-set-options" onclick={(e) => e.stopPropagation()} role="presentation">
            {#each offerableMetadataFields() as f (f)}
              <label class="metric-set-chip" class:active={activeFieldChain.includes(f)}>
                <input
                  type="checkbox"
                  checked={activeFieldChain.includes(f)}
                  onchange={() => toggleFieldChainMember(f)}
                />
                <code>{f}</code>
              </label>
            {/each}
            {#if offerableMetadataFields().length === 0}
              <span class="field-empty"
                >No categorical metadata for this scope (Negative Space).</span
              >
            {/if}
          </div>
        </div>
      {/if}

      <!-- Phase 125 — lead-lag x/y metric pickers (x leads y; both bound to
           channels). -->
      {#if configParams.includes('leadLagAxes')}
        <div class="ctrl-row config-row" role="group" aria-label="Lead-lag metrics">
          <span class="ctrl-eyebrow">X → Y</span>
          <select
            class="config-select"
            value={activeChannels.x ?? ''}
            onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Lead-lag X metric (leads)"
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
            aria-label="Lead-lag Y metric (follows)"
          >
            {#each scalarMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>
      {/if}

      <!-- Phase 125a — faceting / small-multiples. Break the cell into one
           sub-cell per value of a categorical field (no-discovery-bias pool;
           "None" clears). Offered only for per-article presentations. -->
      {#if viewSupportsFaceting && panelPath}
        <div class="ctrl-row config-row" role="group" aria-label="Facet by field">
          <span class="ctrl-eyebrow">Facet by</span>
          <select
            class="config-select"
            value={activeFacetField}
            onchange={(e) => setFacetField((e.currentTarget as HTMLSelectElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Facet by categorical field"
          >
            <option value="">— None —</option>
            {#each facetFieldOptions as f (f)}
              <option value={f}>{f}</option>
            {/each}
          </select>
          {#if facetFieldOptions.length === 0}
            <!-- ISSUE 2 (re-test) — a scope with no categorical fields (e.g.
                 bundesregierung emits no article metadata at all) is an honest
                 structural absence, framed as Negative Space — not a defect.
                 Wording aligned with the other empty-metadata disclosures. -->
            <span class="field-empty">No categorical metadata for this scope (Negative Space).</span
            >
          {/if}
        </div>
      {/if}

      <!-- Phase 126 — panel-level "Reset all cells" clears every per-cell
           override at once. Shown only when at least one cell is overridden;
           the per-cell popover carries the single-cell reset. -->
      {#if cellOverrideCount > 0 && panelPath}
        <div class="ctrl-row config-row" role="group" aria-label="Per-cell overrides">
          <span class="ctrl-eyebrow">Cells</span>
          <button
            type="button"
            class="ctrl-btn"
            onclick={() => panelPath && resetAllCellOverrides(panelPath)}
            title="Clear the custom per-cell configuration on all {cellOverrideCount} overridden cell(s) and return them to the panel default"
          >
            ↺ Reset {cellOverrideCount} custom cell{cellOverrideCount === 1 ? '' : 's'}
          </button>
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

  /* Phase 126 — clarifying note under the panel Scale toggle when a cell has a
     custom X/Y axis (then the panel default doesn't apply to that cell). */
  .scale-note {
    margin: 0 0 0 calc(3.5rem + var(--space-2));
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
  }

  /* Phase 133 — empty-state for the categorical field picker. */
  .field-empty {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
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

  /* Phase 133 (Issue 4) — metadata-metric group: a subtle inline label and a
     dimmer pill, so structural metadata reads as secondary to the NLP measures
     without leaving the same row. */
  .metric-group-label {
    align-self: center;
    margin-left: var(--space-2);
    padding-left: var(--space-2);
    border-left: 1px solid var(--color-border);
    font-size: var(--font-size-2xs, 10px);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-fg-subtle, var(--color-fg-muted));
  }

  .ctrl-btn.metadata-metric {
    border-style: dashed;
  }

  /* Phase 125 — metric-set multi-select chips. */
  .metric-set-options {
    display: inline-flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    min-width: 0;
  }
  .metric-set-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .metric-set-chip.active {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }
  .metric-set-chip code {
    font-family: var(--font-mono);
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

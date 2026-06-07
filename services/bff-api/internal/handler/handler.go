package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// genericInternalError is the opaque message returned to clients on any
// internal failure. Real error details are logged server-side only, so
// internal state (driver errors, SQL fragments, stack hints) never leaks
// across the trust boundary.
const genericInternalError = "internal server error"

// collectLanguagesForScope returns the distinct languages observed in
// `aer_gold.language_detections` for the requested scope and window.
// Phase 115: powers the cross-frame equivalence gate.
func (s *Server) collectLanguagesForScope(ctx context.Context, start, end time.Time, sources []string) ([]string, error) {
	return s.db.LanguagesForScope(ctx, start, end, sources)
}

// resolutionFromParam maps the OpenAPI-validated query enum onto the
// internal storage.Resolution constant. Unknown values fall back to the
// 5-minute baseline; the generated router rejects values outside the
// enum before the handler runs.
func resolutionFromParam(p *GetMetricsParamsResolution) storage.Resolution {
	if p == nil {
		return storage.ResolutionFiveMinute
	}
	switch *p {
	case GetMetricsParamsResolutionHourly:
		return storage.ResolutionHourly
	case GetMetricsParamsResolutionDaily:
		return storage.ResolutionDaily
	case GetMetricsParamsResolutionWeekly:
		return storage.ResolutionWeekly
	case GetMetricsParamsResolutionMonthly:
		return storage.ResolutionMonthly
	default:
		return storage.ResolutionFiveMinute
	}
}

// unionSourceParams merges the legacy single-source filter with the Phase 114
// comma-separated sourceIds parameter, deduplicating the result. An empty
// slice means no source filter — all sources are included.
func unionSourceParams(source, sourceIds *string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if source != nil {
		add(*source)
	}
	if sourceIds != nil {
		for _, src := range strings.Split(*sourceIds, ",") {
			add(src)
		}
	}
	return out
}

// Store abstracts the data access layer for testability.
type Store interface {
	Ping(ctx context.Context) error
	GetMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, error)
	// Phase 131: time-series with per-bucket sample stddev for the ±1σ band.
	GetMetricsWithSpread(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, error)
	GetNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, int64, error)
	// Phase 115: percentile-rank normalization, deviation labelling, cross-frame gate.
	GetPercentileNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, int64, error)
	CountLanguagesForSources(ctx context.Context, start, end time.Time, sources []string) (int, error)
	LanguagesForScope(ctx context.Context, start, end time.Time, sources []string) ([]string, error)
	CheckEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error)
	CheckNormalizationEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error)
	GetProbeEquivalence(ctx context.Context, start, end time.Time, sources []string) ([]storage.ProbeEquivalenceMetric, error)
	GetEquivalenceStatus(ctx context.Context, metricName string) (*storage.EquivalenceStatusRow, error)
	GetTemporalLeadLag(ctx context.Context, referenceSources, comparedSources []string, start, end time.Time, maxLagHours int) (storage.LeadLagResult, error)
	// Phase 125: generalised metric lead-lag (two metrics' hourly series, one scope).
	GetMetricLeadLag(ctx context.Context, sources []string, xMetric, yMetric string, start, end time.Time, maxLagHours int, metadataFilter *storage.MetadataFilter) (storage.LeadLagResult, error)
	// Phase 125: per-article N-metric matrix for parallel coordinates.
	GetParallelCoords(ctx context.Context, metrics, sources []string, start, end time.Time, maxPoints int, metadataFilter *storage.MetadataFilter) (storage.ParallelCoordResult, error)
	CheckBaselineExists(ctx context.Context, metricName string, source *string) (bool, error)
	CheckEquivalenceExists(ctx context.Context, metricName string) (bool, error)
	GetEntities(ctx context.Context, start, end time.Time, sources []string, label *string, limit int) ([]storage.EntityRow, error)
	GetLanguageDetections(ctx context.Context, start, end time.Time, sources []string, language *string, limit int) ([]storage.LanguageDetectionRow, error)
	GetAvailableMetrics(ctx context.Context, start, end time.Time) ([]storage.AvailableMetricRow, error)
	GetScopeAvailableMetrics(ctx context.Context, start, end time.Time, sources []string) (storage.ScopeMetricAvailability, error)
	GetMetricValidationStatus(ctx context.Context, metricName string) (string, error)
	GetMetricCulturalContextNotes(ctx context.Context, metricName string) (string, error)
	// Phase 102: view-mode query endpoints.
	GetMetricDistribution(ctx context.Context, metricName string, sources []string, start, end time.Time, bins int, metadataFilter *storage.MetadataFilter) (storage.DistributionResult, error)
	GetMetricHeatmap(ctx context.Context, metricName string, sources []string, xDim, yDim storage.HeatmapDimension, start, end time.Time) ([]storage.HeatmapCell, error)
	GetMetricCorrelation(ctx context.Context, metricNames []string, sources []string, start, end time.Time, metadataFilter *storage.MetadataFilter) (storage.CorrelationResult, error)
	GetEntityCoOccurrence(ctx context.Context, sources []string, start, end time.Time, topN int, viewerLanguage string, nodeMetric string, minWeight int, nsOverlay bool, colorMetric string) (storage.CoOccurrenceResult, error)
	// Phase 131: paired-metric scatter over aer_gold.metrics (visual-channel binding).
	GetMetricScatter(ctx context.Context, xMetric, yMetric string, sizeMetric, colorMetric *string, sources []string, start, end time.Time, maxPoints int, metadataFilter *storage.MetadataFilter) (storage.ScatterResult, error)
	// Phase 120: BERTopic topic-distribution endpoint over aer_gold.topic_assignments.
	GetTopicDistribution(ctx context.Context, params storage.TopicDistributionParams) ([]storage.TopicDistributionRow, error)
	// Phase 103b: silver-aggregation endpoints over aer_silver.documents.
	GetSilverDistribution(ctx context.Context, field string, source string, start, end time.Time, bins int) (storage.DistributionResult, error)
	GetSilverHeatmap(ctx context.Context, kind storage.SilverAggregationKind, source string, start, end time.Time) ([]storage.HeatmapCell, string, string, error)
	GetSilverCorrelation(ctx context.Context, source string, start, end time.Time) (storage.SilverCorrelationResult, error)
	// Phase 122f: metadata-coverage matrix over aer_gold.metadata_coverage.
	GetMetadataCoverage(ctx context.Context, sources []string) ([]storage.MetadataCoverageCell, error)
	// Phase 133: categorical metadata distribution + per-scope availability gate
	// over aer_gold.article_metadata.
	GetCategoricalDistribution(ctx context.Context, field string, sources []string, start, end time.Time, topN int, metadataFilter *storage.MetadataFilter) (storage.CategoricalDistributionResult, error)
	GetScopeAvailableMetadata(ctx context.Context, start, end time.Time, sources []string) (storage.ScopeMetadataAvailability, error)
	// Phase 125: cross-tab of a categorical field × a numeric metric (article_id JOIN).
	GetCrossTab(ctx context.Context, field, metric string, sources []string, start, end time.Time, topN int, metadataFilter *storage.MetadataFilter) (storage.CrossTabResult, error)
	// Phase 125: alluvial flow across an ordered chain of categorical fields.
	GetSankey(ctx context.Context, fields, sources []string, start, end time.Time, topN int) (storage.SankeyResult, error)
	// Phase 122d.0: Silent-Edit Observability — aggregation + per-article
	// chain over aer_gold.article_revisions.
	GetRevisionActivity(ctx context.Context, sources []string, start, end time.Time, resolution storage.RevisionActivityResolution) ([]storage.RevisionActivityCell, error)
	// Phase 122d.3: Silent-Edit Discourse Shift — re-extraction deltas
	// aggregated by (source, bucket).
	GetRevisionDiscourseShift(ctx context.Context, sources []string, start, end time.Time, resolution storage.RevisionActivityResolution) ([]storage.RevisionDiscourseShiftCell, error)
	GetRevisionEditClusters(ctx context.Context, sources []string, start, end time.Time, resolution storage.RevisionActivityResolution, minSources int) ([]storage.RevisionEditClusterRow, error)
	GetArticleRevisions(ctx context.Context, articleID string) ([]storage.ArticleRevisionRow, error)
	// Phase 122d.1: Silent-Edit Diff Substance + Drilldown.
	GetArticleRevisionDiff(ctx context.Context, articleID string, revisionIndex int) (*storage.ArticleRevisionDiffRow, error)
	GetRevisionsArticles(ctx context.Context, filter storage.RevisionsArticlesFilter) ([]storage.RevisionArticleRow, error)
}

// SourceLister abstracts the source-metadata read path so the handler
// does not care whether its backing store is Postgres, an in-memory fake
// (for tests), or a future alternative. A nil value is valid — the
// /sources endpoint will then return 500, which mirrors the behavior of
// a misconfigured stack where the read path was never wired up.
type SourceLister interface {
	List(ctx context.Context) ([]config.SourceEntry, error)
}

// Server implements the generated StrictServerInterface.
type Server struct {
	db                  Store
	provenance          config.MetricProvenanceMap
	sources             SourceLister
	catalog             config.ContentCatalog
	probes              config.ProbeRegistry
	dossier             DossierStore
	articles            ArticleQuerier
	silver              SilverFetcher
	kAnonymityThreshold int
	// languageManifest gates the `?language=` query parameter (Phase 118a /
	// ADR-024). Nil is permitted only in legacy test constructors that do
	// not exercise language-validated endpoints — callers that hit a
	// language gate with a nil manifest get the same 500 path as a
	// misconfigured stack.
	languageManifest *config.LanguageManifest
}

// ServerOptions carries the optional, Phase 101-introduced dependencies
// (dossier/articles/silver). They are optional because the existing test
// suite constructs Server with only the legacy dependencies.
type ServerOptions struct {
	Dossier             DossierStore
	Articles            ArticleQuerier
	Silver              SilverFetcher
	KAnonymityThreshold int
	LanguageManifest    *config.LanguageManifest
}

// NewServer creates a new API server instance with only the legacy
// dependencies. Tests that do not exercise the Phase 101 endpoints use
// this constructor unchanged.
func NewServer(db Store, provenance config.MetricProvenanceMap, sources SourceLister, catalog config.ContentCatalog, probes config.ProbeRegistry) *Server {
	return &Server{db: db, provenance: provenance, sources: sources, catalog: catalog, probes: probes}
}

// NewServerWithOptions wires the Phase 101 endpoints alongside the
// legacy dependencies. The cmd/server entrypoint uses this form once the
// Postgres dossier store and MinIO Silver store have been initialised.
func NewServerWithOptions(db Store, provenance config.MetricProvenanceMap, sources SourceLister, catalog config.ContentCatalog, probes config.ProbeRegistry, opts ServerOptions) *Server {
	s := NewServer(db, provenance, sources, catalog, probes)
	s.dossier = opts.Dossier
	s.articles = opts.Articles
	s.silver = opts.Silver
	s.kAnonymityThreshold = opts.KAnonymityThreshold
	if s.kAnonymityThreshold <= 0 {
		s.kAnonymityThreshold = 10
	}
	s.languageManifest = opts.LanguageManifest
	return s
}

// GetHealthz handles GET /healthz — liveness probe, always returns 200 if the process is alive.
func (s *Server) GetHealthz(_ context.Context, _ GetHealthzRequestObject) (GetHealthzResponseObject, error) {
	return GetHealthz200JSONResponse{"status": "alive"}, nil
}

// GetReadyz handles GET /readyz — readiness probe, returns 200 only if ClickHouse is reachable.
func (s *Server) GetReadyz(ctx context.Context, _ GetReadyzRequestObject) (GetReadyzResponseObject, error) {
	if err := s.db.Ping(ctx); err != nil {
		slog.Error("handler failure", "op", "GetReadyz", "error", err)
		return GetReadyz503JSONResponse{"clickhouse": "unavailable"}, nil
	}
	return GetReadyz200JSONResponse{"clickhouse": "ok"}, nil
}

// crossFrameAnchor is the canonical pointer into the methodological surface
// (WP-004 §5.2) used by the Phase-115 cross-frame equivalence refusal.
const crossFrameAnchor = "WP-004#section-5.2"

// crossFrameGateID matches RefusalPayloadGate's metric_equivalence value.
const crossFrameGateID = "metric_equivalence"

// crossFrameRefusalAlternatives are the three concrete user-actionable
// fall-backs surfaced by the Phase-115 refusal payload (Brief §7.4).
var crossFrameRefusalAlternatives = []string{
	"drop normalization to Level 1 (temporal patterns only)",
	"constrain scope to one cultural frame (single language)",
	"use deviation labelling instead of an absolute claim",
}

// crossFrameRefusalMessage is the human-readable summary attached to the
// 400 RefusalPayload when a cross-frame normalization request is refused.
const crossFrameRefusalMessage = "cross-cultural normalization requires validated metric equivalence across the resolved language set; granted out-of-band via WP-004 §5.2"

// invalidLanguageGateID matches RefusalPayloadGate's invalid_language value
// (Phase 118a / ADR-024).
const invalidLanguageGateID = "invalid_language"

// invalidLanguageAnchor points into the methodological surface entry that
// describes the Capability Manifest workflow (Operations Playbook section
// "Editing the Language Capability Manifest"). It is intentionally not a
// working-paper anchor — the gate is engineering-procedural, not
// methodological.
const invalidLanguageAnchor = "ops/playbook#language-capability-manifest"

// validateLanguageQueryParam returns nil if the manifest declares the given
// language code (or if no language was supplied / no manifest is wired).
// Otherwise it returns the structured Error body for the invalid_language
// gate, with `alternatives` set to the manifest's sorted language codes.
//
// Phase 118a / ADR-024: replaces any hand-coded language allowlist in BFF
// handlers. Every endpoint that takes a `?language=` query parameter must
// route through this helper before issuing a query.
func (s *Server) validateLanguageQueryParam(raw *string) (errBody *struct {
	Message            string
	Gate               string
	WorkingPaperAnchor string
	Alternatives       []string
}, ok bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	if s.languageManifest == nil {
		// No manifest wired — the validator cannot run. Permit the request
		// rather than 500, matching the legacy behaviour for tests that
		// construct Server without the manifest dependency.
		return nil, true
	}
	if s.languageManifest.IsKnown(*raw) {
		return nil, true
	}
	codes := s.languageManifest.LanguageCodes()
	return &struct {
		Message            string
		Gate               string
		WorkingPaperAnchor string
		Alternatives       []string
	}{
		Message: fmt.Sprintf(
			"unknown language %q; the Language Capability Manifest declares: %v",
			*raw, codes,
		),
		Gate:               invalidLanguageGateID,
		WorkingPaperAnchor: invalidLanguageAnchor,
		Alternatives:       codes,
	}, false
}

// crossFrameRefusal constructs the structured 400 body for the
// metric_equivalence gate. The fields piggy-back on the Error schema's
// optional refusal extensions so existing callers that decode 400 as
// {message: string} still work.
func crossFrameRefusal() GetMetrics400JSONResponse {
	gate := crossFrameGateID
	anchor := crossFrameAnchor
	alts := append([]string(nil), crossFrameRefusalAlternatives...)
	return GetMetrics400JSONResponse{
		Message:            crossFrameRefusalMessage,
		Gate:               &gate,
		WorkingPaperAnchor: &anchor,
		Alternatives:       &alts,
	}
}

// parseMetadataFilter builds the Phase-125a faceting restriction from the two
// optional query params (metadataFilterField + metadataFilterValue). Both must be
// present and non-blank; otherwise it returns nil (no faceting). Trimmed so a
// stray-whitespace value never silently empties a facet.
func parseMetadataFilter(field, value *string) *storage.MetadataFilter {
	if field == nil || value == nil {
		return nil
	}
	f := strings.TrimSpace(*field)
	v := strings.TrimSpace(*value)
	if f == "" || v == "" {
		return nil
	}
	return &storage.MetadataFilter{Field: f, Value: v}
}

// crossFrameRefusalFields carries the structured 400 refusal payload shared by
// every Phase-125 cross-frame equivalence gate. Each handler returns its own
// typed *400JSONResponse, so the gate hands back these fields rather than a
// concrete response type; the caller maps them onto its response struct.
type crossFrameRefusalFields struct {
	Message            string
	Gate               *string
	WorkingPaperAnchor *string
	Alternatives       *[]string
}

// crossFrameGate runs the Phase-125 cross-frame equivalence gate for a set of
// metrics over a resolved scope. Returns:
//   - (refusal, nil) — the scope spans >1 language and at least one metric
//     lacks a cross-cultural equivalence grant; the caller returns its typed
//     400 built from refusal's fields (Level-1 alternative: stay in one frame).
//   - (nil, err)     — an internal error occurred; the caller returns 500.
//   - (nil, nil)     — the gate passed (single frame, or all metrics granted).
//
// Extracted in Phase 125a from four identical copies (GetMetricCorrelation,
// GetCorrelationLeadLag, GetMetricParallelCoords, GetMetadataCrossTab).
func (s *Server) crossFrameGate(ctx context.Context, metrics []string, sources []string, start, end time.Time) (*crossFrameRefusalFields, error) {
	nLangs, err := s.db.CountLanguagesForSources(ctx, start, end, sources)
	if err != nil {
		return nil, err
	}
	if nLangs <= 1 {
		return nil, nil
	}
	languages, err := s.collectLanguagesForScope(ctx, start, end, sources)
	if err != nil {
		return nil, err
	}
	for _, m := range metrics {
		granted, err := s.db.CheckNormalizationEquivalenceForLanguages(ctx, m, languages)
		if err != nil {
			return nil, err
		}
		if !granted {
			gate := crossFrameGateID
			anchor := crossFrameAnchor
			alts := append([]string(nil), crossFrameRefusalAlternatives...)
			return &crossFrameRefusalFields{
				Message:            crossFrameRefusalMessage,
				Gate:               &gate,
				WorkingPaperAnchor: &anchor,
				Alternatives:       &alts,
			}, nil
		}
	}
	return nil, nil
}

// GetMetrics handles the GET /metrics request and fetches time-series data.
// startDate and endDate are OPTIONAL: omit both for the whole dataset (time
// limiting is an optional feature, not the default); supplying one without the
// other is rejected.
//
// When normalization=zscore (Phase 65) or normalization=percentile (Phase 115),
// a validation gate enforces:
// (a) baselines must exist for the requested (metricName, source) pair;
// (b) an admissible equivalence level must be confirmed in `metric_equivalence`.
// The admissible level is metric-class-aware (Phase 124): a temporal-axis
// metric (publication_hour/weekday) is satisfied by a temporal Level-1 grant;
// every other metric requires deviation-or-absolute;
// (c) Phase 115 cross-frame gate — when the resolved scope spans more than one
// language, that admissible equivalence must additionally be granted across
// every language in the scope. Otherwise the response is 400 with a structured
// RefusalPayload (gate=metric_equivalence, anchor=WP-004#section-5.2).
func (s *Server) GetMetrics(ctx context.Context, request GetMetricsRequestObject) (GetMetricsResponseObject, error) {
	if request.Params.Normalization != nil && !request.Params.Normalization.Valid() {
		return GetMetrics400JSONResponse{Message: "invalid normalization; must be one of raw, zscore, percentile"}, nil
	}
	if request.Params.Resolution != nil && !request.Params.Resolution.Valid() {
		return GetMetrics400JSONResponse{Message: "invalid resolution; must be one of 5min, hourly, daily, weekly, monthly"}, nil
	}

	mode := Raw
	if request.Params.Normalization != nil {
		mode = *request.Params.Normalization
	}
	useNormalization := mode == Zscore || mode == Percentile

	// Phase 117 read-side alias: `sentiment_score` → `sentiment_score_sentiws`.
	canonicalMetricNamePtr(request.Params.MetricName)

	sources := unionSourceParams(request.Params.Source, request.Params.SourceIds)

	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetMetrics400JSONResponse{Message: msg}, nil
	}

	if useNormalization {
		if request.Params.MetricName == nil {
			return GetMetrics400JSONResponse{Message: "normalization requires the metricName parameter"}, nil
		}

		// For the baseline check, pass a single source when unambiguous; nil
		// means "any source" which is the correct fallback for multi-source requests.
		var bsSource *string
		if len(sources) == 1 {
			bsSource = &sources[0]
		}
		baselineExists, err := s.db.CheckBaselineExists(ctx, *request.Params.MetricName, bsSource)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CheckBaselineExists", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if !baselineExists {
			return GetMetrics400JSONResponse{Message: "no baseline data exists for the requested metric and source; compute baselines before requesting normalization"}, nil
		}

		equivExists, err := s.db.CheckEquivalenceExists(ctx, *request.Params.MetricName)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CheckEquivalenceExists", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if !equivExists {
			return GetMetrics400JSONResponse{Message: "no validated equivalence entry exists for this metric at a level admissible for normalization; cross-cultural comparability has not been validated"}, nil
		}

		// Phase 115 cross-frame equivalence gate.
		nLangs, err := s.db.CountLanguagesForSources(ctx, start, end, sources)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CountLanguagesForSources", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if nLangs > 1 {
			languages, err := s.collectLanguagesForScope(ctx, start, end, sources)
			if err != nil {
				slog.Error("handler failure", "op", "GetMetrics.collectLanguagesForScope", "error", err)
				return GetMetrics500JSONResponse{Message: genericInternalError}, nil
			}
			ok, err := s.db.CheckNormalizationEquivalenceForLanguages(ctx, *request.Params.MetricName, languages)
			if err != nil {
				slog.Error("handler failure", "op", "GetMetrics.CheckNormalizationEquivalenceForLanguages", "error", err)
				return GetMetrics500JSONResponse{Message: genericInternalError}, nil
			}
			if !ok {
				return crossFrameRefusal(), nil
			}
		}
	}

	resolution := resolutionFromParam(request.Params.Resolution)

	// Phase 131: a spread-bearing request reads the raw layer so the
	// per-bucket sample stddev (absent from the resolution MVs) is
	// available. Mutually exclusive with normalization — a z-score /
	// percentile series has no raw spread to attach.
	includeStddev := request.Params.IncludeStddev != nil && *request.Params.IncludeStddev && !useNormalization

	var data []storage.MetricRow
	var excludedCount int64
	var err error
	switch mode {
	case Zscore:
		data, excludedCount, err = s.db.GetNormalizedMetrics(ctx, start, end, sources, request.Params.MetricName, resolution)
	case Percentile:
		data, excludedCount, err = s.db.GetPercentileNormalizedMetrics(ctx, start, end, sources, request.Params.MetricName, resolution)
	default:
		if includeStddev {
			data, err = s.db.GetMetricsWithSpread(ctx, start, end, sources, request.Params.MetricName, resolution)
		} else {
			data, err = s.db.GetMetrics(ctx, start, end, sources, request.Params.MetricName, resolution)
		}
	}
	if err != nil {
		slog.Error("handler failure", "op", "GetMetrics", "error", err)
		return GetMetrics500JSONResponse{Message: genericInternalError}, nil
	}

	points := make([]struct {
		Count      *int64    `json:"count,omitempty"`
		MetricName string    `json:"metricName"`
		Source     string    `json:"source"`
		Stddev     *float64  `json:"stddev,omitempty"`
		Timestamp  time.Time `json:"timestamp"`
		Value      float64   `json:"value"`
	}, 0, len(data))
	for _, d := range data {
		// Narrow ClickHouse's UInt64 into the generated int64 DTO field.
		// Gold-layer bucket counts in a bounded time window fit well under
		// math.MaxInt64, but we clamp defensively rather than trust the type.
		count := max(int64(d.Count), 0) //nolint:gosec // clamped above
		p := struct {
			Count      *int64    `json:"count,omitempty"`
			MetricName string    `json:"metricName"`
			Source     string    `json:"source"`
			Stddev     *float64  `json:"stddev,omitempty"`
			Timestamp  time.Time `json:"timestamp"`
			Value      float64   `json:"value"`
		}{
			Count:      &count,
			Timestamp:  d.TS,
			Value:      d.Value,
			Source:     d.Source,
			MetricName: d.MetricName,
		}
		if includeStddev {
			stddev := d.Stddev
			p.Stddev = &stddev
		}
		points = append(points, p)
	}

	return GetMetrics200JSONResponse{
		Data:          points,
		ExcludedCount: excludedCount,
	}, nil
}

// GetEntities handles GET /entities — returns aggregated named entities.
// startDate and endDate are OPTIONAL: omit both for the whole dataset;
// supplying one without the other is rejected.
func (s *Server) GetEntities(ctx context.Context, request GetEntitiesRequestObject) (GetEntitiesResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 1000 {
		return GetEntities400JSONResponse{Message: "limit must be between 1 and 1000"}, nil
	}

	sources := unionSourceParams(request.Params.Source, request.Params.SourceIds)
	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetEntities400JSONResponse{Message: msg}, nil
	}
	data, err := s.db.GetEntities(ctx, start, end, sources, request.Params.Label, limit)
	if err != nil {
		slog.Error("handler failure", "op", "GetEntities", "error", err)
		return GetEntities500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetEntities200JSONResponse
	for _, d := range data {
		var qid *string
		var conf *float32
		if d.WikidataQid != "" {
			q := d.WikidataQid
			qid = &q
			c := d.LinkConfidence
			conf = &c
		}
		response = append(response, struct {
			Count          int64    `json:"count"`
			EntityLabel    string   `json:"entityLabel"`
			EntityText     string   `json:"entityText"`
			LinkConfidence *float32 `json:"linkConfidence,omitempty"`
			Sources        []string `json:"sources"`
			WikidataQid    *string  `json:"wikidataQid,omitempty"`
		}{
			EntityText:     d.EntityText,
			EntityLabel:    d.EntityLabel,
			Count:          int64(d.Count),
			Sources:        d.Sources,
			WikidataQid:    qid,
			LinkConfidence: conf,
		})
	}

	return response, nil
}

// GetLanguages handles GET /languages — returns aggregated language detections.
// startDate and endDate are OPTIONAL: omit both for the whole dataset, or supply
// one for an open-ended window (resolveWindow opens the other side).
func (s *Server) GetLanguages(ctx context.Context, request GetLanguagesRequestObject) (GetLanguagesResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 1000 {
		return GetLanguages400JSONResponse{Message: "limit must be between 1 and 1000"}, nil
	}
	if errBody, ok := s.validateLanguageQueryParam(request.Params.Language); !ok {
		gate := errBody.Gate
		anchor := errBody.WorkingPaperAnchor
		alts := errBody.Alternatives
		return GetLanguages400JSONResponse{
			Message:            errBody.Message,
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alts,
		}, nil
	}

	sources := unionSourceParams(request.Params.Source, request.Params.SourceIds)
	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetLanguages400JSONResponse{Message: msg}, nil
	}
	data, err := s.db.GetLanguageDetections(ctx, start, end, sources, request.Params.Language, limit)
	if err != nil {
		slog.Error("handler failure", "op", "GetLanguages", "error", err)
		return GetLanguages500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetLanguages200JSONResponse
	for _, d := range data {
		response = append(response, struct {
			AvgConfidence    float64  `json:"avgConfidence"`
			Count            int64    `json:"count"`
			DetectedLanguage string   `json:"detectedLanguage"`
			Sources          []string `json:"sources"`
		}{
			DetectedLanguage: d.DetectedLanguage,
			Count:            int64(d.Count),
			AvgConfidence:    d.AvgConfidence,
			Sources:          d.Sources,
		})
	}

	return response, nil
}

// GetMetricProvenance handles GET /metrics/{metricName}/provenance.
// Static fields (tier, algorithm description, known limitations, extractor
// version) come from the bundled metric_provenance.yaml config. Dynamic
// fields (validationStatus, culturalContextNotes) are resolved against the
// metric_validity / metric_equivalence ClickHouse tables.
func (s *Server) GetMetricProvenance(ctx context.Context, request GetMetricProvenanceRequestObject) (GetMetricProvenanceResponseObject, error) {
	// Phase 117 read-side alias: legacy `sentiment_score` URL paths still
	// resolve through the rename to `sentiment_score_sentiws`.
	request.MetricName = canonicalMetricName(request.MetricName)
	entry, ok := s.provenance[request.MetricName]
	if !ok {
		return GetMetricProvenance404JSONResponse{Message: "no provenance entry registered for metric"}, nil
	}

	status, err := s.db.GetMetricValidationStatus(ctx, request.MetricName)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricProvenance.GetMetricValidationStatus", "error", err)
		return GetMetricProvenance500JSONResponse{Message: genericInternalError}, nil
	}
	notes, err := s.db.GetMetricCulturalContextNotes(ctx, request.MetricName)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricProvenance.GetMetricCulturalContextNotes", "error", err)
		return GetMetricProvenance500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricProvenance200JSONResponse{
		MetricName:           request.MetricName,
		TierClassification:   MetricProvenanceTierClassification(entry.TierClassification),
		AlgorithmDescription: entry.AlgorithmDescription,
		KnownLimitations:     entry.KnownLimitations,
		ValidationStatus:     MetricProvenanceValidationStatus(status),
		ExtractorVersionHash: entry.ExtractorVersionHash,
	}
	if notes != "" {
		n := notes
		resp.CulturalContextNotes = &n
	}
	return resp, nil
}

// GetSources handles GET /sources — returns the list of known data
// sources with optional methodology documentation URLs. Data comes from
// the PostgreSQL `sources` table (the SSoT) via a TTL-cached read-only
// store. A misconfigured stack (nil source lister) or a Postgres outage
// with no warm cache surfaces as 500. When `silverOnly=true` (Phase 103),
// the response is filtered to sources whose `silver_eligible` flag is set
// so the dashboard's Silver-layer source picker does not surface sources
// the eligibility gate would refuse.
func (s *Server) GetSources(ctx context.Context, request GetSourcesRequestObject) (GetSourcesResponseObject, error) {
	if s.sources == nil {
		slog.Error("handler failure", "op", "GetSources", "error", "source lister is not configured")
		return GetSources500JSONResponse{Message: genericInternalError}, nil
	}
	entries, err := s.sources.List(ctx)
	if err != nil {
		slog.Error("handler failure", "op", "GetSources", "error", err)
		return GetSources500JSONResponse{Message: genericInternalError}, nil
	}
	silverOnly := request.Params.SilverOnly != nil && *request.Params.SilverOnly
	response := make(GetSources200JSONResponse, 0, len(entries))
	for _, src := range entries {
		if silverOnly && !src.SilverEligible {
			continue
		}
		response = append(response, Source{
			Name:             src.Name,
			Type:             src.Type,
			Url:              src.URL,
			DocumentationUrl: src.DocumentationURL,
		})
	}
	return response, nil
}

// GetProbes handles GET /probes — returns the list of active probes
// with emission geometry and bound sources. Registry is loaded from
// YAML at startup (no runtime I/O). Dual-Register editorial content is
// served separately via /content/probe/{probeId}.
func (s *Server) GetProbes(_ context.Context, _ GetProbesRequestObject) (GetProbesResponseObject, error) {
	entries := s.probes.Ordered()
	response := make(GetProbes200JSONResponse, 0, len(entries))
	for _, p := range entries {
		// The EmissionPoints element is an anonymous struct in the
		// generated code (oapi-codegen inlines sub-refs). We build it
		// positionally here rather than introducing a parallel named
		// type that would have to be kept in sync with the generator.
		probe := Probe{
			ProbeId:     p.ProbeID,
			DisplayName: p.Display(),
			ShortName:   p.Short(),
			Language:    p.Language,
			Sources:     append([]string(nil), p.Sources...),
			EmissionPoints: make([]struct {
				Label     string  `json:"label"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}, 0, len(p.EmissionPoints)),
		}
		for _, pt := range p.EmissionPoints {
			probe.EmissionPoints = append(probe.EmissionPoints, struct {
				Label     string  `json:"label"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}{
				Label:     pt.Label,
				Latitude:  pt.Latitude,
				Longitude: pt.Longitude,
			})
		}
		if p.Country != "" {
			c := p.Country
			probe.Country = &c
		}
		response = append(response, probe)
	}
	return response, nil
}

// GetMetricsAvailable handles GET /metrics/available — returns distinct metric names
// with validation status for a time range.
// startDate and endDate are OPTIONAL: omit both for the whole dataset;
// supplying one without the other is rejected.
func (s *Server) GetMetricsAvailable(ctx context.Context, request GetMetricsAvailableRequestObject) (GetMetricsAvailableResponseObject, error) {
	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetMetricsAvailable400JSONResponse{Message: msg}, nil
	}
	rows, err := s.db.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricsAvailable", "error", err)
		return GetMetricsAvailable500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetMetricsAvailable200JSONResponse
	for _, r := range rows {
		// Phase 121b: forward-looking alias guard. Any metric name registered
		// as an alias key in metric_aliases.go is dropped from the response —
		// its canonical replacement already appears in the same set, so the
		// alias entry can only ever surface as a duplicate in MetricSwitcher.
		// Pre-rename rows produced before a renaming Phase remain in the Gold
		// layer for the 365-day TTL window; this filter prevents them from
		// leaking back into the dashboard.
		if _, isAlias := metricNameAliases[r.MetricName]; isAlias {
			continue
		}
		m := AvailableMetric{
			MetricName:       r.MetricName,
			ValidationStatus: AvailableMetricValidationStatus(r.ValidationStatus),
		}
		if r.EticConstruct != nil {
			m.EticConstruct = r.EticConstruct
		}
		if r.EquivalenceLevel != nil {
			lvl := AvailableMetricEquivalenceLevel(*r.EquivalenceLevel)
			m.EquivalenceLevel = &lvl
		}
		if r.EquivalenceStatus != nil {
			es := r.EquivalenceStatus
			status := struct {
				Level          *string    `json:"level,omitempty"`
				Notes          string     `json:"notes"`
				ValidatedBy    *string    `json:"validatedBy,omitempty"`
				ValidationDate *time.Time `json:"validationDate,omitempty"`
			}{Notes: es.Notes}
			if es.Level != nil {
				lvl := *es.Level
				status.Level = &lvl
			}
			if es.ValidatedBy != nil {
				vb := *es.ValidatedBy
				status.ValidatedBy = &vb
			}
			if es.ValidationDate != nil {
				vd := *es.ValidationDate
				status.ValidationDate = &vd
			}
			m.EquivalenceStatus = &status
		}
		if minRes := config.LookupMinMeaningfulResolution(r.MetricName); minRes != "" {
			res := AvailableMetricMinMeaningfulResolution(minRes)
			m.MinMeaningfulResolution = &res
		}
		response = append(response, m)
	}

	return response, nil
}

// GetProbeEquivalence handles GET /probes/{probeId}/equivalence — Phase 115.
// Returns per-metric Level-1 / Level-2 / Level-3 availability for the
// probe's resolved source set. Drives the Probe Dossier "what comparisons
// are valid here" panel.
//
// The window defaults to the last 90 days when no explicit range is
// provided — the same default the Operations Playbook uses for baseline
// computation, so the Dossier matrix and the manual baseline run share a
// horizon.
func (s *Server) GetProbeEquivalence(ctx context.Context, request GetProbeEquivalenceRequestObject) (GetProbeEquivalenceResponseObject, error) {
	probe, ok := s.probes[request.ProbeId]
	if !ok {
		return GetProbeEquivalence404JSONResponse{Message: "probe not found"}, nil
	}

	// Phase 124: with comparedTo the scope is the UNION of both probes'
	// sources, so the matrix reports what is valid for the cross-probe pair.
	scopeSources := append([]string(nil), probe.Sources...)
	var comparedTo *string
	if request.Params.ComparedTo != nil && *request.Params.ComparedTo != "" {
		other, ok := s.probes[*request.Params.ComparedTo]
		if !ok {
			return GetProbeEquivalence404JSONResponse{Message: "comparedTo probe not found"}, nil
		}
		scopeSources = append(scopeSources, other.Sources...)
		scopeSources = uniqueNonEmpty(scopeSources...)
		id := other.ProbeID
		comparedTo = &id
	}

	end := time.Now().UTC()
	start := end.Add(-90 * 24 * time.Hour)

	rows, err := s.db.GetProbeEquivalence(ctx, start, end, scopeSources)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeEquivalence", "error", err)
		return GetProbeEquivalence500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetProbeEquivalence200JSONResponse{
		ProbeId:    probe.ProbeID,
		ComparedTo: comparedTo,
	}
	if len(scopeSources) > 0 {
		sources := append([]string(nil), scopeSources...)
		resp.Sources = &sources
	}
	for _, r := range rows {
		entry := struct {
			EquivalenceStatus *struct {
				Level          *string    `json:"level,omitempty"`
				Notes          string     `json:"notes"`
				ValidatedBy    *string    `json:"validatedBy,omitempty"`
				ValidationDate *time.Time `json:"validationDate,omitempty"`
			} `json:"equivalenceStatus,omitempty"`
			Level1Available bool   `json:"level1Available"`
			Level2Available bool   `json:"level2Available"`
			Level3Available bool   `json:"level3Available"`
			MetricName      string `json:"metricName"`
		}{
			MetricName:      r.MetricName,
			Level1Available: r.Level1Available,
			Level2Available: r.Level2Available,
			Level3Available: r.Level3Available,
		}
		if r.EquivalenceStatus != nil {
			es := r.EquivalenceStatus
			status := struct {
				Level          *string    `json:"level,omitempty"`
				Notes          string     `json:"notes"`
				ValidatedBy    *string    `json:"validatedBy,omitempty"`
				ValidationDate *time.Time `json:"validationDate,omitempty"`
			}{Notes: es.Notes}
			if es.Level != nil {
				lvl := *es.Level
				status.Level = &lvl
			}
			if es.ValidatedBy != nil {
				vb := *es.ValidatedBy
				status.ValidatedBy = &vb
			}
			if es.ValidationDate != nil {
				vd := *es.ValidationDate
				status.ValidationDate = &vd
			}
			entry.EquivalenceStatus = &status
		}
		resp.Metrics = append(resp.Metrics, entry)
	}
	return resp, nil
}

const (
	// leadLagDefaultMaxLagHours is ±7 days of hourly lags.
	leadLagDefaultMaxLagHours = 168
	leadLagMaxAllowedLagHours = 720
	// leadLagSignal names the Phase-124 lead-lag signal: hourly publication
	// activity (distinct-article count). Phase 125 generalises to metric series.
	leadLagSignal = "publication_activity"
	// leadLagGateMetric is the temporal-rhythm metric whose grant authorises
	// the cross-probe temporal lead-lag comparison.
	leadLagGateMetric = "publication_hour"
	// leadLagAnchor is the methodological anchor for the temporal Level-1 grant.
	leadLagAnchor = "WP-004 Appendix B"
)

// GetProbeLeadLag handles GET /probes/{probeId}/lead-lag — Phase 124. The
// lagged cross-correlation of hourly publication activity between the reference
// probe (`probeId`) and the compared probe (`comparedTo`). A cross-cultural
// relational artefact, so it is gated on a temporal-level equivalence grant
// covering both probes' languages; an ungranted pair returns a RefusalPayload-
// shaped 400. Phase 125 generalises this to arbitrary metric series.
func (s *Server) GetProbeLeadLag(ctx context.Context, request GetProbeLeadLagRequestObject) (GetProbeLeadLagResponseObject, error) {
	ref, ok := s.probes[request.ProbeId]
	if !ok {
		return GetProbeLeadLag404JSONResponse{Message: "probe not found"}, nil
	}
	compared, ok := s.probes[request.Params.ComparedTo]
	if !ok {
		return GetProbeLeadLag404JSONResponse{Message: "comparedTo probe not found"}, nil
	}
	if request.ProbeId == request.Params.ComparedTo {
		return GetProbeLeadLag400JSONResponse{Message: "comparedTo must differ from probeId"}, nil
	}

	// Window: explicit range when given, else the last 90 days (the same
	// horizon GetProbeEquivalence and the baseline run use).
	end := time.Now().UTC()
	start := end.Add(-90 * 24 * time.Hour)
	if request.Params.Start != nil {
		start = *request.Params.Start
	}
	if request.Params.End != nil {
		end = *request.Params.End
	}
	if !end.After(start) {
		return GetProbeLeadLag400JSONResponse{Message: "end must be strictly after start"}, nil
	}

	maxLag := leadLagDefaultMaxLagHours
	if request.Params.MaxLagHours != nil {
		maxLag = *request.Params.MaxLagHours
	}
	if maxLag < 1 || maxLag > leadLagMaxAllowedLagHours {
		return GetProbeLeadLag400JSONResponse{Message: "maxLagHours must be between 1 and 720"}, nil
	}

	// Gate: the cross-cultural temporal comparison is authorised only by a
	// temporal-level grant covering both probes' languages.
	languages := uniqueNonEmpty(ref.Language, compared.Language)
	granted, err := s.db.CheckNormalizationEquivalenceForLanguages(ctx, leadLagGateMetric, languages)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeLeadLag.CheckNormalizationEquivalenceForLanguages", "error", err)
		return GetProbeLeadLag500JSONResponse{Message: genericInternalError}, nil
	}
	if !granted {
		gate := crossFrameGateID
		anchor := leadLagAnchor
		alternatives := []string{
			"compare the two probes within a single cultural frame",
			"view each probe's temporal rhythm without a cross-cultural claim",
		}
		return GetProbeLeadLag400JSONResponse{
			Message:            "cross-probe lead-lag requires a temporal-level equivalence grant across both probes' languages; granted out-of-band via WP-004 §6.3",
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alternatives,
		}, nil
	}

	res, err := s.db.GetTemporalLeadLag(ctx, ref.Sources, compared.Sources, start, end, maxLag)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeLeadLag.GetTemporalLeadLag", "error", err)
		return GetProbeLeadLag500JSONResponse{Message: genericInternalError}, nil
	}

	bucketAtZero := res.BucketCountAtZero
	resp := GetProbeLeadLag200JSONResponse{
		ReferenceProbe:    ref.ProbeID,
		ComparedProbe:     compared.ProbeID,
		Signal:            leadLagSignal,
		MaxLagHours:       res.MaxLagHours,
		BucketCountAtZero: &bucketAtZero,
		PeakLagHours:      res.PeakLagHours,
		PeakCorrelation:   res.PeakCorrelation,
	}

	// Grant block for the methodology banner (server-authoritative).
	resp.Grant.Level = "temporal"
	resp.Grant.WorkingPaperAnchor = leadLagAnchor
	status, err := s.db.GetEquivalenceStatus(ctx, leadLagGateMetric)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeLeadLag.GetEquivalenceStatus", "error", err)
		return GetProbeLeadLag500JSONResponse{Message: genericInternalError}, nil
	}
	if status != nil {
		if status.Level != nil {
			resp.Grant.Level = *status.Level
		}
		if status.Notes != "" {
			notes := status.Notes
			resp.Grant.Notes = &notes
		}
		if status.ValidatedBy != nil {
			vb := *status.ValidatedBy
			resp.Grant.ValidatedBy = &vb
		}
	}

	for _, p := range res.Points {
		resp.Points = append(resp.Points, struct {
			Correlation *float64 `json:"correlation"`
			LagHours    int      `json:"lagHours"`
		}{Correlation: p.Correlation, LagHours: p.LagHours})
	}
	return resp, nil
}

// uniqueNonEmpty returns the distinct, non-empty values among its arguments,
// preserving first-seen order. Used to collapse a probe pair's languages into
// the set the equivalence gate must cover.
func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// GetContent handles GET /content/{entityType}/{entityId} — returns Dual-Register content
// for an entity. Locale defaults to "en".
func (s *Server) GetContent(_ context.Context, request GetContentRequestObject) (GetContentResponseObject, error) {
	if !request.EntityType.Valid() {
		return GetContent400JSONResponse{Message: "invalid entityType; must be one of metric, probe, discourse_function, refusal"}, nil
	}

	locale := string(GetContentParamsLocaleEn)
	if request.Params.Locale != nil {
		locale = string(*request.Params.Locale)
	}

	key := config.CatalogKey(locale, string(request.EntityType), request.EntityId)
	record, ok := s.catalog[key]
	if !ok {
		return GetContent404JSONResponse{Message: "no content found for the requested entity and locale"}, nil
	}

	date, err := time.Parse("2006-01-02", record.LastReviewedDate)
	if err != nil {
		slog.Error("handler failure", "op", "GetContent", "error", "invalid date in content record", "key", key)
		return GetContent500JSONResponse{Message: genericInternalError}, nil
	}

	var resp GetContent200JSONResponse
	resp.EntityId = record.EntityID
	resp.EntityType = ContentResponseEntityType(record.EntityType)
	resp.Locale = ContentResponseLocale(record.Locale)
	resp.Registers.Semantic.Short = record.Registers.Semantic.Short
	resp.Registers.Semantic.Long = record.Registers.Semantic.Long
	resp.Registers.Methodological.Short = record.Registers.Methodological.Short
	resp.Registers.Methodological.Long = record.Registers.Methodological.Long
	resp.ContentVersion = record.ContentVersion
	resp.LastReviewedBy = record.LastReviewedBy
	resp.LastReviewedDate = openapi_types.Date{Time: date}
	if len(record.WorkingPaperAnchors) > 0 {
		anchors := make([]string, len(record.WorkingPaperAnchors))
		copy(anchors, record.WorkingPaperAnchors)
		resp.WorkingPaperAnchors = &anchors
	}
	return resp, nil
}

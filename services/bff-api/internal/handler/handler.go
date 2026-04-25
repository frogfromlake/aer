package handler

import (
	"context"
	"log/slog"
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

// Store abstracts the data access layer for testability.
type Store interface {
	Ping(ctx context.Context) error
	GetMetrics(ctx context.Context, start, end time.Time, source, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, error)
	GetNormalizedMetrics(ctx context.Context, start, end time.Time, source, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, int64, error)
	CheckBaselineExists(ctx context.Context, metricName string, source *string) (bool, error)
	CheckEquivalenceExists(ctx context.Context, metricName string) (bool, error)
	GetEntities(ctx context.Context, start, end time.Time, source, label *string, limit int) ([]storage.EntityRow, error)
	GetLanguageDetections(ctx context.Context, start, end time.Time, source, language *string, limit int) ([]storage.LanguageDetectionRow, error)
	GetAvailableMetrics(ctx context.Context, start, end time.Time) ([]storage.AvailableMetricRow, error)
	GetMetricValidationStatus(ctx context.Context, metricName string) (string, error)
	GetMetricCulturalContextNotes(ctx context.Context, metricName string) (string, error)
	// Phase 102: view-mode query endpoints.
	GetMetricDistribution(ctx context.Context, metricName string, sources []string, start, end time.Time, bins int) (storage.DistributionResult, error)
	GetMetricHeatmap(ctx context.Context, metricName string, sources []string, xDim, yDim storage.HeatmapDimension, start, end time.Time) ([]storage.HeatmapCell, error)
	GetMetricCorrelation(ctx context.Context, metricNames []string, sources []string, start, end time.Time) (storage.CorrelationResult, error)
	GetEntityCoOccurrence(ctx context.Context, sources []string, start, end time.Time, topN int) (storage.CoOccurrenceResult, error)
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
}

// ServerOptions carries the optional, Phase 101-introduced dependencies
// (dossier/articles/silver). They are optional because the existing test
// suite constructs Server with only the legacy dependencies.
type ServerOptions struct {
	Dossier             DossierStore
	Articles            ArticleQuerier
	Silver              SilverFetcher
	KAnonymityThreshold int
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

// GetMetrics handles the GET /metrics request and fetches time-series data.
// startDate and endDate are required — the framework returns 400 before this handler
// is called if either is absent.
//
// When normalization=zscore, a validation gate enforces two preconditions:
// (a) baselines must exist for the requested (metricName, source) pair, and
// (b) at least deviation-level equivalence must be confirmed in metric_equivalence.
// This prevents normalized comparisons before interdisciplinary validation.
func (s *Server) GetMetrics(ctx context.Context, request GetMetricsRequestObject) (GetMetricsResponseObject, error) {
	if request.Params.Normalization != nil && !request.Params.Normalization.Valid() {
		return GetMetrics400JSONResponse{Message: "invalid normalization; must be one of raw, zscore"}, nil
	}
	if request.Params.Resolution != nil && !request.Params.Resolution.Valid() {
		return GetMetrics400JSONResponse{Message: "invalid resolution; must be one of 5min, hourly, daily, weekly, monthly"}, nil
	}

	useZscore := request.Params.Normalization != nil && *request.Params.Normalization == Zscore

	if useZscore {
		if request.Params.MetricName == nil {
			return GetMetrics400JSONResponse{Message: "normalization=zscore requires the metricName parameter"}, nil
		}

		baselineExists, err := s.db.CheckBaselineExists(ctx, *request.Params.MetricName, request.Params.Source)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CheckBaselineExists", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if !baselineExists {
			return GetMetrics400JSONResponse{Message: "no baseline data exists for the requested metric and source; compute baselines before requesting z-score normalization"}, nil
		}

		equivExists, err := s.db.CheckEquivalenceExists(ctx, *request.Params.MetricName)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CheckEquivalenceExists", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if !equivExists {
			return GetMetrics400JSONResponse{Message: "no equivalence entry with at least deviation-level equivalence exists for this metric; cross-cultural comparability has not been validated"}, nil
		}
	}

	resolution := resolutionFromParam(request.Params.Resolution)

	var data []storage.MetricRow
	var excludedCount int64
	var err error
	if useZscore {
		data, excludedCount, err = s.db.GetNormalizedMetrics(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.MetricName, resolution)
	} else {
		data, err = s.db.GetMetrics(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.MetricName, resolution)
	}
	if err != nil {
		slog.Error("handler failure", "op", "GetMetrics", "error", err)
		return GetMetrics500JSONResponse{Message: genericInternalError}, nil
	}

	points := make([]struct {
		Count      *int64    `json:"count,omitempty"`
		MetricName string    `json:"metricName"`
		Source     string    `json:"source"`
		Timestamp  time.Time `json:"timestamp"`
		Value      float64   `json:"value"`
	}, 0, len(data))
	for _, d := range data {
		// Narrow ClickHouse's UInt64 into the generated int64 DTO field.
		// Gold-layer bucket counts in a bounded time window fit well under
		// math.MaxInt64, but we clamp defensively rather than trust the type.
		count := max(int64(d.Count), 0) //nolint:gosec // clamped above
		points = append(points, struct {
			Count      *int64    `json:"count,omitempty"`
			MetricName string    `json:"metricName"`
			Source     string    `json:"source"`
			Timestamp  time.Time `json:"timestamp"`
			Value      float64   `json:"value"`
		}{
			Count:      &count,
			Timestamp:  d.TS,
			Value:      d.Value,
			Source:     d.Source,
			MetricName: d.MetricName,
		})
	}

	return GetMetrics200JSONResponse{
		Data:          points,
		ExcludedCount: excludedCount,
	}, nil
}

// GetEntities handles GET /entities — returns aggregated named entities.
// startDate and endDate are required — the framework returns 400 before this handler
// is called if either is absent.
func (s *Server) GetEntities(ctx context.Context, request GetEntitiesRequestObject) (GetEntitiesResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 1000 {
		return GetEntities400JSONResponse{Message: "limit must be between 1 and 1000"}, nil
	}

	data, err := s.db.GetEntities(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.Label, limit)
	if err != nil {
		slog.Error("handler failure", "op", "GetEntities", "error", err)
		return GetEntities500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetEntities200JSONResponse
	for _, d := range data {
		response = append(response, struct {
			Count       int64    `json:"count"`
			EntityLabel string   `json:"entityLabel"`
			EntityText  string   `json:"entityText"`
			Sources     []string `json:"sources"`
		}{
			EntityText:  d.EntityText,
			EntityLabel: d.EntityLabel,
			Count:       int64(d.Count),
			Sources:     d.Sources,
		})
	}

	return response, nil
}

// GetLanguages handles GET /languages — returns aggregated language detections.
// startDate and endDate are required — the framework returns 400 before this handler
// is called if either is absent.
func (s *Server) GetLanguages(ctx context.Context, request GetLanguagesRequestObject) (GetLanguagesResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 1000 {
		return GetLanguages400JSONResponse{Message: "limit must be between 1 and 1000"}, nil
	}

	data, err := s.db.GetLanguageDetections(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.Language, limit)
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
// with no warm cache surfaces as 500.
func (s *Server) GetSources(ctx context.Context, _ GetSourcesRequestObject) (GetSourcesResponseObject, error) {
	if s.sources == nil {
		slog.Error("handler failure", "op", "GetSources", "error", "source lister is not configured")
		return GetSources500JSONResponse{Message: genericInternalError}, nil
	}
	entries, err := s.sources.List(ctx)
	if err != nil {
		slog.Error("handler failure", "op", "GetSources", "error", err)
		return GetSources500JSONResponse{Message: genericInternalError}, nil
	}
	response := make(GetSources200JSONResponse, 0, len(entries))
	for _, src := range entries {
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
			ProbeId:        p.ProbeID,
			Language:       p.Language,
			Sources:        append([]string(nil), p.Sources...),
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
		response = append(response, probe)
	}
	return response, nil
}

// GetMetricsAvailable handles GET /metrics/available — returns distinct metric names
// with validation status for a time range.
// startDate and endDate are required — the framework returns 400 before this handler
// is called if either is absent.
func (s *Server) GetMetricsAvailable(ctx context.Context, request GetMetricsAvailableRequestObject) (GetMetricsAvailableResponseObject, error) {
	rows, err := s.db.GetAvailableMetrics(ctx, request.Params.StartDate, request.Params.EndDate)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricsAvailable", "error", err)
		return GetMetricsAvailable500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetMetricsAvailable200JSONResponse
	for _, r := range rows {
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
		if minRes := config.LookupMinMeaningfulResolution(r.MetricName); minRes != "" {
			res := AvailableMetricMinMeaningfulResolution(minRes)
			m.MinMeaningfulResolution = &res
		}
		response = append(response, m)
	}

	return response, nil
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

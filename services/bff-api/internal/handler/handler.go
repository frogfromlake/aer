package handler

import (
	"context"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

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
	GetNormalizedMetrics(ctx context.Context, start, end time.Time, source, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, error)
	CheckBaselineExists(ctx context.Context, metricName string, source *string) (bool, error)
	CheckEquivalenceExists(ctx context.Context, metricName string) (bool, error)
	GetEntities(ctx context.Context, start, end time.Time, source, label *string, limit int) ([]storage.EntityRow, error)
	GetLanguageDetections(ctx context.Context, start, end time.Time, source, language *string, limit int) ([]storage.LanguageDetectionRow, error)
	GetAvailableMetrics(ctx context.Context, start, end time.Time) ([]storage.AvailableMetricRow, error)
}

// Server implements the generated StrictServerInterface.
type Server struct {
	db Store
}

// NewServer creates a new API server instance.
func NewServer(db Store) *Server {
	return &Server{db: db}
}

// GetHealthz handles GET /healthz — liveness probe, always returns 200 if the process is alive.
func (s *Server) GetHealthz(_ context.Context, _ GetHealthzRequestObject) (GetHealthzResponseObject, error) {
	return GetHealthz200JSONResponse{"status": "alive"}, nil
}

// GetReadyz handles GET /readyz — readiness probe, returns 200 only if ClickHouse is reachable.
func (s *Server) GetReadyz(ctx context.Context, _ GetReadyzRequestObject) (GetReadyzResponseObject, error) {
	if err := s.db.Ping(ctx); err != nil {
		return GetReadyz503JSONResponse{"clickhouse": err.Error()}, nil
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
	useZscore := request.Params.Normalization != nil && *request.Params.Normalization == Zscore

	if useZscore {
		if request.Params.MetricName == nil {
			return GetMetrics400JSONResponse{Message: "normalization=zscore requires the metricName parameter"}, nil
		}

		baselineExists, err := s.db.CheckBaselineExists(ctx, *request.Params.MetricName, request.Params.Source)
		if err != nil {
			return GetMetrics500JSONResponse{Message: err.Error()}, nil
		}
		if !baselineExists {
			return GetMetrics400JSONResponse{Message: "no baseline data exists for the requested metric and source; compute baselines before requesting z-score normalization"}, nil
		}

		equivExists, err := s.db.CheckEquivalenceExists(ctx, *request.Params.MetricName)
		if err != nil {
			return GetMetrics500JSONResponse{Message: err.Error()}, nil
		}
		if !equivExists {
			return GetMetrics400JSONResponse{Message: "no equivalence entry with at least deviation-level equivalence exists for this metric; cross-cultural comparability has not been validated"}, nil
		}
	}

	resolution := resolutionFromParam(request.Params.Resolution)

	var data []storage.MetricRow
	var err error
	if useZscore {
		data, err = s.db.GetNormalizedMetrics(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.MetricName, resolution)
	} else {
		data, err = s.db.GetMetrics(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.MetricName, resolution)
	}
	if err != nil {
		return GetMetrics500JSONResponse{Message: err.Error()}, nil
	}

	var response GetMetrics200JSONResponse
	for _, d := range data {
		response = append(response, struct {
			MetricName string    `json:"metricName"`
			Source     string    `json:"source"`
			Timestamp  time.Time `json:"timestamp"`
			Value      float64   `json:"value"`
		}{
			Timestamp:  d.TS,
			Value:      d.Value,
			Source:     d.Source,
			MetricName: d.MetricName,
		})
	}

	return response, nil
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
		return GetEntities500JSONResponse{Message: err.Error()}, nil
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
		return GetLanguages500JSONResponse{Message: err.Error()}, nil
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

// GetMetricsAvailable handles GET /metrics/available — returns distinct metric names
// with validation status for a time range.
// startDate and endDate are required — the framework returns 400 before this handler
// is called if either is absent.
func (s *Server) GetMetricsAvailable(ctx context.Context, request GetMetricsAvailableRequestObject) (GetMetricsAvailableResponseObject, error) {
	rows, err := s.db.GetAvailableMetrics(ctx, request.Params.StartDate, request.Params.EndDate)
	if err != nil {
		return GetMetricsAvailable500JSONResponse{Message: err.Error()}, nil
	}

	var response GetMetricsAvailable200JSONResponse
	for _, r := range rows {
		m := AvailableMetric{
			MetricName:       r.MetricName,
			ValidationStatus: ValidationStatus(r.ValidationStatus),
		}
		if r.EticConstruct != nil {
			m.EticConstruct = r.EticConstruct
		}
		if r.EquivalenceLevel != nil {
			lvl := EquivalenceLevel(*r.EquivalenceLevel)
			m.EquivalenceLevel = &lvl
		}
		if minRes := config.LookupMinMeaningfulResolution(r.MetricName); minRes != "" {
			res := Resolution(minRes)
			m.MinMeaningfulResolution = &res
		}
		response = append(response, m)
	}

	return response, nil
}

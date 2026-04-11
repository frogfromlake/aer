package handler

import (
	"context"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// Store abstracts the data access layer for testability.
type Store interface {
	Ping(ctx context.Context) error
	GetMetrics(ctx context.Context, start, end time.Time, source, metricName *string) ([]storage.MetricRow, error)
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
func (s *Server) GetMetrics(ctx context.Context, request GetMetricsRequestObject) (GetMetricsResponseObject, error) {
	data, err := s.db.GetMetrics(ctx, request.Params.StartDate, request.Params.EndDate, request.Params.Source, request.Params.MetricName)
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
		response = append(response, AvailableMetric{
			MetricName:       r.MetricName,
			ValidationStatus: ValidationStatus(r.ValidationStatus),
		})
	}

	return response, nil
}

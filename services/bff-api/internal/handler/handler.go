package handler

import (
	"context"
	"time"
)

// MetricsStore abstracts the data access layer for testability.
type MetricsStore interface {
	Ping(ctx context.Context) error
	GetMetrics(ctx context.Context, start, end time.Time) ([]struct {
		TS    time.Time
		Value float64
	}, error)
}

// Server implements the generated StrictServerInterface.
type Server struct {
	db MetricsStore
}

// NewServer creates a new API server instance.
func NewServer(db MetricsStore) *Server {
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
func (s *Server) GetMetrics(ctx context.Context, request GetMetricsRequestObject) (GetMetricsResponseObject, error) {
	// Fallback time range: Last 24 hours if no query parameters are provided
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if request.Params.StartDate != nil {
		start = *request.Params.StartDate
	}
	if request.Params.EndDate != nil {
		end = *request.Params.EndDate
	}

	data, err := s.db.GetMetrics(ctx, start, end)
	if err != nil {
		return GetMetrics500JSONResponse{Message: err.Error()}, nil
	}

	var response GetMetrics200JSONResponse
	for _, d := range data {
		// Append using the generated anonymous struct type
		response = append(response, struct {
			Timestamp time.Time `json:"timestamp"`
			Value     float64   `json:"value"`
		}{
			Timestamp: d.TS,
			Value:     d.Value,
		})
	}

	return response, nil
}

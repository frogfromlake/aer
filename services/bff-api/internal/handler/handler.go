package handler

import (
	"context"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// Server implements the generated StrictServerInterface.
type Server struct {
	db *storage.ClickHouseStorage
}

// NewServer creates a new API server instance.
func NewServer(db *storage.ClickHouseStorage) *Server {
	return &Server{db: db}
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

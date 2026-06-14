package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// GetSourceDiscoveryCoverage handles GET /sources/{sourceId}/discovery-coverage.
//
// Phase 122g (ADR-031) operationalises the publisher's discovery
// surface as a runtime signal. The handler resolves the source name →
// id, queries the per-channel telemetry from
// crawler_discovery_runs, and renders the OpenAPI response. Sibling to
// the Phase 122f metadata-coverage handler — same dossier-resolve gate
// + same fail-open semantics when telemetry rows are absent (a fresh
// install or a never-crawled source yields an empty per-channel array,
// not a 500).
//
// NOTE TO MERGER: this handler is implemented against the OpenAPI
// types `GetSourceDiscoveryCoverage{200,404,500}JSONResponse` and
// `DiscoveryCoverageResponse` that `make codegen` produces from
// `api/openapi.yaml` after Phase 122g lands. Until codegen runs in CI
// these types are unresolved at compile time — this is by design (we
// commit the OpenAPI source-of-truth + the handler skeleton; CI
// regenerates the bindings and the build settles).
func (s *Server) GetSourceDiscoveryCoverage(
	ctx context.Context,
	request GetSourceDiscoveryCoverageRequestObject,
) (GetSourceDiscoveryCoverageResponseObject, error) {
	if s.dossier == nil {
		return GetSourceDiscoveryCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	id, name, err := s.dossier.ResolveSource(ctx, request.SourceID)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetSourceDiscoveryCoverage404JSONResponse{Message: "source not found"}, nil
		}
		slog.Error("handler failure", "op", "GetSourceDiscoveryCoverage.ResolveSource", "error", err)
		return GetSourceDiscoveryCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	windowDays := 30
	if request.Params.WindowDays != nil && *request.Params.WindowDays > 0 {
		windowDays = *request.Params.WindowDays
		if windowDays > 365 {
			windowDays = 365
		}
	}

	summary, err := s.dossier.GetDiscoveryCoverage(ctx, id, windowDays)
	if err != nil {
		slog.Error("handler failure", "op", "GetSourceDiscoveryCoverage", "error", err)
		return GetSourceDiscoveryCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetSourceDiscoveryCoverage200JSONResponse{
		SourceID:                    name,
		WindowDays:                  windowDays,
		TotalUrlsDiscoveredLastRun:  int(summary.TotalDiscoveredLastRun),  //nolint:gosec // bounded
		UniqueUrlsAfterDedupLastRun: int(summary.UniqueAfterDedupLastRun), //nolint:gosec // bounded
		UnderflowAlertActive:        summary.UnderflowAlertActive,
	}
	if summary.ExpectedFloorPerRun.Valid {
		v := int(summary.ExpectedFloorPerRun.Int64)
		resp.ExpectedFloorPerRun = &v
	}
	perChannel := make([]struct {
		AverageUrlsDiscoveredPerRun float32 `json:"averageUrlsDiscoveredPerRun"`
		Channel                     string  `json:"channel"`
		LastRunUrlsAfterDedup       int     `json:"lastRunUrlsAfterDedup"`
		LastRunUrlsDiscovered       int     `json:"lastRunUrlsDiscovered"`
		UnderflowAlertActive        bool    `json:"underflowAlertActive"`
	}, 0, len(summary.PerChannel))
	for _, row := range summary.PerChannel {
		perChannel = append(perChannel, struct {
			AverageUrlsDiscoveredPerRun float32 `json:"averageUrlsDiscoveredPerRun"`
			Channel                     string  `json:"channel"`
			LastRunUrlsAfterDedup       int     `json:"lastRunUrlsAfterDedup"`
			LastRunUrlsDiscovered       int     `json:"lastRunUrlsDiscovered"`
			UnderflowAlertActive        bool    `json:"underflowAlertActive"`
		}{
			AverageUrlsDiscoveredPerRun: float32(row.AverageDiscoveredPerRun),
			Channel:                     row.Channel,
			LastRunUrlsAfterDedup:       int(row.LastRunAfterDedup), //nolint:gosec // bounded
			LastRunUrlsDiscovered:       int(row.LastRunDiscovered), //nolint:gosec // bounded
			// The source-level alert state is the OR across channels;
			// per-channel telemetry today fires the alert at source
			// granularity (the underflow gate compares total dedup-
			// adjusted URL count to the source's floor). Surface the
			// same boolean per-channel so the dashboard panel can
			// render either rendering without a second BFF call.
			UnderflowAlertActive: summary.UnderflowAlertActive,
		})
	}
	resp.PerChannel = perChannel
	return resp, nil
}

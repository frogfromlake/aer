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

	summary, err := s.dossier.GetDiscoveryCoverage(ctx, id, name, windowDays)
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

	// Phase 148d — source-level completeness verdict (WP-007 §4.1).
	resp.CompletenessIndeterminate = &summary.Completeness.Indeterminate
	icc := summary.Completeness.IndeterminateChannelCount
	resp.IndeterminateChannelCount = &icc
	if summary.Completeness.DeclaredTotal.Valid {
		dt := int(summary.Completeness.DeclaredTotal.Int64)
		resp.DeclaredTotalLastRun = &dt
	}
	if summary.Completeness.Completeness.Valid {
		c := float32(summary.Completeness.Completeness.Float64)
		resp.Completeness = &c
	}

	perChannel := make([]struct {
		AverageUrlsDiscoveredPerRun float32 `json:"averageUrlsDiscoveredPerRun"`
		Channel                     string  `json:"channel"`
		Declared                    *int    `json:"declared,omitempty"`
		DeclaredIndeterminate       *bool   `json:"declaredIndeterminate,omitempty"`
		LastRunUrlsAfterDedup       int     `json:"lastRunUrlsAfterDedup"`
		LastRunUrlsDiscovered       int     `json:"lastRunUrlsDiscovered"`
		UnderflowAlertActive        bool    `json:"underflowAlertActive"`
	}, 0, len(summary.PerChannel))
	for _, row := range summary.PerChannel {
		var declared *int
		if row.Declared.Valid {
			d := int(row.Declared.Int64)
			declared = &d
		}
		indet := row.DeclaredIndeterminate
		perChannel = append(perChannel, struct {
			AverageUrlsDiscoveredPerRun float32 `json:"averageUrlsDiscoveredPerRun"`
			Channel                     string  `json:"channel"`
			Declared                    *int    `json:"declared,omitempty"`
			DeclaredIndeterminate       *bool   `json:"declaredIndeterminate,omitempty"`
			LastRunUrlsAfterDedup       int     `json:"lastRunUrlsAfterDedup"`
			LastRunUrlsDiscovered       int     `json:"lastRunUrlsDiscovered"`
			UnderflowAlertActive        bool    `json:"underflowAlertActive"`
		}{
			AverageUrlsDiscoveredPerRun: float32(row.AverageDiscoveredPerRun),
			Channel:                     row.Channel,
			Declared:                    declared,
			DeclaredIndeterminate:       &indet,
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

	// Phase 148d — the per-source funnel (WP-007 §5). Always emit `present`
	// so the dashboard can distinguish "no funnel recorded" from a funnel of
	// all-zeros; fill the stages + reconciled Gold tail only when present.
	f := summary.Funnel
	ip := func(v int64) *int { x := int(v); return &x } //nolint:gosec // bounded
	resp.Funnel = &struct {
		AlreadyCollected      *int     `json:"alreadyCollected,omitempty"`
		ContentDropped        *int     `json:"contentDropped,omitempty"`
		Discovered            *int     `json:"discovered,omitempty"`
		Errored               *int     `json:"errored,omitempty"`
		ExtractionSuccessRate *float32 `json:"extractionSuccessRate,omitempty"`
		Fetched               *int     `json:"fetched,omitempty"`
		GoldRows              *int     `json:"goldRows,omitempty"`
		NonArticleRate        *float32 `json:"nonArticleRate,omitempty"`
		NotModified           *int     `json:"notModified,omitempty"`
		Present               bool     `json:"present"`
		Submitted             *int     `json:"submitted,omitempty"`
		ThinContentDropped    *int     `json:"thinContentDropped,omitempty"`
		URLFiltered           *int     `json:"urlFiltered,omitempty"`
	}{Present: f.Present}
	if f.Present {
		resp.Funnel.Discovered = ip(f.Discovered)
		resp.Funnel.URLFiltered = ip(f.URLFiltered)
		resp.Funnel.AlreadyCollected = ip(f.AlreadyCollected)
		resp.Funnel.Fetched = ip(f.Fetched)
		resp.Funnel.NotModified = ip(f.NotModified)
		resp.Funnel.ContentDropped = ip(f.ContentDropped)
		resp.Funnel.ThinContentDropped = ip(f.ThinContentDropped)
		resp.Funnel.Submitted = ip(f.Submitted)
		resp.Funnel.Errored = ip(f.Errored)
		resp.Funnel.GoldRows = ip(f.GoldRows)
		if f.ExtractionSuccessRate.Valid {
			v := float32(f.ExtractionSuccessRate.Float64)
			resp.Funnel.ExtractionSuccessRate = &v
		}
		if f.NonArticleRate.Valid {
			v := float32(f.NonArticleRate.Float64)
			resp.Funnel.NonArticleRate = &v
		}
	}
	return resp, nil
}

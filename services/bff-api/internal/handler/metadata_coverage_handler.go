package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// GetProbeMetadataCoverage handles GET /probes/{probeId}/metadata-coverage.
//
// Phase 122f operationalises WP-003 §3.2 metadata-richness asymmetry as a
// runtime signal. The handler resolves the probe → source list, asks
// ClickHouse for the per-source-per-field-per-method coverage cells, and
// assembles them into the response shape declared in
// `api/schemas/MetadataCoverageResponse.yaml`. All sources bound to the
// probe are returned even when one has no observed cells (empty `fields`
// slice) — same shape, semantically "no observations yet".
func (s *Server) GetProbeMetadataCoverage(ctx context.Context, request GetProbeMetadataCoverageRequestObject) (GetProbeMetadataCoverageResponseObject, error) {
	probe, ok := s.probes[request.ProbeID]
	if !ok {
		return GetProbeMetadataCoverage404JSONResponse{Message: "probe not found"}, nil
	}

	cells, err := s.db.GetMetadataCoverage(ctx, probe.Sources)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeMetadataCoverage", "error", err)
		return GetProbeMetadataCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetProbeMetadataCoverage200JSONResponse{Scope: probe.ProbeID}
	resp.Sources = renderCoverageSources(probe.Sources, cells)
	return resp, nil
}

// GetSourceMetadataCoverage handles GET /sources/{sourceId}/metadata-coverage.
// Single-source view of the same matrix — backs per-source dossier
// surfaces and any caller that already has a `sourceId` in hand.
func (s *Server) GetSourceMetadataCoverage(ctx context.Context, request GetSourceMetadataCoverageRequestObject) (GetSourceMetadataCoverageResponseObject, error) {
	if s.dossier == nil {
		return GetSourceMetadataCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	_, name, err := s.dossier.ResolveSource(ctx, request.SourceID)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetSourceMetadataCoverage404JSONResponse{Message: "source not found"}, nil
		}
		slog.Error("handler failure", "op", "GetSourceMetadataCoverage.ResolveSource", "error", err)
		return GetSourceMetadataCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	cells, err := s.db.GetMetadataCoverage(ctx, []string{name})
	if err != nil {
		slog.Error("handler failure", "op", "GetSourceMetadataCoverage", "error", err)
		return GetSourceMetadataCoverage500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetSourceMetadataCoverage200JSONResponse{Scope: name}
	resp.Sources = renderCoverageSources([]string{name}, cells)
	return resp, nil
}

// renderCoverageSources adapts the storage-layer aggregate to the
// generated response shape. A scope source with no cells still appears
// in the response with an empty `fields` array — the dashboard wants the
// scope membership to be unambiguous regardless of observation status.
func renderCoverageSources(scope []string, cells []storage.MetadataCoverageCell) []struct {
	Fields []struct {
		ByMethod           map[string]int `json:"byMethod"`
		Field              string         `json:"field"`
		PopulationRate     float64        `json:"populationRate"`
		StructurallyAbsent bool           `json:"structurallyAbsent"`
		TotalArticles      int            `json:"totalArticles"`
	} `json:"fields"`
	Name string `json:"name"`
} {
	summaries := storage.AssembleCoverage(cells)
	byName := make(map[string]storage.CoverageSourceSummary, len(summaries))
	for _, s := range summaries {
		byName[s.Name] = s
	}
	out := make([]struct {
		Fields []struct {
			ByMethod           map[string]int `json:"byMethod"`
			Field              string         `json:"field"`
			PopulationRate     float64        `json:"populationRate"`
			StructurallyAbsent bool           `json:"structurallyAbsent"`
			TotalArticles      int            `json:"totalArticles"`
		} `json:"fields"`
		Name string `json:"name"`
	}, 0, len(scope))
	for _, name := range scope {
		summary := byName[name]
		fields := make([]struct {
			ByMethod           map[string]int `json:"byMethod"`
			Field              string         `json:"field"`
			PopulationRate     float64        `json:"populationRate"`
			StructurallyAbsent bool           `json:"structurallyAbsent"`
			TotalArticles      int            `json:"totalArticles"`
		}, 0, len(summary.Fields))
		for _, f := range summary.Fields {
			bm := make(map[string]int, len(f.ByMethod))
			for k, v := range f.ByMethod {
				bm[k] = int(v) //nolint:gosec // bounded by 30-day raw-table horizon
			}
			fields = append(fields, struct {
				ByMethod           map[string]int `json:"byMethod"`
				Field              string         `json:"field"`
				PopulationRate     float64        `json:"populationRate"`
				StructurallyAbsent bool           `json:"structurallyAbsent"`
				TotalArticles      int            `json:"totalArticles"`
			}{
				ByMethod:           bm,
				Field:              f.Field,
				PopulationRate:     f.PopulationRate,
				StructurallyAbsent: f.StructurallyAbsent,
				TotalArticles:      int(f.TotalArticles), //nolint:gosec // bounded
			})
		}
		out = append(out, struct {
			Fields []struct {
				ByMethod           map[string]int `json:"byMethod"`
				Field              string         `json:"field"`
				PopulationRate     float64        `json:"populationRate"`
				StructurallyAbsent bool           `json:"structurallyAbsent"`
				TotalArticles      int            `json:"totalArticles"`
			} `json:"fields"`
			Name string `json:"name"`
		}{Fields: fields, Name: name})
	}
	return out
}

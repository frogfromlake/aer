package handler

import (
	"context"
	"log/slog"
	"strings"
)

// GetMetadataDistribution returns the top-N values of a categorical metadata
// field over the resolved scope, ranked by distinct in-scope article count,
// with the long tail disclosed (Phase 133). Mirrors GetMetricDistribution's
// scope/window resolution; reads aer_gold.article_metadata.
func (s *Server) GetMetadataDistribution(ctx context.Context, request GetMetadataDistributionRequestObject) (GetMetadataDistributionResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetadataDistribution404JSONResponse{Message: reason}, nil
		}
		return GetMetadataDistribution400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetadataDistribution400JSONResponse{Message: msg}, nil
	}
	if request.Field == "" {
		return GetMetadataDistribution400JSONResponse{Message: "field is required"}, nil
	}

	topN := 20
	if request.Params.TopN != nil {
		topN = *request.Params.TopN
	}

	res, err := s.db.GetCategoricalDistribution(ctx, request.Field, sources, start, end, topN)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetadataDistribution", "error", err)
		return GetMetadataDistribution500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetadataDistribution200JSONResponse{
		Field:          request.Field,
		Scope:          strPtr(string(kind)),
		ScopeId:        request.Params.ScopeId,
		WindowStart:    request.Params.Start,
		WindowEnd:      request.Params.End,
		TotalArticles:  int(res.TotalArticles),
		DistinctValues: int(res.DistinctValues),
		OtherArticles:  int(res.OtherArticles),
	}
	resp.Categories = make([]struct {
		Articles int    `json:"articles"`
		Value    string `json:"value"`
	}, len(res.Categories))
	for i, c := range res.Categories {
		resp.Categories[i] = struct {
			Articles int    `json:"articles"`
			Value    string `json:"value"`
		}{Articles: int(c.Articles), Value: c.Value}
	}
	return resp, nil
}

// GetScopeAvailableMetadata returns which categorical metadata fields are
// present for every scoped source (available) versus only some (partial) —
// the categorical analog of GetScopeAvailableMetrics (Phase 133).
func (s *Server) GetScopeAvailableMetadata(ctx context.Context, request GetScopeAvailableMetadataRequestObject) (GetScopeAvailableMetadataResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	_, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetScopeAvailableMetadata404JSONResponse{Message: reason}, nil
		}
		return GetScopeAvailableMetadata400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetScopeAvailableMetadata400JSONResponse{Message: msg}, nil
	}

	avail, err := s.db.GetScopeAvailableMetadata(ctx, start, end, sources)
	if err != nil {
		slog.Error("handler failure", "op", "GetScopeAvailableMetadata", "error", err)
		return GetScopeAvailableMetadata500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetScopeAvailableMetadata200JSONResponse{
		ScopedSources: avail.ScopedSources,
		Available:     avail.Available,
		WindowStart:   request.Params.Start,
		WindowEnd:     request.Params.End,
	}
	resp.Partial = make([]struct {
		Field   string   `json:"field"`
		Sources []string `json:"sources"`
	}, len(avail.Partial))
	for i, p := range avail.Partial {
		resp.Partial[i] = struct {
			Field   string   `json:"field"`
			Sources []string `json:"sources"`
		}{Field: p.Field, Sources: p.Sources}
	}
	return resp, nil
}

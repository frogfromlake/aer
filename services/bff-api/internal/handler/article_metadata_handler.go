package handler

import (
	"context"
	"log/slog"
	"math"
	"strings"
)

// safeFloat replaces NaN/±Inf with 0 so the JSON encoder (which rejects them)
// never fails. stddevSamp over a single article returns NaN — a legitimate
// "no spread" case rendered as 0.
func safeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// GetMetadataDistribution returns the top-N values of a categorical metadata
// field over the resolved scope, ranked by distinct in-scope article count,
// with the long tail disclosed (Phase 133). Mirrors GetMetricDistribution's
// scope/window resolution; reads aer_gold.article_metadata.
func (s *Server) GetMetadataDistribution(ctx context.Context, request GetMetadataDistributionRequestObject) (GetMetadataDistributionResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
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

	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)
	res, err := s.db.GetCategoricalDistribution(ctx, request.Field, sources, start, end, topN, mf)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetadataDistribution", "error", err)
		return GetMetadataDistribution500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetadataDistribution200JSONResponse{
		Field:          request.Field,
		Scope:          strPtr(string(kind)),
		ScopeID:        request.Params.ScopeID,
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

// GetMetadataCrossTab returns, for one categorical metadata field × one numeric
// metric, the per-category article count + mean/spread of the metric (Phase 125).
// Cross-frame (multi-language) requests gate on the metric's equivalence grant.
func (s *Server) GetMetadataCrossTab(ctx context.Context, request GetMetadataCrossTabRequestObject) (GetMetadataCrossTabResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetadataCrossTab404JSONResponse{Message: reason}, nil
		}
		return GetMetadataCrossTab400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetadataCrossTab400JSONResponse{Message: msg}, nil
	}
	if request.Field == "" {
		return GetMetadataCrossTab400JSONResponse{Message: "field is required"}, nil
	}
	if request.Metric == "" {
		return GetMetadataCrossTab400JSONResponse{Message: "metric is required"}, nil
	}
	metric := canonicalMetricNames([]string{request.Metric})[0]

	// Cross-frame gate — correlating a metric across languages is only
	// meaningful when the metric's cross-cultural equivalence is granted.
	if refusal, err := s.crossFrameGate(ctx, []string{metric}, sources, start, end); err != nil {
		slog.Error("handler failure", "op", "GetMetadataCrossTab.crossFrameGate", "error", err)
		return GetMetadataCrossTab500JSONResponse{Message: genericInternalError}, nil
	} else if refusal != nil {
		return GetMetadataCrossTab400JSONResponse{
			Message:            refusal.Message,
			Gate:               refusal.Gate,
			WorkingPaperAnchor: refusal.WorkingPaperAnchor,
			Alternatives:       refusal.Alternatives,
		}, nil
	}

	topN := 20
	if request.Params.TopN != nil {
		topN = *request.Params.TopN
	}

	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)
	res, err := s.db.GetCrossTab(ctx, request.Field, metric, sources, start, end, topN, mf)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetadataCrossTab", "error", err)
		return GetMetadataCrossTab500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetadataCrossTab200JSONResponse{
		Field:          request.Field,
		Metric:         metric,
		Scope:          strPtr(string(kind)),
		ScopeID:        request.Params.ScopeID,
		WindowStart:    request.Params.Start,
		WindowEnd:      request.Params.End,
		DistinctValues: res.DistinctValues,
	}
	resp.Categories = make([]struct {
		Articles int64   `json:"articles"`
		Mean     float64 `json:"mean"`
		Std      float64 `json:"std"`
		Value    string  `json:"value"`
	}, len(res.Buckets))
	for i, b := range res.Buckets {
		resp.Categories[i] = struct {
			Articles int64   `json:"articles"`
			Mean     float64 `json:"mean"`
			Std      float64 `json:"std"`
			Value    string  `json:"value"`
		}{Articles: int64(b.Articles), Mean: safeFloat(b.Mean), Std: safeFloat(b.Std), Value: b.Value} //nolint:gosec // bounded by 365-day TTL
	}
	return resp, nil
}

// GetMetadataSankey returns the alluvial flow across an ordered chain of
// categorical metadata fields (Phase 125). Article counts only — no metric
// normalization, so no equivalence gate. Field chain capped at 8 for legibility.
func (s *Server) GetMetadataSankey(ctx context.Context, request GetMetadataSankeyRequestObject) (GetMetadataSankeyResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetadataSankey404JSONResponse{Message: reason}, nil
		}
		return GetMetadataSankey400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetadataSankey400JSONResponse{Message: msg}, nil
	}

	fields := splitAndTrim(request.Params.Fields)
	if len(fields) < 2 {
		return GetMetadataSankey400JSONResponse{Message: "fields must list at least 2 names"}, nil
	}
	if len(fields) > 8 {
		fields = fields[:8]
	}

	topN := 50
	if request.Params.TopN != nil {
		topN = *request.Params.TopN
	}

	res, err := s.db.GetSankey(ctx, fields, sources, start, end, topN)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetadataSankey", "error", err)
		return GetMetadataSankey500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetadataSankey200JSONResponse{
		Fields:      res.Fields,
		Scope:       strPtr(string(kind)),
		ScopeID:     request.Params.ScopeID,
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}
	resp.Nodes = make([]struct {
		Field string `json:"field"`
		ID    string `json:"id"`
		Layer int    `json:"layer"`
		Value string `json:"value"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		resp.Nodes[i] = struct {
			Field string `json:"field"`
			ID    string `json:"id"`
			Layer int    `json:"layer"`
			Value string `json:"value"`
		}{Field: n.Field, ID: n.ID, Layer: n.Layer, Value: n.Value}
	}
	resp.Links = make([]struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Value  int64  `json:"value"`
	}, len(res.Links))
	for i, l := range res.Links {
		resp.Links[i] = struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Value  int64  `json:"value"`
		}{Source: l.Source, Target: l.Target, Value: l.Value}
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
	_, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
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

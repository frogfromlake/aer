package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// scopeKind enumerates the resolved scope of a view-mode query.
type scopeKind string

const (
	scopeProbe  scopeKind = "probe"
	scopeSource scopeKind = "source"
)

// resolveScope converts the raw query parameters into (kind, sources). It is
// the single point where probe-id lookups and source-name validation happen
// for the four Phase 102 endpoints. A return of (false, _, _) means the
// caller should respond with 400/404 — the bool reports whether resolution
// succeeded; the error message in `reason` is suitable for the response body
// (it does not leak storage details).
func (s *Server) resolveScope(rawScope, rawScopeId string) (kind scopeKind, sources []string, reason string, ok bool) {
	id := strings.TrimSpace(rawScopeId)
	if id == "" {
		return "", nil, "scopeId is required", false
	}
	var resolved scopeKind
	switch strings.ToLower(strings.TrimSpace(rawScope)) {
	case "", string(scopeProbe):
		resolved = scopeProbe
	case string(scopeSource):
		resolved = scopeSource
	default:
		return "", nil, "scope must be probe or source", false
	}

	if resolved == scopeProbe {
		probe, ok := s.probes[id]
		if !ok {
			return "", nil, fmt.Sprintf("unknown probe %q", id), false
		}
		out := make([]string, len(probe.Sources))
		copy(out, probe.Sources)
		return scopeProbe, out, "", true
	}

	// scope=source: trust the source name (the metrics table is the
	// authoritative source-name registry; an unknown source simply yields
	// an empty result, which is correct).
	return scopeSource, []string{id}, "", true
}

// validateWindow rejects malformed time windows before reaching ClickHouse.
func validateWindow(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "start and end are required"
	}
	if !end.After(start) {
		return "end must be strictly after start"
	}
	return ""
}

// strPtr is a tiny helper for the optional Scope/ScopeId echo fields.
func strPtr(s string) *string { return &s }

// GetMetricDistribution returns the per-scope value distribution for a metric
// over a window. Backs the EDA x ridgeline / violin / density view-mode cells.
func (s *Server) GetMetricDistribution(ctx context.Context, request GetMetricDistributionRequestObject) (GetMetricDistributionResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, reason, ok := s.resolveScope(rawScope, request.Params.ScopeId)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricDistribution404JSONResponse{Message: reason}, nil
		}
		return GetMetricDistribution400JSONResponse{Message: reason}, nil
	}
	if msg := validateWindow(request.Params.Start, request.Params.End); msg != "" {
		return GetMetricDistribution400JSONResponse{Message: msg}, nil
	}
	if request.MetricName == "" {
		return GetMetricDistribution400JSONResponse{Message: "metricName is required"}, nil
	}

	bins := 30
	if request.Params.Bins != nil {
		bins = *request.Params.Bins
	}

	res, err := s.db.GetMetricDistribution(ctx, request.MetricName, sources, request.Params.Start, request.Params.End, bins)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricDistribution", "error", err)
		return GetMetricDistribution500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricDistribution200JSONResponse{
		MetricName:  request.MetricName,
		Scope:       strPtr(string(kind)),
		ScopeId:     strPtr(request.Params.ScopeId),
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}
	resp.Bins = make([]struct {
		Count int64   `json:"count"`
		Lower float64 `json:"lower"`
		Upper float64 `json:"upper"`
	}, len(res.Bins))
	for i, b := range res.Bins {
		resp.Bins[i] = struct {
			Count int64   `json:"count"`
			Lower float64 `json:"lower"`
			Upper float64 `json:"upper"`
		}{Count: b.Count, Lower: b.Lower, Upper: b.Upper}
	}
	resp.Summary.Count = res.Summary.Count
	resp.Summary.Min = res.Summary.Min
	resp.Summary.Max = res.Summary.Max
	resp.Summary.Mean = res.Summary.Mean
	resp.Summary.Median = res.Summary.Median
	resp.Summary.P05 = res.Summary.P05
	resp.Summary.P25 = res.Summary.P25
	resp.Summary.P75 = res.Summary.P75
	resp.Summary.P95 = res.Summary.P95
	return resp, nil
}

// GetMetricHeatmap returns a 2D aggregation of a metric for the requested
// xDimension / yDimension within a window. Backs the EDA x heatmap view-mode
// cells; entityLabel and language dimensions trigger Gold-table joins.
func (s *Server) GetMetricHeatmap(ctx context.Context, request GetMetricHeatmapRequestObject) (GetMetricHeatmapResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, reason, ok := s.resolveScope(rawScope, request.Params.ScopeId)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricHeatmap404JSONResponse{Message: reason}, nil
		}
		return GetMetricHeatmap400JSONResponse{Message: reason}, nil
	}
	if msg := validateWindow(request.Params.Start, request.Params.End); msg != "" {
		return GetMetricHeatmap400JSONResponse{Message: msg}, nil
	}
	if !request.Params.XDimension.Valid() || !request.Params.YDimension.Valid() {
		return GetMetricHeatmap400JSONResponse{Message: "xDimension and yDimension must be one of dayOfWeek, hour, source, entityLabel, language"}, nil
	}

	cells, err := s.db.GetMetricHeatmap(
		ctx,
		request.MetricName,
		sources,
		storage.HeatmapDimension(request.Params.XDimension),
		storage.HeatmapDimension(request.Params.YDimension),
		request.Params.Start,
		request.Params.End,
	)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricHeatmap", "error", err)
		return GetMetricHeatmap500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricHeatmap200JSONResponse{
		MetricName:  request.MetricName,
		Scope:       strPtr(string(kind)),
		ScopeId:     strPtr(request.Params.ScopeId),
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
		XDimension:  string(request.Params.XDimension),
		YDimension:  string(request.Params.YDimension),
	}
	resp.Cells = make([]struct {
		Count int64   `json:"count"`
		Value float64 `json:"value"`
		X     string  `json:"x"`
		Y     string  `json:"y"`
	}, len(cells))
	for i, c := range cells {
		resp.Cells[i] = struct {
			Count int64   `json:"count"`
			Value float64 `json:"value"`
			X     string  `json:"x"`
			Y     string  `json:"y"`
		}{Count: c.Count, Value: c.Value, X: c.X, Y: c.Y}
	}
	return resp, nil
}

// GetMetricCorrelation returns a pairwise Pearson correlation matrix over
// the requested metric set, restricted to the resolved scope and window.
func (s *Server) GetMetricCorrelation(ctx context.Context, request GetMetricCorrelationRequestObject) (GetMetricCorrelationResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, reason, ok := s.resolveScope(rawScope, request.Params.ScopeId)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricCorrelation404JSONResponse{Message: reason}, nil
		}
		return GetMetricCorrelation400JSONResponse{Message: reason}, nil
	}
	if msg := validateWindow(request.Params.Start, request.Params.End); msg != "" {
		return GetMetricCorrelation400JSONResponse{Message: msg}, nil
	}

	metrics := splitAndTrim(request.Params.Metrics)
	if len(metrics) < 2 {
		return GetMetricCorrelation400JSONResponse{Message: "metrics must list at least 2 names"}, nil
	}
	if len(metrics) > 10 {
		return GetMetricCorrelation400JSONResponse{Message: "metrics is capped at 10 names per request"}, nil
	}

	res, err := s.db.GetMetricCorrelation(ctx, metrics, sources, request.Params.Start, request.Params.End)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricCorrelation", "error", err)
		return GetMetricCorrelation500JSONResponse{Message: genericInternalError}, nil
	}

	return GetMetricCorrelation200JSONResponse{
		Metrics:     res.Metrics,
		Matrix:      res.Matrix,
		BucketCount: res.BucketCount,
		Resolution:  res.Resolution,
		Scope:       strPtr(string(kind)),
		ScopeId:     strPtr(request.Params.ScopeId),
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}, nil
}

// GetEntityCoOccurrence returns the top-N entity-pair edges aggregated over
// the window, plus the union of incident nodes with degree and total weight.
func (s *Server) GetEntityCoOccurrence(ctx context.Context, request GetEntityCoOccurrenceRequestObject) (GetEntityCoOccurrenceResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, reason, ok := s.resolveScope(rawScope, request.Params.ScopeId)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetEntityCoOccurrence404JSONResponse{Message: reason}, nil
		}
		return GetEntityCoOccurrence400JSONResponse{Message: reason}, nil
	}
	if msg := validateWindow(request.Params.Start, request.Params.End); msg != "" {
		return GetEntityCoOccurrence400JSONResponse{Message: msg}, nil
	}

	topN := 50
	if request.Params.TopN != nil {
		topN = *request.Params.TopN
	}

	res, err := s.db.GetEntityCoOccurrence(ctx, sources, request.Params.Start, request.Params.End, topN)
	if err != nil {
		slog.Error("handler failure", "op", "GetEntityCoOccurrence", "error", err)
		return GetEntityCoOccurrence500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetEntityCoOccurrence200JSONResponse{
		TopN:        res.TopN,
		Scope:       strPtr(string(kind)),
		ScopeId:     strPtr(request.Params.ScopeId),
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}
	resp.Edges = make([]struct {
		A            string  `json:"a"`
		ALabel       *string `json:"aLabel,omitempty"`
		ArticleCount int64   `json:"articleCount"`
		B            string  `json:"b"`
		BLabel       *string `json:"bLabel,omitempty"`
		Weight       int64   `json:"weight"`
	}, len(res.Edges))
	for i, e := range res.Edges {
		var aLabel, bLabel *string
		if e.ALabel != "" {
			a := e.ALabel
			aLabel = &a
		}
		if e.BLabel != "" {
			b := e.BLabel
			bLabel = &b
		}
		resp.Edges[i] = struct {
			A            string  `json:"a"`
			ALabel       *string `json:"aLabel,omitempty"`
			ArticleCount int64   `json:"articleCount"`
			B            string  `json:"b"`
			BLabel       *string `json:"bLabel,omitempty"`
			Weight       int64   `json:"weight"`
		}{A: e.A, ALabel: aLabel, ArticleCount: e.ArticleCount, B: e.B, BLabel: bLabel, Weight: e.Weight}
	}
	resp.Nodes = make([]struct {
		Degree     int64  `json:"degree"`
		Label      string `json:"label"`
		Text       string `json:"text"`
		TotalCount int64  `json:"totalCount"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		resp.Nodes[i] = struct {
			Degree     int64  `json:"degree"`
			Label      string `json:"label"`
			Text       string `json:"text"`
			TotalCount int64  `json:"totalCount"`
		}{Degree: n.Degree, Label: n.Label, Text: n.Text, TotalCount: n.TotalCount}
	}
	return resp, nil
}

// splitAndTrim splits a comma-separated list and drops empty tokens.
func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

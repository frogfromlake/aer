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

// probeSegment is one probe's resolved source list used for per-probe streams.
type probeSegment struct {
	id      string
	sources []string
}

// resolveScopeMulti resolves the composite scope from the legacy scopeId plus
// the Phase 114 probeIds and sourceIds parameters. The union of all resolved
// source names is returned together with per-probe segment data for
// segmentBy=probe streams. At least one non-empty input is required; the
// function returns ok=false with a human-readable reason otherwise.
func (s *Server) resolveScopeMulti(
	rawScope string, scopeId, probeIds, sourceIds *string,
) (kind scopeKind, sources []string, probeSegs []probeSegment, reason string, ok bool) {
	var resolvedKind scopeKind
	switch strings.ToLower(strings.TrimSpace(rawScope)) {
	case "", string(scopeProbe):
		resolvedKind = scopeProbe
	case string(scopeSource):
		resolvedKind = scopeSource
	default:
		return "", nil, nil, "scope must be probe or source", false
	}

	seen := map[string]bool{}
	addSrc := func(src string) {
		if src = strings.TrimSpace(src); src != "" && !seen[src] {
			seen[src] = true
			sources = append(sources, src)
		}
	}

	hasProbes := false

	// 1. Legacy scopeId (single probe id or source name).
	if scopeId != nil && strings.TrimSpace(*scopeId) != "" {
		id := strings.TrimSpace(*scopeId)
		if resolvedKind == scopeProbe {
			probe, exists := s.probes[id]
			if !exists {
				return "", nil, nil, fmt.Sprintf("unknown probe %q", id), false
			}
			for _, src := range probe.Sources {
				addSrc(src)
			}
			probeSegs = append(probeSegs, probeSegment{id: id, sources: probe.Sources})
			hasProbes = true
		} else {
			addSrc(id)
		}
	}

	// 2. Comma-separated probeIds (Phase 114).
	if probeIds != nil {
		for _, pid := range splitAndTrim(*probeIds) {
			probe, exists := s.probes[pid]
			if !exists {
				return "", nil, nil, fmt.Sprintf("unknown probe %q", pid), false
			}
			for _, src := range probe.Sources {
				addSrc(src)
			}
			probeSegs = append(probeSegs, probeSegment{id: pid, sources: probe.Sources})
			hasProbes = true
		}
	}

	// 3. Explicit sourceIds — added regardless of scope kind (Phase 114).
	if sourceIds != nil {
		for _, src := range splitAndTrim(*sourceIds) {
			addSrc(src)
		}
	}

	if len(sources) == 0 {
		return "", nil, nil, "at least one of scopeId, probeIds, or sourceIds is required", false
	}

	if hasProbes {
		resolvedKind = scopeProbe
	} else {
		resolvedKind = scopeSource
	}
	return resolvedKind, sources, probeSegs, "", true
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
	kind, sources, probeSegs, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
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

	// segmentBy: build per-segment streams when requested.
	if request.Params.SegmentBy != nil {
		switch *request.Params.SegmentBy {
		case GetMetricDistributionParamsSegmentBySource:
			streams := make([]struct {
				Bins []struct {
					Count int64   `json:"count"`
					Lower float64 `json:"lower"`
					Upper float64 `json:"upper"`
				} `json:"bins"`
				Id        string `json:"id"`
				Label     string `json:"label"`
				ScopeKind string `json:"scopeKind"`
				Summary   struct {
					Count  int64   `json:"count"`
					Max    float64 `json:"max"`
					Mean   float64 `json:"mean"`
					Median float64 `json:"median"`
					Min    float64 `json:"min"`
					P05    float64 `json:"p05"`
					P25    float64 `json:"p25"`
					P75    float64 `json:"p75"`
					P95    float64 `json:"p95"`
				} `json:"summary"`
			}, 0, len(sources))
			for _, src := range sources {
				sr, serr := s.db.GetMetricDistribution(ctx, request.MetricName, []string{src}, request.Params.Start, request.Params.End, bins)
				if serr != nil {
					slog.Error("handler failure", "op", "GetMetricDistribution.stream", "source", src, "error", serr)
					return GetMetricDistribution500JSONResponse{Message: genericInternalError}, nil
				}
				elem := struct {
					Bins []struct {
						Count int64   `json:"count"`
						Lower float64 `json:"lower"`
						Upper float64 `json:"upper"`
					} `json:"bins"`
					Id        string `json:"id"`
					Label     string `json:"label"`
					ScopeKind string `json:"scopeKind"`
					Summary   struct {
						Count  int64   `json:"count"`
						Max    float64 `json:"max"`
						Mean   float64 `json:"mean"`
						Median float64 `json:"median"`
						Min    float64 `json:"min"`
						P05    float64 `json:"p05"`
						P25    float64 `json:"p25"`
						P75    float64 `json:"p75"`
						P95    float64 `json:"p95"`
					} `json:"summary"`
				}{Id: src, Label: src, ScopeKind: "source"}
				elem.Bins = make([]struct {
					Count int64   `json:"count"`
					Lower float64 `json:"lower"`
					Upper float64 `json:"upper"`
				}, len(sr.Bins))
				for i, b := range sr.Bins {
					elem.Bins[i] = struct {
						Count int64   `json:"count"`
						Lower float64 `json:"lower"`
						Upper float64 `json:"upper"`
					}{Count: b.Count, Lower: b.Lower, Upper: b.Upper}
				}
				elem.Summary.Count = sr.Summary.Count
				elem.Summary.Min = sr.Summary.Min
				elem.Summary.Max = sr.Summary.Max
				elem.Summary.Mean = sr.Summary.Mean
				elem.Summary.Median = sr.Summary.Median
				elem.Summary.P05 = sr.Summary.P05
				elem.Summary.P25 = sr.Summary.P25
				elem.Summary.P75 = sr.Summary.P75
				elem.Summary.P95 = sr.Summary.P95
				streams = append(streams, elem)
			}
			resp.Streams = &streams
		case GetMetricDistributionParamsSegmentByProbe:
			if len(probeSegs) == 0 {
				return GetMetricDistribution400JSONResponse{Message: "segmentBy=probe requires at least one probe ID in probeIds"}, nil
			}
			streams := make([]struct {
				Bins []struct {
					Count int64   `json:"count"`
					Lower float64 `json:"lower"`
					Upper float64 `json:"upper"`
				} `json:"bins"`
				Id        string `json:"id"`
				Label     string `json:"label"`
				ScopeKind string `json:"scopeKind"`
				Summary   struct {
					Count  int64   `json:"count"`
					Max    float64 `json:"max"`
					Mean   float64 `json:"mean"`
					Median float64 `json:"median"`
					Min    float64 `json:"min"`
					P05    float64 `json:"p05"`
					P25    float64 `json:"p25"`
					P75    float64 `json:"p75"`
					P95    float64 `json:"p95"`
				} `json:"summary"`
			}, 0, len(probeSegs))
			for _, seg := range probeSegs {
				sr, serr := s.db.GetMetricDistribution(ctx, request.MetricName, seg.sources, request.Params.Start, request.Params.End, bins)
				if serr != nil {
					slog.Error("handler failure", "op", "GetMetricDistribution.stream", "probe", seg.id, "error", serr)
					return GetMetricDistribution500JSONResponse{Message: genericInternalError}, nil
				}
				elem := struct {
					Bins []struct {
						Count int64   `json:"count"`
						Lower float64 `json:"lower"`
						Upper float64 `json:"upper"`
					} `json:"bins"`
					Id        string `json:"id"`
					Label     string `json:"label"`
					ScopeKind string `json:"scopeKind"`
					Summary   struct {
						Count  int64   `json:"count"`
						Max    float64 `json:"max"`
						Mean   float64 `json:"mean"`
						Median float64 `json:"median"`
						Min    float64 `json:"min"`
						P05    float64 `json:"p05"`
						P25    float64 `json:"p25"`
						P75    float64 `json:"p75"`
						P95    float64 `json:"p95"`
					} `json:"summary"`
				}{Id: seg.id, Label: seg.id, ScopeKind: "probe"}
				elem.Bins = make([]struct {
					Count int64   `json:"count"`
					Lower float64 `json:"lower"`
					Upper float64 `json:"upper"`
				}, len(sr.Bins))
				for i, b := range sr.Bins {
					elem.Bins[i] = struct {
						Count int64   `json:"count"`
						Lower float64 `json:"lower"`
						Upper float64 `json:"upper"`
					}{Count: b.Count, Lower: b.Lower, Upper: b.Upper}
				}
				elem.Summary.Count = sr.Summary.Count
				elem.Summary.Min = sr.Summary.Min
				elem.Summary.Max = sr.Summary.Max
				elem.Summary.Mean = sr.Summary.Mean
				elem.Summary.Median = sr.Summary.Median
				elem.Summary.P05 = sr.Summary.P05
				elem.Summary.P25 = sr.Summary.P25
				elem.Summary.P75 = sr.Summary.P75
				elem.Summary.P95 = sr.Summary.P95
				streams = append(streams, elem)
			}
			resp.Streams = &streams
		}
	}

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
	kind, sources, probeSegs, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
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

	// segmentBy: build per-segment streams when requested.
	if request.Params.SegmentBy != nil {
		switch *request.Params.SegmentBy {
		case GetMetricHeatmapParamsSegmentBySource:
			streams := make([]struct {
				Cells []struct {
					Count int64   `json:"count"`
					Value float64 `json:"value"`
					X     string  `json:"x"`
					Y     string  `json:"y"`
				} `json:"cells"`
				Id        string `json:"id"`
				Label     string `json:"label"`
				ScopeKind string `json:"scopeKind"`
			}, 0, len(sources))
			for _, src := range sources {
				sc, serr := s.db.GetMetricHeatmap(ctx, request.MetricName, []string{src},
					storage.HeatmapDimension(request.Params.XDimension),
					storage.HeatmapDimension(request.Params.YDimension),
					request.Params.Start, request.Params.End)
				if serr != nil {
					slog.Error("handler failure", "op", "GetMetricHeatmap.stream", "source", src, "error", serr)
					return GetMetricHeatmap500JSONResponse{Message: genericInternalError}, nil
				}
				elem := struct {
					Cells []struct {
						Count int64   `json:"count"`
						Value float64 `json:"value"`
						X     string  `json:"x"`
						Y     string  `json:"y"`
					} `json:"cells"`
					Id        string `json:"id"`
					Label     string `json:"label"`
					ScopeKind string `json:"scopeKind"`
				}{Id: src, Label: src, ScopeKind: "source"}
				elem.Cells = make([]struct {
					Count int64   `json:"count"`
					Value float64 `json:"value"`
					X     string  `json:"x"`
					Y     string  `json:"y"`
				}, len(sc))
				for i, c := range sc {
					elem.Cells[i] = struct {
						Count int64   `json:"count"`
						Value float64 `json:"value"`
						X     string  `json:"x"`
						Y     string  `json:"y"`
					}{Count: c.Count, Value: c.Value, X: c.X, Y: c.Y}
				}
				streams = append(streams, elem)
			}
			resp.Streams = &streams
		case GetMetricHeatmapParamsSegmentByProbe:
			if len(probeSegs) == 0 {
				return GetMetricHeatmap400JSONResponse{Message: "segmentBy=probe requires at least one probe ID in probeIds"}, nil
			}
			streams := make([]struct {
				Cells []struct {
					Count int64   `json:"count"`
					Value float64 `json:"value"`
					X     string  `json:"x"`
					Y     string  `json:"y"`
				} `json:"cells"`
				Id        string `json:"id"`
				Label     string `json:"label"`
				ScopeKind string `json:"scopeKind"`
			}, 0, len(probeSegs))
			for _, seg := range probeSegs {
				sc, serr := s.db.GetMetricHeatmap(ctx, request.MetricName, seg.sources,
					storage.HeatmapDimension(request.Params.XDimension),
					storage.HeatmapDimension(request.Params.YDimension),
					request.Params.Start, request.Params.End)
				if serr != nil {
					slog.Error("handler failure", "op", "GetMetricHeatmap.stream", "probe", seg.id, "error", serr)
					return GetMetricHeatmap500JSONResponse{Message: genericInternalError}, nil
				}
				elem := struct {
					Cells []struct {
						Count int64   `json:"count"`
						Value float64 `json:"value"`
						X     string  `json:"x"`
						Y     string  `json:"y"`
					} `json:"cells"`
					Id        string `json:"id"`
					Label     string `json:"label"`
					ScopeKind string `json:"scopeKind"`
				}{Id: seg.id, Label: seg.id, ScopeKind: "probe"}
				elem.Cells = make([]struct {
					Count int64   `json:"count"`
					Value float64 `json:"value"`
					X     string  `json:"x"`
					Y     string  `json:"y"`
				}, len(sc))
				for i, c := range sc {
					elem.Cells[i] = struct {
						Count int64   `json:"count"`
						Value float64 `json:"value"`
						X     string  `json:"x"`
						Y     string  `json:"y"`
					}{Count: c.Count, Value: c.Value, X: c.X, Y: c.Y}
				}
				streams = append(streams, elem)
			}
			resp.Streams = &streams
		}
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
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
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
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
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
		Degree     int64     `json:"degree"`
		Label      string    `json:"label"`
		Presence   *[]string `json:"presence,omitempty"`
		Text       string    `json:"text"`
		TotalCount int64     `json:"totalCount"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		var presence *[]string
		if len(n.Presence) > 0 {
			p := n.Presence
			presence = &p
		}
		resp.Nodes[i] = struct {
			Degree     int64     `json:"degree"`
			Label      string    `json:"label"`
			Presence   *[]string `json:"presence,omitempty"`
			Text       string    `json:"text"`
			TotalCount int64     `json:"totalCount"`
		}{Degree: n.Degree, Label: n.Label, Presence: presence, Text: n.Text, TotalCount: n.TotalCount}
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

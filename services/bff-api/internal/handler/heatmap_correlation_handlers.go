package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// maxHeatmapSegments caps the segmentBy fan-out (SEC-072). Each segment is one
// sequential ClickHouse query sharing the request's single deadline, so an
// unbounded segment count is an N+1 scaling cliff that would 500 the whole
// request mid-loop. Generous for the current corpus; refuse (400) above it.
const maxHeatmapSegments = 50

// GetMetricHeatmap returns a 2D aggregation of a metric for the requested
// xDimension / yDimension within a window. Backs the EDA x heatmap view-mode
// cells; entityLabel and language dimensions trigger Gold-table joins.
func (s *Server) GetMetricHeatmap(ctx context.Context, request GetMetricHeatmapRequestObject) (GetMetricHeatmapResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, probeSegs, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricHeatmap404JSONResponse{Message: reason}, nil
		}
		return GetMetricHeatmap400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetricHeatmap400JSONResponse{Message: msg}, nil
	}
	if !request.Params.XDimension.Valid() || !request.Params.YDimension.Valid() {
		return GetMetricHeatmap400JSONResponse{Message: "xDimension and yDimension must be one of dayOfWeek, hour, source, entityLabel, language"}, nil
	}
	// Phase 117 read-side alias.
	request.MetricName = canonicalMetricName(request.MetricName)

	cells, err := s.db.GetMetricHeatmap(
		ctx,
		request.MetricName,
		sources,
		storage.HeatmapDimension(request.Params.XDimension),
		storage.HeatmapDimension(request.Params.YDimension),
		start,
		end,
	)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricHeatmap", "error", err)
		return GetMetricHeatmap500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricHeatmap200JSONResponse{
		MetricName:  request.MetricName,
		Scope:       strPtr(string(kind)),
		ScopeID:     request.Params.ScopeID,
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
		// SEC-072: refuse an unbounded fan-out before issuing the sequential
		// per-segment queries below, rather than blow the request budget.
		segCount := len(sources)
		if *request.Params.SegmentBy == GetMetricHeatmapParamsSegmentByProbe {
			segCount = len(probeSegs)
		}
		if segCount > maxHeatmapSegments {
			return GetMetricHeatmap400JSONResponse{Message: fmt.Sprintf(
				"segmentBy fan-out exceeds the %d-segment limit; narrow the scope (fewer sources or probes)",
				maxHeatmapSegments)}, nil
		}
		switch *request.Params.SegmentBy {
		case GetMetricHeatmapParamsSegmentBySource:
			streams := make([]struct {
				Cells []struct {
					Count int64   `json:"count"`
					Value float64 `json:"value"`
					X     string  `json:"x"`
					Y     string  `json:"y"`
				} `json:"cells"`
				ID        string `json:"id"`
				Label     string `json:"label"`
				ScopeKind string `json:"scopeKind"`
			}, 0, len(sources))
			for _, src := range sources {
				sc, serr := s.db.GetMetricHeatmap(ctx, request.MetricName, []string{src},
					storage.HeatmapDimension(request.Params.XDimension),
					storage.HeatmapDimension(request.Params.YDimension),
					start, end)
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
					ID        string `json:"id"`
					Label     string `json:"label"`
					ScopeKind string `json:"scopeKind"`
				}{ID: src, Label: src, ScopeKind: "source"}
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
				ID        string `json:"id"`
				Label     string `json:"label"`
				ScopeKind string `json:"scopeKind"`
			}, 0, len(probeSegs))
			for _, seg := range probeSegs {
				sc, serr := s.db.GetMetricHeatmap(ctx, request.MetricName, seg.sources,
					storage.HeatmapDimension(request.Params.XDimension),
					storage.HeatmapDimension(request.Params.YDimension),
					start, end)
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
					ID        string `json:"id"`
					Label     string `json:"label"`
					ScopeKind string `json:"scopeKind"`
				}{ID: seg.id, Label: seg.id, ScopeKind: "probe"}
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
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricCorrelation404JSONResponse{Message: reason}, nil
		}
		return GetMetricCorrelation400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetricCorrelation400JSONResponse{Message: msg}, nil
	}

	metrics := splitAndTrim(request.Params.Metrics)
	if len(metrics) < 2 {
		return GetMetricCorrelation400JSONResponse{Message: "metrics must list at least 2 names"}, nil
	}
	if len(metrics) > 10 {
		return GetMetricCorrelation400JSONResponse{Message: "metrics is capped at 10 names per request"}, nil
	}
	// Phase 117 read-side alias.
	metrics = canonicalMetricNames(metrics)

	// Phase 125 — cross-frame gate. Correlating raw per-bucket means across
	// languages is only meaningful when each metric's cross-cultural
	// equivalence is granted; otherwise refuse with the standard cross-frame
	// payload (Level-1 alternative: stay within one cultural frame).
	if refusal, err := s.crossFrameGate(ctx, metrics, sources, start, end); err != nil {
		slog.Error("handler failure", "op", "GetMetricCorrelation.crossFrameGate", "error", err)
		return GetMetricCorrelation500JSONResponse{Message: genericInternalError}, nil
	} else if refusal != nil {
		return GetMetricCorrelation400JSONResponse{
			Message:            refusal.Message,
			Gate:               refusal.Gate,
			WorkingPaperAnchor: refusal.WorkingPaperAnchor,
			Alternatives:       refusal.Alternatives,
		}, nil
	}

	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)
	res, err := s.db.GetMetricCorrelation(ctx, metrics, sources, start, end, mf)
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
		ScopeID:     request.Params.ScopeID,
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}, nil
}

// GetCorrelationLeadLag computes the lagged cross-correlation of two metrics'
// hourly mean series over one scope (Phase 125). Generalises the Phase-124
// publication-activity lead-lag; reuses computeLeadLag/pearsonXY. A multi-
// language scope gates on both metrics' equivalence.
func (s *Server) GetCorrelationLeadLag(ctx context.Context, request GetCorrelationLeadLagRequestObject) (GetCorrelationLeadLagResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetCorrelationLeadLag404JSONResponse{Message: reason}, nil
		}
		return GetCorrelationLeadLag400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetCorrelationLeadLag400JSONResponse{Message: msg}, nil
	}
	xMetric := canonicalMetricNames([]string{request.Params.XMetric})[0]
	yMetric := canonicalMetricNames([]string{request.Params.YMetric})[0]
	if xMetric == "" || yMetric == "" {
		return GetCorrelationLeadLag400JSONResponse{Message: "xMetric and yMetric are required"}, nil
	}

	maxLag := leadLagDefaultMaxLagHours
	if request.Params.MaxLagHours != nil {
		maxLag = *request.Params.MaxLagHours
	}
	if maxLag < 1 || maxLag > leadLagMaxAllowedLagHours {
		return GetCorrelationLeadLag400JSONResponse{Message: "maxLagHours must be between 1 and 720"}, nil
	}

	// Cross-frame gate — correlating metric series across languages is only
	// meaningful when both metrics' cross-cultural equivalence is granted.
	if refusal, err := s.crossFrameGate(ctx, []string{xMetric, yMetric}, sources, start, end); err != nil {
		slog.Error("handler failure", "op", "GetCorrelationLeadLag.crossFrameGate", "error", err)
		return GetCorrelationLeadLag500JSONResponse{Message: genericInternalError}, nil
	} else if refusal != nil {
		return GetCorrelationLeadLag400JSONResponse{
			Message:            refusal.Message,
			Gate:               refusal.Gate,
			WorkingPaperAnchor: refusal.WorkingPaperAnchor,
			Alternatives:       refusal.Alternatives,
		}, nil
	}

	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)
	res, err := s.db.GetMetricLeadLag(ctx, sources, xMetric, yMetric, start, end, maxLag, mf)
	if err != nil {
		slog.Error("handler failure", "op", "GetCorrelationLeadLag", "error", err)
		return GetCorrelationLeadLag500JSONResponse{Message: genericInternalError}, nil
	}

	bucketAtZero := res.BucketCountAtZero
	resp := GetCorrelationLeadLag200JSONResponse{
		XMetric:           xMetric,
		YMetric:           yMetric,
		MaxLagHours:       res.MaxLagHours,
		BucketCountAtZero: &bucketAtZero,
		PeakLagHours:      res.PeakLagHours,
		PeakCorrelation:   res.PeakCorrelation,
		Scope:             strPtr(string(kind)),
		ScopeID:           request.Params.ScopeID,
		WindowStart:       request.Params.Start,
		WindowEnd:         request.Params.End,
	}
	resp.Points = make([]struct {
		// Correlation Pearson correlation at this lag; null when too few overlapping buckets or zero variance.
		Correlation *float64 `json:"correlation"`
		LagHours    int      `json:"lagHours"`
	}, len(res.Points))
	for i, p := range res.Points {
		resp.Points[i] = struct {
			Correlation *float64 `json:"correlation"`
			LagHours    int      `json:"lagHours"`
		}{Correlation: p.Correlation, LagHours: p.LagHours}
	}
	return resp, nil
}

// GetMetricParallelCoords returns the per-article N-metric matrix for the
// parallel-coordinates cell (Phase 125). Capped at 8 axes for readability; a
// multi-language scope gates on every metric's equivalence.
func (s *Server) GetMetricParallelCoords(ctx context.Context, request GetMetricParallelCoordsRequestObject) (GetMetricParallelCoordsResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricParallelCoords404JSONResponse{Message: reason}, nil
		}
		return GetMetricParallelCoords400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetricParallelCoords400JSONResponse{Message: msg}, nil
	}

	metrics := canonicalMetricNames(splitAndTrim(request.Params.Metrics))
	if len(metrics) < 2 {
		return GetMetricParallelCoords400JSONResponse{Message: "metrics must list at least 2 names"}, nil
	}
	if len(metrics) > 8 {
		metrics = metrics[:8] // cap axes for readability
	}

	// Cross-frame gate — a multi-language scope requires each metric's grant.
	if refusal, err := s.crossFrameGate(ctx, metrics, sources, start, end); err != nil {
		slog.Error("handler failure", "op", "GetMetricParallelCoords.crossFrameGate", "error", err)
		return GetMetricParallelCoords500JSONResponse{Message: genericInternalError}, nil
	} else if refusal != nil {
		return GetMetricParallelCoords400JSONResponse{
			Message:            refusal.Message,
			Gate:               refusal.Gate,
			WorkingPaperAnchor: refusal.WorkingPaperAnchor,
			Alternatives:       refusal.Alternatives,
		}, nil
	}

	maxPoints := 3000
	if request.Params.MaxPoints != nil {
		maxPoints = *request.Params.MaxPoints
	}

	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)
	res, err := s.db.GetParallelCoords(ctx, metrics, sources, start, end, maxPoints, mf)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricParallelCoords", "error", err)
		return GetMetricParallelCoords500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricParallelCoords200JSONResponse{
		Metrics:     res.Metrics,
		Truncated:   res.Truncated,
		Scope:       strPtr(string(kind)),
		ScopeID:     request.Params.ScopeID,
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}
	resp.Rows = make([]struct {
		ArticleID string    `json:"articleId"`
		Source    string    `json:"source"`
		Values    []float64 `json:"values"`
	}, len(res.Rows))
	for i, r := range res.Rows {
		resp.Rows[i] = struct {
			ArticleID string    `json:"articleId"`
			Source    string    `json:"source"`
			Values    []float64 `json:"values"`
		}{ArticleID: r.ArticleID, Source: r.Source, Values: r.Values}
	}
	return resp, nil
}

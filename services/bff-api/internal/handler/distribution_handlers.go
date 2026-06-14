package handler

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// GetMetricDistribution returns the per-scope value distribution for a metric
// over a window. Backs the EDA x ridgeline / violin / density view-mode cells.
func (s *Server) GetMetricDistribution(ctx context.Context, request GetMetricDistributionRequestObject) (GetMetricDistributionResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, probeSegs, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricDistribution404JSONResponse{Message: reason}, nil
		}
		return GetMetricDistribution400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetricDistribution400JSONResponse{Message: msg}, nil
	}
	if request.MetricName == "" {
		return GetMetricDistribution400JSONResponse{Message: "metricName is required"}, nil
	}
	// Phase 117 read-side alias.
	request.MetricName = canonicalMetricName(request.MetricName)

	bins := 30
	if request.Params.Bins != nil {
		bins = *request.Params.Bins
	}
	// Phase 125a faceting: applied uniformly to the aggregate and every segment.
	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)

	res, err := s.db.GetMetricDistribution(ctx, request.MetricName, sources, start, end, bins, mf)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricDistribution", "error", err)
		return GetMetricDistribution500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricDistribution200JSONResponse{
		MetricName:  request.MetricName,
		Scope:       strPtr(string(kind)),
		ScopeID:     request.Params.ScopeID,
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
	resp.ClampedUpper = res.ClampedUpper
	resp.OverflowCount = res.OverflowCount

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
				ID        string `json:"id"`
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
				sr, serr := s.db.GetMetricDistribution(ctx, request.MetricName, []string{src}, start, end, bins, mf)
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
					ID        string `json:"id"`
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
				}{ID: src, Label: src, ScopeKind: "source"}
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
				ID        string `json:"id"`
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
				sr, serr := s.db.GetMetricDistribution(ctx, request.MetricName, seg.sources, start, end, bins, mf)
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
					ID        string `json:"id"`
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
				}{ID: seg.id, Label: seg.id, ScopeKind: "probe"}
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

// GetMetricScatter returns one per-article point positioned by two metrics
// (xMetric, yMetric) with optional size / colour channels bound to further
// metrics. Backs the Phase-131 metadata-mining × scatter view-mode cell where
// visual channels are bound to chosen metric dimensions, and is the
// forward-compatible substrate for the Phase-133+ metric-chaining work.
func (s *Server) GetMetricScatter(ctx context.Context, request GetMetricScatterRequestObject) (GetMetricScatterResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetMetricScatter404JSONResponse{Message: reason}, nil
		}
		return GetMetricScatter400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetMetricScatter400JSONResponse{Message: msg}, nil
	}

	// Phase 117 read-side alias on every bound metric.
	xMetric := canonicalMetricName(strings.TrimSpace(request.Params.XMetric))
	yMetric := canonicalMetricName(strings.TrimSpace(request.Params.YMetric))
	if xMetric == "" || yMetric == "" {
		return GetMetricScatter400JSONResponse{Message: "xMetric and yMetric are required"}, nil
	}

	var sizeMetric, colorMetric *string
	if request.Params.SizeMetric != nil && strings.TrimSpace(*request.Params.SizeMetric) != "" {
		v := canonicalMetricName(strings.TrimSpace(*request.Params.SizeMetric))
		sizeMetric = &v
	}
	if request.Params.ColorMetric != nil && strings.TrimSpace(*request.Params.ColorMetric) != "" {
		v := canonicalMetricName(strings.TrimSpace(*request.Params.ColorMetric))
		colorMetric = &v
	}

	maxPoints := 2000
	if request.Params.MaxPoints != nil {
		maxPoints = *request.Params.MaxPoints
	}

	// Phase 125b — cross-frame gate parity with the other per-article cells
	// (correlation/cross-tab/lead-lag/parallel): a scatter pooling per-article
	// points across cultural frames is only meaningful when each bound metric's
	// equivalence is granted. Gate on x/y + any bound size/colour metric.
	gateMetrics := []string{xMetric, yMetric}
	if sizeMetric != nil {
		gateMetrics = append(gateMetrics, *sizeMetric)
	}
	if colorMetric != nil {
		gateMetrics = append(gateMetrics, *colorMetric)
	}
	if refusal, err := s.crossFrameGate(ctx, gateMetrics, sources, start, end); err != nil {
		slog.Error("handler failure", "op", "GetMetricScatter.crossFrameGate", "error", err)
		return GetMetricScatter500JSONResponse{Message: genericInternalError}, nil
	} else if refusal != nil {
		return GetMetricScatter400JSONResponse{
			Message:            refusal.Message,
			Gate:               refusal.Gate,
			WorkingPaperAnchor: refusal.WorkingPaperAnchor,
			Alternatives:       refusal.Alternatives,
		}, nil
	}

	mf := parseMetadataFilter(request.Params.MetadataFilterField, request.Params.MetadataFilterValue)
	res, err := s.db.GetMetricScatter(ctx, xMetric, yMetric, sizeMetric, colorMetric, sources, start, end, maxPoints, mf)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricScatter", "error", err)
		return GetMetricScatter500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricScatter200JSONResponse{
		XMetric:     xMetric,
		YMetric:     yMetric,
		SizeMetric:  sizeMetric,
		ColorMetric: colorMetric,
		Scope:       strPtr(string(kind)),
		ScopeID:     request.Params.ScopeID,
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
		Truncated:   res.Truncated,
	}
	resp.Points = make([]struct {
		ArticleID *string   `json:"articleId,omitempty"`
		Color     *float64  `json:"color,omitempty"`
		Size      *float64  `json:"size,omitempty"`
		Source    string    `json:"source"`
		Timestamp time.Time `json:"timestamp"`
		X         float64   `json:"x"`
		Y         float64   `json:"y"`
	}, len(res.Points))
	for i, p := range res.Points {
		elem := struct {
			ArticleID *string   `json:"articleId,omitempty"`
			Color     *float64  `json:"color,omitempty"`
			Size      *float64  `json:"size,omitempty"`
			Source    string    `json:"source"`
			Timestamp time.Time `json:"timestamp"`
			X         float64   `json:"x"`
			Y         float64   `json:"y"`
		}{
			Source:    p.Source,
			Timestamp: p.TS,
			X:         p.X,
			Y:         p.Y,
			Size:      p.Size,
			Color:     p.Color,
		}
		if p.ArticleID != "" {
			aid := p.ArticleID
			elem.ArticleID = &aid
		}
		resp.Points[i] = elem
	}
	return resp, nil
}

// GetScopeAvailableMetrics returns, for the resolved scope and window, the
// metrics present in Gold for every scoped source (available) versus only some
// (partial). Backs the Phase-123c cross-probe metric guard: the dashboard
// offers only `available` metrics for binding so a panel spanning probes with
// asymmetric capability never selects a metric that yields silent empty cells.
func (s *Server) GetScopeAvailableMetrics(ctx context.Context, request GetScopeAvailableMetricsRequestObject) (GetScopeAvailableMetricsResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	_, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetScopeAvailableMetrics404JSONResponse{Message: reason}, nil
		}
		return GetScopeAvailableMetrics400JSONResponse{Message: reason}, nil
	}
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetScopeAvailableMetrics400JSONResponse{Message: msg}, nil
	}

	avail, err := s.db.GetScopeAvailableMetrics(ctx, start, end, sources)
	if err != nil {
		slog.Error("handler failure", "op", "GetScopeAvailableMetrics", "error", err)
		return GetScopeAvailableMetrics500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetScopeAvailableMetrics200JSONResponse{
		ScopedSources: avail.ScopedSources,
		Available:     avail.Available,
		WindowStart:   request.Params.Start,
		WindowEnd:     request.Params.End,
	}
	resp.Partial = make([]struct {
		MetricName string   `json:"metricName"`
		Sources    []string `json:"sources"`
	}, len(avail.Partial))
	for i, p := range avail.Partial {
		resp.Partial[i] = struct {
			MetricName string   `json:"metricName"`
			Sources    []string `json:"sources"`
		}{MetricName: p.MetricName, Sources: p.Sources}
	}
	return resp, nil
}

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

// wholeDatasetStart is the lower sentinel for an unbounded query window. It
// predates any retained Gold row (data is TTL-bounded), so a
// [wholeDatasetStart, now] range returned for absent bounds yields the whole
// retained corpus — letting the storage layer keep its closed-[start,end]
// filters unchanged while time-limiting becomes an OPTIONAL request feature
// rather than a required default.
var wholeDatasetStart = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// resolveWindow turns an optional request window into the concrete [start,end]
// the storage layer queries. Each bound is INDEPENDENTLY optional: an absent
// bound opens that side to the dataset extent (lower → wholeDatasetStart,
// upper → now). So both absent ⇒ the whole dataset, and supplying exactly one
// bound is a valid open-ended window (e.g. "everything up to X") — never a 400.
// Only an inverted window (end not after start) is rejected. An empty msg means
// OK.
func resolveWindow(start, end *time.Time) (time.Time, time.Time, string) {
	s := wholeDatasetStart
	if start != nil {
		s = *start
	}
	e := time.Now().UTC()
	if end != nil {
		e = *end
	}
	if !e.After(s) {
		return time.Time{}, time.Time{}, "end must be strictly after start"
	}
	return s, e, ""
}

// validateWindow rejects malformed time windows before reaching ClickHouse.
// Used by endpoints whose window stays REQUIRED (e.g. the eligibility-gated
// Silver-aggregation surface); the analytical view-mode cells use the
// optional-aware resolveWindow above.
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
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
		Truncated:   res.Truncated,
	}
	resp.Points = make([]struct {
		ArticleId *string   `json:"articleId,omitempty"`
		Color     *float64  `json:"color,omitempty"`
		Size      *float64  `json:"size,omitempty"`
		Source    string    `json:"source"`
		Timestamp time.Time `json:"timestamp"`
		X         float64   `json:"x"`
		Y         float64   `json:"y"`
	}, len(res.Points))
	for i, p := range res.Points {
		elem := struct {
			ArticleId *string   `json:"articleId,omitempty"`
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
			elem.ArticleId = &aid
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
	_, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
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
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:           request.Params.ScopeId,
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
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
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
		ScopeId:     request.Params.ScopeId,
		WindowStart: request.Params.Start,
		WindowEnd:   request.Params.End,
	}
	resp.Rows = make([]struct {
		ArticleId string    `json:"articleId"`
		Source    string    `json:"source"`
		Values    []float64 `json:"values"`
	}, len(res.Rows))
	for i, r := range res.Rows {
		resp.Rows[i] = struct {
			ArticleId string    `json:"articleId"`
			Source    string    `json:"source"`
			Values    []float64 `json:"values"`
		}{ArticleId: r.ArticleID, Source: r.Source, Values: r.Values}
	}
	return resp, nil
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
	start, end, msg := resolveWindow(request.Params.Start, request.Params.End)
	if msg != "" {
		return GetEntityCoOccurrence400JSONResponse{Message: msg}, nil
	}

	topN := 50
	if request.Params.TopN != nil {
		topN = *request.Params.TopN
	}

	viewerLanguage := ""
	if request.Params.ViewerLanguage != nil {
		viewerLanguage = *request.Params.ViewerLanguage
	}
	nodeMetric := ""
	if request.Params.NodeMetric != nil {
		nodeMetric = canonicalMetricNames([]string{*request.Params.NodeMetric})[0]
	}
	// Phase 125b — min co-occurrence weight (edge threshold for the at-scale view).
	minWeight := 0
	if request.Params.MinWeight != nil {
		minWeight = *request.Params.MinWeight
	}
	// Phase 122d.2 — Negative-Space overlay: compute per-edge NS-support
	// (contributing articles with no real publication date) when requested.
	nsOverlay := request.Params.NegativeSpaceOverlay != nil && *request.Params.NegativeSpaceOverlay == "ghost"

	res, err := s.db.GetEntityCoOccurrence(ctx, sources, start, end, topN, viewerLanguage, nodeMetric, minWeight, nsOverlay)
	if err != nil {
		slog.Error("handler failure", "op", "GetEntityCoOccurrence", "error", err)
		return GetEntityCoOccurrence500JSONResponse{Message: genericInternalError}, nil
	}

	// Phase 122i revision (A6 observability). Surface how many edges
	// and nodes the storage layer returned and over which source set,
	// so a "3 nodes regardless of scope" complaint can be diagnosed by
	// reading the BFF log instead of guessing.
	slog.Info(
		"cooccurrence result",
		"op", "GetEntityCoOccurrence",
		"sources", strings.Join(sources, ","),
		"sourceCount", len(sources),
		"topN", topN,
		"edges", len(res.Edges),
		"nodes", len(res.Nodes),
	)

	articlesInScope := res.ArticlesInScope
	linkedNodeCount := res.LinkedNodeCount
	labeledNodeCount := res.LabeledNodeCount
	resp := GetEntityCoOccurrence200JSONResponse{
		TopN:             res.TopN,
		Scope:            strPtr(string(kind)),
		ScopeId:          request.Params.ScopeId,
		WindowStart:      request.Params.Start,
		WindowEnd:        request.Params.End,
		ArticlesInScope:  &articlesInScope,
		LinkedNodeCount:  &linkedNodeCount,
		LabeledNodeCount: &labeledNodeCount,
	}
	resp.Edges = make([]struct {
		A            string    `json:"a"`
		ALabel       *string   `json:"aLabel,omitempty"`
		ArticleCount int64     `json:"articleCount"`
		B            string    `json:"b"`
		BLabel       *string   `json:"bLabel,omitempty"`
		NsSupport    *int64    `json:"nsSupport,omitempty"`
		Presence     *[]string `json:"presence,omitempty"`
		Weight       int64     `json:"weight"`
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
		var presence *[]string
		if len(e.Presence) > 0 {
			p := e.Presence
			presence = &p
		}
		// Phase 122d.2 — per-edge NS-support, surfaced only when computed (>0;
		// the overlay is GET-only, so POST edges always omit it).
		var nsSupport *int64
		if e.NsSupportCount > 0 {
			v := e.NsSupportCount
			nsSupport = &v
		}
		resp.Edges[i] = struct {
			A            string    `json:"a"`
			ALabel       *string   `json:"aLabel,omitempty"`
			ArticleCount int64     `json:"articleCount"`
			B            string    `json:"b"`
			BLabel       *string   `json:"bLabel,omitempty"`
			NsSupport    *int64    `json:"nsSupport,omitempty"`
			Presence     *[]string `json:"presence,omitempty"`
			Weight       int64     `json:"weight"`
		}{A: e.A, ALabel: aLabel, ArticleCount: e.ArticleCount, B: e.B, BLabel: bLabel, NsSupport: nsSupport, Presence: presence, Weight: e.Weight}
	}
	resp.Nodes = make([]struct {
		Degree      int64     `json:"degree"`
		Label       string    `json:"label"`
		MetricValue *float64  `json:"metricValue,omitempty"`
		Presence    *[]string `json:"presence,omitempty"`
		Text        string    `json:"text"`
		TotalCount  int64     `json:"totalCount"`
		ViewerLabel *string   `json:"viewerLabel,omitempty"`
		WikidataQid *string   `json:"wikidataQid,omitempty"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		var presence *[]string
		if len(n.Presence) > 0 {
			p := n.Presence
			presence = &p
		}
		var qid *string
		if n.WikidataQid != "" {
			q := n.WikidataQid
			qid = &q
		}
		var viewerLabel *string
		if n.ViewerLabel != "" {
			vl := n.ViewerLabel
			viewerLabel = &vl
		}
		var metricValue *float64
		if n.MetricValue != nil {
			mv := safeFloat(*n.MetricValue)
			metricValue = &mv
		}
		resp.Nodes[i] = struct {
			Degree      int64     `json:"degree"`
			Label       string    `json:"label"`
			MetricValue *float64  `json:"metricValue,omitempty"`
			Presence    *[]string `json:"presence,omitempty"`
			Text        string    `json:"text"`
			TotalCount  int64     `json:"totalCount"`
			ViewerLabel *string   `json:"viewerLabel,omitempty"`
			WikidataQid *string   `json:"wikidataQid,omitempty"`
		}{Degree: n.Degree, Label: n.Label, MetricValue: metricValue, Presence: presence, Text: n.Text, TotalCount: n.TotalCount, ViewerLabel: viewerLabel, WikidataQid: qid}
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

// Phase 122i / ADR-034 — Multi-scope CoOccurrence POST endpoint.
//
// The Multi-Panel Workbench lets a single Rhizome Cell merge several
// `(probeIds, sourceIds)` ScopeGroups into one co-occurrence query. The
// legacy GET endpoint only accepts a single `(scope, scopeId)` target;
// the POST endpoint adds richer composition with two structural gates:
//
//   - **413 scope_limit_exceeded** — caps the union of all groups at
//     `maxCoOccurrenceUnionSources` unique source IDs and
//     `maxCoOccurrenceUnionProbes` unique probe IDs so a runaway
//     dashboard request can never spin up an unbounded ClickHouse scan.
//   - **422 cross_language_merge_unsupported** — refuses scopes whose
//     probe union spans more than one Language Capability Manifest
//     language. Network embeddings are language-specific (ADR-024);
//     merging them yields incompatible feature spaces and the dashboard
//     surfaces a refusal pointing the user to split-composition.
//
// The handler unions all groups into the existing per-source query
// path; ClickHouse storage is unchanged.

const (
	maxCoOccurrenceUnionSources = 100
	maxCoOccurrenceUnionProbes  = 25
)

// PostEntityCoOccurrenceQuery is the multi-scope counterpart to
// GetEntityCoOccurrence. See block comment above.
func (s *Server) PostEntityCoOccurrenceQuery(ctx context.Context, request PostEntityCoOccurrenceQueryRequestObject) (PostEntityCoOccurrenceQueryResponseObject, error) {
	if request.Body == nil || len(request.Body.Scopes) == 0 {
		return PostEntityCoOccurrenceQuery400JSONResponse{Message: "scopes is required and must contain at least one group"}, nil
	}
	body := *request.Body

	start, end, msg := resolveWindow(body.WindowStart, body.WindowEnd)
	if msg != "" {
		return PostEntityCoOccurrenceQuery400JSONResponse{Message: msg}, nil
	}

	// Resolve groups → union of source names + union of probe ids +
	// union of probe languages. The probe registry (`s.probes`) is the
	// authoritative source for both `Sources` and `Language` per probe.
	srcSeen := map[string]bool{}
	probeSeen := map[string]bool{}
	langSeen := map[string]bool{}
	var sources []string
	var languages []string

	addSource := func(src string) {
		src = strings.TrimSpace(src)
		if src == "" || srcSeen[src] {
			return
		}
		srcSeen[src] = true
		sources = append(sources, src)
	}
	addLanguage := func(lang string) {
		lang = strings.TrimSpace(lang)
		if lang == "" || langSeen[lang] {
			return
		}
		langSeen[lang] = true
		languages = append(languages, lang)
	}

	for i, group := range body.Scopes {
		if len(group.ProbeIds) == 0 {
			return PostEntityCoOccurrenceQuery400JSONResponse{Message: fmt.Sprintf("scopes[%d].probeIds must contain at least one probe id", i)}, nil
		}
		// Per-group source allowlist: when the group lists explicit
		// sourceIds, restrict that group's contribution to the
		// intersection; otherwise contribute all of the probe's
		// sources. Source ids outside the group's probes are dropped
		// silently (the dashboard can only pick from the dossier so
		// this is a belt-and-braces filter).
		allowed := map[string]bool{}
		for _, sid := range group.SourceIds {
			sid = strings.TrimSpace(sid)
			if sid != "" {
				allowed[sid] = true
			}
		}
		for _, pid := range group.ProbeIds {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			probe, exists := s.probes[pid]
			if !exists {
				return PostEntityCoOccurrenceQuery404JSONResponse{Message: fmt.Sprintf("unknown probe %q", pid)}, nil
			}
			if !probeSeen[pid] {
				probeSeen[pid] = true
				addLanguage(probe.Language)
			}
			if len(allowed) == 0 {
				for _, src := range probe.Sources {
					addSource(src)
				}
			} else {
				for _, src := range probe.Sources {
					if allowed[src] {
						addSource(src)
					}
				}
			}
		}
	}

	if len(sources) == 0 {
		return PostEntityCoOccurrenceQuery400JSONResponse{Message: "scope union resolved to zero sources"}, nil
	}
	if len(sources) > maxCoOccurrenceUnionSources || len(probeSeen) > maxCoOccurrenceUnionProbes {
		gate := "scope_limit_exceeded"
		alts := []string{
			fmt.Sprintf("narrow to <= %d sources and <= %d probes per request", maxCoOccurrenceUnionSources, maxCoOccurrenceUnionProbes),
			"split composition: render each ScopeGroup as its own Cell",
		}
		return PostEntityCoOccurrenceQuery413JSONResponse{
			Message:      fmt.Sprintf("scope union exceeds caps: %d sources (max %d), %d probes (max %d)", len(sources), maxCoOccurrenceUnionSources, len(probeSeen), maxCoOccurrenceUnionProbes),
			Gate:         &gate,
			Alternatives: &alts,
		}, nil
	}
	if len(languages) > 1 {
		gate := "cross_language_merge_unsupported"
		anchor := "ADR-034#cross-language"
		alts := []string{
			"narrow the scope to a single language",
			"split composition: each Cell renders one language",
		}
		return PostEntityCoOccurrenceQuery422JSONResponse{
			Message:            fmt.Sprintf("cross-language merge not supported (scope spans %d languages: %s)", len(languages), strings.Join(languages, ", ")),
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alts,
		}, nil
	}

	topN := 50
	if body.TopN != nil {
		topN = *body.TopN
	}
	if topN < 1 {
		topN = 1
	}
	if topN > 500 {
		topN = 500
	}

	viewerLanguage := ""
	if body.ViewerLanguage != nil {
		viewerLanguage = *body.ViewerLanguage
	}

	// The POST multi-scope path is the merged-graph (SVG) renderer only — the
	// at-scale WebGL view is single-cell (GET). No minWeight here (topN<=500
	// already bounds it).
	res, err := s.db.GetEntityCoOccurrence(ctx, sources, start, end, topN, viewerLanguage, "", 0, false)
	if err != nil {
		slog.Error("handler failure", "op", "PostEntityCoOccurrenceQuery", "error", err)
		return PostEntityCoOccurrenceQuery500JSONResponse{Message: genericInternalError}, nil
	}

	// Phase 122i revision (A6 observability) — same shape as the GET handler.
	slog.Info(
		"cooccurrence result",
		"op", "PostEntityCoOccurrenceQuery",
		"sources", strings.Join(sources, ","),
		"sourceCount", len(sources),
		"probeCount", len(probeSeen),
		"topN", topN,
		"edges", len(res.Edges),
		"nodes", len(res.Nodes),
	)

	articlesInScope := res.ArticlesInScope
	linkedNodeCount := res.LinkedNodeCount
	labeledNodeCount := res.LabeledNodeCount
	resp := PostEntityCoOccurrenceQuery200JSONResponse{
		TopN:             res.TopN,
		WindowStart:      body.WindowStart,
		WindowEnd:        body.WindowEnd,
		ArticlesInScope:  &articlesInScope,
		LinkedNodeCount:  &linkedNodeCount,
		LabeledNodeCount: &labeledNodeCount,
	}
	resp.Edges = make([]struct {
		A            string    `json:"a"`
		ALabel       *string   `json:"aLabel,omitempty"`
		ArticleCount int64     `json:"articleCount"`
		B            string    `json:"b"`
		BLabel       *string   `json:"bLabel,omitempty"`
		NsSupport    *int64    `json:"nsSupport,omitempty"`
		Presence     *[]string `json:"presence,omitempty"`
		Weight       int64     `json:"weight"`
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
		var presence *[]string
		if len(e.Presence) > 0 {
			p := e.Presence
			presence = &p
		}
		// Phase 122d.2 — per-edge NS-support, surfaced only when computed (>0;
		// the overlay is GET-only, so POST edges always omit it).
		var nsSupport *int64
		if e.NsSupportCount > 0 {
			v := e.NsSupportCount
			nsSupport = &v
		}
		resp.Edges[i] = struct {
			A            string    `json:"a"`
			ALabel       *string   `json:"aLabel,omitempty"`
			ArticleCount int64     `json:"articleCount"`
			B            string    `json:"b"`
			BLabel       *string   `json:"bLabel,omitempty"`
			NsSupport    *int64    `json:"nsSupport,omitempty"`
			Presence     *[]string `json:"presence,omitempty"`
			Weight       int64     `json:"weight"`
		}{A: e.A, ALabel: aLabel, ArticleCount: e.ArticleCount, B: e.B, BLabel: bLabel, NsSupport: nsSupport, Presence: presence, Weight: e.Weight}
	}
	resp.Nodes = make([]struct {
		Degree      int64     `json:"degree"`
		Label       string    `json:"label"`
		MetricValue *float64  `json:"metricValue,omitempty"`
		Presence    *[]string `json:"presence,omitempty"`
		Text        string    `json:"text"`
		TotalCount  int64     `json:"totalCount"`
		ViewerLabel *string   `json:"viewerLabel,omitempty"`
		WikidataQid *string   `json:"wikidataQid,omitempty"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		var presence *[]string
		if len(n.Presence) > 0 {
			p := n.Presence
			presence = &p
		}
		var qid *string
		if n.WikidataQid != "" {
			q := n.WikidataQid
			qid = &q
		}
		var viewerLabel *string
		if n.ViewerLabel != "" {
			vl := n.ViewerLabel
			viewerLabel = &vl
		}
		var metricValue *float64
		if n.MetricValue != nil {
			mv := safeFloat(*n.MetricValue)
			metricValue = &mv
		}
		resp.Nodes[i] = struct {
			Degree      int64     `json:"degree"`
			Label       string    `json:"label"`
			MetricValue *float64  `json:"metricValue,omitempty"`
			Presence    *[]string `json:"presence,omitempty"`
			Text        string    `json:"text"`
			TotalCount  int64     `json:"totalCount"`
			ViewerLabel *string   `json:"viewerLabel,omitempty"`
			WikidataQid *string   `json:"wikidataQid,omitempty"`
		}{Degree: n.Degree, Label: n.Label, MetricValue: metricValue, Presence: presence, Text: n.Text, TotalCount: n.TotalCount, ViewerLabel: viewerLabel, WikidataQid: qid}
	}
	return resp, nil
}

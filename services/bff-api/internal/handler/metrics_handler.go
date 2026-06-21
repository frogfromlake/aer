package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// GetMetrics handles the GET /metrics request and fetches time-series data.
// startDate and endDate are OPTIONAL: omit both for the whole dataset (time
// limiting is an optional feature, not the default); supplying one without the
// other is rejected.
//
// When normalization=zscore (Phase 65) or normalization=percentile (Phase 115),
// a validation gate enforces:
// (a) baselines must exist for the requested (metricName, source) pair;
// (b) an admissible equivalence level must be confirmed in `metric_equivalence`.
// The admissible level is metric-class-aware (Phase 124): a temporal-axis
// metric (publication_hour/weekday) is satisfied by a temporal Level-1 grant;
// every other metric requires deviation-or-absolute;
// (c) Phase 115 cross-frame gate — when the resolved scope spans more than one
// language, that admissible equivalence must additionally be granted across
// every language in the scope. Otherwise the response is 400 with a structured
// RefusalPayload (gate=metric_equivalence, anchor=WP-004#section-5.2).
func (s *Server) GetMetrics(ctx context.Context, request GetMetricsRequestObject) (GetMetricsResponseObject, error) {
	if request.Params.Normalization != nil && !request.Params.Normalization.Valid() {
		return GetMetrics400JSONResponse{Message: "invalid normalization; must be one of raw, zscore, percentile"}, nil
	}
	if request.Params.Resolution != nil && !request.Params.Resolution.Valid() {
		return GetMetrics400JSONResponse{Message: "invalid resolution; must be one of 5min, hourly, daily, weekly, monthly"}, nil
	}

	mode := Raw
	if request.Params.Normalization != nil {
		mode = *request.Params.Normalization
	}
	useNormalization := mode == Zscore || mode == Percentile

	// Phase 117 read-side alias: `sentiment_score` → `sentiment_score_sentiws`.
	canonicalMetricNamePtr(request.Params.MetricName)

	sources := unionSourceParams(request.Params.Source, request.Params.SourceIds)

	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetMetrics400JSONResponse{Message: msg}, nil
	}

	if useNormalization {
		if request.Params.MetricName == nil {
			return GetMetrics400JSONResponse{Message: "normalization requires the metricName parameter"}, nil
		}

		// For the baseline check, pass a single source when unambiguous; nil
		// means "any source" which is the correct fallback for multi-source requests.
		var bsSource *string
		if len(sources) == 1 {
			bsSource = &sources[0]
		}
		baselineExists, err := s.db.CheckBaselineExists(ctx, *request.Params.MetricName, bsSource)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CheckBaselineExists", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if !baselineExists {
			return GetMetrics400JSONResponse{Message: "no baseline data exists for the requested metric and source; compute baselines before requesting normalization"}, nil
		}

		equivExists, err := s.db.CheckEquivalenceExists(ctx, *request.Params.MetricName)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CheckEquivalenceExists", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if !equivExists {
			return GetMetrics400JSONResponse{Message: "no validated equivalence entry exists for this metric at a level admissible for normalization; cross-cultural comparability has not been validated"}, nil
		}

		// Phase 115 cross-frame equivalence gate.
		nLangs, err := s.db.CountLanguagesForSources(ctx, start, end, sources)
		if err != nil {
			slog.Error("handler failure", "op", "GetMetrics.CountLanguagesForSources", "error", err)
			return GetMetrics500JSONResponse{Message: genericInternalError}, nil
		}
		if nLangs > 1 {
			languages, err := s.collectLanguagesForScope(ctx, start, end, sources)
			if err != nil {
				slog.Error("handler failure", "op", "GetMetrics.collectLanguagesForScope", "error", err)
				return GetMetrics500JSONResponse{Message: genericInternalError}, nil
			}
			ok, err := s.db.CheckNormalizationEquivalenceForLanguages(ctx, *request.Params.MetricName, languages)
			if err != nil {
				slog.Error("handler failure", "op", "GetMetrics.CheckNormalizationEquivalenceForLanguages", "error", err)
				return GetMetrics500JSONResponse{Message: genericInternalError}, nil
			}
			if !ok {
				return crossFrameRefusal(), nil
			}
		}
	}

	resolution := resolutionFromParam(request.Params.Resolution)

	// Phase 131: a spread-bearing request reads the raw layer so the
	// per-bucket sample stddev (absent from the resolution MVs) is
	// available. Mutually exclusive with normalization — a z-score /
	// percentile series has no raw spread to attach.
	includeStddev := request.Params.IncludeStddev != nil && *request.Params.IncludeStddev && !useNormalization

	var result storage.MetricsResult
	var excludedCount int64
	var err error
	switch mode {
	case Zscore:
		result, excludedCount, err = s.db.GetNormalizedMetrics(ctx, start, end, sources, request.Params.MetricName, resolution)
	case Percentile:
		result, excludedCount, err = s.db.GetPercentileNormalizedMetrics(ctx, start, end, sources, request.Params.MetricName, resolution)
	default:
		if includeStddev {
			result, err = s.db.GetMetricsWithSpread(ctx, start, end, sources, request.Params.MetricName, resolution)
		} else {
			result, err = s.db.GetMetrics(ctx, start, end, sources, request.Params.MetricName, resolution)
		}
	}
	if err != nil {
		slog.Error("handler failure", "op", "GetMetrics", "error", err)
		return GetMetrics500JSONResponse{Message: genericInternalError}, nil
	}
	data := result.Rows

	points := make([]struct {
		Count      *int64    `json:"count,omitempty"`
		MetricName string    `json:"metricName"`
		Source     string    `json:"source"`
		Stddev     *float64  `json:"stddev,omitempty"`
		Timestamp  time.Time `json:"timestamp"`
		Value      float64   `json:"value"`
	}, 0, len(data))
	for _, d := range data {
		// Narrow ClickHouse's UInt64 into the generated int64 DTO field.
		// Gold-layer bucket counts in a bounded time window fit well under
		// math.MaxInt64, but we clamp defensively rather than trust the type.
		count := max(int64(d.Count), 0) //nolint:gosec // clamped above
		p := struct {
			Count      *int64    `json:"count,omitempty"`
			MetricName string    `json:"metricName"`
			Source     string    `json:"source"`
			Stddev     *float64  `json:"stddev,omitempty"`
			Timestamp  time.Time `json:"timestamp"`
			Value      float64   `json:"value"`
		}{
			Count:      &count,
			Timestamp:  d.TS,
			Value:      d.Value,
			Source:     d.Source,
			MetricName: d.MetricName,
		}
		if includeStddev {
			stddev := d.Stddev
			p.Stddev = &stddev
		}
		points = append(points, p)
	}

	return GetMetrics200JSONResponse{
		Data:          points,
		ExcludedCount: excludedCount,
		Truncated:     result.Truncated,
	}, nil
}

// GetEntities handles GET /entities — returns aggregated named entities.
// startDate and endDate are OPTIONAL: omit both for the whole dataset;
// supplying one without the other is rejected.
func (s *Server) GetEntities(ctx context.Context, request GetEntitiesRequestObject) (GetEntitiesResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 1000 {
		return GetEntities400JSONResponse{Message: "limit must be between 1 and 1000"}, nil
	}

	sources := unionSourceParams(request.Params.Source, request.Params.SourceIds)
	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetEntities400JSONResponse{Message: msg}, nil
	}
	data, err := s.db.GetEntities(ctx, start, end, sources, request.Params.Label, limit)
	if err != nil {
		slog.Error("handler failure", "op", "GetEntities", "error", err)
		return GetEntities500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetEntities200JSONResponse
	for _, d := range data {
		var qid *string
		var conf *float32
		if d.WikidataQid != "" {
			q := d.WikidataQid
			qid = &q
			c := d.LinkConfidence
			conf = &c
		}
		response = append(response, struct {
			Count          int64    `json:"count"`
			EntityLabel    string   `json:"entityLabel"`
			EntityText     string   `json:"entityText"`
			LinkConfidence *float32 `json:"linkConfidence,omitempty"`
			Sources        []string `json:"sources"`
			WikidataQid    *string  `json:"wikidataQid,omitempty"`
		}{
			EntityText:     d.EntityText,
			EntityLabel:    d.EntityLabel,
			Count:          int64(d.Count),
			Sources:        d.Sources,
			WikidataQid:    qid,
			LinkConfidence: conf,
		})
	}

	return response, nil
}

// GetLanguages handles GET /languages — returns aggregated language detections.
// startDate and endDate are OPTIONAL: omit both for the whole dataset, or supply
// one for an open-ended window (resolveWindow opens the other side).
func (s *Server) GetLanguages(ctx context.Context, request GetLanguagesRequestObject) (GetLanguagesResponseObject, error) {
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 1000 {
		return GetLanguages400JSONResponse{Message: "limit must be between 1 and 1000"}, nil
	}
	if errBody, ok := s.validateLanguageQueryParam(request.Params.Language); !ok {
		gate := errBody.Gate
		anchor := errBody.WorkingPaperAnchor
		alts := errBody.Alternatives
		return GetLanguages400JSONResponse{
			Message:            errBody.Message,
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alts,
		}, nil
	}

	sources := unionSourceParams(request.Params.Source, request.Params.SourceIds)
	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetLanguages400JSONResponse{Message: msg}, nil
	}
	data, err := s.db.GetLanguageDetections(ctx, start, end, sources, request.Params.Language, limit)
	if err != nil {
		slog.Error("handler failure", "op", "GetLanguages", "error", err)
		return GetLanguages500JSONResponse{Message: genericInternalError}, nil
	}

	var response GetLanguages200JSONResponse
	for _, d := range data {
		response = append(response, struct {
			AvgConfidence    float64  `json:"avgConfidence"`
			Count            int64    `json:"count"`
			DetectedLanguage string   `json:"detectedLanguage"`
			Sources          []string `json:"sources"`
		}{
			DetectedLanguage: d.DetectedLanguage,
			Count:            int64(d.Count),
			AvgConfidence:    d.AvgConfidence,
			Sources:          d.Sources,
		})
	}

	return response, nil
}

// GetMetricProvenance handles GET /metrics/{metricName}/provenance.
// Static fields (tier, algorithm description, known limitations, extractor
// version) come from the bundled metric_provenance.yaml config. Dynamic
// fields (validationStatus, culturalContextNotes) are resolved against the
// metric_validity / metric_equivalence ClickHouse tables.
func (s *Server) GetMetricProvenance(ctx context.Context, request GetMetricProvenanceRequestObject) (GetMetricProvenanceResponseObject, error) {
	// Phase 117 read-side alias: legacy `sentiment_score` URL paths still
	// resolve through the rename to `sentiment_score_sentiws`.
	request.MetricName = canonicalMetricName(request.MetricName)
	entry, ok := s.provenance[request.MetricName]
	if !ok {
		return GetMetricProvenance404JSONResponse{Message: "no provenance entry registered for metric"}, nil
	}

	status, err := s.db.GetMetricValidationStatus(ctx, request.MetricName)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricProvenance.GetMetricValidationStatus", "error", err)
		return GetMetricProvenance500JSONResponse{Message: genericInternalError}, nil
	}
	notes, err := s.db.GetMetricCulturalContextNotes(ctx, request.MetricName)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricProvenance.GetMetricCulturalContextNotes", "error", err)
		return GetMetricProvenance500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetricProvenance200JSONResponse{
		MetricName:           request.MetricName,
		TierClassification:   MetricProvenanceTierClassification(entry.TierClassification),
		AlgorithmDescription: entry.AlgorithmDescription,
		KnownLimitations:     entry.KnownLimitations,
		ValidationStatus:     MetricProvenanceValidationStatus(status),
		ExtractorVersionHash: entry.ExtractorVersionHash,
	}
	if notes != "" {
		n := notes
		resp.CulturalContextNotes = &n
	}
	return resp, nil
}

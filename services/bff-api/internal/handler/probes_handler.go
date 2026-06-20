package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
)

// GetSources handles GET /sources — returns the list of known data
// sources with optional methodology documentation URLs. Data comes from
// the PostgreSQL `sources` table (the SSoT) via a TTL-cached read-only
// store. A misconfigured stack (nil source lister) or a Postgres outage
// with no warm cache surfaces as 500. When `silverOnly=true` (Phase 103),
// the response is filtered to sources whose `silver_eligible` flag is set
// so the dashboard's Silver-layer source picker does not surface sources
// the eligibility gate would refuse.
func (s *Server) GetSources(ctx context.Context, request GetSourcesRequestObject) (GetSourcesResponseObject, error) {
	if s.sources == nil {
		slog.Error("handler failure", "op", "GetSources", "error", "source lister is not configured")
		return GetSources500JSONResponse{Message: genericInternalError}, nil
	}
	entries, err := s.sources.List(ctx)
	if err != nil {
		slog.Error("handler failure", "op", "GetSources", "error", err)
		return GetSources500JSONResponse{Message: genericInternalError}, nil
	}
	silverOnly := request.Params.SilverOnly != nil && *request.Params.SilverOnly
	response := make(GetSources200JSONResponse, 0, len(entries))
	for _, src := range entries {
		if silverOnly && !src.SilverEligible {
			continue
		}
		response = append(response, Source{
			Name:             src.Name,
			Type:             src.Type,
			URL:              src.URL,
			DocumentationURL: src.DocumentationURL,
		})
	}
	return response, nil
}

// GetProbes handles GET /probes — returns the list of active probes
// with emission geometry and bound sources. Registry is loaded from
// YAML at startup (no runtime I/O). Dual-Register editorial content is
// served separately via /content/probe/{probeId}.
func (s *Server) GetProbes(ctx context.Context, _ GetProbesRequestObject) (GetProbesResponseObject, error) {
	entries := s.probes.Ordered()

	// Phase 151 — attach the all-time distinct-document count per probe (the
	// dataset-overview readout on the Atmosphere surface). One grouped query
	// over the union of all bound sources, summed back per probe. This is
	// best-effort: if the analytical store is unavailable, `totals` is nil and
	// every probe's DocumentCount stays nil (the geometry feed must keep
	// rendering the globe regardless), and the client shows the count as
	// unavailable rather than a fabricated zero.
	var totals map[string]int64
	if s.db != nil {
		seen := make(map[string]struct{})
		allSources := make([]string, 0)
		for _, p := range entries {
			for _, src := range p.Sources {
				if _, ok := seen[src]; !ok {
					seen[src] = struct{}{}
					allSources = append(allSources, src)
				}
			}
		}
		if t, err := s.db.GetDocumentTotalsBySource(ctx, allSources); err != nil {
			slog.Error("handler degraded", "op", "GetProbes", "error", err)
		} else {
			totals = t
		}
	}

	response := make(GetProbes200JSONResponse, 0, len(entries))
	for _, p := range entries {
		// The EmissionPoints element is an anonymous struct in the
		// generated code (oapi-codegen inlines sub-refs). We build it
		// positionally here rather than introducing a parallel named
		// type that would have to be kept in sync with the generator.
		probe := Probe{
			ProbeID:     p.ProbeID,
			DisplayName: p.Display(),
			ShortName:   p.Short(),
			Language:    p.Language,
			Sources:     append([]string(nil), p.Sources...),
			EmissionPoints: make([]struct {
				Label     string  `json:"label"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}, 0, len(p.EmissionPoints)),
		}
		for _, pt := range p.EmissionPoints {
			probe.EmissionPoints = append(probe.EmissionPoints, struct {
				Label     string  `json:"label"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}{
				Label:     pt.Label,
				Latitude:  pt.Latitude,
				Longitude: pt.Longitude,
			})
		}
		if p.Country != "" {
			c := p.Country
			probe.Country = &c
		}
		if totals != nil {
			var sum int64
			for _, src := range p.Sources {
				sum += totals[src]
			}
			probe.DocumentCount = &sum
		}
		response = append(response, probe)
	}
	return response, nil
}

// GetMetricsAvailable handles GET /metrics/available — returns distinct metric names
// with validation status for a time range.
// startDate and endDate are OPTIONAL: omit both for the whole dataset;
// supplying one without the other is rejected.
func (s *Server) GetMetricsAvailable(ctx context.Context, request GetMetricsAvailableRequestObject) (GetMetricsAvailableResponseObject, error) {
	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetMetricsAvailable400JSONResponse{Message: msg}, nil
	}
	rows, err := s.db.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetricsAvailable", "error", err)
		return GetMetricsAvailable500JSONResponse{Message: genericInternalError}, nil
	}

	// Task B — per-metric display label in the requested locale (default en).
	// Resolved from the in-memory content catalogue post-cache (the cache holds
	// only the locale-independent Gold rows), so adding the label costs nothing
	// per request beyond a map lookup.
	locale := "en"
	if request.Params.Locale != nil {
		locale = string(*request.Params.Locale)
	}

	var response GetMetricsAvailable200JSONResponse
	for _, r := range rows {
		// Phase 121b: forward-looking alias guard. Any metric name registered
		// as an alias key in metric_aliases.go is dropped from the response —
		// its canonical replacement already appears in the same set, so the
		// alias entry can only ever surface as a duplicate in MetricSwitcher.
		// Pre-rename rows produced before a renaming Phase remain in the Gold
		// layer for the 365-day TTL window; this filter prevents them from
		// leaking back into the dashboard.
		if _, isAlias := metricNameAliases[r.MetricName]; isAlias {
			continue
		}
		m := AvailableMetric{
			MetricName:       r.MetricName,
			ValidationStatus: AvailableMetricValidationStatus(r.ValidationStatus),
		}
		if label := s.catalog.DisplayLabel(locale, "metric", r.MetricName); label != "" {
			m.DisplayLabel = &label
		}
		if r.EticConstruct != nil {
			m.EticConstruct = r.EticConstruct
		}
		if r.EquivalenceLevel != nil {
			lvl := AvailableMetricEquivalenceLevel(*r.EquivalenceLevel)
			m.EquivalenceLevel = &lvl
		}
		if r.EquivalenceStatus != nil {
			es := r.EquivalenceStatus
			status := struct {
				Level          *string    `json:"level,omitempty"`
				Notes          string     `json:"notes"`
				ValidatedBy    *string    `json:"validatedBy,omitempty"`
				ValidationDate *time.Time `json:"validationDate,omitempty"`
			}{Notes: es.Notes}
			if es.Level != nil {
				lvl := *es.Level
				status.Level = &lvl
			}
			if es.ValidatedBy != nil {
				vb := *es.ValidatedBy
				status.ValidatedBy = &vb
			}
			if es.ValidationDate != nil {
				vd := *es.ValidationDate
				status.ValidationDate = &vd
			}
			m.EquivalenceStatus = &status
		}
		if minRes := config.LookupMinMeaningfulResolution(r.MetricName); minRes != "" {
			res := AvailableMetricMinMeaningfulResolution(minRes)
			m.MinMeaningfulResolution = &res
		}
		response = append(response, m)
	}

	return response, nil
}

// GetProbeEquivalence handles GET /probes/{probeId}/equivalence — Phase 115.
// Returns per-metric Level-1 / Level-2 / Level-3 availability for the
// probe's resolved source set. Drives the Probe Dossier "what comparisons
// are valid here" panel.
//
// The window defaults to the last 90 days when no explicit range is
// provided — the same default the Operations Playbook uses for baseline
// computation, so the Dossier matrix and the manual baseline run share a
// horizon.
func (s *Server) GetProbeEquivalence(ctx context.Context, request GetProbeEquivalenceRequestObject) (GetProbeEquivalenceResponseObject, error) {
	probe, ok := s.probes[request.ProbeID]
	if !ok {
		return GetProbeEquivalence404JSONResponse{Message: "probe not found"}, nil
	}

	// Phase 124: with comparedTo the scope is the UNION of both probes'
	// sources, so the matrix reports what is valid for the cross-probe pair.
	scopeSources := append([]string(nil), probe.Sources...)
	var comparedTo *string
	if request.Params.ComparedTo != nil && *request.Params.ComparedTo != "" {
		other, ok := s.probes[*request.Params.ComparedTo]
		if !ok {
			return GetProbeEquivalence404JSONResponse{Message: "comparedTo probe not found"}, nil
		}
		scopeSources = append(scopeSources, other.Sources...)
		scopeSources = uniqueNonEmpty(scopeSources...)
		id := other.ProbeID
		comparedTo = &id
	}

	end := time.Now().UTC()
	start := end.Add(-90 * 24 * time.Hour)

	rows, err := s.db.GetProbeEquivalence(ctx, start, end, scopeSources)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeEquivalence", "error", err)
		return GetProbeEquivalence500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetProbeEquivalence200JSONResponse{
		ProbeID:    probe.ProbeID,
		ComparedTo: comparedTo,
	}
	if len(scopeSources) > 0 {
		sources := append([]string(nil), scopeSources...)
		resp.Sources = &sources
	}
	for _, r := range rows {
		entry := struct {
			EquivalenceStatus *struct {
				Level          *string    `json:"level,omitempty"`
				Notes          string     `json:"notes"`
				ValidatedBy    *string    `json:"validatedBy,omitempty"`
				ValidationDate *time.Time `json:"validationDate,omitempty"`
			} `json:"equivalenceStatus,omitempty"`
			Level1Available bool   `json:"level1Available"`
			Level2Available bool   `json:"level2Available"`
			Level3Available bool   `json:"level3Available"`
			MetricName      string `json:"metricName"`
		}{
			MetricName:      r.MetricName,
			Level1Available: r.Level1Available,
			Level2Available: r.Level2Available,
			Level3Available: r.Level3Available,
		}
		if r.EquivalenceStatus != nil {
			es := r.EquivalenceStatus
			status := struct {
				Level          *string    `json:"level,omitempty"`
				Notes          string     `json:"notes"`
				ValidatedBy    *string    `json:"validatedBy,omitempty"`
				ValidationDate *time.Time `json:"validationDate,omitempty"`
			}{Notes: es.Notes}
			if es.Level != nil {
				lvl := *es.Level
				status.Level = &lvl
			}
			if es.ValidatedBy != nil {
				vb := *es.ValidatedBy
				status.ValidatedBy = &vb
			}
			if es.ValidationDate != nil {
				vd := *es.ValidationDate
				status.ValidationDate = &vd
			}
			entry.EquivalenceStatus = &status
		}
		resp.Metrics = append(resp.Metrics, entry)
	}
	return resp, nil
}

const (
	// leadLagDefaultMaxLagHours is ±7 days of hourly lags.
	leadLagDefaultMaxLagHours = 168
	leadLagMaxAllowedLagHours = 720
	// leadLagSignal names the Phase-124 lead-lag signal: hourly publication
	// activity (distinct-article count). Phase 125 generalises to metric series.
	leadLagSignal = "publication_activity"
	// leadLagGateMetric is the temporal-rhythm metric whose grant authorises
	// the cross-probe temporal lead-lag comparison.
	leadLagGateMetric = "publication_hour"
	// leadLagAnchor is the methodological anchor for the temporal Level-1 grant.
	leadLagAnchor = "WP-004 Appendix B"
)

// GetProbeLeadLag handles GET /probes/{probeId}/lead-lag — Phase 124. The
// lagged cross-correlation of hourly publication activity between the reference
// probe (`probeId`) and the compared probe (`comparedTo`). A cross-cultural
// relational artefact, so it is gated on a temporal-level equivalence grant
// covering both probes' languages; an ungranted pair returns a RefusalPayload-
// shaped 400. Phase 125 generalises this to arbitrary metric series.
func (s *Server) GetProbeLeadLag(ctx context.Context, request GetProbeLeadLagRequestObject) (GetProbeLeadLagResponseObject, error) {
	ref, ok := s.probes[request.ProbeID]
	if !ok {
		return GetProbeLeadLag404JSONResponse{Message: "probe not found"}, nil
	}
	compared, ok := s.probes[request.Params.ComparedTo]
	if !ok {
		return GetProbeLeadLag404JSONResponse{Message: "comparedTo probe not found"}, nil
	}
	if request.ProbeID == request.Params.ComparedTo {
		return GetProbeLeadLag400JSONResponse{Message: "comparedTo must differ from probeId"}, nil
	}

	// Window: explicit range when given, else the last 90 days (the same
	// horizon GetProbeEquivalence and the baseline run use).
	end := time.Now().UTC()
	start := end.Add(-90 * 24 * time.Hour)
	if request.Params.Start != nil {
		start = *request.Params.Start
	}
	if request.Params.End != nil {
		end = *request.Params.End
	}
	if !end.After(start) {
		return GetProbeLeadLag400JSONResponse{Message: "end must be strictly after start"}, nil
	}

	maxLag := leadLagDefaultMaxLagHours
	if request.Params.MaxLagHours != nil {
		maxLag = *request.Params.MaxLagHours
	}
	if maxLag < 1 || maxLag > leadLagMaxAllowedLagHours {
		return GetProbeLeadLag400JSONResponse{Message: "maxLagHours must be between 1 and 720"}, nil
	}

	// Gate: the cross-cultural temporal comparison is authorised only by a
	// temporal-level grant covering both probes' languages.
	languages := uniqueNonEmpty(ref.Language, compared.Language)
	granted, err := s.db.CheckNormalizationEquivalenceForLanguages(ctx, leadLagGateMetric, languages)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeLeadLag.CheckNormalizationEquivalenceForLanguages", "error", err)
		return GetProbeLeadLag500JSONResponse{Message: genericInternalError}, nil
	}
	if !granted {
		gate := crossFrameGateID
		anchor := leadLagAnchor
		alternatives := []string{
			"compare the two probes within a single cultural frame",
			"view each probe's temporal rhythm without a cross-cultural claim",
		}
		return GetProbeLeadLag400JSONResponse{
			Message:            "cross-probe lead-lag requires a temporal-level equivalence grant across both probes' languages; granted out-of-band via WP-004 §6.3",
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alternatives,
		}, nil
	}

	res, err := s.db.GetTemporalLeadLag(ctx, ref.Sources, compared.Sources, start, end, maxLag)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeLeadLag.GetTemporalLeadLag", "error", err)
		return GetProbeLeadLag500JSONResponse{Message: genericInternalError}, nil
	}

	bucketAtZero := res.BucketCountAtZero
	resp := GetProbeLeadLag200JSONResponse{
		ReferenceProbe:    ref.ProbeID,
		ComparedProbe:     compared.ProbeID,
		Signal:            leadLagSignal,
		MaxLagHours:       res.MaxLagHours,
		BucketCountAtZero: &bucketAtZero,
		PeakLagHours:      res.PeakLagHours,
		PeakCorrelation:   res.PeakCorrelation,
	}

	// Grant block for the methodology banner (server-authoritative).
	resp.Grant.Level = "temporal"
	resp.Grant.WorkingPaperAnchor = leadLagAnchor
	status, err := s.db.GetEquivalenceStatus(ctx, leadLagGateMetric)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeLeadLag.GetEquivalenceStatus", "error", err)
		return GetProbeLeadLag500JSONResponse{Message: genericInternalError}, nil
	}
	if status != nil {
		if status.Level != nil {
			resp.Grant.Level = *status.Level
		}
		if status.Notes != "" {
			notes := status.Notes
			resp.Grant.Notes = &notes
		}
		if status.ValidatedBy != nil {
			vb := *status.ValidatedBy
			resp.Grant.ValidatedBy = &vb
		}
	}

	for _, p := range res.Points {
		resp.Points = append(resp.Points, struct {
			Correlation *float64 `json:"correlation"`
			LagHours    int      `json:"lagHours"`
		}{Correlation: p.Correlation, LagHours: p.LagHours})
	}
	return resp, nil
}

// uniqueNonEmpty returns the distinct, non-empty values among its arguments,
// preserving first-seen order. Used to collapse a probe pair's languages into
// the set the equivalence gate must cover.
func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

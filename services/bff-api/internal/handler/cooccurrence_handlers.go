package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// GetEntityCoOccurrence returns the top-N entity-pair edges aggregated over
// the window, plus the union of incident nodes with degree and total weight.
func (s *Server) GetEntityCoOccurrence(ctx context.Context, request GetEntityCoOccurrenceRequestObject) (GetEntityCoOccurrenceResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeID, request.Params.ProbeIds, request.Params.SourceIds)
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
	topN = storage.ClampCoOccurrenceTopN(topN)

	viewerLanguage := ""
	if request.Params.ViewerLanguage != nil {
		viewerLanguage = *request.Params.ViewerLanguage
	}
	nodeMetric := ""
	if request.Params.NodeMetric != nil {
		nodeMetric = canonicalMetricNames([]string{*request.Params.NodeMetric})[0]
	}
	// Phase 125 / ISSUE 7 — optional separate colour-channel metric.
	colorMetric := ""
	if request.Params.NodeColorMetric != nil {
		colorMetric = canonicalMetricNames([]string{*request.Params.NodeColorMetric})[0]
	}
	// Phase 125b — min co-occurrence weight (edge threshold for the at-scale view).
	minWeight := 0
	if request.Params.MinWeight != nil {
		minWeight = *request.Params.MinWeight
	}
	// Phase 122d.2 — Negative-Space overlay: compute per-edge NS-support
	// (contributing articles with no real publication date) when requested.
	nsOverlay := request.Params.NegativeSpaceOverlay != nil && *request.Params.NegativeSpaceOverlay == "ghost"
	// Phase 148g — node-first breadth control (top-N entities by weight; edges
	// among them). 0/absent = legacy edge-first.
	maxNodes := 0
	if request.Params.MaxNodes != nil {
		maxNodes = *request.Params.MaxNodes
	}

	res, err := s.db.GetEntityCoOccurrence(ctx, sources, start, end, topN, viewerLanguage, nodeMetric, minWeight, nsOverlay, colorMetric, maxNodes)
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
		ScopeID:          request.Params.ScopeID,
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
		Degree                int64     `json:"degree"`
		Label                 string    `json:"label"`
		MetricValue           *float64  `json:"metricValue,omitempty"`
		MetricValueColor      *float64  `json:"metricValueColor,omitempty"`
		Presence              *[]string `json:"presence,omitempty"`
		PresenceArticleCounts *[]int64  `json:"presenceArticleCounts,omitempty"`
		Text                  string    `json:"text"`
		TotalCount            int64     `json:"totalCount"`
		ViewerLabel           *string   `json:"viewerLabel,omitempty"`
		WikidataQid           *string   `json:"wikidataQid,omitempty"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		var presence *[]string
		if len(n.Presence) > 0 {
			p := n.Presence
			presence = &p
		}
		var presenceCounts *[]int64
		if len(n.PresenceArticleCounts) > 0 {
			pc := n.PresenceArticleCounts
			presenceCounts = &pc
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
		var metricValueColor *float64
		if n.MetricValueColor != nil {
			mvc := safeFloat(*n.MetricValueColor)
			metricValueColor = &mvc
		}
		resp.Nodes[i] = struct {
			Degree                int64     `json:"degree"`
			Label                 string    `json:"label"`
			MetricValue           *float64  `json:"metricValue,omitempty"`
			MetricValueColor      *float64  `json:"metricValueColor,omitempty"`
			Presence              *[]string `json:"presence,omitempty"`
			PresenceArticleCounts *[]int64  `json:"presenceArticleCounts,omitempty"`
			Text                  string    `json:"text"`
			TotalCount            int64     `json:"totalCount"`
			ViewerLabel           *string   `json:"viewerLabel,omitempty"`
			WikidataQid           *string   `json:"wikidataQid,omitempty"`
		}{Degree: n.Degree, Label: n.Label, MetricValue: metricValue, MetricValueColor: metricValueColor, Presence: presence, PresenceArticleCounts: presenceCounts, Text: n.Text, TotalCount: n.TotalCount, ViewerLabel: viewerLabel, WikidataQid: qid}
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
	// Cross-language merge is refused BY DEFAULT (entity nodes are surface forms;
	// merging across languages would conflate identity — ADR-034). Phase 148g
	// turns the hard refusal into an explicit USER opt-in: with
	// allowCrossLanguage=true the union renders as one graph WITHOUT merging node
	// identity across languages (the honest "two discourse spheres, stitched at
	// shared actors" view — the caller surfaces that disclosure). QID-based
	// cross-language node merging is a separate future step.
	allowCrossLanguage := body.AllowCrossLanguage != nil && *body.AllowCrossLanguage
	if len(languages) > 1 && !allowCrossLanguage {
		gate := "cross_language_merge_unsupported"
		anchor := "ADR-034#cross-language"
		alts := []string{
			"confirm the cross-language view (allowCrossLanguage) — nodes are NOT merged across languages",
			"narrow the scope to a single language",
			"split composition: each Cell renders one language",
		}
		return PostEntityCoOccurrenceQuery422JSONResponse{
			Message:            fmt.Sprintf("cross-language merge not supported without confirmation (scope spans %d languages: %s)", len(languages), strings.Join(languages, ", ")),
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alts,
		}, nil
	}

	topN := 50
	if body.TopN != nil {
		topN = *body.TopN
	}
	// SEC-069 — same [1, 6000] ceiling as GET/storage/UI (was a stale 500 cap
	// that silently clamped topN the cooccurrence config UI offers up to 6000).
	topN = storage.ClampCoOccurrenceTopN(topN)

	viewerLanguage := ""
	if body.ViewerLanguage != nil {
		viewerLanguage = *body.ViewerLanguage
	}
	// Phase 148g — node-first breadth control on the merged path too (0 = edge-first).
	maxNodes := 0
	if body.MaxNodes != nil {
		maxNodes = *body.MaxNodes
	}

	// The POST multi-scope path serves both the merged SVG renderer and (Phase
	// 148g) the merged at-scale view. No minWeight here (topN already bounds the
	// edge set; node-first breadth is controlled by maxNodes).
	res, err := s.db.GetEntityCoOccurrence(ctx, sources, start, end, topN, viewerLanguage, "", 0, false, "", maxNodes)
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
		Degree                int64     `json:"degree"`
		Label                 string    `json:"label"`
		MetricValue           *float64  `json:"metricValue,omitempty"`
		MetricValueColor      *float64  `json:"metricValueColor,omitempty"`
		Presence              *[]string `json:"presence,omitempty"`
		PresenceArticleCounts *[]int64  `json:"presenceArticleCounts,omitempty"`
		Text                  string    `json:"text"`
		TotalCount            int64     `json:"totalCount"`
		ViewerLabel           *string   `json:"viewerLabel,omitempty"`
		WikidataQid           *string   `json:"wikidataQid,omitempty"`
	}, len(res.Nodes))
	for i, n := range res.Nodes {
		var presence *[]string
		if len(n.Presence) > 0 {
			p := n.Presence
			presence = &p
		}
		var presenceCounts *[]int64
		if len(n.PresenceArticleCounts) > 0 {
			pc := n.PresenceArticleCounts
			presenceCounts = &pc
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
		var metricValueColor *float64
		if n.MetricValueColor != nil {
			mvc := safeFloat(*n.MetricValueColor)
			metricValueColor = &mvc
		}
		resp.Nodes[i] = struct {
			Degree                int64     `json:"degree"`
			Label                 string    `json:"label"`
			MetricValue           *float64  `json:"metricValue,omitempty"`
			MetricValueColor      *float64  `json:"metricValueColor,omitempty"`
			Presence              *[]string `json:"presence,omitempty"`
			PresenceArticleCounts *[]int64  `json:"presenceArticleCounts,omitempty"`
			Text                  string    `json:"text"`
			TotalCount            int64     `json:"totalCount"`
			ViewerLabel           *string   `json:"viewerLabel,omitempty"`
			WikidataQid           *string   `json:"wikidataQid,omitempty"`
		}{Degree: n.Degree, Label: n.Label, MetricValue: metricValue, MetricValueColor: metricValueColor, Presence: presence, PresenceArticleCounts: presenceCounts, Text: n.Text, TotalCount: n.TotalCount, ViewerLabel: viewerLabel, WikidataQid: qid}
	}
	return resp, nil
}

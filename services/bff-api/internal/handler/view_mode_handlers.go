package handler

import (
	"fmt"
	"strings"
	"time"
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
	rawScope string, scopeID, probeIds, sourceIds *string,
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
	if scopeID != nil && strings.TrimSpace(*scopeID) != "" {
		id := strings.TrimSpace(*scopeID)
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

// strPtr is a tiny helper for the optional Scope/ScopeID echo fields.
func strPtr(s string) *string { return &s }

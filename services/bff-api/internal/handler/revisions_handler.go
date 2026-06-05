package handler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// errRevisionsProbeNotFound is the typed sentinel returned by the
// scope resolver when a `?scope=probe` request references an unknown
// probe id.
var errRevisionsProbeNotFound = errors.New("probe not found")

// GetRevisionActivity handles GET /revisions — Phase 122d.0 (ADR-032).
//
// The handler resolves scope → source list, validates the window,
// maps the URL-level resolution enum onto the storage-side constant,
// and aggregates `aer_gold.article_revisions`. Sources with zero rows
// in the window are intentionally absent from the response — the
// dashboard renders absences from the scope membership, so the
// "we didn't observe edits" signal is honestly distinguishable from
// "we have not yet observed this source at all".
func (s *Server) GetRevisionActivity(ctx context.Context, request GetRevisionActivityRequestObject) (GetRevisionActivityResponseObject, error) {
	scope := GetRevisionActivityParamsScopeProbe
	if request.Params.Scope != nil {
		scope = *request.Params.Scope
	}
	sources, err := s.resolveRevisionScope(ctx, scope, request.Params.ScopeId)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) || errors.Is(err, errRevisionsProbeNotFound) {
			return GetRevisionActivity404JSONResponse{Message: err.Error()}, nil
		}
		slog.Error("handler failure", "op", "GetRevisionActivity.resolveScope", "error", err)
		return GetRevisionActivity500JSONResponse{Message: genericInternalError}, nil
	}

	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetRevisionActivity400JSONResponse{Message: msg}, nil
	}
	resolution := revisionResolutionFromParam(request.Params.Resolution)

	cells, err := s.db.GetRevisionActivity(
		ctx,
		sources,
		start,
		end,
		resolution,
	)
	if err != nil {
		slog.Error("handler failure", "op", "GetRevisionActivity", "error", err)
		return GetRevisionActivity500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetRevisionActivity200JSONResponse{
		Scope:      revisionScopeToResponse(scope),
		ScopeId:    request.Params.ScopeId,
		Resolution: revisionResolutionToResponse(resolution),
	}
	resp.WindowStart = request.Params.StartDate
	resp.WindowEnd = request.Params.EndDate

	entries := make([]struct {
		ArticlesAffected int             `json:"articlesAffected"`
		Bucket           time.Time       `json:"bucket"`
		ByTrigger        *map[string]int `json:"byTrigger,omitempty"`
		Revisions        int             `json:"revisions"`
		Source           string          `json:"source"`
	}, 0, len(cells))
	for _, c := range cells {
		breakdown := map[string]int{}
		if c.CdxSnapshotCount > 0 {
			breakdown["cdx_snapshot"] = int(c.CdxSnapshotCount) //nolint:gosec // bounded
		}
		if c.RepublicationCount > 0 {
			breakdown["republication_trigger"] = int(c.RepublicationCount) //nolint:gosec // bounded
		}
		if c.UnknownTriggerCount > 0 {
			breakdown["unknown"] = int(c.UnknownTriggerCount) //nolint:gosec // bounded
		}
		var bt *map[string]int
		if len(breakdown) > 0 {
			bt = &breakdown
		}
		entries = append(entries, struct {
			ArticlesAffected int             `json:"articlesAffected"`
			Bucket           time.Time       `json:"bucket"`
			ByTrigger        *map[string]int `json:"byTrigger,omitempty"`
			Revisions        int             `json:"revisions"`
			Source           string          `json:"source"`
		}{
			ArticlesAffected: int(c.ArticlesAffected), //nolint:gosec // bounded
			Bucket:           c.Bucket,
			ByTrigger:        bt,
			Revisions:        int(c.Revisions), //nolint:gosec // bounded
			Source:           c.Source,
		})
	}
	resp.Entries = entries
	return resp, nil
}

// GetRevisionDiscourseShift handles GET /revisions/discourse-shift —
// Phase 122d.3. Same scope/window/resolution grammar as
// `GetRevisionActivity`; aggregates the re-extraction deltas
// (`sentiment_delta`, `topic_shift_score`, entity add/remove) over the
// computed-delta edits. Sources with no such edits in the window are
// absent (rendered from scope membership, like every revision surface).
func (s *Server) GetRevisionDiscourseShift(ctx context.Context, request GetRevisionDiscourseShiftRequestObject) (GetRevisionDiscourseShiftResponseObject, error) {
	scope := GetRevisionActivityParamsScopeProbe
	if request.Params.Scope != nil {
		scope = GetRevisionActivityParamsScope(string(*request.Params.Scope))
	}
	sources, err := s.resolveRevisionScope(ctx, scope, request.Params.ScopeId)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) || errors.Is(err, errRevisionsProbeNotFound) {
			return GetRevisionDiscourseShift404JSONResponse{Message: err.Error()}, nil
		}
		slog.Error("handler failure", "op", "GetRevisionDiscourseShift.resolveScope", "error", err)
		return GetRevisionDiscourseShift500JSONResponse{Message: genericInternalError}, nil
	}

	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetRevisionDiscourseShift400JSONResponse{Message: msg}, nil
	}
	resolution := discourseShiftResolutionFromParam(request.Params.Resolution)

	cells, err := s.db.GetRevisionDiscourseShift(ctx, sources, start, end, resolution)
	if err != nil {
		slog.Error("handler failure", "op", "GetRevisionDiscourseShift", "error", err)
		return GetRevisionDiscourseShift500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetRevisionDiscourseShift200JSONResponse{
		Scope:      discourseShiftScopeToResponse(scope),
		ScopeId:    request.Params.ScopeId,
		Resolution: discourseShiftResolutionToResponse(resolution),
	}
	resp.WindowStart = request.Params.StartDate
	resp.WindowEnd = request.Params.EndDate

	entries := make([]struct {
		AvgSentimentDelta    float64   `json:"avgSentimentDelta"`
		AvgTopicShift        float64   `json:"avgTopicShift"`
		Bucket               time.Time `json:"bucket"`
		EditsWithDeltas      int       `json:"editsWithDeltas"`
		EntitiesAddedTotal   int       `json:"entitiesAddedTotal"`
		EntitiesRemovedTotal int       `json:"entitiesRemovedTotal"`
		NetSentimentDrift    float64   `json:"netSentimentDrift"`
		Source               string    `json:"source"`
	}, 0, len(cells))
	for _, c := range cells {
		entries = append(entries, struct {
			AvgSentimentDelta    float64   `json:"avgSentimentDelta"`
			AvgTopicShift        float64   `json:"avgTopicShift"`
			Bucket               time.Time `json:"bucket"`
			EditsWithDeltas      int       `json:"editsWithDeltas"`
			EntitiesAddedTotal   int       `json:"entitiesAddedTotal"`
			EntitiesRemovedTotal int       `json:"entitiesRemovedTotal"`
			NetSentimentDrift    float64   `json:"netSentimentDrift"`
			Source               string    `json:"source"`
		}{
			AvgSentimentDelta:    c.AvgSentimentDelta,
			AvgTopicShift:        c.AvgTopicShift,
			Bucket:               c.Bucket,
			EditsWithDeltas:      int(c.EditsWithDeltas),      //nolint:gosec // bounded
			EntitiesAddedTotal:   int(c.EntitiesAddedTotal),   //nolint:gosec // bounded
			EntitiesRemovedTotal: int(c.EntitiesRemovedTotal), //nolint:gosec // bounded
			NetSentimentDrift:    c.NetSentimentDrift,
			Source:               c.Source,
		})
	}
	resp.Entries = entries
	return resp, nil
}

// discourseShiftResolutionFromParam maps the URL resolution enum onto the
// storage constant, defaulting to `daily` (the Episteme trajectory grain).
func discourseShiftResolutionFromParam(p *GetRevisionDiscourseShiftParamsResolution) storage.RevisionActivityResolution {
	if p == nil {
		return storage.RevisionResolutionDaily
	}
	switch storage.RevisionActivityResolution(*p) {
	case storage.RevisionResolutionSnapshot,
		storage.RevisionResolutionDaily,
		storage.RevisionResolutionWeekly,
		storage.RevisionResolutionMonthly:
		return storage.RevisionActivityResolution(*p)
	default:
		return storage.RevisionResolutionDaily
	}
}

func discourseShiftResolutionToResponse(r storage.RevisionActivityResolution) RevisionDiscourseShiftResponseResolution {
	switch r {
	case storage.RevisionResolutionDaily:
		return RevisionDiscourseShiftResponseResolutionDaily
	case storage.RevisionResolutionWeekly:
		return RevisionDiscourseShiftResponseResolutionWeekly
	case storage.RevisionResolutionMonthly:
		return RevisionDiscourseShiftResponseResolutionMonthly
	default:
		return RevisionDiscourseShiftResponseResolutionSnapshot
	}
}

func discourseShiftScopeToResponse(scope GetRevisionActivityParamsScope) RevisionDiscourseShiftResponseScope {
	if strings.EqualFold(string(scope), string(GetRevisionActivityParamsScopeSource)) {
		return RevisionDiscourseShiftResponseScopeSource
	}
	return RevisionDiscourseShiftResponseScopeProbe
}

// GetRevisionEditClusters handles GET /revisions/edit-clusters —
// Phase 122d.3 (Rhizome). Surfaces cross-source temporally-clustered
// silent edits on shared entities. Same scope/window/resolution grammar
// as the other revision surfaces, plus a `minSources` threshold (clamped
// [2, 10], default 2). A single-source scope yields no clusters.
func (s *Server) GetRevisionEditClusters(ctx context.Context, request GetRevisionEditClustersRequestObject) (GetRevisionEditClustersResponseObject, error) {
	scope := GetRevisionActivityParamsScopeProbe
	if request.Params.Scope != nil {
		scope = GetRevisionActivityParamsScope(string(*request.Params.Scope))
	}
	sources, err := s.resolveRevisionScope(ctx, scope, request.Params.ScopeId)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) || errors.Is(err, errRevisionsProbeNotFound) {
			return GetRevisionEditClusters404JSONResponse{Message: err.Error()}, nil
		}
		slog.Error("handler failure", "op", "GetRevisionEditClusters.resolveScope", "error", err)
		return GetRevisionEditClusters500JSONResponse{Message: genericInternalError}, nil
	}

	start, end, msg := resolveWindow(request.Params.StartDate, request.Params.EndDate)
	if msg != "" {
		return GetRevisionEditClusters400JSONResponse{Message: msg}, nil
	}
	resolution := editClustersResolutionFromParam(request.Params.Resolution)

	minSources := 2
	if request.Params.MinSources != nil {
		minSources = *request.Params.MinSources
	}
	if minSources < 2 {
		minSources = 2
	}
	if minSources > 10 {
		minSources = 10
	}

	rows, err := s.db.GetRevisionEditClusters(ctx, sources, start, end, resolution, minSources)
	if err != nil {
		slog.Error("handler failure", "op", "GetRevisionEditClusters", "error", err)
		return GetRevisionEditClusters500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetRevisionEditClusters200JSONResponse{
		Scope:      editClustersScopeToResponse(scope),
		ScopeId:    request.Params.ScopeId,
		Resolution: editClustersResolutionToResponse(resolution),
		MinSources: minSources,
	}
	resp.WindowStart = request.Params.StartDate
	resp.WindowEnd = request.Params.EndDate

	clusters := make([]struct {
		AvgTopicShift float64   `json:"avgTopicShift"`
		Bucket        time.Time `json:"bucket"`
		EditCount     int       `json:"editCount"`
		Entity        string    `json:"entity"`
		Sources       []string  `json:"sources"`
	}, 0, len(rows))
	for _, r := range rows {
		clusters = append(clusters, struct {
			AvgTopicShift float64   `json:"avgTopicShift"`
			Bucket        time.Time `json:"bucket"`
			EditCount     int       `json:"editCount"`
			Entity        string    `json:"entity"`
			Sources       []string  `json:"sources"`
		}{
			AvgTopicShift: r.AvgTopicShift,
			Bucket:        r.Bucket,
			EditCount:     int(r.EditCount), //nolint:gosec // bounded
			Entity:        r.Entity,
			Sources:       append([]string(nil), r.Sources...),
		})
	}
	resp.Clusters = clusters
	return resp, nil
}

func editClustersResolutionFromParam(p *GetRevisionEditClustersParamsResolution) storage.RevisionActivityResolution {
	if p == nil {
		return storage.RevisionResolutionDaily
	}
	switch storage.RevisionActivityResolution(*p) {
	case storage.RevisionResolutionSnapshot,
		storage.RevisionResolutionDaily,
		storage.RevisionResolutionWeekly,
		storage.RevisionResolutionMonthly:
		return storage.RevisionActivityResolution(*p)
	default:
		return storage.RevisionResolutionDaily
	}
}

func editClustersResolutionToResponse(r storage.RevisionActivityResolution) RevisionEditClustersResponseResolution {
	switch r {
	case storage.RevisionResolutionDaily:
		return RevisionEditClustersResponseResolutionDaily
	case storage.RevisionResolutionWeekly:
		return RevisionEditClustersResponseResolutionWeekly
	case storage.RevisionResolutionMonthly:
		return RevisionEditClustersResponseResolutionMonthly
	default:
		return RevisionEditClustersResponseResolutionSnapshot
	}
}

func editClustersScopeToResponse(scope GetRevisionActivityParamsScope) RevisionEditClustersResponseScope {
	if strings.EqualFold(string(scope), string(GetRevisionActivityParamsScopeSource)) {
		return RevisionEditClustersResponseScopeSource
	}
	return RevisionEditClustersResponseScopeProbe
}

// GetArticleRevisions handles GET /articles/{id}/revisions — the L5
// Evidence per-article chain. Reuses the Silver-eligibility gate that
// governs every other per-article read so a non-eligible source
// cannot leak an article identifier (or revision timeline) through
// this surface.
func (s *Server) GetArticleRevisions(ctx context.Context, request GetArticleRevisionsRequestObject) (GetArticleRevisionsResponseObject, error) {
	if s.dossier == nil {
		return GetArticleRevisions500JSONResponse{Message: genericInternalError}, nil
	}

	res, err := s.dossier.ResolveArticle(ctx, request.Id)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetArticleRevisions404JSONResponse{Message: "article not found"}, nil
		}
		slog.Error("handler failure", "op", "GetArticleRevisions.ResolveArticle", "error", err)
		return GetArticleRevisions500JSONResponse{Message: genericInternalError}, nil
	}

	if _, err := s.requireSilverEligible(ctx, res.SourceName); err != nil {
		if errors.Is(err, errSilverNotEligible) {
			return articleRevisionsEligibilityRefusal(), nil
		}
		if errors.Is(err, errSilverSourceNotFound) {
			return GetArticleRevisions404JSONResponse{Message: "article source not found"}, nil
		}
		if errors.Is(err, errSilverNotConfigured) {
			return GetArticleRevisions500JSONResponse{Message: genericInternalError}, nil
		}
		slog.Error("handler failure", "op", "GetArticleRevisions.requireSilverEligible", "error", err)
		return GetArticleRevisions500JSONResponse{Message: genericInternalError}, nil
	}

	rows, err := s.db.GetArticleRevisions(ctx, request.Id)
	if err != nil {
		slog.Error("handler failure", "op", "GetArticleRevisions", "error", err)
		return GetArticleRevisions500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetArticleRevisions200JSONResponse{
		ArticleId: request.Id,
		Source:    res.SourceName,
	}
	if len(rows) == 0 {
		// Gold has no rows. The Silver MinIO envelope carries the real
		// `wayback_lookup_status`; the L5EvidenceReader cross-references
		// it via the existing `GET /articles/{id}` endpoint when the
		// revision list is empty, so the BFF surface can stay
		// strict-projection-only here.
		resp.LookupStatus = Empty
	} else {
		resp.LookupStatus = Ok
	}

	revisions := make([]struct {
		ArchiveUrl         *string                                     `json:"archiveUrl,omitempty"`
		ContentHash        string                                      `json:"contentHash"`
		DeltasComputed     bool                                        `json:"deltasComputed"`
		DiffStatus         ArticleRevisionsResponseRevisionsDiffStatus `json:"diffStatus"`
		EntitiesAdded      *[]string                                   `json:"entitiesAdded,omitempty"`
		EntitiesRemoved    *[]string                                   `json:"entitiesRemoved,omitempty"`
		PrevContentHash    *string                                     `json:"prevContentHash,omitempty"`
		RevisionIndex      int                                         `json:"revisionIndex"`
		SentimentDelta     *float64                                    `json:"sentimentDelta,omitempty"`
		SnapshotAt         time.Time                                   `json:"snapshotAt"`
		TimeSincePrevHours *float64                                    `json:"timeSincePrevHours,omitempty"`
		TopicShiftScore    *float64                                    `json:"topicShiftScore,omitempty"`
		Trigger            ArticleRevisionsResponseRevisionsTrigger    `json:"trigger"`
	}, 0, len(rows))
	for _, r := range rows {
		entry := struct {
			ArchiveUrl         *string                                     `json:"archiveUrl,omitempty"`
			ContentHash        string                                      `json:"contentHash"`
			DeltasComputed     bool                                        `json:"deltasComputed"`
			DiffStatus         ArticleRevisionsResponseRevisionsDiffStatus `json:"diffStatus"`
			EntitiesAdded      *[]string                                   `json:"entitiesAdded,omitempty"`
			EntitiesRemoved    *[]string                                   `json:"entitiesRemoved,omitempty"`
			PrevContentHash    *string                                     `json:"prevContentHash,omitempty"`
			RevisionIndex      int                                         `json:"revisionIndex"`
			SentimentDelta     *float64                                    `json:"sentimentDelta,omitempty"`
			SnapshotAt         time.Time                                   `json:"snapshotAt"`
			TimeSincePrevHours *float64                                    `json:"timeSincePrevHours,omitempty"`
			TopicShiftScore    *float64                                    `json:"topicShiftScore,omitempty"`
			Trigger            ArticleRevisionsResponseRevisionsTrigger    `json:"trigger"`
		}{
			ContentHash:    r.ContentHash,
			DeltasComputed: r.DeltasComputed,
			DiffStatus:     ArticleRevisionsResponseRevisionsDiffStatus(r.DiffStatus),
			RevisionIndex:  int(r.RevisionIndex), //nolint:gosec // bounded
			SnapshotAt:     r.SnapshotAt,
			Trigger:        ArticleRevisionsResponseRevisionsTrigger(r.Trigger),
		}
		// Phase 122d.3 — surface the discourse-shift deltas only when they
		// were actually computed (a real edit with a known language). When
		// not computed the omitempty pointers stay nil so the client never
		// reads a default as a measurement. entitiesAdded/Removed are emitted
		// even when empty (an empty set is a real "no entity change" datum).
		if r.DeltasComputed {
			sentimentDelta := r.SentimentDelta
			topicShift := r.TopicShiftScore
			added := append([]string(nil), r.EntitiesAdded...)
			removed := append([]string(nil), r.EntitiesRemoved...)
			entry.SentimentDelta = &sentimentDelta
			entry.TopicShiftScore = &topicShift
			entry.EntitiesAdded = &added
			entry.EntitiesRemoved = &removed
		}
		// Honour the OpenAPI omitempty contract for chain-head rows.
		// `prevContentHash` and `timeSincePrevHours` are documented as
		// "absent for the chain head"; a value of `""` / `0.0` is a
		// real datum (e.g. two same-second snapshots could legitimately
		// have `timeSincePrevHours=0`). Emitting the pointers for
		// index=0 would force consumers to special-case "is this the
		// real head or a zero-gap from the previous?" against an
		// already-empty PrevContentHash. Leaving them nil makes the
		// JSON shape `{revisionIndex:0, contentHash:'…', …}` —
		// unambiguously the head.
		if r.RevisionIndex > 0 {
			prev := r.PrevContentHash
			tsp := r.TimeSincePrevHours
			entry.PrevContentHash = &prev
			entry.TimeSincePrevHours = &tsp
		}
		// Surface the Internet Archive playback URL when present (empty for
		// republication-trigger rows that have no archive page yet).
		if r.ArchiveURL != "" {
			archiveURL := r.ArchiveURL
			entry.ArchiveUrl = &archiveURL
		}
		revisions = append(revisions, entry)
	}
	resp.Revisions = revisions
	return resp, nil
}

// resolveRevisionScope expands a scope/scopeId pair into the source
// list the storage layer needs. `probe` resolves against the bundled
// probe registry; `source` resolves the literal source name through
// the dossier store.
func (s *Server) resolveRevisionScope(ctx context.Context, scope GetRevisionActivityParamsScope, scopeID string) ([]string, error) {
	switch scope {
	case GetRevisionActivityParamsScopeSource:
		if s.dossier == nil {
			return nil, errSilverNotConfigured
		}
		_, name, err := s.dossier.ResolveSource(ctx, scopeID)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		return []string{name}, nil
	default:
		probe, ok := s.probes[scopeID]
		if !ok {
			return nil, errRevisionsProbeNotFound
		}
		return append([]string(nil), probe.Sources...), nil
	}
}

// revisionResolutionFromParam maps the URL-level enum onto the
// storage-side constant. Empty / unknown values fall through to
// `snapshot` (the synchronic default).
func revisionResolutionFromParam(p *GetRevisionActivityParamsResolution) storage.RevisionActivityResolution {
	if p == nil {
		return storage.RevisionResolutionSnapshot
	}
	switch *p {
	case GetRevisionActivityParamsResolutionDaily:
		return storage.RevisionResolutionDaily
	case GetRevisionActivityParamsResolutionWeekly:
		return storage.RevisionResolutionWeekly
	case GetRevisionActivityParamsResolutionMonthly:
		return storage.RevisionResolutionMonthly
	default:
		return storage.RevisionResolutionSnapshot
	}
}

func revisionResolutionToResponse(r storage.RevisionActivityResolution) RevisionActivityResponseResolution {
	switch r {
	case storage.RevisionResolutionDaily:
		return RevisionActivityResponseResolutionDaily
	case storage.RevisionResolutionWeekly:
		return RevisionActivityResponseResolutionWeekly
	case storage.RevisionResolutionMonthly:
		return RevisionActivityResponseResolutionMonthly
	default:
		return RevisionActivityResponseResolutionSnapshot
	}
}

func revisionScopeToResponse(scope GetRevisionActivityParamsScope) RevisionActivityResponseScope {
	if strings.EqualFold(string(scope), string(GetRevisionActivityParamsScopeSource)) {
		return RevisionActivityResponseScopeSource
	}
	return RevisionActivityResponseScopeProbe
}

// articleRevisionsEligibilityRefusal builds the 403 refusal payload
// returned when the article's source is not Silver-eligible. Mirrors
// the shape used by the existing Silver document detail refusal
// (silver_handlers.go) so the dashboard's RefusalSurface renders it
// without a per-endpoint code path.
func articleRevisionsEligibilityRefusal() GetArticleRevisions403JSONResponse {
	anchor := silverEligibilityAnchor
	return GetArticleRevisions403JSONResponse{
		Gate:               SilverEligibility,
		Message:            silverEligibilityRefusalMessage,
		WorkingPaperAnchor: &anchor,
	}
}

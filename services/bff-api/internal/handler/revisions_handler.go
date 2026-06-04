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
		DiffStatus         ArticleRevisionsResponseRevisionsDiffStatus `json:"diffStatus"`
		PrevContentHash    *string                                     `json:"prevContentHash,omitempty"`
		RevisionIndex      int                                         `json:"revisionIndex"`
		SnapshotAt         time.Time                                   `json:"snapshotAt"`
		TimeSincePrevHours *float64                                    `json:"timeSincePrevHours,omitempty"`
		Trigger            ArticleRevisionsResponseRevisionsTrigger    `json:"trigger"`
	}, 0, len(rows))
	for _, r := range rows {
		entry := struct {
			ArchiveUrl         *string                                     `json:"archiveUrl,omitempty"`
			ContentHash        string                                      `json:"contentHash"`
			DiffStatus         ArticleRevisionsResponseRevisionsDiffStatus `json:"diffStatus"`
			PrevContentHash    *string                                     `json:"prevContentHash,omitempty"`
			RevisionIndex      int                                         `json:"revisionIndex"`
			SnapshotAt         time.Time                                   `json:"snapshotAt"`
			TimeSincePrevHours *float64                                    `json:"timeSincePrevHours,omitempty"`
			Trigger            ArticleRevisionsResponseRevisionsTrigger    `json:"trigger"`
		}{
			ContentHash:   r.ContentHash,
			DiffStatus:    ArticleRevisionsResponseRevisionsDiffStatus(r.DiffStatus),
			RevisionIndex: int(r.RevisionIndex), //nolint:gosec // bounded
			SnapshotAt:    r.SnapshotAt,
			Trigger:       ArticleRevisionsResponseRevisionsTrigger(r.Trigger),
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

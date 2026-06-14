package handler

import (
	"context"
	"strings"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

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

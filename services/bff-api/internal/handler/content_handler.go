package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// GetContent handles GET /content/{entityType}/{entityId} — returns Dual-Register content
// for an entity. Locale defaults to "en".
func (s *Server) GetContent(_ context.Context, request GetContentRequestObject) (GetContentResponseObject, error) {
	if !request.EntityType.Valid() {
		return GetContent400JSONResponse{Message: "invalid entityType; must be one of metric, field, probe, source, discourse_function, refusal, view_mode, empty_lane, open_research_question, primer"}, nil
	}

	locale := string(GetContentParamsLocaleEn)
	if request.Params.Locale != nil {
		locale = string(*request.Params.Locale)
	}

	key := config.CatalogKey(locale, string(request.EntityType), request.EntityID)
	record, ok := s.catalog[key]
	if !ok && locale != string(GetContentParamsLocaleEn) {
		// Phase 144 (ADR-041) — EN-fallback: a non-base locale that lacks this
		// entry serves the English content rather than 404'ing, so a single
		// missing translation degrades to English instead of an empty surface.
		// Logged at WARN so the gap is visible; the response Locale reflects the
		// English content actually served (honest, never a relabelled lie). A CI
		// parity gate (configs/content/en ⇄ de) keeps this path rare.
		fallbackKey := config.CatalogKey(string(GetContentParamsLocaleEn), string(request.EntityType), request.EntityID)
		if fb, fbOK := s.catalog[fallbackKey]; fbOK {
			slog.Warn("content locale fallback to en",
				"op", "GetContent", "requestedLocale", locale,
				"entityType", request.EntityType, "entityID", request.EntityID)
			record, ok = fb, true
		}
	}
	if !ok {
		return GetContent404JSONResponse{Message: "no content found for the requested entity and locale"}, nil
	}

	date, err := time.Parse("2006-01-02", record.LastReviewedDate)
	if err != nil {
		slog.Error("handler failure", "op", "GetContent", "error", "invalid date in content record", "key", key)
		return GetContent500JSONResponse{Message: genericInternalError}, nil
	}

	var resp GetContent200JSONResponse
	resp.EntityID = record.EntityID
	resp.EntityType = ContentResponseEntityType(record.EntityType)
	resp.Locale = ContentResponseLocale(record.Locale)
	resp.Registers.Semantic.Short = record.Registers.Semantic.Short
	resp.Registers.Semantic.Long = record.Registers.Semantic.Long
	resp.Registers.Methodological.Short = record.Registers.Methodological.Short
	resp.Registers.Methodological.Long = record.Registers.Methodological.Long
	resp.ContentVersion = record.ContentVersion
	resp.LastReviewedBy = record.LastReviewedBy
	resp.LastReviewedDate = openapi_types.Date{Time: date}
	if len(record.WorkingPaperAnchors) > 0 {
		anchors := make([]string, len(record.WorkingPaperAnchors))
		copy(anchors, record.WorkingPaperAnchors)
		resp.WorkingPaperAnchors = &anchors
	}
	return resp, nil
}

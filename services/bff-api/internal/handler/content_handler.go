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
		return GetContent400JSONResponse{Message: "invalid entityType; must be one of metric, probe, discourse_function, refusal"}, nil
	}

	locale := string(GetContentParamsLocaleEn)
	if request.Params.Locale != nil {
		locale = string(*request.Params.Locale)
	}

	key := config.CatalogKey(locale, string(request.EntityType), request.EntityID)
	record, ok := s.catalog[key]
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

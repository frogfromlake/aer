package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// silverEligibilityAnchor is the canonical pointer into the methodological
// surface for the eligibility gate. Returned in every refusal payload so
// the frontend can deep-link to the WP-006 §5.2 explainer.
const silverEligibilityAnchor = "WP-006#section-5.2"

// silverEligibilityRefusalMessage is the human-readable message attached to
// every Silver-eligibility refusal. It is intentionally non-actionable from
// the API surface — eligibility is granted by an out-of-band review, not by
// a request parameter.
const silverEligibilityRefusalMessage = "source is not approved for Silver-layer access; eligibility is granted via the WP-006 §5.2 review process"

// requireSilverEligible resolves the source identifier and gates on the
// `silver_eligible` flag. Returns the resolved row on success; on the
// not-found / not-eligible paths it returns nil plus a typed error the
// caller maps to the appropriate HTTP shape.
//
// The caller-supplied closure pattern from elsewhere in this package
// would hide the typed-response generation; doing the eligibility check
// inline keeps each Silver handler explicit about the refusal payload it
// emits (the strict-server response types are per-endpoint).
func (s *Server) requireSilverEligible(ctx context.Context, identifier string) (*storage.SourceEligibilityRow, error) {
	if s.dossier == nil {
		return nil, errSilverNotConfigured
	}
	row, err := s.dossier.ResolveSourceWithEligibility(ctx, identifier)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return nil, errSilverSourceNotFound
		}
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	if !row.SilverEligible {
		return nil, errSilverNotEligible
	}
	return row, nil
}

// Sentinel errors used by the eligibility gate.
var (
	errSilverNotConfigured  = errors.New("silver endpoint not configured")
	errSilverSourceNotFound = errors.New("source not found")
	errSilverNotEligible    = errors.New("source not silver eligible")
)

// GetSourceById handles GET /sources/{id} — returns the source detail
// with Silver-eligibility metadata. No eligibility gate here: the
// endpoint exists precisely so reviewers/operators can see *whether* a
// source is eligible and on what grounds.
func (s *Server) GetSourceById(ctx context.Context, request GetSourceByIdRequestObject) (GetSourceByIdResponseObject, error) {
	if s.dossier == nil {
		slog.Error("handler failure", "op", "GetSourceById", "error", "dossier store not configured")
		return GetSourceById500JSONResponse{Message: genericInternalError}, nil
	}
	row, err := s.dossier.ResolveSourceWithEligibility(ctx, request.Id)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetSourceById404JSONResponse{Message: "source not found"}, nil
		}
		slog.Error("handler failure", "op", "GetSourceById", "error", err)
		return GetSourceById500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetSourceById200JSONResponse{
		Name:           row.Name,
		Type:           row.Type,
		SilverEligible: row.SilverEligible,
	}
	if row.URL.Valid {
		v := row.URL.String
		resp.Url = &v
	}
	if row.DocumentationURL.Valid {
		v := row.DocumentationURL.String
		resp.DocumentationUrl = &v
	}
	if row.SilverReviewReviewer.Valid {
		v := row.SilverReviewReviewer.String
		resp.SilverReviewReviewer = &v
	}
	if row.SilverReviewDate.Valid {
		d := openapi_types.Date{Time: row.SilverReviewDate.Time}
		resp.SilverReviewDate = &d
	}
	if row.SilverReviewRationale.Valid {
		v := row.SilverReviewRationale.String
		resp.SilverReviewRationale = &v
	}
	if row.SilverReviewReference.Valid {
		v := row.SilverReviewReference.String
		resp.SilverReviewReference = &v
	}
	return resp, nil
}

// ListSilverDocuments handles GET /silver/documents — paginated Silver
// document summaries for one source within an optional time window.
// Subject to the eligibility gate: a non-eligible source returns 403 +
// RefusalPayload.
//
// The summary rows are sourced from `aer_gold.metrics` via the existing
// ArticleQuerier (one ClickHouse round-trip per page) so the list path
// does not require a per-document MinIO read. `cleanedTextLength` and
// the SilverMeta blob are returned only by the detail endpoint.
func (s *Server) ListSilverDocuments(ctx context.Context, request ListSilverDocumentsRequestObject) (ListSilverDocumentsResponseObject, error) {
	if s.dossier == nil || s.articles == nil {
		slog.Error("handler failure", "op", "ListSilverDocuments", "error", "dossier or articles store not configured")
		return ListSilverDocuments500JSONResponse{Message: genericInternalError}, nil
	}

	row, err := s.requireSilverEligible(ctx, request.Params.SourceId)
	if response, ok := mapSilverGateError(err); ok {
		switch r := response.(type) {
		case ListSilverDocuments404JSONResponse:
			return r, nil
		case ListSilverDocuments403JSONResponse:
			return r, nil
		case ListSilverDocuments500JSONResponse:
			return r, nil
		}
	}
	if err != nil && !errors.Is(err, errSilverSourceNotFound) && !errors.Is(err, errSilverNotEligible) && !errors.Is(err, errSilverNotConfigured) {
		slog.Error("handler failure", "op", "ListSilverDocuments.requireSilverEligible", "error", err)
		return ListSilverDocuments500JSONResponse{Message: genericInternalError}, nil
	}

	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
		if limit < 1 || limit > 200 {
			return ListSilverDocuments400JSONResponse{Message: "limit must be between 1 and 200"}, nil
		}
	}
	offset := 0
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		o, err := decodeCursor(*request.Params.Cursor)
		if err != nil {
			return ListSilverDocuments400JSONResponse{Message: "invalid cursor"}, nil
		}
		offset = o
	}

	filter := storage.ArticleQueryFilter{
		Start:  request.Params.Start,
		End:    request.Params.End,
		Limit:  limit + 1, // fetch one extra to detect hasMore
		Offset: offset,
	}

	rows, err := s.articles.GetSourceArticles(ctx, row.Name, filter)
	if err != nil {
		slog.Error("handler failure", "op", "ListSilverDocuments.GetSourceArticles", "error", err)
		return ListSilverDocuments500JSONResponse{Message: genericInternalError}, nil
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	page := ListSilverDocuments200JSONResponse{
		Source:  row.Name,
		HasMore: hasMore,
	}
	if hasMore {
		next := encodeCursor(offset + limit)
		page.NextCursor = &next
	}
	for _, r := range rows {
		item := struct {
			ArticleId string     `json:"articleId"`
			Language  *string    `json:"language,omitempty"`
			Source    string     `json:"source"`
			Timestamp time.Time  `json:"timestamp"`
			WordCount *int       `json:"wordCount,omitempty"`
		}{
			ArticleId: r.ArticleID,
			Source:    r.Source,
			Timestamp: r.Timestamp,
		}
		if r.HasLanguage {
			lang := r.Language
			item.Language = &lang
		}
		if r.HasWordCount {
			wc := int(r.WordCount)
			item.WordCount = &wc
		}
		page.Items = append(page.Items, item)
	}
	if page.Items == nil {
		page.Items = []struct {
			ArticleId string     `json:"articleId"`
			Language  *string    `json:"language,omitempty"`
			Source    string     `json:"source"`
			Timestamp time.Time  `json:"timestamp"`
			WordCount *int       `json:"wordCount,omitempty"`
		}{}
	}
	return page, nil
}

// GetSilverDocumentDetail handles GET /silver/documents/{id} — full
// SilverEnvelope for a single article. Resolves article_id → bronze
// object key + source name via Postgres, gates on the source's
// `silver_eligible` flag, then fetches the envelope from MinIO.
func (s *Server) GetSilverDocumentDetail(ctx context.Context, request GetSilverDocumentDetailRequestObject) (GetSilverDocumentDetailResponseObject, error) {
	if s.dossier == nil || s.silver == nil {
		slog.Error("handler failure", "op", "GetSilverDocumentDetail", "error", "dossier or silver store not configured")
		return GetSilverDocumentDetail500JSONResponse{Message: genericInternalError}, nil
	}

	res, err := s.dossier.ResolveArticle(ctx, request.Id)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetSilverDocumentDetail404JSONResponse{Message: "article not found"}, nil
		}
		slog.Error("handler failure", "op", "GetSilverDocumentDetail.ResolveArticle", "error", err)
		return GetSilverDocumentDetail500JSONResponse{Message: genericInternalError}, nil
	}

	_, err = s.requireSilverEligible(ctx, res.SourceName)
	if err != nil {
		switch {
		case errors.Is(err, errSilverNotEligible):
			return silverDetailRefusal(), nil
		case errors.Is(err, errSilverSourceNotFound):
			return GetSilverDocumentDetail404JSONResponse{Message: "source not found"}, nil
		case errors.Is(err, errSilverNotConfigured):
			return GetSilverDocumentDetail500JSONResponse{Message: genericInternalError}, nil
		}
		slog.Error("handler failure", "op", "GetSilverDocumentDetail.requireSilverEligible", "error", err)
		return GetSilverDocumentDetail500JSONResponse{Message: genericInternalError}, nil
	}

	envelope, err := s.silver.GetEnvelope(ctx, res.BronzeObjectKey)
	if err != nil {
		if errors.Is(err, storage.ErrSilverNotFound) {
			return GetSilverDocumentDetail404JSONResponse{Message: "silver object not found"}, nil
		}
		slog.Error("handler failure", "op", "GetSilverDocumentDetail.GetEnvelope", "error", err)
		return GetSilverDocumentDetail500JSONResponse{Message: genericInternalError}, nil
	}

	timestamp, err := time.Parse(time.RFC3339, envelope.Core.Timestamp)
	if err != nil {
		timestamp = time.Time{}
	}

	resp := GetSilverDocumentDetail200JSONResponse{
		ArticleId:     envelope.Core.DocumentID,
		Source:        envelope.Core.Source,
		Timestamp:     timestamp,
		CleanedText:   envelope.Core.CleanedText,
		SchemaVersion: envelope.Core.SchemaVersion,
		WordCount:     envelope.Core.WordCount,
	}
	if envelope.Core.SourceType != "" {
		st := envelope.Core.SourceType
		resp.SourceType = &st
	}
	if envelope.Core.URL != "" {
		u := envelope.Core.URL
		resp.Url = &u
	}
	if envelope.Core.Language != "" {
		l := envelope.Core.Language
		resp.Language = &l
	}
	if envelope.Core.RawText != "" {
		rt := envelope.Core.RawText
		resp.RawText = &rt
	}
	if len(envelope.Meta) > 0 {
		m := envelope.Meta
		resp.Meta = &m
	}
	if len(envelope.ExtractionProvenance) > 0 {
		p := envelope.ExtractionProvenance
		resp.ExtractionProvenance = &p
	}
	return resp, nil
}

// silverListRefusal builds the canonical 403 RefusalPayload for the list endpoint.
func silverListRefusal() ListSilverDocuments403JSONResponse {
	anchor := silverEligibilityAnchor
	return ListSilverDocuments403JSONResponse(RefusalPayload{
		Gate:               SilverEligibility,
		Message:            silverEligibilityRefusalMessage,
		WorkingPaperAnchor: &anchor,
	})
}

// silverDetailRefusal builds the canonical 403 RefusalPayload for the detail endpoint.
func silverDetailRefusal() GetSilverDocumentDetail403JSONResponse {
	anchor := silverEligibilityAnchor
	return GetSilverDocumentDetail403JSONResponse(RefusalPayload{
		Gate:               SilverEligibility,
		Message:            silverEligibilityRefusalMessage,
		WorkingPaperAnchor: &anchor,
	})
}

// mapSilverGateError converts a sentinel returned by requireSilverEligible
// into the per-endpoint typed response. Returns (nil, false) when the
// caller should fall through to its happy path.
//
// This is a small dispatch shim — Go's type system forces per-endpoint
// response types because the strict-server interface defines distinct 403
// shapes for each operation.
func mapSilverGateError(err error) (any, bool) {
	if err == nil {
		return nil, false
	}
	// list-endpoint mapping (caller selects via type assertion).
	switch {
	case errors.Is(err, errSilverNotConfigured):
		return ListSilverDocuments500JSONResponse{Message: genericInternalError}, true
	case errors.Is(err, errSilverSourceNotFound):
		return ListSilverDocuments404JSONResponse{Message: "source not found"}, true
	case errors.Is(err, errSilverNotEligible):
		return silverListRefusal(), true
	}
	return nil, false
}

// Lightweight helper kept for symmetry with strings.HasPrefix usage in
// view_mode_handlers — currently unused but reserved if the eligibility
// reason set grows.
var _ = strings.HasPrefix

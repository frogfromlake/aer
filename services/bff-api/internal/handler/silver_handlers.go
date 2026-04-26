package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
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
		SchemaVersion: strconv.Itoa(envelope.Core.SchemaVersion),
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

// silverAggregationRefusal builds the canonical 403 RefusalPayload for the
// aggregation endpoint. Uses the same eligibility anchor as the document
// endpoints so the frontend can route every Silver refusal through one
// methodological-explainer link.
func silverAggregationRefusal() GetSilverAggregation403JSONResponse {
	anchor := silverEligibilityAnchor
	return GetSilverAggregation403JSONResponse(RefusalPayload{
		Gate:               SilverEligibility,
		Message:            silverEligibilityRefusalMessage,
		WorkingPaperAnchor: &anchor,
	})
}

// GetSilverAggregation handles GET /silver/aggregations/{aggregationType} —
// distribution / heatmap / correlation queries over the
// `aer_silver.documents` projection table for one Silver-eligible source.
func (s *Server) GetSilverAggregation(ctx context.Context, request GetSilverAggregationRequestObject) (GetSilverAggregationResponseObject, error) {
	if s.dossier == nil || s.db == nil {
		slog.Error("handler failure", "op", "GetSilverAggregation", "error", "dossier or db not configured")
		return GetSilverAggregation500JSONResponse{Message: genericInternalError}, nil
	}

	// The generated path-param type is enum-validated upstream, but be
	// defensive: an unknown kind here means a contract drift, not user
	// error, so 400 with a clear message is correct.
	kind := storage.SilverAggregationKind(request.AggregationType)
	if !storage.IsSilverDistributionKind(kind) && !storage.IsSilverHeatmapKind(kind) && !storage.IsSilverCorrelationKind(kind) {
		return GetSilverAggregation400JSONResponse{Message: fmt.Sprintf("unsupported aggregationType: %s", request.AggregationType)}, nil
	}

	if msg := validateWindow(request.Params.Start, request.Params.End); msg != "" {
		return GetSilverAggregation400JSONResponse{Message: msg}, nil
	}

	row, err := s.requireSilverEligible(ctx, request.Params.SourceId)
	if err != nil {
		switch {
		case errors.Is(err, errSilverNotEligible):
			return silverAggregationRefusal(), nil
		case errors.Is(err, errSilverSourceNotFound):
			return GetSilverAggregation404JSONResponse{Message: "source not found"}, nil
		case errors.Is(err, errSilverNotConfigured):
			return GetSilverAggregation500JSONResponse{Message: genericInternalError}, nil
		}
		slog.Error("handler failure", "op", "GetSilverAggregation.requireSilverEligible", "error", err)
		return GetSilverAggregation500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetSilverAggregation200JSONResponse{
		AggregationType: string(request.AggregationType),
		Source:          row.Name,
		WindowStart:     request.Params.Start,
		WindowEnd:       request.Params.End,
	}

	switch {
	case storage.IsSilverDistributionKind(kind):
		bins := 30
		if request.Params.Bins != nil {
			bins = *request.Params.Bins
		}
		field := string(kind)
		res, err := s.db.GetSilverDistribution(ctx, field, row.Name, request.Params.Start, request.Params.End, bins)
		if err != nil {
			slog.Error("handler failure", "op", "GetSilverAggregation.GetSilverDistribution", "error", err)
			return GetSilverAggregation500JSONResponse{Message: genericInternalError}, nil
		}
		dist := struct {
			Bins []struct {
				Count int64   `json:"count"`
				Lower float64 `json:"lower"`
				Upper float64 `json:"upper"`
			} `json:"bins"`
			Summary struct {
				Count  int64   `json:"count"`
				Max    float64 `json:"max"`
				Mean   float64 `json:"mean"`
				Median float64 `json:"median"`
				Min    float64 `json:"min"`
				P05    float64 `json:"p05"`
				P25    float64 `json:"p25"`
				P75    float64 `json:"p75"`
				P95    float64 `json:"p95"`
			} `json:"summary"`
		}{}
		dist.Bins = make([]struct {
			Count int64   `json:"count"`
			Lower float64 `json:"lower"`
			Upper float64 `json:"upper"`
		}, len(res.Bins))
		for i, b := range res.Bins {
			dist.Bins[i] = struct {
				Count int64   `json:"count"`
				Lower float64 `json:"lower"`
				Upper float64 `json:"upper"`
			}{Count: b.Count, Lower: b.Lower, Upper: b.Upper}
		}
		dist.Summary.Count = res.Summary.Count
		dist.Summary.Min = res.Summary.Min
		dist.Summary.Max = res.Summary.Max
		dist.Summary.Mean = res.Summary.Mean
		dist.Summary.Median = res.Summary.Median
		dist.Summary.P05 = res.Summary.P05
		dist.Summary.P25 = res.Summary.P25
		dist.Summary.P75 = res.Summary.P75
		dist.Summary.P95 = res.Summary.P95
		resp.Distribution = &dist
	case storage.IsSilverHeatmapKind(kind):
		cells, xDim, yDim, err := s.db.GetSilverHeatmap(ctx, kind, row.Name, request.Params.Start, request.Params.End)
		if err != nil {
			slog.Error("handler failure", "op", "GetSilverAggregation.GetSilverHeatmap", "error", err)
			return GetSilverAggregation500JSONResponse{Message: genericInternalError}, nil
		}
		hm := struct {
			Cells []struct {
				Count int64 `json:"count"`

				// Value Mean of the projection field across rows in this cell.
				Value float64 `json:"value"`
				X     string  `json:"x"`
				Y     string  `json:"y"`
			} `json:"cells"`
			XDimension string `json:"xDimension"`
			YDimension string `json:"yDimension"`
		}{XDimension: xDim, YDimension: yDim}
		hm.Cells = make([]struct {
			Count int64   `json:"count"`
			Value float64 `json:"value"`
			X     string  `json:"x"`
			Y     string  `json:"y"`
		}, len(cells))
		for i, c := range cells {
			hm.Cells[i] = struct {
				Count int64   `json:"count"`
				Value float64 `json:"value"`
				X     string  `json:"x"`
				Y     string  `json:"y"`
			}{Count: c.Count, Value: c.Value, X: c.X, Y: c.Y}
		}
		resp.Heatmap = &hm
	case storage.IsSilverCorrelationKind(kind):
		res, err := s.db.GetSilverCorrelation(ctx, row.Name, request.Params.Start, request.Params.End)
		if err != nil {
			slog.Error("handler failure", "op", "GetSilverAggregation.GetSilverCorrelation", "error", err)
			return GetSilverAggregation500JSONResponse{Message: genericInternalError}, nil
		}
		resp.Correlation = &struct {
			Fields      []string     `json:"fields"`
			Matrix      [][]*float64 `json:"matrix"`
			SampleCount int64        `json:"sampleCount"`
		}{
			Fields:      res.Fields,
			Matrix:      res.Matrix,
			SampleCount: res.SampleCount,
		}
	}

	return resp, nil
}

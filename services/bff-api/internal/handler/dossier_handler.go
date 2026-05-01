package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// DossierStore abstracts the Postgres queries used by the Probe Dossier
// and article endpoints. A nil value disables those endpoints (tests that
// do not exercise them inject nil and skip the routes).
type DossierStore interface {
	FetchSources(ctx context.Context, sourceNames []string, windowStart, windowEnd *time.Time) ([]storage.DossierSourceRow, error)
	ResolveSource(ctx context.Context, identifier string) (int64, string, error)
	ResolveArticle(ctx context.Context, articleID string) (*storage.ArticleResolution, error)
	// Phase 103: ResolveSourceWithEligibility returns the eligibility tuple
	// used by both the source-detail endpoint and the Silver-eligibility gate.
	ResolveSourceWithEligibility(ctx context.Context, identifier string) (*storage.SourceEligibilityRow, error)
}

// ArticleQuerier abstracts the ClickHouse-side article queries.
type ArticleQuerier interface {
	GetSourceArticles(ctx context.Context, sourceName string, f storage.ArticleQueryFilter) ([]storage.ArticleAggRow, error)
	CountAggregationGroup(ctx context.Context, sourceName, metricName string, articleTimestamp time.Time) (int, error)
	GetArticleProvenance(ctx context.Context, articleID string) (map[string]string, error)
}

// SilverFetcher abstracts the MinIO Silver read used for L5 Evidence.
type SilverFetcher interface {
	GetEnvelope(ctx context.Context, objectKey string) (*storage.SilverEnvelope, error)
}

// GetProbeDossier handles GET /probes/{id}/dossier.
func (s *Server) GetProbeDossier(ctx context.Context, request GetProbeDossierRequestObject) (GetProbeDossierResponseObject, error) {
	if s.dossier == nil {
		return GetProbeDossier500JSONResponse{Message: genericInternalError}, nil
	}

	probe, ok := s.probes[request.Id]
	if !ok {
		return GetProbeDossier404JSONResponse{Message: "probe not found"}, nil
	}

	winStart, winEnd, err := normaliseWindow(request.Params.WindowStart, request.Params.WindowEnd)
	if err != nil {
		return GetProbeDossier400JSONResponse{Message: err.Error()}, nil
	}

	rows, err := s.dossier.FetchSources(ctx, probe.Sources, winStart, winEnd)
	if err != nil {
		slog.Error("handler failure", "op", "GetProbeDossier.FetchSources", "error", err)
		return GetProbeDossier500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetProbeDossier200JSONResponse{
		ProbeId:  probe.ProbeID,
		Language: probe.Language,
	}
	if winStart != nil {
		t := *winStart
		resp.WindowStart = &t
	}
	if winEnd != nil {
		t := *winEnd
		resp.WindowEnd = &t
	}

	functionsSet := map[string]struct{}{}
	for _, r := range rows {
		card := struct {
			ArticlesInWindow           int                                   `json:"articlesInWindow"`
			ArticlesTotal              int                                   `json:"articlesTotal"`
			DocumentationUrl           *string                               `json:"documentationUrl,omitempty"`
			EmicContext                *string                               `json:"emicContext,omitempty"`
			EmicDesignation            *string                               `json:"emicDesignation,omitempty"`
			Name                       string                                `json:"name"`
			PrimaryFunction            *ProbeDossierSourcesPrimaryFunction   `json:"primaryFunction,omitempty"`
			PublicationFrequencyPerDay *float32                              `json:"publicationFrequencyPerDay,omitempty"`
			SecondaryFunction          *ProbeDossierSourcesSecondaryFunction `json:"secondaryFunction,omitempty"`
			SilverEligible             bool                                  `json:"silverEligible"`
			SilverReviewDate           *openapi_types.Date                   `json:"silverReviewDate,omitempty"`
			Type                       string                                `json:"type"`
			Url                        *string                               `json:"url,omitempty"`
		}{
			Name:             r.Name,
			Type:             r.Type,
			ArticlesTotal:    int(r.ArticlesTotal),
			ArticlesInWindow: int(r.ArticlesInWindow),
			SilverEligible:   r.SilverEligible,
		}
		if r.URL.Valid {
			v := r.URL.String
			card.Url = &v
		}
		if r.DocumentationURL.Valid {
			v := r.DocumentationURL.String
			card.DocumentationUrl = &v
		}
		if r.PublicationFreqPerDay.Valid {
			v := float32(r.PublicationFreqPerDay.Float64)
			card.PublicationFrequencyPerDay = &v
		}
		if r.PrimaryFunction.Valid {
			f := ProbeDossierSourcesPrimaryFunction(r.PrimaryFunction.String)
			card.PrimaryFunction = &f
			functionsSet[r.PrimaryFunction.String] = struct{}{}
		}
		if r.SecondaryFunction.Valid {
			f := ProbeDossierSourcesSecondaryFunction(r.SecondaryFunction.String)
			card.SecondaryFunction = &f
		}
		if r.EmicDesignation.Valid {
			v := r.EmicDesignation.String
			card.EmicDesignation = &v
		}
		if r.EmicContext.Valid {
			v := r.EmicContext.String
			card.EmicContext = &v
		}
		if r.SilverReviewDate.Valid {
			d := openapi_types.Date{Time: r.SilverReviewDate.Time}
			card.SilverReviewDate = &d
		}
		resp.Sources = append(resp.Sources, card)
	}

	resp.FunctionCoverage.Total = 4
	resp.FunctionCoverage.Covered = len(functionsSet)
	for f := range functionsSet {
		resp.FunctionCoverage.Functions = append(resp.FunctionCoverage.Functions, ProbeDossierFunctionCoverageFunctions(f))
	}

	return resp, nil
}

// GetSourceArticles handles GET /sources/{id}/articles.
func (s *Server) GetSourceArticles(ctx context.Context, request GetSourceArticlesRequestObject) (GetSourceArticlesResponseObject, error) {
	if s.dossier == nil || s.articles == nil {
		return GetSourceArticles500JSONResponse{Message: genericInternalError}, nil
	}

	_, sourceName, err := s.dossier.ResolveSource(ctx, request.Id)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetSourceArticles404JSONResponse{Message: "source not found"}, nil
		}
		slog.Error("handler failure", "op", "GetSourceArticles.ResolveSource", "error", err)
		return GetSourceArticles500JSONResponse{Message: genericInternalError}, nil
	}

	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
		if limit < 1 || limit > 200 {
			return GetSourceArticles400JSONResponse{Message: "limit must be between 1 and 200"}, nil
		}
	}

	offset := 0
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		o, err := decodeCursor(*request.Params.Cursor)
		if err != nil {
			return GetSourceArticles400JSONResponse{Message: "invalid cursor"}, nil
		}
		offset = o
	}

	if request.Params.SentimentBand != nil && !request.Params.SentimentBand.Valid() {
		return GetSourceArticles400JSONResponse{Message: "invalid sentimentBand"}, nil
	}

	filter := storage.ArticleQueryFilter{
		Start:       request.Params.Start,
		End:         request.Params.End,
		Language:    request.Params.Language,
		EntityMatch: request.Params.EntityMatch,
		Limit:       limit + 1, // fetch one extra to detect hasMore
		Offset:      offset,
	}
	if request.Params.SentimentBand != nil {
		band := string(*request.Params.SentimentBand)
		filter.SentimentBand = &band
	}

	rows, err := s.articles.GetSourceArticles(ctx, sourceName, filter)
	if err != nil {
		slog.Error("handler failure", "op", "GetSourceArticles", "error", err)
		return GetSourceArticles500JSONResponse{Message: genericInternalError}, nil
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	page := GetSourceArticles200JSONResponse{HasMore: hasMore}
	if hasMore {
		next := encodeCursor(offset + limit)
		page.NextCursor = &next
	}
	for _, r := range rows {
		item := struct {
			ArticleId      string    `json:"articleId"`
			Language       *string   `json:"language,omitempty"`
			SentimentScore *float32  `json:"sentimentScore,omitempty"`
			Source         string    `json:"source"`
			Timestamp      time.Time `json:"timestamp"`
			WordCount      *int      `json:"wordCount,omitempty"`
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
		if r.HasSentiment {
			s32 := float32(r.SentimentScore)
			item.SentimentScore = &s32
		}
		page.Items = append(page.Items, item)
	}
	if page.Items == nil {
		page.Items = []struct {
			ArticleId      string    `json:"articleId"`
			Language       *string   `json:"language,omitempty"`
			SentimentScore *float32  `json:"sentimentScore,omitempty"`
			Source         string    `json:"source"`
			Timestamp      time.Time `json:"timestamp"`
			WordCount      *int      `json:"wordCount,omitempty"`
		}{}
	}
	return page, nil
}

// GetArticleDetail handles GET /articles/{id} — L5 Evidence with k-anonymity gate.
func (s *Server) GetArticleDetail(ctx context.Context, request GetArticleDetailRequestObject) (GetArticleDetailResponseObject, error) {
	if s.dossier == nil || s.articles == nil || s.silver == nil {
		return GetArticleDetail404JSONResponse{Message: "article-detail endpoint not configured"}, nil
	}

	res, err := s.dossier.ResolveArticle(ctx, request.Id)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetArticleDetail404JSONResponse{Message: "article not found"}, nil
		}
		slog.Error("handler failure", "op", "GetArticleDetail.ResolveArticle", "error", err)
		return nil, err //nolint:wrapcheck // surfaces as 500 via strict-server
	}

	envelope, err := s.silver.GetEnvelope(ctx, res.BronzeObjectKey)
	if err != nil {
		if errors.Is(err, storage.ErrSilverNotFound) {
			return GetArticleDetail404JSONResponse{Message: "article silver object not found"}, nil
		}
		slog.Error("handler failure", "op", "GetArticleDetail.GetEnvelope", "error", err)
		return nil, err //nolint:wrapcheck
	}

	timestamp, err := time.Parse(time.RFC3339, envelope.Core.Timestamp)
	if err != nil {
		// Fall back to the SilverCore timestamp as-typed; the worker writes
		// RFC3339 today, but a divergent format should not 500 the request.
		timestamp = time.Time{}
	}

	metricName := "word_count"
	if request.Params.MetricName != nil && *request.Params.MetricName != "" {
		metricName = *request.Params.MetricName
	}
	// Phase 117 read-side alias.
	metricName = canonicalMetricName(metricName)
	count, err := s.articles.CountAggregationGroup(ctx, res.SourceName, metricName, timestamp)
	if err != nil {
		slog.Error("handler failure", "op", "GetArticleDetail.CountAggregationGroup", "error", err)
		return nil, err //nolint:wrapcheck
	}
	if count < s.kAnonymityThreshold {
		threshold := s.kAnonymityThreshold
		observed := count
		anchor := "WP-006#section-7"
		return GetArticleDetail403JSONResponse{
			Gate:                KAnonymity,
			Message:             fmt.Sprintf("article aggregation group has %d documents on this date for metric %q; minimum required is %d", count, metricName, threshold),
			Threshold:           &threshold,
			Observed:            &observed,
			WorkingPaperAnchor:  &anchor,
		}, nil
	}

	provenance, err := s.articles.GetArticleProvenance(ctx, request.Id)
	if err != nil {
		slog.Warn("article provenance lookup failed; continuing", "error", err)
		provenance = map[string]string{}
	}
	// Prefer the SilverEnvelope's recorded provenance (per-extractor versions
	// captured at extraction time) over an empty BFF stub.
	if len(envelope.ExtractionProvenance) > 0 {
		provenance = envelope.ExtractionProvenance
	}

	resp := GetArticleDetail200JSONResponse{
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
	if len(provenance) > 0 {
		p := provenance
		resp.ExtractionProvenance = &p
	}
	return resp, nil
}

// normaliseWindow validates a (start, end) pair. Both must be present
// together or both absent; if present, start must be <= end.
func normaliseWindow(start, end *time.Time) (*time.Time, *time.Time, error) {
	if start == nil && end == nil {
		return nil, nil, nil
	}
	if start == nil || end == nil {
		return nil, nil, errors.New("windowStart and windowEnd must be supplied together")
	}
	if end.Before(*start) {
		return nil, nil, errors.New("windowEnd must be >= windowStart")
	}
	return start, end, nil
}

// encodeCursor wraps an integer offset in an opaque base64 token. The
// shape is intentionally minimal — pagination is offset-based at the
// storage layer, but the cursor is opaque so we can swap in a richer
// scheme (deterministic article-id sort key, e.g.) later without an API
// break.
func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o=" + strconv.Itoa(offset)))
}

func decodeCursor(token string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err //nolint:wrapcheck
	}
	s := string(raw)
	if !strings.HasPrefix(s, "o=") {
		return 0, errors.New("malformed cursor")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "o="))
	if err != nil {
		return 0, err //nolint:wrapcheck
	}
	if n < 0 {
		return 0, errors.New("negative offset")
	}
	return n, nil
}


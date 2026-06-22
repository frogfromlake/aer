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

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
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
	// Phase 122g / ADR-031: per-source discovery-coverage telemetry over
	// the trailing window. Backs `GET /sources/{id}/discovery-coverage`.
	GetDiscoveryCoverage(ctx context.Context, sourceID int64, sourceName string, windowDays int) (*storage.DiscoveryCoverageSummary, error)
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
	// GetBronzeRawHTML sources the raw HTML from Bronze on-demand (Phase 148c —
	// no longer duplicated into Silver); ErrBronzeNotFound past the 90-day TTL.
	GetBronzeRawHTML(ctx context.Context, objectKey string) (string, error)
}

// GetProbeDossier handles GET /probes/{id}/dossier.
func (s *Server) GetProbeDossier(ctx context.Context, request GetProbeDossierRequestObject) (GetProbeDossierResponseObject, error) {
	if s.dossier == nil {
		return GetProbeDossier500JSONResponse{Message: genericInternalError}, nil
	}

	probe, ok := s.probes[request.ID]
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
		ProbeID:     probe.ProbeID,
		DisplayName: probe.Display(),
		ShortName:   probe.Short(),
		Language:    probe.Language,
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
			ArticlesInWindow              int                                   `json:"articlesInWindow"`
			ArticlesTotal                 int                                   `json:"articlesTotal"`
			DocumentationURL              *string                               `json:"documentationUrl,omitempty"`
			EmicContext                   *string                               `json:"emicContext,omitempty"`
			EmicDesignation               *string                               `json:"emicDesignation,omitempty"`
			Name                          string                                `json:"name"`
			PrimaryFunction               *ProbeDossierSourcesPrimaryFunction   `json:"primaryFunction,omitempty"`
			PublicationFrequencyPerDay    *float32                              `json:"publicationFrequencyPerDay,omitempty"`
			SecondaryFunction             *ProbeDossierSourcesSecondaryFunction `json:"secondaryFunction,omitempty"`
			SilverEligible                bool                                  `json:"silverEligible"`
			SilverReviewDate              *openapi_types.Date                   `json:"silverReviewDate,omitempty"`
			TemporalProvenanceAbsentCount *int                                  `json:"temporalProvenanceAbsentCount,omitempty"`
			Type                          string                                `json:"type"`
			URL                           *string                               `json:"url,omitempty"`
		}{
			Name:             r.Name,
			Type:             r.Type,
			ArticlesTotal:    int(r.ArticlesTotal),
			ArticlesInWindow: int(r.ArticlesInWindow),
			SilverEligible:   r.SilverEligible,
		}
		if r.URL.Valid {
			v := r.URL.String
			card.URL = &v
		}
		if r.DocumentationURL.Valid {
			v := r.DocumentationURL.String
			card.DocumentationURL = &v
		}
		if r.PublicationFreqPerDay.Valid {
			v := float32(r.PublicationFreqPerDay.Float64)
			card.PublicationFrequencyPerDay = &v
		}
		// Phase 122d.2 — per-source Temporal-Provenance-Absence count.
		tpa := int(r.TemporalProvenanceAbsent)
		card.TemporalProvenanceAbsentCount = &tpa
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

	// Phase 123a — capability matrix. Describes what AĒR can compute for
	// this probe; asserts no results. Silent-edit observability (Phase 122d
	// Wayback CDX sidecar) is active when the probe has any web source.
	// Sentiment backbone/enrichments come from the Language Capability
	// Manifest, keyed by the probe's language. The per-article discourse-
	// function classifier is deferred (ADR-030) → source-level only.
	silentEdit := false
	for _, r := range rows {
		if r.Type == "web" {
			silentEdit = true
			break
		}
	}
	caps := struct {
		DiscourseFunctionClassifier string   `json:"discourseFunctionClassifier"`
		SentimentBackbone           *string  `json:"sentimentBackbone,omitempty"`
		SentimentEnrichments        []string `json:"sentimentEnrichments"`
		SilentEditObservability     bool     `json:"silentEditObservability"`
	}{
		DiscourseFunctionClassifier: "deferred / source-level only",
		SentimentEnrichments:        []string{},
		SilentEditObservability:     silentEdit,
	}
	if s.languageManifest != nil {
		if entry, ok := s.languageManifest.Languages[probe.Language]; ok {
			if bb := entry.SentimentBackbone(); bb != "" {
				b := bb
				caps.SentimentBackbone = &b
			}
			caps.SentimentEnrichments = entry.SentimentEnrichments()
		}
	}
	resp.Capabilities = &caps

	return resp, nil
}

// GetSourceArticles handles GET /sources/{id}/articles.
func (s *Server) GetSourceArticles(ctx context.Context, request GetSourceArticlesRequestObject) (GetSourceArticlesResponseObject, error) {
	if s.dossier == nil || s.articles == nil {
		return GetSourceArticles500JSONResponse{Message: genericInternalError}, nil
	}

	_, sourceName, err := s.dossier.ResolveSource(ctx, request.ID)
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
	if request.Params.IncludeRevisions != nil && *request.Params.IncludeRevisions {
		filter.IncludeRevisions = true
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
			ArticleID            string     `json:"articleId"`
			ChainLength          *int       `json:"chainLength,omitempty"`
			EditorialChangeCount *int       `json:"editorialChangeCount,omitempty"`
			HasHeadlineChange    *bool      `json:"hasHeadlineChange,omitempty"`
			Language             *string    `json:"language,omitempty"`
			LatestRevisionAt     *time.Time `json:"latestRevisionAt,omitempty"`
			SentimentScore       *float32   `json:"sentimentScore,omitempty"`
			Source               string     `json:"source"`
			Timestamp            time.Time  `json:"timestamp"`
			TimestampSource      *string    `json:"timestampSource,omitempty"`
			WordCount            *int       `json:"wordCount,omitempty"`
		}{
			ArticleID: r.ArticleID,
			Source:    r.Source,
			Timestamp: r.Timestamp,
		}
		if r.HasLanguage {
			lang := r.Language
			item.Language = &lang
		}
		// Phase 122d.2 — timestamp provenance (Temporal-Provenance-Absence NS-class).
		// Emitted only when non-empty; absent = legacy/non-web row.
		if r.TimestampSource != "" {
			ts := r.TimestampSource
			item.TimestampSource = &ts
		}
		if r.HasWordCount {
			wc := int(r.WordCount)
			item.WordCount = &wc
		}
		if r.HasSentiment {
			s32 := float32(r.SentimentScore)
			item.SentimentScore = &s32
		}
		if r.HasRevisions {
			cl := int(r.ChainLength) //nolint:gosec // bounded
			item.ChainLength = &cl
			ec := int(r.EditorialChangeCount) //nolint:gosec // bounded
			item.EditorialChangeCount = &ec
			h := r.HasHeadlineChange
			item.HasHeadlineChange = &h
			if !r.LatestRevisionAt.IsZero() {
				t := r.LatestRevisionAt
				item.LatestRevisionAt = &t
			}
		}
		page.Items = append(page.Items, item)
	}
	if page.Items == nil {
		page.Items = []struct {
			ArticleID            string     `json:"articleId"`
			ChainLength          *int       `json:"chainLength,omitempty"`
			EditorialChangeCount *int       `json:"editorialChangeCount,omitempty"`
			HasHeadlineChange    *bool      `json:"hasHeadlineChange,omitempty"`
			Language             *string    `json:"language,omitempty"`
			LatestRevisionAt     *time.Time `json:"latestRevisionAt,omitempty"`
			SentimentScore       *float32   `json:"sentimentScore,omitempty"`
			Source               string     `json:"source"`
			Timestamp            time.Time  `json:"timestamp"`
			TimestampSource      *string    `json:"timestampSource,omitempty"`
			WordCount            *int       `json:"wordCount,omitempty"`
		}{}
	}
	return page, nil
}

// GetArticleDetail handles GET /articles/{id} — L5 Evidence with k-anonymity gate.
func (s *Server) GetArticleDetail(ctx context.Context, request GetArticleDetailRequestObject) (GetArticleDetailResponseObject, error) {
	if s.dossier == nil || s.articles == nil || s.silver == nil {
		return GetArticleDetail404JSONResponse{Message: "article-detail endpoint not configured"}, nil
	}

	res, err := s.dossier.ResolveArticle(ctx, request.ID)
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
	// k-anonymity gate (WP-006 §7), scoped by corpus class (Phase 133). The
	// gate guards against re-identifying individuals — meaningful for social
	// / user-generated corpora, but NOT for PUBLIC institutional publishers
	// (government press offices, newsrooms) whose article text is already
	// public and whose "author" is an institution. For those the effective
	// threshold drops to 1, so an existing article is always readable; the
	// gate stays in full force for every other corpus class.
	threshold := s.kAnonymityThreshold
	if s.sourceIsPublicInstitutional(res.SourceName) {
		threshold = 1
	}
	if count < threshold {
		observed := count
		anchor := "WP-006#section-7"
		return GetArticleDetail403JSONResponse{
			Gate:               KAnonymity,
			Message:            fmt.Sprintf("article aggregation group has %d documents on this date for metric %q; minimum required is %d", count, metricName, threshold),
			Threshold:          &threshold,
			Observed:           &observed,
			WorkingPaperAnchor: &anchor,
		}, nil
	}

	provenance, err := s.articles.GetArticleProvenance(ctx, request.ID)
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
		ArticleID:     envelope.Core.DocumentID,
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
		resp.URL = &u
	}
	if envelope.Core.Language != "" {
		l := envelope.Core.Language
		resp.Language = &l
	}
	// Phase 148c — raw HTML is no longer stored in Silver; fetch it from Bronze
	// on-demand (Bronze + Silver share the object key). Best-effort: a missing
	// Bronze object (past the 90-day TTL) or any error just omits rawText.
	if raw, rerr := s.silver.GetBronzeRawHTML(ctx, res.BronzeObjectKey); rerr == nil && raw != "" {
		resp.RawText = &raw
	} else if rerr != nil && !errors.Is(rerr, storage.ErrBronzeNotFound) {
		slog.Warn("bronze raw fetch failed", "op", "dossier.GetBronzeRawHTML", "error", rerr)
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
//
// Each bound is INDEPENDENTLY optional. Both absent ⇒ (nil, nil) = whole
// dataset (no time filter). A single supplied bound opens the other side to the
// dataset extent (lower → wholeDatasetStart, upper → now) so the article-count
// window stays well-formed instead of 400-ing. Only an inverted window is
// rejected.
func normaliseWindow(start, end *time.Time) (*time.Time, *time.Time, error) {
	if start == nil && end == nil {
		return nil, nil, nil
	}
	s := start
	if s == nil {
		floor := wholeDatasetStart
		s = &floor
	}
	e := end
	if e == nil {
		now := time.Now().UTC()
		e = &now
	}
	if e.Before(*s) {
		return nil, nil, errors.New("windowEnd must be >= windowStart")
	}
	return s, e, nil
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

// sourceIsPublicInstitutional reports whether the source belongs to a probe
// whose corpus class is a public institutional publisher class exempt from
// the L5 k-anonymity gate (Phase 133). A source not matched to any probe is
// treated as NON-public — the gate stays enforced (fail safe).
func (s *Server) sourceIsPublicInstitutional(sourceName string) bool {
	for _, p := range s.probes {
		for _, src := range p.Sources {
			if src == sourceName && config.IsPublicCorpusClass(p.CorpusClass()) {
				return true
			}
		}
	}
	return false
}

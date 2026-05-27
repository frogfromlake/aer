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

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// GetArticleRevisionDiff handles GET /articles/{id}/revisions/{revisionIndex}/diff.
//
// Phase 122d.1: returns the paragraph-aligned diff for one snapshot
// pair. Silver-eligibility-gated like the rest of the per-article
// surface. Reuses the eligibility helper from the existing
// /articles/{id}/revisions handler; that means a non-eligible source's
// article surfaces with the same `silver_eligibility` refusal it does
// on the other per-article endpoints — consistent oracle posture
// (acknowledged in ADR-032 amendment).
func (s *Server) GetArticleRevisionDiff(
	ctx context.Context,
	request GetArticleRevisionDiffRequestObject,
) (GetArticleRevisionDiffResponseObject, error) {
	if s.dossier == nil {
		return GetArticleRevisionDiff500JSONResponse{Message: genericInternalError}, nil
	}

	res, err := s.dossier.ResolveArticle(ctx, request.Id)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetArticleRevisionDiff404JSONResponse{Message: "article not found"}, nil
		}
		slog.Error("handler failure", "op", "GetArticleRevisionDiff.ResolveArticle", "error", err)
		return GetArticleRevisionDiff500JSONResponse{Message: genericInternalError}, nil
	}

	if _, err := s.requireSilverEligible(ctx, res.SourceName); err != nil {
		if errors.Is(err, errSilverNotEligible) {
			anchor := silverEligibilityAnchor
			return GetArticleRevisionDiff403JSONResponse{
				Gate:               SilverEligibility,
				Message:            silverEligibilityRefusalMessage,
				WorkingPaperAnchor: &anchor,
			}, nil
		}
		if errors.Is(err, errSilverSourceNotFound) {
			return GetArticleRevisionDiff404JSONResponse{Message: "article source not found"}, nil
		}
		if errors.Is(err, errSilverNotConfigured) {
			return GetArticleRevisionDiff500JSONResponse{Message: genericInternalError}, nil
		}
		slog.Error("handler failure", "op", "GetArticleRevisionDiff.requireSilverEligible", "error", err)
		return GetArticleRevisionDiff500JSONResponse{Message: genericInternalError}, nil
	}

	row, err := s.db.GetArticleRevisionDiff(ctx, request.Id, request.RevisionIndex)
	if err != nil {
		if errors.Is(err, storage.ErrRevisionDiffHeadRequested) {
			return GetArticleRevisionDiff404JSONResponse{Message: "revisionIndex=0 has no predecessor to diff against"}, nil
		}
		if errors.Is(err, storage.ErrRevisionDiffPending) {
			return GetArticleRevisionDiff404JSONResponse{Message: "diff not yet computed; the worker sweep has not yet processed this snapshot pair"}, nil
		}
		if errors.Is(err, storage.ErrSourceNotFound) {
			return GetArticleRevisionDiff404JSONResponse{Message: "revision pair not found"}, nil
		}
		slog.Error("handler failure", "op", "GetArticleRevisionDiff", "error", err)
		return GetArticleRevisionDiff500JSONResponse{Message: genericInternalError}, nil
	}

	ops, err := storage.DecodeDiffParagraphs(row.DiffParagraphs)
	if err != nil {
		slog.Error("handler failure", "op", "GetArticleRevisionDiff.decode", "error", err)
		return GetArticleRevisionDiff500JSONResponse{Message: genericInternalError}, nil
	}

	// Project the storage op slice onto the generated inline shape.
	opShapes := make([]struct {
		After  *string                              `json:"after,omitempty"`
		Before *string                              `json:"before,omitempty"`
		Op     ArticleRevisionDiffDiffParagraphsOp  `json:"op"`
	}, 0, len(ops))
	for _, op := range ops {
		opShape := struct {
			After  *string                             `json:"after,omitempty"`
			Before *string                             `json:"before,omitempty"`
			Op     ArticleRevisionDiffDiffParagraphsOp `json:"op"`
		}{
			Op: ArticleRevisionDiffDiffParagraphsOp(op.Op),
		}
		if op.Before != "" {
			b := op.Before
			opShape.Before = &b
		}
		if op.After != "" {
			a := op.After
			opShape.After = &a
		}
		opShapes = append(opShapes, opShape)
	}

	resp := GetArticleRevisionDiff200JSONResponse{
		ArticleId:        row.ArticleID,
		RevisionIndex:    int(row.RevisionIndex), //nolint:gosec // bounded
		SnapshotAtBefore: row.SnapshotAtBefore,
		SnapshotAtAfter:  row.SnapshotAtAfter,
		HeadlineChanged:  row.HeadlineChanged,
		DiffParagraphs:   opShapes,
	}
	if row.HeadlineBefore != "" {
		b := row.HeadlineBefore
		resp.HeadlineBefore = &b
	}
	if row.HeadlineAfter != "" {
		a := row.HeadlineAfter
		resp.HeadlineAfter = &a
	}
	return resp, nil
}

// GetRevisionsArticles handles GET /revisions/articles.
//
// Workbench drill-down — paginated articles with ≥1 revision in the
// active window. Cursor decoding mirrors the source-articles handler;
// no Silver-eligibility gate here because this is an article LIST,
// not the article body — same posture as `/sources/{id}/articles`.
func (s *Server) GetRevisionsArticles(
	ctx context.Context,
	request GetRevisionsArticlesRequestObject,
) (GetRevisionsArticlesResponseObject, error) {
	scope := GetRevisionsArticlesParamsScopeProbe
	if request.Params.Scope != nil {
		scope = *request.Params.Scope
	}
	sources, err := s.resolveRevisionsArticlesScope(ctx, scope, request.Params.ScopeId)
	if err != nil {
		if errors.Is(err, storage.ErrSourceNotFound) || errors.Is(err, errRevisionsProbeNotFound) {
			return GetRevisionsArticles404JSONResponse{Message: err.Error()}, nil
		}
		slog.Error("handler failure", "op", "GetRevisionsArticles.resolveScope", "error", err)
		return GetRevisionsArticles500JSONResponse{Message: genericInternalError}, nil
	}

	if !request.Params.EndDate.After(request.Params.StartDate) {
		return GetRevisionsArticles400JSONResponse{Message: "endDate must be strictly after startDate"}, nil
	}

	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
		if limit < 1 || limit > 200 {
			return GetRevisionsArticles400JSONResponse{Message: "limit must be between 1 and 200"}, nil
		}
	}
	offset := 0
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		o, err := decodeCursor(*request.Params.Cursor)
		if err != nil {
			return GetRevisionsArticles400JSONResponse{Message: "invalid cursor"}, nil
		}
		offset = o
	}

	filter := storage.RevisionsArticlesFilter{
		Sources: sources,
		Start:   request.Params.StartDate,
		End:     request.Params.EndDate,
		Limit:   limit + 1, // fetch one extra to detect hasMore
		Offset:  offset,
	}
	if request.Params.HasHeadlineChange != nil {
		filter.HasHeadlineChange = *request.Params.HasHeadlineChange
	}
	if request.Params.MinChainLength != nil {
		filter.MinChainLength = *request.Params.MinChainLength
	}

	rows, err := s.db.GetRevisionsArticles(ctx, filter)
	if err != nil {
		slog.Error("handler failure", "op", "GetRevisionsArticles", "error", err)
		return GetRevisionsArticles500JSONResponse{Message: genericInternalError}, nil
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	page := GetRevisionsArticles200JSONResponse{HasMore: hasMore}
	if hasMore {
		next := encodeRevisionsCursor(offset + limit)
		page.NextCursor = &next
	}
	for _, r := range rows {
		item := struct {
			ArticleId         string     `json:"articleId"`
			ChainLength       int        `json:"chainLength"`
			HasHeadlineChange bool       `json:"hasHeadlineChange"`
			Language          *string    `json:"language,omitempty"`
			LatestRevisionAt  *time.Time `json:"latestRevisionAt,omitempty"`
			Source            string     `json:"source"`
			Timestamp         time.Time  `json:"timestamp"`
			WordCount         *int       `json:"wordCount,omitempty"`
		}{
			ArticleId:         r.ArticleID,
			Source:            r.Source,
			Timestamp:         r.Timestamp,
			ChainLength:       int(r.ChainLength), //nolint:gosec // bounded
			HasHeadlineChange: r.HasHeadlineChange,
		}
		if r.Language != "" {
			lang := r.Language
			item.Language = &lang
		}
		if r.WordCount > 0 {
			wc := int(r.WordCount) //nolint:gosec // bounded
			item.WordCount = &wc
		}
		if !r.LatestRevisionAt.IsZero() {
			t := r.LatestRevisionAt
			item.LatestRevisionAt = &t
		}
		page.Items = append(page.Items, item)
	}
	if page.Items == nil {
		page.Items = []struct {
			ArticleId         string     `json:"articleId"`
			ChainLength       int        `json:"chainLength"`
			HasHeadlineChange bool       `json:"hasHeadlineChange"`
			Language          *string    `json:"language,omitempty"`
			LatestRevisionAt  *time.Time `json:"latestRevisionAt,omitempty"`
			Source            string     `json:"source"`
			Timestamp         time.Time  `json:"timestamp"`
			WordCount         *int       `json:"wordCount,omitempty"`
		}{}
	}
	return page, nil
}

// resolveRevisionsArticlesScope expands the scope/scopeId pair into a
// source list. Mirrors the existing helper in revisions_handler.go
// but for the GetRevisionsArticlesParamsScope enum (codegen produces
// a distinct enum type per endpoint).
func (s *Server) resolveRevisionsArticlesScope(
	ctx context.Context,
	scope GetRevisionsArticlesParamsScope,
	scopeID string,
) ([]string, error) {
	switch scope {
	case GetRevisionsArticlesParamsScopeSource:
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

// encodeRevisionsCursor mirrors `encodeCursor` (dossier_handler.go)
// — opaque base64-wrapped offset. Kept separate so the cursor
// vocabulary can diverge later without touching the source-articles
// pagination.
func encodeRevisionsCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o=" + strconv.Itoa(offset)))
}

// guard against unused-import on `fmt` / `strings` when shifting code.
var _ = fmt.Sprintf
var _ = strings.TrimSpace

package handler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// AnalysesBackend is the saved-analyses persistence surface (Phase 135),
// satisfied by *storage.AnalysesStore.
type AnalysesBackend interface {
	ListVisible(ctx context.Context, userID string) ([]storage.AnalysisListItem, error)
	Get(ctx context.Context, id, userID string) (*storage.Analysis, error)
	Create(ctx context.Context, ownerID, name, description, state string) (storage.AnalysisListItem, error)
	Update(ctx context.Context, id, userID, name, description, state string) (bool, error)
	Delete(ctx context.Context, id, userID string) (bool, error)
	// CountOwned reports how many analyses the user owns, for the per-user row
	// cap (SEC-016).
	CountOwned(ctx context.Context, ownerID string) (int, error)
	IsOwner(ctx context.Context, id, userID string) (bool, error)
	ListShares(ctx context.Context, analysisID string) ([]storage.ShareItem, error)
	AddShare(ctx context.Context, analysisID, ownerID, granteeEmail string, canEdit bool) (storage.ShareItem, error)
	RemoveShare(ctx context.Context, analysisID, granteeID string) (bool, error)
}

// Saved-analysis field caps (SEC-016). Lengths are byte caps (Go len() ==
// Postgres octet_length, so the handler guard and the DB CHECK in migration
// 000028 agree exactly). `state` is the serialized Workbench URL grammar — 256
// KiB is far above any real analysis yet bounds the unbounded-TEXT write into
// the shared auth DB. maxAnalysesPerUser caps the row count per owner.
const (
	maxAnalysisNameLen  = 200
	maxAnalysisDescLen  = 2 << 10   // 2 KiB
	maxAnalysisStateLen = 256 << 10 // 256 KiB
	maxAnalysesPerUser  = 500
)

// validateAnalysisFields enforces the per-field length caps. Returns a non-empty
// client-safe message on the first violation, or "" when all fields are within
// bounds (SEC-016).
func validateAnalysisFields(name, description, state string) string {
	switch {
	case len(name) > maxAnalysisNameLen:
		return "name is too long"
	case len(description) > maxAnalysisDescLen:
		return "description is too long"
	case len(state) > maxAnalysisStateLen:
		return "analysis state is too long"
	}
	return ""
}

// analysisListEntry is the generated anonymous list-item struct shape, shared
// by the list and create responses.
type analysisListEntry = struct {
	CreatedAt   time.Time           `json:"createdAt"`
	Description string              `json:"description"`
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Owned       bool                `json:"owned"`
	OwnerEmail  openapi_types.Email `json:"ownerEmail"`
	Permission  string              `json:"permission"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

func listEntry(it storage.AnalysisListItem) analysisListEntry {
	return analysisListEntry{
		CreatedAt:   it.CreatedAt,
		Description: it.Description,
		ID:          it.ID,
		Name:        it.Name,
		Owned:       it.Owned,
		OwnerEmail:  openapi_types.Email(it.OwnerEmail),
		Permission:  it.Permission,
		UpdatedAt:   it.UpdatedAt,
	}
}

func fullAnalysis(a *storage.Analysis) GetAnalysis200JSONResponse {
	return GetAnalysis200JSONResponse{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		State:       a.State,
		OwnerEmail:  openapi_types.Email(a.OwnerEmail),
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
		Permission:  a.Permission,
		Owned:       a.Owned,
	}
}

// GetAnalyses lists the analyses the user owns or has been granted.
func (s *Server) GetAnalyses(ctx context.Context, _ GetAnalysesRequestObject) (GetAnalysesResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return GetAnalyses401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	items, err := s.analysesBackend.ListVisible(ctx, id.UserID)
	if err != nil {
		slog.Error("analyses: list", "error", err)
		return GetAnalyses500JSONResponse{Message: genericInternalError}, nil
	}
	var out GetAnalyses200JSONResponse
	for _, it := range items {
		out.Analyses = append(out.Analyses, listEntry(it))
	}
	return out, nil
}

// PostAnalyses saves the current analysis as a new owned record.
func (s *Server) PostAnalyses(ctx context.Context, request PostAnalysesRequestObject) (PostAnalysesResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PostAnalyses401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if request.Body == nil {
		return PostAnalyses400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	name := strings.TrimSpace(request.Body.Name)
	if name == "" {
		return PostAnalyses400JSONResponse{Code: "invalid_request", Message: "name is required"}, nil
	}
	desc := ""
	if request.Body.Description != nil {
		desc = *request.Body.Description
	}
	if msg := validateAnalysisFields(name, desc, request.Body.State); msg != "" {
		return PostAnalyses400JSONResponse{Code: "invalid_request", Message: msg}, nil
	}
	// Per-user row cap (SEC-016): bound how many analyses one user can persist to
	// the shared auth DB. Generous for a real researcher; blocks a write-loop.
	count, err := s.analysesBackend.CountOwned(ctx, id.UserID)
	if err != nil {
		slog.Error("analyses: count owned", "error", err)
		return PostAnalyses500JSONResponse{Message: genericInternalError}, nil
	}
	if count >= maxAnalysesPerUser {
		return PostAnalyses400JSONResponse{Code: "quota_exceeded", Message: "saved-analysis limit reached"}, nil
	}
	it, err := s.analysesBackend.Create(ctx, id.UserID, name, desc, request.Body.State)
	if err != nil {
		slog.Error("analyses: create", "error", err)
		return PostAnalyses500JSONResponse{Message: genericInternalError}, nil
	}
	e := listEntry(it)
	return PostAnalyses201JSONResponse(e), nil
}

// GetAnalysis returns one analysis incl. its state if visible to the user.
func (s *Server) GetAnalysis(ctx context.Context, request GetAnalysisRequestObject) (GetAnalysisResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return GetAnalysis401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	a, err := s.analysesBackend.Get(ctx, request.ID, id.UserID)
	if err != nil {
		slog.Error("analyses: get", "error", err)
		return GetAnalysis500JSONResponse{Message: genericInternalError}, nil
	}
	if a == nil {
		return GetAnalysis404JSONResponse{Code: "not_found", Message: "no such analysis"}, nil
	}
	return fullAnalysis(a), nil
}

// PatchAnalysis updates name/description/state (owner or edit-grantee).
func (s *Server) PatchAnalysis(ctx context.Context, request PatchAnalysisRequestObject) (PatchAnalysisResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PatchAnalysis401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if request.Body == nil {
		return PatchAnalysis403JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	curr, err := s.analysesBackend.Get(ctx, request.ID, id.UserID)
	if err != nil {
		slog.Error("analyses: patch load", "error", err)
		return PatchAnalysis500JSONResponse{Message: genericInternalError}, nil
	}
	if curr == nil {
		return PatchAnalysis403JSONResponse{Code: "forbidden_not_shared", Message: "no such analysis"}, nil
	}
	name, desc, state := curr.Name, curr.Description, curr.State
	if request.Body.Name != nil {
		name = strings.TrimSpace(*request.Body.Name)
	}
	if request.Body.Description != nil {
		desc = *request.Body.Description
	}
	if request.Body.State != nil {
		state = *request.Body.State
	}
	if msg := validateAnalysisFields(name, desc, state); msg != "" {
		return PatchAnalysis403JSONResponse{Code: "invalid_request", Message: msg}, nil
	}
	ok, err := s.analysesBackend.Update(ctx, request.ID, id.UserID, name, desc, state)
	if err != nil {
		slog.Error("analyses: patch", "error", err)
		return PatchAnalysis500JSONResponse{Message: genericInternalError}, nil
	}
	if !ok {
		return PatchAnalysis403JSONResponse{Code: "forbidden_not_shared", Message: "not allowed to edit this analysis"}, nil
	}
	fresh, err := s.analysesBackend.Get(ctx, request.ID, id.UserID)
	if err != nil || fresh == nil {
		slog.Error("analyses: patch reload", "error", err)
		return PatchAnalysis500JSONResponse{Message: genericInternalError}, nil
	}
	return PatchAnalysis200JSONResponse(fullAnalysis(fresh)), nil
}

// DeleteAnalysis removes an owned analysis.
func (s *Server) DeleteAnalysis(ctx context.Context, request DeleteAnalysisRequestObject) (DeleteAnalysisResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return DeleteAnalysis401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	ok, err := s.analysesBackend.Delete(ctx, request.ID, id.UserID)
	if err != nil {
		slog.Error("analyses: delete", "error", err)
		return DeleteAnalysis500JSONResponse{Message: genericInternalError}, nil
	}
	if !ok {
		return DeleteAnalysis403JSONResponse{Code: "forbidden_not_owner", Message: "only the owner can delete this analysis"}, nil
	}
	return DeleteAnalysis204Response{}, nil
}

// GetAnalysisShares lists the grantees of an owned analysis.
func (s *Server) GetAnalysisShares(ctx context.Context, request GetAnalysisSharesRequestObject) (GetAnalysisSharesResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return GetAnalysisShares401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	owner, err := s.analysesBackend.IsOwner(ctx, request.ID, id.UserID)
	if err != nil {
		slog.Error("analyses: shares owner check", "error", err)
		return GetAnalysisShares500JSONResponse{Message: genericInternalError}, nil
	}
	if !owner {
		return GetAnalysisShares403JSONResponse{Code: "forbidden_not_owner", Message: "only the owner can manage shares"}, nil
	}
	shares, err := s.analysesBackend.ListShares(ctx, request.ID)
	if err != nil {
		slog.Error("analyses: list shares", "error", err)
		return GetAnalysisShares500JSONResponse{Message: genericInternalError}, nil
	}
	var out GetAnalysisShares200JSONResponse
	for _, sh := range shares {
		out.Shares = append(out.Shares, struct {
			CanEdit   bool                `json:"canEdit"`
			Email     openapi_types.Email `json:"email"`
			GranteeID string              `json:"granteeId"`
		}{CanEdit: sh.CanEdit, Email: openapi_types.Email(sh.Email), GranteeID: sh.GranteeID})
	}
	return out, nil
}

// PostAnalysisShare grants a named user access to an owned analysis.
func (s *Server) PostAnalysisShare(ctx context.Context, request PostAnalysisShareRequestObject) (PostAnalysisShareResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PostAnalysisShare401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if request.Body == nil {
		return PostAnalysisShare400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	owner, err := s.analysesBackend.IsOwner(ctx, request.ID, id.UserID)
	if err != nil {
		slog.Error("analyses: share owner check", "error", err)
		return PostAnalysisShare500JSONResponse{Message: genericInternalError}, nil
	}
	if !owner {
		return PostAnalysisShare403JSONResponse{Code: "forbidden_not_owner", Message: "only the owner can share"}, nil
	}
	canEdit := false
	if request.Body.CanEdit != nil {
		canEdit = *request.Body.CanEdit
	}
	sh, err := s.analysesBackend.AddShare(ctx, request.ID, id.UserID, strings.TrimSpace(string(request.Body.Email)), canEdit)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrGranteeNotFound):
			return PostAnalysisShare404JSONResponse{Code: "grantee_not_found", Message: "no account exists for that email"}, nil
		case errors.Is(err, storage.ErrCannotShareWithSelf):
			return PostAnalysisShare400JSONResponse{Code: "cannot_share_with_self", Message: "you cannot share with yourself"}, nil
		default:
			slog.Error("analyses: add share", "error", err)
			return PostAnalysisShare500JSONResponse{Message: genericInternalError}, nil
		}
	}
	return PostAnalysisShare201JSONResponse{
		GranteeID: sh.GranteeID,
		Email:     openapi_types.Email(sh.Email),
		CanEdit:   sh.CanEdit,
	}, nil
}

// DeleteAnalysisShare revokes a grantee from an owned analysis.
func (s *Server) DeleteAnalysisShare(ctx context.Context, request DeleteAnalysisShareRequestObject) (DeleteAnalysisShareResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return DeleteAnalysisShare401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	owner, err := s.analysesBackend.IsOwner(ctx, request.ID, id.UserID)
	if err != nil {
		slog.Error("analyses: revoke owner check", "error", err)
		return DeleteAnalysisShare500JSONResponse{Message: genericInternalError}, nil
	}
	if !owner {
		return DeleteAnalysisShare403JSONResponse{Code: "forbidden_not_owner", Message: "only the owner can manage shares"}, nil
	}
	ok, err := s.analysesBackend.RemoveShare(ctx, request.ID, request.GranteeID)
	if err != nil {
		slog.Error("analyses: revoke", "error", err)
		return DeleteAnalysisShare500JSONResponse{Message: genericInternalError}, nil
	}
	if !ok {
		return DeleteAnalysisShare404JSONResponse{Code: "not_found", Message: "no such grant"}, nil
	}
	return DeleteAnalysisShare204Response{}, nil
}

// compile-time assertion that *storage.AnalysesStore satisfies AnalysesBackend.
var _ AnalysesBackend = (*storage.AnalysesStore)(nil)

package handler

import (
	"context"
	"log/slog"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
)

// GetAuthMeExport returns everything AĒR stores about the authenticated user
// (DSGVO Art. 15 / 20).
func (s *Server) GetAuthMeExport(ctx context.Context, _ GetAuthMeExportRequestObject) (GetAuthMeExportResponseObject, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return GetAuthMeExport401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	e, err := s.authBackend.ExportUser(ctx, id.UserID)
	if err != nil {
		slog.Error("export: load user", "error", err)
		return GetAuthMeExport500JSONResponse{Message: genericInternalError}, nil
	}
	if e == nil {
		// Session validated moments ago but the row is gone — treat as logged out.
		return GetAuthMeExport401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	resp := GetAuthMeExport200JSONResponse{
		ID:                 e.ID,
		Email:              openapi_types.Email(e.Email),
		Role:               e.Role,
		Status:             e.Status,
		CreatedAt:          e.CreatedAt,
		ActiveSessionCount: e.ActiveSessionCount,
	}
	if e.ResponsibleUseAcceptedAt.Valid {
		t := e.ResponsibleUseAcceptedAt.Time
		resp.ResponsibleUseAcceptedAt = &t
	}
	if e.LastSeenAt.Valid {
		t := e.LastSeenAt.Time
		resp.LastSeenAt = &t
	}
	return resp, nil
}

// deleteAccountCookieResponse clears the session cookie on account deletion.
type deleteAccountCookieResponse struct {
	cookie *http.Cookie
	inner  DeleteAuthMeResponseObject
}

func (r deleteAccountCookieResponse) VisitDeleteAuthMeResponse(w http.ResponseWriter) error {
	if r.cookie != nil {
		http.SetCookie(w, r.cookie)
	}
	return r.inner.VisitDeleteAuthMeResponse(w)
}

// DeleteAuthMe permanently deletes the authenticated user's account (DSGVO
// Art. 17) and clears the session cookie. Sessions and tokens cascade-delete.
func (s *Server) DeleteAuthMe(ctx context.Context, _ DeleteAuthMeRequestObject) (DeleteAuthMeResponseObject, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return DeleteAuthMe401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if _, err := s.authBackend.DeleteUser(ctx, id.UserID); err != nil {
		slog.Error("delete: remove user", "error", err)
		return DeleteAuthMe500JSONResponse{Message: genericInternalError}, nil
	}
	return deleteAccountCookieResponse{cookie: s.clearSessionCookie(), inner: DeleteAuthMe204Response{}}, nil
}

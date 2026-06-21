package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
)

// GetAuthSessions returns the authenticated user's own active sessions so they
// can review where they are logged in (SEC-005). Privacy-minimal: the session
// id hash never leaves the BFF — only coarse metadata plus a `current` flag for
// the device making this request.
func (s *Server) GetAuthSessions(ctx context.Context, _ GetAuthSessionsRequestObject) (GetAuthSessionsResponseObject, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return GetAuthSessions401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	rows, err := s.authBackend.ListUserSessions(ctx, id.UserID)
	if err != nil {
		slog.Error("sessions: list", "error", err)
		return GetAuthSessions500JSONResponse{Message: genericInternalError}, nil
	}
	var out GetAuthSessions200JSONResponse
	for _, r := range rows {
		var ua *string
		if r.UserAgent != "" {
			v := r.UserAgent
			ua = &v
		}
		out.Sessions = append(out.Sessions, struct {
			CreatedAt  time.Time `json:"createdAt"`
			Current    bool      `json:"current"`
			LastSeenAt time.Time `json:"lastSeenAt"`
			UserAgent  *string   `json:"userAgent,omitempty"`
		}{
			CreatedAt:  r.CreatedAt,
			Current:    r.IDHash == id.SessionIDHash,
			LastSeenAt: r.LastSeenAt,
			UserAgent:  ua,
		})
	}
	return out, nil
}

// revokeAllSessionsCookieResponse clears the session cookie after a
// log-out-everywhere (the current session is revoked too).
type revokeAllSessionsCookieResponse struct {
	cookie *http.Cookie
	inner  DeleteAuthSessionsResponseObject
}

func (r revokeAllSessionsCookieResponse) VisitDeleteAuthSessionsResponse(w http.ResponseWriter) error {
	if r.cookie != nil {
		http.SetCookie(w, r.cookie)
	}
	return r.inner.VisitDeleteAuthSessionsResponse(w)
}

// DeleteAuthSessions revokes every session for the authenticated user, including
// the current one, and clears the session cookie — the "log out everywhere"
// action for a lost device (SEC-005).
func (s *Server) DeleteAuthSessions(ctx context.Context, _ DeleteAuthSessionsRequestObject) (DeleteAuthSessionsResponseObject, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return DeleteAuthSessions401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if err := s.authBackend.RevokeAllUserSessions(ctx, id.UserID); err != nil {
		slog.Error("sessions: revoke all", "error", err)
		return DeleteAuthSessions500JSONResponse{Message: genericInternalError}, nil
	}
	return revokeAllSessionsCookieResponse{cookie: s.clearSessionCookie(), inner: DeleteAuthSessions204Response{}}, nil
}

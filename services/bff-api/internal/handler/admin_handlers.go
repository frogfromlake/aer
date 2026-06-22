package handler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// adminAllowed reports whether the context identity is an admin user. Admin
// handlers call it as defense-in-depth (SEC-025): the path-prefix middleware
// gate is the primary control, but no handler may rely solely on its mount path
// — a future admin route misfiled outside /api/v1/admin/ would otherwise
// de-gate silently. Machine (X-API-Key) callers are never admins.
func adminAllowed(ctx context.Context) bool {
	id, ok := auth.IdentityFromContext(ctx)
	return ok && !id.Machine && id.Role == auth.RoleAdmin
}

// issueActionLink mints a single-use token for the given purpose, persists its
// hash, and returns the raw link the admin can deliver (also dispatched via the
// email seam). pathAndFragment is e.g. "/accept-invite#token=" — the token rides
// in the URL fragment, never the query string, so it is never sent to a server
// and never lands in access logs (SEC-009).
func (s *Server) issueActionLink(ctx context.Context, userID, purpose, pathAndFragment string) (string, error) {
	raw, hash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return "", err
	}
	ttl := s.authConfig.ResetTTL
	if purpose == "invite" {
		ttl = s.authConfig.InviteTTL
	}
	if err := s.authBackend.CreateToken(ctx, userID, purpose, hash, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	return s.authConfig.PublicBaseURL + pathAndFragment + raw, nil
}

// GetAdminUsers lists all users (admin only; gate enforced by middleware).
func (s *Server) GetAdminUsers(ctx context.Context, _ GetAdminUsersRequestObject) (GetAdminUsersResponseObject, error) {
	if !adminAllowed(ctx) {
		return GetAdminUsers403JSONResponse{Code: "forbidden_role", Message: "admin role required"}, nil
	}
	rows, err := s.authBackend.ListUsers(ctx)
	if err != nil {
		slog.Error("admin: list users", "error", err)
		return GetAdminUsers500JSONResponse{Message: genericInternalError}, nil
	}
	var out GetAdminUsers200JSONResponse
	for _, r := range rows {
		out.Users = append(out.Users, struct {
			CreatedAt time.Time           `json:"createdAt"`
			Email     openapi_types.Email `json:"email"`
			FirstName string              `json:"firstName"`
			ID        string              `json:"id"`
			LastName  string              `json:"lastName"`
			Role      string              `json:"role"`
			Status    string              `json:"status"`
		}{
			CreatedAt: r.CreatedAt,
			Email:     openapi_types.Email(r.Email),
			FirstName: r.FirstName,
			ID:        r.ID,
			LastName:  r.LastName,
			Role:      r.Role,
			Status:    r.Status,
		})
	}
	return out, nil
}

// PostAdminUsers creates an invited user and returns the accept-invite link.
func (s *Server) PostAdminUsers(ctx context.Context, request PostAdminUsersRequestObject) (PostAdminUsersResponseObject, error) {
	if !adminAllowed(ctx) {
		return PostAdminUsers403JSONResponse{Code: "forbidden_role", Message: "admin role required"}, nil
	}
	if request.Body == nil {
		return PostAdminUsers400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	email := strings.TrimSpace(string(request.Body.Email))
	role := request.Body.Role
	if email == "" {
		return PostAdminUsers400JSONResponse{Code: "invalid_request", Message: "email is required"}, nil
	}
	if role != string(auth.RoleAdmin) && role != string(auth.RoleResearcher) {
		return PostAdminUsers400JSONResponse{Code: "invalid_role", Message: "role must be admin or researcher"}, nil
	}
	userID, err := s.authBackend.CreateInvitedUser(ctx, email, role)
	if err != nil {
		if errors.Is(err, storage.ErrEmailExists) {
			return PostAdminUsers409JSONResponse{Code: "email_exists", Message: "a user with this email already exists"}, nil
		}
		slog.Error("admin: create user", "error", err)
		return PostAdminUsers500JSONResponse{Message: genericInternalError}, nil
	}
	link, err := s.issueActionLink(ctx, userID, "invite", "/accept-invite#token=")
	if err != nil {
		slog.Error("admin: issue invite", "error", err)
		return PostAdminUsers500JSONResponse{Message: genericInternalError}, nil
	}
	delivered := false
	if s.mailer != nil {
		if err := s.mailer.SendInvite(ctx, email, link); err != nil {
			// Never a silent drop: log for operator visibility. The link is
			// still returned below so the admin can deliver it manually.
			slog.Error("admin: deliver invite email", "email", email, "error", err)
		} else {
			delivered = s.emailEnabled
		}
	}
	return PostAdminUsers201JSONResponse{
		UserID:    userID,
		Email:     openapi_types.Email(email),
		Kind:      "invite",
		Link:      link,
		Delivered: &delivered,
	}, nil
}

// PostAdminUserSuspend suspends a user. An admin cannot suspend themselves.
func (s *Server) PostAdminUserSuspend(ctx context.Context, request PostAdminUserSuspendRequestObject) (PostAdminUserSuspendResponseObject, error) {
	if !adminAllowed(ctx) {
		return PostAdminUserSuspend403JSONResponse{Code: "forbidden_role", Message: "admin role required"}, nil
	}
	if id, ok := auth.IdentityFromContext(ctx); ok && id.UserID == request.ID {
		return PostAdminUserSuspend400JSONResponse{Code: "cannot_suspend_self", Message: "an admin cannot suspend their own account"}, nil
	}
	updated, err := s.authBackend.SetUserStatus(ctx, request.ID, "suspended")
	if err != nil {
		slog.Error("admin: suspend", "error", err)
		return PostAdminUserSuspend500JSONResponse{Message: genericInternalError}, nil
	}
	if !updated {
		return PostAdminUserSuspend404JSONResponse{Code: "not_found", Message: "no such user"}, nil
	}
	return PostAdminUserSuspend204Response{}, nil
}

// PostAdminUserReactivate returns a suspended user to active.
func (s *Server) PostAdminUserReactivate(ctx context.Context, request PostAdminUserReactivateRequestObject) (PostAdminUserReactivateResponseObject, error) {
	if !adminAllowed(ctx) {
		return PostAdminUserReactivate403JSONResponse{Code: "forbidden_role", Message: "admin role required"}, nil
	}
	updated, err := s.authBackend.SetUserStatus(ctx, request.ID, "active")
	if err != nil {
		slog.Error("admin: reactivate", "error", err)
		return PostAdminUserReactivate500JSONResponse{Message: genericInternalError}, nil
	}
	if !updated {
		return PostAdminUserReactivate404JSONResponse{Code: "not_found", Message: "no such user"}, nil
	}
	return PostAdminUserReactivate204Response{}, nil
}

// PostAdminUserResetPassword issues a reset token for a user and returns the link.
func (s *Server) PostAdminUserResetPassword(ctx context.Context, request PostAdminUserResetPasswordRequestObject) (PostAdminUserResetPasswordResponseObject, error) {
	if !adminAllowed(ctx) {
		return PostAdminUserResetPassword403JSONResponse{Code: "forbidden_role", Message: "admin role required"}, nil
	}
	user, err := s.authBackend.GetUserByID(ctx, request.ID)
	if err != nil {
		slog.Error("admin: reset lookup", "error", err)
		return PostAdminUserResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if user == nil {
		return PostAdminUserResetPassword404JSONResponse{Code: "not_found", Message: "no such user"}, nil
	}
	link, err := s.issueActionLink(ctx, user.ID, "password_reset", "/reset-password#token=")
	if err != nil {
		slog.Error("admin: issue reset", "error", err)
		return PostAdminUserResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	// SEC-008 — an admin-initiated reset immediately revokes the target's live
	// sessions, so a compromised/phished account is locked out at once rather
	// than only once the legitimate user consumes the link (mirrors the
	// self-service reset, which already evicts all sessions). Suspend remains
	// the documented instant-lockout primitive; this closes the least-surprise
	// gap where "reset password" left an attacker's session validating.
	if err := s.authBackend.RevokeAllUserSessions(ctx, user.ID); err != nil {
		slog.Error("admin: revoke sessions on reset", "error", err)
		return PostAdminUserResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	delivered := false
	if s.mailer != nil {
		if err := s.mailer.SendPasswordReset(ctx, user.Email, link); err != nil {
			slog.Error("admin: deliver reset email", "email", user.Email, "error", err)
		} else {
			delivered = s.emailEnabled
		}
	}
	return PostAdminUserResetPassword200JSONResponse{
		UserID:    user.ID,
		Email:     openapi_types.Email(user.Email),
		Kind:      "password_reset",
		Link:      link,
		Delivered: &delivered,
	}, nil
}

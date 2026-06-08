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

// issueActionLink mints a single-use token for the given purpose, persists its
// hash, and returns the raw link the admin can deliver (also dispatched via the
// email seam). pathAndQuery is e.g. "/accept-invite?token=".
func (s *Server) issueActionLink(ctx context.Context, userID, purpose, pathAndQuery string) (string, error) {
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
	return s.authConfig.PublicBaseURL + pathAndQuery + raw, nil
}

// GetAdminUsers lists all users (admin only; gate enforced by middleware).
func (s *Server) GetAdminUsers(ctx context.Context, _ GetAdminUsersRequestObject) (GetAdminUsersResponseObject, error) {
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
			Id        string              `json:"id"`
			Role      string              `json:"role"`
			Status    string              `json:"status"`
		}{
			CreatedAt: r.CreatedAt,
			Email:     openapi_types.Email(r.Email),
			Id:        r.ID,
			Role:      r.Role,
			Status:    r.Status,
		})
	}
	return out, nil
}

// PostAdminUsers creates an invited user and returns the accept-invite link.
func (s *Server) PostAdminUsers(ctx context.Context, request PostAdminUsersRequestObject) (PostAdminUsersResponseObject, error) {
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
	link, err := s.issueActionLink(ctx, userID, "invite", "/accept-invite?token=")
	if err != nil {
		slog.Error("admin: issue invite", "error", err)
		return PostAdminUsers500JSONResponse{Message: genericInternalError}, nil
	}
	if s.mailer != nil {
		_ = s.mailer.SendInvite(ctx, email, link)
	}
	return PostAdminUsers201JSONResponse{
		UserId: userID,
		Email:  openapi_types.Email(email),
		Kind:   "invite",
		Link:   link,
	}, nil
}

// PostAdminUserSuspend suspends a user. An admin cannot suspend themselves.
func (s *Server) PostAdminUserSuspend(ctx context.Context, request PostAdminUserSuspendRequestObject) (PostAdminUserSuspendResponseObject, error) {
	if id, ok := auth.IdentityFromContext(ctx); ok && id.UserID == request.Id {
		return PostAdminUserSuspend400JSONResponse{Code: "cannot_suspend_self", Message: "an admin cannot suspend their own account"}, nil
	}
	updated, err := s.authBackend.SetUserStatus(ctx, request.Id, "suspended")
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
	updated, err := s.authBackend.SetUserStatus(ctx, request.Id, "active")
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
	user, err := s.authBackend.GetUserByID(ctx, request.Id)
	if err != nil {
		slog.Error("admin: reset lookup", "error", err)
		return PostAdminUserResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if user == nil {
		return PostAdminUserResetPassword404JSONResponse{Code: "not_found", Message: "no such user"}, nil
	}
	link, err := s.issueActionLink(ctx, user.ID, "password_reset", "/reset-password?token=")
	if err != nil {
		slog.Error("admin: issue reset", "error", err)
		return PostAdminUserResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if s.mailer != nil {
		_ = s.mailer.SendPasswordReset(ctx, user.Email, link)
	}
	return PostAdminUserResetPassword200JSONResponse{
		UserId: user.ID,
		Email:  openapi_types.Email(user.Email),
		Kind:   "password_reset",
		Link:   link,
	}, nil
}

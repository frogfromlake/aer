package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// minPasswordLength mirrors the OpenAPI `minLength: 12` on the password fields;
// the strict server does not enforce string minLength, so we re-check here.
const minPasswordLength = 12

func passwordOK(pw string) bool { return len(pw) >= minPasswordLength }

// --- cookie response wrappers ------------------------------------------------
//
// The generated strict response types only write a body. To also set/clear the
// session cookie we implement the generated response interface with a thin
// wrapper whose Visit sets the cookie before delegating to the inner response.
// This keeps cookie handling out of the contract while staying contract-first.

type loginCookieResponse struct {
	cookie *http.Cookie
	inner  PostAuthLoginResponseObject
}

func (r loginCookieResponse) VisitPostAuthLoginResponse(w http.ResponseWriter) error {
	if r.cookie != nil {
		http.SetCookie(w, r.cookie)
	}
	return r.inner.VisitPostAuthLoginResponse(w)
}

type acceptInviteCookieResponse struct {
	cookie *http.Cookie
	inner  PostAuthAcceptInviteResponseObject
}

func (r acceptInviteCookieResponse) VisitPostAuthAcceptInviteResponse(w http.ResponseWriter) error {
	if r.cookie != nil {
		http.SetCookie(w, r.cookie)
	}
	return r.inner.VisitPostAuthAcceptInviteResponse(w)
}

type logoutCookieResponse struct {
	cookie *http.Cookie
	inner  PostAuthLogoutResponseObject
}

func (r logoutCookieResponse) VisitPostAuthLogoutResponse(w http.ResponseWriter) error {
	if r.cookie != nil {
		http.SetCookie(w, r.cookie)
	}
	return r.inner.VisitPostAuthLogoutResponse(w)
}

// --- cookie + session helpers ------------------------------------------------

func (s *Server) buildSessionCookie(rawToken string) *http.Cookie {
	return &http.Cookie{
		Name:     s.authConfig.CookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.authConfig.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(s.authConfig.SessionAbsolute.Seconds()),
	}
}

func (s *Server) clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     s.authConfig.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.authConfig.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}
}

// establishSession mints an opaque session, persists it (hashed), and returns
// the cookie carrying the raw id.
func (s *Server) establishSession(ctx context.Context, userID string) (*http.Cookie, error) {
	raw, hash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	idleExp := now.Add(s.authConfig.SessionIdle)
	absExp := now.Add(s.authConfig.SessionAbsolute)
	if err := s.authBackend.CreateSession(ctx, hash, userID, idleExp, absExp, ""); err != nil {
		return nil, err
	}
	return s.buildSessionCookie(raw), nil
}

// --- handlers ----------------------------------------------------------------

// PostAuthLogin verifies credentials and, on success, sets the session cookie.
func (s *Server) PostAuthLogin(ctx context.Context, request PostAuthLoginRequestObject) (PostAuthLoginResponseObject, error) {
	if request.Body == nil {
		return PostAuthLogin400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	email := strings.TrimSpace(string(request.Body.Email))
	// Brute-force throttle (security review M-3): exponential backoff keyed by
	// account + client IP. Checked before any work.
	keys := s.loginKeys(ctx, email)
	if blocked, _ := s.loginThrottle.Blocked(keys...); blocked {
		return PostAuthLogin429JSONResponse{Code: "too_many_attempts", Message: "too many failed attempts; please try again later"}, nil
	}

	user, err := s.authBackend.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Error("login: get user", "error", err)
		return PostAuthLogin500JSONResponse{Message: genericInternalError}, nil
	}
	// Generic failure for unknown user / no password / inactive — no enumeration.
	// Anti-timing: ALWAYS run one argon2id operation so the response time does
	// not reveal whether the account exists or is active. For a real active
	// account we verify the stored hash; otherwise we hash the candidate and
	// discard it (same dominant argon2.IDKey cost).
	if user == nil || !user.PasswordHash.Valid || user.Status != "active" {
		_, _ = auth.HashPassword(request.Body.Password, s.authConfig.Argon2)
		s.loginThrottle.Fail(keys...)
		return PostAuthLogin401JSONResponse{Code: "invalid_credentials", Message: "invalid email or password"}, nil
	}
	ok, err := auth.VerifyPassword(request.Body.Password, user.PasswordHash.String)
	if err != nil {
		slog.Error("login: verify password", "error", err)
		return PostAuthLogin500JSONResponse{Message: genericInternalError}, nil
	}
	if !ok {
		s.loginThrottle.Fail(keys...)
		return PostAuthLogin401JSONResponse{Code: "invalid_credentials", Message: "invalid email or password"}, nil
	}
	cookie, err := s.establishSession(ctx, user.ID)
	if err != nil {
		slog.Error("login: establish session", "error", err)
		return PostAuthLogin500JSONResponse{Message: genericInternalError}, nil
	}
	s.loginThrottle.Succeed(keys...)
	return loginCookieResponse{cookie: cookie, inner: loginUserResponse(user)}, nil
}

// loginKeys builds the throttle keys for a login attempt: the account (email)
// and, when available, the client IP.
func (s *Server) loginKeys(ctx context.Context, email string) []string {
	keys := []string{"email:" + strings.ToLower(email)}
	if ip := auth.ClientIPFromContext(ctx); ip != "" {
		keys = append(keys, "ip:"+ip)
	}
	return keys
}

// PostAuthLogout revokes the current session and clears the cookie. Idempotent.
func (s *Server) PostAuthLogout(ctx context.Context, _ PostAuthLogoutRequestObject) (PostAuthLogoutResponseObject, error) {
	if id, ok := auth.IdentityFromContext(ctx); ok && id.SessionIDHash != "" {
		if err := s.authBackend.RevokeSession(ctx, id.SessionIDHash); err != nil {
			slog.Error("logout: revoke session", "error", err)
			return PostAuthLogout500JSONResponse{Message: genericInternalError}, nil
		}
	}
	return logoutCookieResponse{cookie: s.clearSessionCookie(), inner: PostAuthLogout204Response{}}, nil
}

// GetAuthMe returns the user bound to the session. Machine (API-key) callers
// have no user identity and get 401.
func (s *Server) GetAuthMe(ctx context.Context, _ GetAuthMeRequestObject) (GetAuthMeResponseObject, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return GetAuthMe401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	return GetAuthMe200JSONResponse{
		Id:     id.UserID,
		Email:  openapi_types.Email(id.Email),
		Role:   string(id.Role),
		Status: "active",
	}, nil
}

// PostAuthAcceptInvite exchanges an invite token for an activated account,
// records consent, and auto-logs-in (sets the session cookie).
func (s *Server) PostAuthAcceptInvite(ctx context.Context, request PostAuthAcceptInviteRequestObject) (PostAuthAcceptInviteResponseObject, error) {
	if request.Body == nil {
		return PostAuthAcceptInvite400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	b := request.Body
	if !b.AcceptResponsibleUse {
		return PostAuthAcceptInvite400JSONResponse{Code: "consent_required", Message: "the responsible-use agreement must be accepted"}, nil
	}
	if !passwordOK(b.Password) {
		return PostAuthAcceptInvite400JSONResponse{Code: "weak_password", Message: "password too short"}, nil
	}
	userID, err := s.authBackend.ConsumeToken(ctx, auth.HashOpaqueToken(b.Token), "invite")
	if err != nil {
		slog.Error("accept-invite: consume token", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	if userID == "" {
		return PostAuthAcceptInvite400JSONResponse{Code: "invalid_token", Message: "the invitation link is invalid or has expired"}, nil
	}
	pwHash, err := auth.HashPassword(b.Password, s.authConfig.Argon2)
	if err != nil {
		slog.Error("accept-invite: hash password", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.authBackend.ActivateUser(ctx, userID, pwHash); err != nil {
		slog.Error("accept-invite: activate user", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	user, err := s.authBackend.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		slog.Error("accept-invite: reload user", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	cookie, err := s.establishSession(ctx, userID)
	if err != nil {
		slog.Error("accept-invite: establish session", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	return acceptInviteCookieResponse{cookie: cookie, inner: acceptInviteUserResponse(user)}, nil
}

// PostAuthForgotPassword issues a single-use reset token and dispatches it.
// Always returns 202 — no user enumeration.
func (s *Server) PostAuthForgotPassword(ctx context.Context, request PostAuthForgotPasswordRequestObject) (PostAuthForgotPasswordResponseObject, error) {
	if request.Body != nil {
		email := strings.TrimSpace(string(request.Body.Email))
		user, err := s.authBackend.GetUserByEmail(ctx, email)
		if err != nil {
			// Log but still return 202 so the response shape never reveals
			// whether the account exists.
			slog.Error("forgot-password: get user", "error", err)
		} else if user != nil && user.Status == "active" {
			if raw, hash, gErr := auth.GenerateOpaqueToken(); gErr == nil {
				exp := time.Now().Add(s.authConfig.ResetTTL)
				if cErr := s.authBackend.CreateToken(ctx, user.ID, "password_reset", hash, exp); cErr == nil {
					link := s.authConfig.PublicBaseURL + "/reset-password?token=" + raw
					if s.mailer != nil {
						_ = s.mailer.SendPasswordReset(ctx, user.Email, link)
					}
				} else {
					slog.Error("forgot-password: create token", "error", cErr)
				}
			}
		}
	}
	return PostAuthForgotPassword202Response{}, nil
}

// PostAuthResetPassword consumes a reset token, sets the new password, and
// invalidates all of the user's sessions.
func (s *Server) PostAuthResetPassword(ctx context.Context, request PostAuthResetPasswordRequestObject) (PostAuthResetPasswordResponseObject, error) {
	if request.Body == nil {
		return PostAuthResetPassword400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	if !passwordOK(request.Body.Password) {
		return PostAuthResetPassword400JSONResponse{Code: "weak_password", Message: "password too short"}, nil
	}
	userID, err := s.authBackend.ConsumeToken(ctx, auth.HashOpaqueToken(request.Body.Token), "password_reset")
	if err != nil {
		slog.Error("reset-password: consume token", "error", err)
		return PostAuthResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if userID == "" {
		return PostAuthResetPassword400JSONResponse{Code: "invalid_token", Message: "the reset link is invalid or has expired"}, nil
	}
	pwHash, err := auth.HashPassword(request.Body.Password, s.authConfig.Argon2)
	if err != nil {
		slog.Error("reset-password: hash password", "error", err)
		return PostAuthResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.authBackend.UpdateUserPassword(ctx, userID, pwHash); err != nil {
		slog.Error("reset-password: update password", "error", err)
		return PostAuthResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.authBackend.RevokeAllUserSessions(ctx, userID); err != nil {
		slog.Error("reset-password: revoke sessions", "error", err)
		return PostAuthResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	return PostAuthResetPassword204Response{}, nil
}

// PostAuthChangePassword changes the password for an authenticated user who
// supplies their current password, then invalidates all OTHER sessions.
func (s *Server) PostAuthChangePassword(ctx context.Context, request PostAuthChangePasswordRequestObject) (PostAuthChangePasswordResponseObject, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return PostAuthChangePassword401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if request.Body == nil {
		return PostAuthChangePassword400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	if !passwordOK(request.Body.NewPassword) {
		return PostAuthChangePassword400JSONResponse{Code: "weak_password", Message: "password too short"}, nil
	}
	user, err := s.authBackend.GetUserByID(ctx, id.UserID)
	if err != nil || user == nil {
		slog.Error("change-password: reload user", "error", err)
		return PostAuthChangePassword500JSONResponse{Message: genericInternalError}, nil
	}
	if !user.PasswordHash.Valid {
		return PostAuthChangePassword401JSONResponse{Code: "invalid_credentials", Message: "current password is incorrect"}, nil
	}
	matches, err := auth.VerifyPassword(request.Body.CurrentPassword, user.PasswordHash.String)
	if err != nil {
		slog.Error("change-password: verify current", "error", err)
		return PostAuthChangePassword500JSONResponse{Message: genericInternalError}, nil
	}
	if !matches {
		return PostAuthChangePassword401JSONResponse{Code: "invalid_credentials", Message: "current password is incorrect"}, nil
	}
	pwHash, err := auth.HashPassword(request.Body.NewPassword, s.authConfig.Argon2)
	if err != nil {
		slog.Error("change-password: hash new", "error", err)
		return PostAuthChangePassword500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.authBackend.UpdateUserPassword(ctx, id.UserID, pwHash); err != nil {
		slog.Error("change-password: update password", "error", err)
		return PostAuthChangePassword500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.authBackend.RevokeOtherUserSessions(ctx, id.UserID, id.SessionIDHash); err != nil {
		slog.Error("change-password: revoke other sessions", "error", err)
		return PostAuthChangePassword500JSONResponse{Message: genericInternalError}, nil
	}
	return PostAuthChangePassword204Response{}, nil
}

// --- AuthUser response mappers ----------------------------------------------

func loginUserResponse(u *storage.AuthUser) PostAuthLogin200JSONResponse {
	return PostAuthLogin200JSONResponse{
		Id:     u.ID,
		Email:  openapi_types.Email(u.Email),
		Role:   u.Role,
		Status: u.Status,
	}
}

func acceptInviteUserResponse(u *storage.AuthUser) PostAuthAcceptInvite200JSONResponse {
	return PostAuthAcceptInvite200JSONResponse{
		Id:     u.ID,
		Email:  openapi_types.Email(u.Email),
		Role:   u.Role,
		Status: u.Status,
	}
}

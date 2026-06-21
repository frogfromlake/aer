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
	// account + client IP. SEC-020 — we evaluate the throttle but do NOT
	// hard-block before verifying the password. A correct credential must
	// always succeed and clear the throttle, so a third party arming the
	// account-only key with wrong guesses can never lock the legitimate owner
	// out; a wrong guess made while armed is what earns the 429.
	keys := s.loginKeys(ctx, email)
	blocked, _ := s.loginThrottle.Blocked(keys...)

	user, err := s.authBackend.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Error("login: get user", "error", err)
		return PostAuthLogin500JSONResponse{Message: genericInternalError}, nil
	}
	// Anti-timing: ALWAYS run exactly one argon2id operation so the response
	// time does not reveal whether the account exists or is active — and we
	// must verify the password even while throttled to tell the legitimate
	// owner from an attacker. For a real active account we verify the stored
	// hash; otherwise we hash the candidate and discard it (same dominant cost).
	passwordCorrect := false
	if user == nil || !user.PasswordHash.Valid || user.Status != "active" {
		_, _ = auth.HashPassword(request.Body.Password, s.authConfig.Argon2)
	} else {
		ok, vErr := auth.VerifyPassword(request.Body.Password, user.PasswordHash.String)
		if vErr != nil {
			slog.Error("login: verify password", "error", vErr)
			return PostAuthLogin500JSONResponse{Message: genericInternalError}, nil
		}
		passwordCorrect = ok
	}

	if !passwordCorrect {
		s.loginThrottle.Fail(keys...)
		// A wrong attempt during the backoff window is throttled; otherwise it
		// is a generic 401 (the Fail above may arm the backoff).
		if blocked {
			return PostAuthLogin429JSONResponse{Code: "too_many_attempts", Message: "too many failed attempts; please try again later"}, nil
		}
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
		ID:     id.UserID,
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
	// Hash before the transaction so the CPU-bound argon2 work never holds the
	// token+activate tx open.
	pwHash, err := auth.HashPassword(b.Password, s.authConfig.Argon2)
	if err != nil {
		slog.Error("accept-invite: hash password", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	// SEC-078 — consume the single-use token and activate atomically: a partial
	// failure can no longer burn the token while leaving the account inactive.
	userID, err := s.authBackend.ConsumeTokenAndActivate(ctx, auth.HashOpaqueToken(b.Token), pwHash)
	if err != nil {
		slog.Error("accept-invite: consume token and activate", "error", err)
		return PostAuthAcceptInvite500JSONResponse{Message: genericInternalError}, nil
	}
	if userID == "" {
		return PostAuthAcceptInvite400JSONResponse{Code: "invalid_token", Message: "the invitation link is invalid or has expired"}, nil
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

// passwordResetDispatchTimeout bounds the detached token-mint + email-send that
// runs off the forgot-password request path (SEC-019). It exceeds the SMTP
// sendTimeout so the dispatch ends on its own deadline rather than abruptly.
const passwordResetDispatchTimeout = 45 * time.Second

// PostAuthForgotPassword issues a single-use reset token and dispatches it.
// Always returns 202 — no user enumeration.
//
// The lookup + token-mint + email-send run OFF the request path (SEC-019): the
// response time is constant whether or not the account exists, so the
// synchronous-SMTP timing oracle is gone and a slow relay can never hang the
// request goroutine. A dedicated per-account + per-IP throttle (SEC-006/022)
// bounds issuance — when armed the request still returns 202 but does no work,
// preserving the uniform 202 (and the no-enumeration guarantee).
func (s *Server) PostAuthForgotPassword(ctx context.Context, request PostAuthForgotPasswordRequestObject) (PostAuthForgotPasswordResponseObject, error) {
	if request.Body == nil {
		return PostAuthForgotPassword202Response{}, nil
	}
	email := strings.TrimSpace(string(request.Body.Email))
	keys := s.loginKeys(ctx, email)
	if blocked, _ := s.resetThrottle.Blocked(keys...); blocked {
		return PostAuthForgotPassword202Response{}, nil
	}
	s.resetThrottle.Fail(keys...)
	s.dispatchReset(ctx, email)
	return PostAuthForgotPassword202Response{}, nil
}

// dispatchPasswordReset performs the off-request-path token-mint + email-send
// for forgot-password (SEC-019). Errors are logged for operator visibility but
// never surfaced — the caller already returned the uniform 202.
func (s *Server) dispatchPasswordReset(ctx context.Context, email string) {
	ctx, cancel := context.WithTimeout(ctx, passwordResetDispatchTimeout)
	defer cancel()

	user, err := s.authBackend.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Error("forgot-password: get user", "error", err)
		return
	}
	if user == nil || user.Status != "active" {
		return
	}
	// SEC-022 — consume any prior unconsumed reset tokens so only the newest
	// link stays live.
	if err := s.authBackend.InvalidateUserTokens(ctx, user.ID, "password_reset"); err != nil {
		slog.Error("forgot-password: invalidate prior tokens", "error", err)
		return
	}
	raw, hash, err := auth.GenerateOpaqueToken()
	if err != nil {
		slog.Error("forgot-password: generate token", "error", err)
		return
	}
	if err := s.authBackend.CreateToken(ctx, user.ID, "password_reset", hash, time.Now().Add(s.authConfig.ResetTTL)); err != nil {
		slog.Error("forgot-password: create token", "error", err)
		return
	}
	if s.mailer == nil {
		return
	}
	// SEC-009 — the token rides in the URL fragment, never the query string, so
	// it is never sent to a server and never lands in access logs.
	link := s.authConfig.PublicBaseURL + "/reset-password#token=" + raw
	if err := s.mailer.SendPasswordReset(ctx, user.Email, link); err != nil {
		slog.Error("forgot-password: deliver reset email", "error", err)
	}
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
	// Hash before the transaction so the CPU-bound argon2 work never holds the
	// token+password+revoke tx open.
	pwHash, err := auth.HashPassword(request.Body.Password, s.authConfig.Argon2)
	if err != nil {
		slog.Error("reset-password: hash password", "error", err)
		return PostAuthResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	// SEC-078 — consume the token, set the password, and revoke all sessions
	// atomically: the password change and the session revocation co-commit, so a
	// partial failure can neither burn the token nor leave stale sessions live.
	userID, err := s.authBackend.ConsumeTokenAndResetPassword(ctx, auth.HashOpaqueToken(request.Body.Token), pwHash)
	if err != nil {
		slog.Error("reset-password: consume token and reset", "error", err)
		return PostAuthResetPassword500JSONResponse{Message: genericInternalError}, nil
	}
	if userID == "" {
		return PostAuthResetPassword400JSONResponse{Code: "invalid_token", Message: "the reset link is invalid or has expired"}, nil
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
		ID:     u.ID,
		Email:  openapi_types.Email(u.Email),
		Role:   u.Role,
		Status: u.Status,
	}
}

func acceptInviteUserResponse(u *storage.AuthUser) PostAuthAcceptInvite200JSONResponse {
	return PostAuthAcceptInvite200JSONResponse{
		ID:     u.ID,
		Email:  openapi_types.Email(u.Email),
		Role:   u.Role,
		Status: u.Status,
	}
}

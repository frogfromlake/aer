package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// --- in-memory AuthBackend mock ---------------------------------------------

type mockToken struct {
	userID   string
	purpose  string
	consumed bool
}

type mockAuth struct {
	byEmail    map[string]*storage.AuthUser
	byID       map[string]*storage.AuthUser
	tokens     map[string]*mockToken // tokenHash -> token
	sessions   map[string]string     // idHash -> userID
	revokedAll []string
}

func newMockAuth() *mockAuth {
	return &mockAuth{
		byEmail:  map[string]*storage.AuthUser{},
		byID:     map[string]*storage.AuthUser{},
		tokens:   map[string]*mockToken{},
		sessions: map[string]string{},
	}
}

func (m *mockAuth) addUser(u *storage.AuthUser) {
	m.byEmail[strings.ToLower(u.Email)] = u
	m.byID[u.ID] = u
}

func (m *mockAuth) GetUserByEmail(_ context.Context, email string) (*storage.AuthUser, error) {
	return m.byEmail[strings.ToLower(email)], nil
}
func (m *mockAuth) GetUserByID(_ context.Context, id string) (*storage.AuthUser, error) {
	return m.byID[id], nil
}
func (m *mockAuth) CreateSession(_ context.Context, idHash, userID string, _, _ time.Time, _ string) error {
	m.sessions[idHash] = userID
	return nil
}
func (m *mockAuth) ValidateAndTouchSession(_ context.Context, _ string, _ time.Duration) (*auth.Identity, error) {
	return nil, nil // handlers read identity from context; middleware is tested elsewhere
}
func (m *mockAuth) RevokeSession(_ context.Context, idHash string) error {
	delete(m.sessions, idHash)
	return nil
}
func (m *mockAuth) RevokeAllUserSessions(_ context.Context, userID string) error {
	m.revokedAll = append(m.revokedAll, userID)
	return nil
}
func (m *mockAuth) RevokeOtherUserSessions(_ context.Context, _, _ string) error { return nil }
func (m *mockAuth) CreateToken(_ context.Context, userID, purpose, tokenHash string, _ time.Time) error {
	m.tokens[tokenHash] = &mockToken{userID: userID, purpose: purpose}
	return nil
}
func (m *mockAuth) ConsumeToken(_ context.Context, tokenHash, purpose string) (string, error) {
	tok, ok := m.tokens[tokenHash]
	if !ok || tok.consumed || tok.purpose != purpose {
		return "", nil
	}
	tok.consumed = true
	return tok.userID, nil
}
func (m *mockAuth) ActivateUser(_ context.Context, id, passwordHash string) error {
	if u := m.byID[id]; u != nil {
		u.Status = "active"
		u.PasswordHash = sql.NullString{String: passwordHash, Valid: true}
	}
	return nil
}
func (m *mockAuth) UpdateUserPassword(_ context.Context, id, passwordHash string) error {
	if u := m.byID[id]; u != nil {
		u.PasswordHash = sql.NullString{String: passwordHash, Valid: true}
	}
	return nil
}

// --- test scaffolding --------------------------------------------------------

func authTestServer(m *mockAuth) *Server {
	return &Server{
		authBackend: m,
		mailer:      stubMailer{},
		authConfig: AuthConfig{
			CookieName:      "aer_session",
			SecureCookies:   false,
			SessionIdle:     time.Hour,
			SessionAbsolute: 24 * time.Hour,
			Argon2:          auth.DefaultArgon2Params(),
			ResetTTL:        time.Hour,
			InviteTTL:       time.Hour,
		},
	}
}

type stubMailer struct{}

func (stubMailer) SendInvite(context.Context, string, string) error        { return nil }
func (stubMailer) SendPasswordReset(context.Context, string, string) error { return nil }

func activeUser(t *testing.T, id, email, password string) *storage.AuthUser {
	t.Helper()
	hash, err := auth.HashPassword(password, auth.DefaultArgon2Params())
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return &storage.AuthUser{
		ID: id, Email: email, Role: "researcher", Status: "active",
		PasswordHash: sql.NullString{String: hash, Valid: true},
	}
}

func setCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "aer_session" {
			return c
		}
	}
	return nil
}

// --- tests -------------------------------------------------------------------

func TestLoginSuccessSetsCookie(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "hunter2hunter2"))
	s := authTestServer(m)

	resp, err := s.PostAuthLogin(context.Background(), PostAuthLoginRequestObject{
		Body: &PostAuthLoginJSONRequestBody{Email: openapi_types.Email("alice@example.org"), Password: "hunter2hunter2"},
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	rec := httptest.NewRecorder()
	if err := resp.VisitPostAuthLoginResponse(rec); err != nil {
		t.Fatalf("visit: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	c := setCookie(rec)
	if c == nil || c.Value == "" {
		t.Fatal("expected a session cookie to be set")
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected httpOnly + SameSite=Strict cookie, got %+v", c)
	}
	if len(m.sessions) != 1 {
		t.Fatalf("expected one session stored, got %d", len(m.sessions))
	}
}

func TestLoginWrongPasswordIsGeneric401(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "hunter2hunter2"))
	s := authTestServer(m)

	resp, _ := s.PostAuthLogin(context.Background(), PostAuthLoginRequestObject{
		Body: &PostAuthLoginJSONRequestBody{Email: openapi_types.Email("alice@example.org"), Password: "wrongwrongwrong"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthLoginResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if setCookie(rec) != nil {
		t.Fatal("no cookie should be set on failed login")
	}
}

func TestLoginUnknownUserIsGeneric401(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.PostAuthLogin(context.Background(), PostAuthLoginRequestObject{
		Body: &PostAuthLoginJSONRequestBody{Email: openapi_types.Email("nobody@example.org"), Password: "whatever12345"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthLoginResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown user (no enumeration), got %d", rec.Code)
	}
}

func TestMeRequiresSession(t *testing.T) {
	s := authTestServer(newMockAuth())

	// No identity in context → 401.
	resp, _ := s.GetAuthMe(context.Background(), GetAuthMeRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitGetAuthMeResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}

	// Identity in context → 200.
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Email: "alice@example.org", Role: auth.RoleResearcher})
	resp, _ = s.GetAuthMe(ctx, GetAuthMeRequestObject{})
	rec = httptest.NewRecorder()
	_ = resp.VisitGetAuthMeResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with session, got %d", rec.Code)
	}
}

func TestLogoutClearsCookieAndRevokes(t *testing.T) {
	m := newMockAuth()
	m.sessions["sess-hash"] = "u1"
	s := authTestServer(m)

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", SessionIDHash: "sess-hash"})
	resp, _ := s.PostAuthLogout(ctx, PostAuthLogoutRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthLogoutResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	c := setCookie(rec)
	if c == nil || c.MaxAge >= 0 {
		t.Fatalf("expected a cleared cookie (MaxAge<0), got %+v", c)
	}
	if _, ok := m.sessions["sess-hash"]; ok {
		t.Fatal("expected session to be revoked")
	}
}

func TestAcceptInviteActivatesAndLogsIn(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "new@example.org", Role: "researcher", Status: "invited"})
	rawTok, hashTok, _ := auth.GenerateOpaqueToken()
	m.tokens[hashTok] = &mockToken{userID: "u1", purpose: "invite"}
	s := authTestServer(m)

	resp, err := s.PostAuthAcceptInvite(context.Background(), PostAuthAcceptInviteRequestObject{
		Body: &PostAuthAcceptInviteJSONRequestBody{Token: rawTok, Password: "freshpassword123", AcceptResponsibleUse: true},
	})
	if err != nil {
		t.Fatalf("accept-invite: %v", err)
	}
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthAcceptInviteResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if setCookie(rec) == nil {
		t.Fatal("expected a session cookie (auto-login)")
	}
	if m.byID["u1"].Status != "active" {
		t.Fatal("expected user to be activated")
	}
	// Token is single-use: a replay must now fail with 400 invalid_token.
	resp, _ = s.PostAuthAcceptInvite(context.Background(), PostAuthAcceptInviteRequestObject{
		Body: &PostAuthAcceptInviteJSONRequestBody{Token: rawTok, Password: "freshpassword123", AcceptResponsibleUse: true},
	})
	rec = httptest.NewRecorder()
	_ = resp.VisitPostAuthAcceptInviteResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on token replay, got %d", rec.Code)
	}
}

func TestAcceptInviteRequiresConsent(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "new@example.org", Role: "researcher", Status: "invited"})
	rawTok, hashTok, _ := auth.GenerateOpaqueToken()
	m.tokens[hashTok] = &mockToken{userID: "u1", purpose: "invite"}
	s := authTestServer(m)

	resp, _ := s.PostAuthAcceptInvite(context.Background(), PostAuthAcceptInviteRequestObject{
		Body: &PostAuthAcceptInviteJSONRequestBody{Token: rawTok, Password: "freshpassword123", AcceptResponsibleUse: false},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthAcceptInviteResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 consent_required, got %d", rec.Code)
	}
	if m.byID["u1"].Status == "active" {
		t.Fatal("user must not be activated without consent")
	}
}

func TestForgotPasswordAlways202(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "hunter2hunter2"))
	s := authTestServer(m)

	for _, email := range []string{"alice@example.org", "nobody@example.org"} {
		resp, _ := s.PostAuthForgotPassword(context.Background(), PostAuthForgotPasswordRequestObject{
			Body: &PostAuthForgotPasswordJSONRequestBody{Email: openapi_types.Email(email)},
		})
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthForgotPasswordResponse(rec)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 for %q (no enumeration), got %d", email, rec.Code)
		}
	}
	// Exactly one reset token minted (for the existing active user only).
	if len(m.tokens) != 1 {
		t.Fatalf("expected one reset token, got %d", len(m.tokens))
	}
}

func TestResetPasswordConsumesTokenAndRevokes(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "oldpassword1234"))
	rawTok, hashTok, _ := auth.GenerateOpaqueToken()
	m.tokens[hashTok] = &mockToken{userID: "u1", purpose: "password_reset"}
	s := authTestServer(m)

	resp, _ := s.PostAuthResetPassword(context.Background(), PostAuthResetPasswordRequestObject{
		Body: &PostAuthResetPasswordJSONRequestBody{Token: rawTok, Password: "brandnewpassword1"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthResetPasswordResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if len(m.revokedAll) != 1 || m.revokedAll[0] != "u1" {
		t.Fatalf("expected all sessions revoked for u1, got %v", m.revokedAll)
	}
	// The new password verifies.
	ok, _ := auth.VerifyPassword("brandnewpassword1", m.byID["u1"].PasswordHash.String)
	if !ok {
		t.Fatal("expected the new password to verify after reset")
	}
}

func TestResetPasswordBadTokenIs400(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.PostAuthResetPassword(context.Background(), PostAuthResetPasswordRequestObject{
		Body: &PostAuthResetPasswordJSONRequestBody{Token: "bogus", Password: "brandnewpassword1"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthResetPasswordResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d", rec.Code)
	}
}

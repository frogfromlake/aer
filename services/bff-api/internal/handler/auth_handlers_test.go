package handler

import (
	"context"
	"database/sql"
	"fmt"
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
	idSeq      int
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
func (m *mockAuth) ListUserSessions(_ context.Context, userID string) ([]storage.SessionInfo, error) {
	var out []storage.SessionInfo
	for idHash, uid := range m.sessions {
		if uid == userID {
			out = append(out, storage.SessionInfo{IDHash: idHash})
		}
	}
	return out, nil
}
func (m *mockAuth) CreateToken(_ context.Context, userID, purpose, tokenHash string, _ time.Time) error {
	m.tokens[tokenHash] = &mockToken{userID: userID, purpose: purpose}
	return nil
}
func (m *mockAuth) InvalidateUserTokens(_ context.Context, userID, purpose string) error {
	for _, tok := range m.tokens {
		if tok.userID == userID && tok.purpose == purpose {
			tok.consumed = true
		}
	}
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
func (m *mockAuth) ConsumeTokenAndActivate(ctx context.Context, tokenHash, passwordHash, firstName, lastName string) (string, error) {
	userID, _ := m.ConsumeToken(ctx, tokenHash, "invite")
	if userID == "" {
		return "", nil
	}
	_ = m.ActivateUser(ctx, userID, passwordHash)
	if u := m.byID[userID]; u != nil {
		u.FirstName = firstName
		u.LastName = lastName
	}
	return userID, nil
}
func (m *mockAuth) ConsumeTokenAndResetPassword(ctx context.Context, tokenHash, passwordHash string) (string, error) {
	userID, _ := m.ConsumeToken(ctx, tokenHash, "password_reset")
	if userID == "" {
		return "", nil
	}
	_ = m.UpdateUserPassword(ctx, userID, passwordHash)
	_ = m.RevokeAllUserSessions(ctx, userID)
	return userID, nil
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
func (m *mockAuth) UpdateUserNames(_ context.Context, id, firstName, lastName string) error {
	if u := m.byID[id]; u != nil {
		u.FirstName = firstName
		u.LastName = lastName
	}
	return nil
}
func (m *mockAuth) CreateInvitedUser(_ context.Context, email, role string) (string, error) {
	if _, exists := m.byEmail[strings.ToLower(email)]; exists {
		return "", storage.ErrEmailExists
	}
	m.idSeq++
	id := fmt.Sprintf("u%d", m.idSeq)
	m.addUser(&storage.AuthUser{ID: id, Email: email, Role: role, Status: "invited"})
	return id, nil
}
func (m *mockAuth) ListUsers(_ context.Context) ([]storage.AdminUserRow, error) {
	out := make([]storage.AdminUserRow, 0, len(m.byID))
	for _, u := range m.byID {
		out = append(out, storage.AdminUserRow{ID: u.ID, Email: u.Email, Role: u.Role, Status: u.Status})
	}
	return out, nil
}
func (m *mockAuth) SetUserStatus(_ context.Context, id, status string) (bool, error) {
	u, ok := m.byID[id]
	if !ok {
		return false, nil
	}
	u.Status = status
	return true, nil
}
func (m *mockAuth) ExportUser(_ context.Context, id string) (*storage.UserExport, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	return &storage.UserExport{ID: u.ID, Email: u.Email, Role: u.Role, Status: u.Status}, nil
}
func (m *mockAuth) DeleteUser(_ context.Context, id string) (bool, error) {
	u, ok := m.byID[id]
	if !ok {
		return false, nil
	}
	delete(m.byID, id)
	delete(m.byEmail, strings.ToLower(u.Email))
	return true, nil
}

// --- test scaffolding --------------------------------------------------------

func authTestServer(m *mockAuth) *Server {
	s := &Server{
		authBackend:   m,
		mailer:        stubMailer{},
		loginThrottle: auth.NewLoginThrottle(5, time.Second, 5*time.Minute, 15*time.Minute),
		resetThrottle: auth.NewLoginThrottle(3, time.Second, 15*time.Minute, 15*time.Minute),
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
	// Run the forgot-password dispatch synchronously in tests for deterministic
	// assertions (production detaches it to a goroutine — SEC-019).
	s.dispatchReset = func(ctx context.Context, email string) { s.dispatchPasswordReset(ctx, email) }
	return s
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

func TestLoginThrottleReturns429AfterRepeatedFailures(t *testing.T) {
	s := authTestServer(newMockAuth()) // no users → every attempt fails
	req := PostAuthLoginRequestObject{
		Body: &PostAuthLoginJSONRequestBody{Email: openapi_types.Email("target@example.org"), Password: "wrongwrongwrong"},
	}
	// 5 free failures (all 401), then the 6th is throttled (429).
	var lastCode int
	for i := 0; i < 7; i++ {
		resp, _ := s.PostAuthLogin(context.Background(), req)
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthLoginResponse(rec)
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after repeated failures, got %d", lastCode)
	}
}

func TestLoginCorrectPasswordBypassesArmedThrottle(t *testing.T) {
	// SEC-020 — a third party arming the account-only throttle with wrong
	// guesses must not lock out the legitimate owner: a correct password always
	// succeeds and clears the throttle. (No client IP in the test context, so
	// only the email key is armed — exactly the targeted-lockout vector.)
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "hunter2hunter2"))
	s := authTestServer(m)

	wrong := PostAuthLoginRequestObject{
		Body: &PostAuthLoginJSONRequestBody{Email: openapi_types.Email("alice@example.org"), Password: "wrongwrongwrong"},
	}
	// Attacker arms the email throttle (5 free + arming attempts).
	for i := 0; i < 7; i++ {
		resp, _ := s.PostAuthLogin(context.Background(), wrong)
		_ = resp.VisitPostAuthLoginResponse(httptest.NewRecorder())
	}
	// Confirm the throttle is armed: another wrong guess is now 429.
	resp, _ := s.PostAuthLogin(context.Background(), wrong)
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthLoginResponse(rec)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected an armed throttle (429) for the wrong password, got %d", rec.Code)
	}

	// The legitimate owner's correct password must still succeed.
	resp, _ = s.PostAuthLogin(context.Background(), PostAuthLoginRequestObject{
		Body: &PostAuthLoginJSONRequestBody{Email: openapi_types.Email("alice@example.org"), Password: "hunter2hunter2"},
	})
	rec = httptest.NewRecorder()
	_ = resp.VisitPostAuthLoginResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("SEC-020: correct password must bypass the armed throttle, got %d (%s)", rec.Code, rec.Body.String())
	}
	if setCookie(rec) == nil {
		t.Fatal("expected a session cookie on the bypassing login")
	}
}

func TestMeRequiresSession(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "alice@example.org", FirstName: "Alice", LastName: "Active", Role: "researcher", Status: "active"})
	s := authTestServer(m)

	// No identity in context → 401.
	resp, _ := s.GetAuthMe(context.Background(), GetAuthMeRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitGetAuthMeResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}

	// Identity in context → 200 with the display name (Phase 148e).
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Email: "alice@example.org", Role: auth.RoleResearcher})
	resp, _ = s.GetAuthMe(ctx, GetAuthMeRequestObject{})
	rec = httptest.NewRecorder()
	_ = resp.VisitGetAuthMeResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with session, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"firstName":"Alice"`) {
		t.Fatalf("expected the display name in /me, got %s", rec.Body.String())
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
		Body: &PostAuthAcceptInviteJSONRequestBody{Token: rawTok, FirstName: "Test", LastName: "User", Password: "freshpassword123", AcceptResponsibleUse: true},
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
	if m.byID["u1"].FirstName != "Test" || m.byID["u1"].LastName != "User" {
		t.Fatalf("expected the name set at activation, got %q %q", m.byID["u1"].FirstName, m.byID["u1"].LastName)
	}
	// Token is single-use: a replay must now fail with 400 invalid_token.
	resp, _ = s.PostAuthAcceptInvite(context.Background(), PostAuthAcceptInviteRequestObject{
		Body: &PostAuthAcceptInviteJSONRequestBody{Token: rawTok, FirstName: "Test", LastName: "User", Password: "freshpassword123", AcceptResponsibleUse: true},
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

func TestAcceptInviteRejectsEmptyName(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "new@example.org", Role: "researcher", Status: "invited"})
	rawTok, hashTok, _ := auth.GenerateOpaqueToken()
	m.tokens[hashTok] = &mockToken{userID: "u1", purpose: "invite"}
	s := authTestServer(m)

	// A blank (whitespace-only) name is rejected; the token must NOT be burned.
	resp, _ := s.PostAuthAcceptInvite(context.Background(), PostAuthAcceptInviteRequestObject{
		Body: &PostAuthAcceptInviteJSONRequestBody{Token: rawTok, FirstName: "  ", LastName: "User", Password: "freshpassword123", AcceptResponsibleUse: true},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAuthAcceptInviteResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid_name, got %d", rec.Code)
	}
	if m.byID["u1"].Status == "active" {
		t.Fatal("user must not be activated with an invalid name")
	}
}

func TestPatchAuthMeUpdatesName(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "alice@example.org", FirstName: "Alice", LastName: "Active", Role: "researcher", Status: "active"})
	s := authTestServer(m)
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Email: "alice@example.org", Role: auth.RoleResearcher})

	// No session → 401.
	resp, _ := s.PatchAuthMe(context.Background(), PatchAuthMeRequestObject{
		Body: &PatchAuthMeJSONRequestBody{FirstName: "X", LastName: "Y"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPatchAuthMeResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}

	// Blank name → 400, no mutation.
	resp, _ = s.PatchAuthMe(ctx, PatchAuthMeRequestObject{Body: &PatchAuthMeJSONRequestBody{FirstName: "", LastName: "Y"}})
	rec = httptest.NewRecorder()
	_ = resp.VisitPatchAuthMeResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid_name, got %d", rec.Code)
	}
	if m.byID["u1"].FirstName != "Alice" {
		t.Fatal("a rejected edit must not mutate the name")
	}

	// Valid edit → 200, trimmed, returned + persisted.
	resp, _ = s.PatchAuthMe(ctx, PatchAuthMeRequestObject{Body: &PatchAuthMeJSONRequestBody{FirstName: " Alicia ", LastName: " Renamed "}})
	rec = httptest.NewRecorder()
	_ = resp.VisitPatchAuthMeResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if m.byID["u1"].FirstName != "Alicia" || m.byID["u1"].LastName != "Renamed" {
		t.Fatalf("expected the trimmed name persisted, got %q %q", m.byID["u1"].FirstName, m.byID["u1"].LastName)
	}
	if !strings.Contains(rec.Body.String(), `"firstName":"Alicia"`) {
		t.Fatalf("expected the updated name in the response, got %s", rec.Body.String())
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

func TestForgotPasswordThrottleKeepsSingleLiveToken(t *testing.T) {
	// SEC-006/SEC-022 — repeated forgot-password requests all return 202 (no
	// enumeration), the per-account throttle bounds issuance, and prior-token
	// invalidation keeps exactly one live reset link.
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "hunter2hunter2"))
	s := authTestServer(m)
	req := PostAuthForgotPasswordRequestObject{
		Body: &PostAuthForgotPasswordJSONRequestBody{Email: openapi_types.Email("alice@example.org")},
	}
	for i := 0; i < 6; i++ {
		resp, _ := s.PostAuthForgotPassword(context.Background(), req)
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthForgotPasswordResponse(rec)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("call %d: expected 202, got %d", i, rec.Code)
		}
	}
	live := 0
	for _, tok := range m.tokens {
		if !tok.consumed {
			live++
		}
	}
	if live != 1 {
		t.Fatalf("SEC-022: expected exactly one live reset token, got %d", live)
	}
}

func TestForgotPasswordInvalidatesPriorToken(t *testing.T) {
	// SEC-022 — issuing a new reset link consumes a prior unconsumed one.
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "hunter2hunter2"))
	m.tokens["old-hash"] = &mockToken{userID: "u1", purpose: "password_reset"}
	s := authTestServer(m)

	resp, _ := s.PostAuthForgotPassword(context.Background(), PostAuthForgotPasswordRequestObject{
		Body: &PostAuthForgotPasswordJSONRequestBody{Email: openapi_types.Email("alice@example.org")},
	})
	_ = resp.VisitPostAuthForgotPasswordResponse(httptest.NewRecorder())

	if !m.tokens["old-hash"].consumed {
		t.Fatal("a new reset must invalidate the prior unconsumed token")
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

// --- PostAuthChangePassword --------------------------------------------------

func changePwCtx(userID string) context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{UserID: userID, SessionIDHash: "sess-hash"})
}

func TestChangePassword_Success204(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "currentpass12"))
	s := authTestServer(m)

	resp, err := s.PostAuthChangePassword(changePwCtx("u1"), PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "currentpass12", NewPassword: "brandnewpass34"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(PostAuthChangePassword204Response); !ok {
		t.Fatalf("response = %T, want 204", resp)
	}
	// The new password must now verify against the stored hash.
	ok, _ := auth.VerifyPassword("brandnewpass34", m.byID["u1"].PasswordHash.String)
	if !ok {
		t.Error("stored hash does not verify against the new password")
	}
}

func TestChangePassword_NoSession401(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.PostAuthChangePassword(context.Background(), PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "x", NewPassword: "brandnewpass34"},
	})
	if _, ok := resp.(PostAuthChangePassword401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401", resp)
	}
}

func TestChangePassword_MachineIdentity401(t *testing.T) {
	s := authTestServer(newMockAuth())
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Machine: true})
	resp, _ := s.PostAuthChangePassword(ctx, PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "currentpass12", NewPassword: "brandnewpass34"},
	})
	if _, ok := resp.(PostAuthChangePassword401JSONResponse); !ok {
		t.Fatalf("machine credential must not change a password; response = %T, want 401", resp)
	}
}

func TestChangePassword_NilBody400(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.PostAuthChangePassword(changePwCtx("u1"), PostAuthChangePasswordRequestObject{Body: nil})
	if _, ok := resp.(PostAuthChangePassword400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400", resp)
	}
}

func TestChangePassword_WeakNewPassword400(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "currentpass12"))
	s := authTestServer(m)
	resp, _ := s.PostAuthChangePassword(changePwCtx("u1"), PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "currentpass12", NewPassword: "short"},
	})
	if _, ok := resp.(PostAuthChangePassword400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400 for a sub-minimum password", resp)
	}
}

func TestChangePassword_WrongCurrent401(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "currentpass12"))
	s := authTestServer(m)
	resp, _ := s.PostAuthChangePassword(changePwCtx("u1"), PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "wrongcurrent9", NewPassword: "brandnewpass34"},
	})
	if _, ok := resp.(PostAuthChangePassword401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401 for a wrong current password", resp)
	}
}

func TestChangePassword_NoPasswordHash401(t *testing.T) {
	m := newMockAuth()
	// An invited (not-yet-activated) user has no password hash.
	m.addUser(&storage.AuthUser{ID: "u1", Email: "invited@example.org", Role: "researcher", Status: "invited"})
	s := authTestServer(m)
	resp, _ := s.PostAuthChangePassword(changePwCtx("u1"), PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "anything12345", NewPassword: "brandnewpass34"},
	})
	if _, ok := resp.(PostAuthChangePassword401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401 when no password hash is set", resp)
	}
}

func TestChangePassword_UnknownUser500(t *testing.T) {
	s := authTestServer(newMockAuth()) // no users → GetUserByID returns nil
	resp, _ := s.PostAuthChangePassword(changePwCtx("ghost"), PostAuthChangePasswordRequestObject{
		Body: &PostAuthChangePasswordJSONRequestBody{CurrentPassword: "anything12345", NewPassword: "brandnewpass34"},
	})
	if _, ok := resp.(PostAuthChangePassword500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500 when the user vanished", resp)
	}
}

func TestResetPassword_Success204(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "oldpassword12"))
	rawTok, hashTok, _ := auth.GenerateOpaqueToken()
	m.tokens[hashTok] = &mockToken{userID: "u1", purpose: "password_reset"}
	s := authTestServer(m)

	resp, err := s.PostAuthResetPassword(context.Background(), PostAuthResetPasswordRequestObject{
		Body: &PostAuthResetPasswordJSONRequestBody{Token: rawTok, Password: "brandnewpass34"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(PostAuthResetPassword204Response); !ok {
		t.Fatalf("response = %T, want 204", resp)
	}
	// Reset must revoke every session (the link could be in the wrong hands).
	if len(m.revokedAll) != 1 || m.revokedAll[0] != "u1" {
		t.Errorf("revokedAll = %v, want [u1]", m.revokedAll)
	}
}

func TestResetPassword_WeakPassword400(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.PostAuthResetPassword(context.Background(), PostAuthResetPasswordRequestObject{
		Body: &PostAuthResetPasswordJSONRequestBody{Token: "anything", Password: "short"},
	})
	if _, ok := resp.(PostAuthResetPassword400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400", resp)
	}
}

func TestGetAuthMeExport_Success(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "currentpass12"))
	s := authTestServer(m)
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Email: "alice@example.org", Role: auth.RoleResearcher})

	resp, err := s.GetAuthMeExport(ctx, GetAuthMeExportRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetAuthMeExport200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.ID != "u1" || string(got.Email) != "alice@example.org" {
		t.Errorf("export = %+v, want u1/alice", got)
	}
}

func TestGetAuthMeExport_NoSession401(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.GetAuthMeExport(context.Background(), GetAuthMeExportRequestObject{})
	if _, ok := resp.(GetAuthMeExport401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401", resp)
	}
}

func TestDeleteAuthMe_Success204(t *testing.T) {
	m := newMockAuth()
	m.addUser(activeUser(t, "u1", "alice@example.org", "currentpass12"))
	s := authTestServer(m)
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", SessionIDHash: "sess"})

	resp, err := s.DeleteAuthMe(ctx, DeleteAuthMeRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec := httptest.NewRecorder()
	_ = resp.VisitDeleteAuthMeResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	// The session cookie must be cleared on account deletion.
	if c := setCookie(rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("expected a cleared session cookie, got %+v", c)
	}
}

func TestDeleteAuthMe_NoSession401(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.DeleteAuthMe(context.Background(), DeleteAuthMeRequestObject{})
	if _, ok := resp.(DeleteAuthMe401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401", resp)
	}
}

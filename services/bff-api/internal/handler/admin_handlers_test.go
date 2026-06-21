package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// adminCtx returns a context carrying an admin identity. Admin handlers now
// re-check the role in-handler as defense-in-depth (SEC-025), so a handler-level
// test must supply one (the middleware that normally injects it is exercised in
// the auth package).
func adminCtx() context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{UserID: "admin-test", Role: auth.RoleAdmin})
}

func TestAdminHandlersRejectNonAdminInHandler(t *testing.T) {
	// SEC-025 — even if a request somehow reached an admin handler without the
	// path-prefix gate (e.g. a future route misfiled outside /api/v1/admin/),
	// the in-handler check must still refuse a non-admin identity with 403.
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u2", Email: "b@example.org", Role: "researcher", Status: "active"})
	s := authTestServer(m)
	researcher := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "r", Role: auth.RoleResearcher})

	resp, _ := s.GetAdminUsers(researcher, GetAdminUsersRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitGetAdminUsersResponse(rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a researcher reaching GetAdminUsers, got %d", rec.Code)
	}

	// A machine (X-API-Key) identity is never an admin either.
	machine := auth.WithIdentity(context.Background(), &auth.Identity{Machine: true})
	resp2, _ := s.PostAdminUserResetPassword(machine, PostAdminUserResetPasswordRequestObject{ID: "u2"})
	rec = httptest.NewRecorder()
	_ = resp2.VisitPostAdminUserResetPasswordResponse(rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a machine caller reaching reset-password, got %d", rec.Code)
	}
	if len(m.revokedAll) != 0 {
		t.Fatal("a refused admin reset must not revoke any sessions")
	}
}

func TestAdminCreateUserIssuesInvite(t *testing.T) {
	m := newMockAuth()
	s := authTestServer(m)

	resp, err := s.PostAdminUsers(adminCtx(), PostAdminUsersRequestObject{
		Body: &PostAdminUsersJSONRequestBody{Email: openapi_types.Email("new@example.org"), Role: "researcher"},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUsersResponse(rec)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/accept-invite#token=") {
		t.Fatalf("expected an accept-invite link in body, got %s", rec.Body.String())
	}
	if len(m.byEmail) != 1 || len(m.tokens) != 1 {
		t.Fatalf("expected one invited user and one invite token, got users=%d tokens=%d", len(m.byEmail), len(m.tokens))
	}
}

func TestAdminCreateUserDuplicateIs409(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "taken@example.org", Role: "researcher", Status: "active"})
	s := authTestServer(m)

	resp, _ := s.PostAdminUsers(adminCtx(), PostAdminUsersRequestObject{
		Body: &PostAdminUsersJSONRequestBody{Email: openapi_types.Email("taken@example.org"), Role: "researcher"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUsersResponse(rec)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", rec.Code)
	}
}

func TestAdminCreateUserInvalidRoleIs400(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.PostAdminUsers(adminCtx(), PostAdminUsersRequestObject{
		Body: &PostAdminUsersJSONRequestBody{Email: openapi_types.Email("x@example.org"), Role: "superuser"},
	})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUsersResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d", rec.Code)
	}
}

func TestAdminListUsers(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "a@example.org", Role: "admin", Status: "active"})
	m.addUser(&storage.AuthUser{ID: "u2", Email: "b@example.org", Role: "researcher", Status: "invited"})
	s := authTestServer(m)

	resp, _ := s.GetAdminUsers(adminCtx(), GetAdminUsersRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitGetAdminUsersResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "a@example.org") || !strings.Contains(body, "b@example.org") {
		t.Fatalf("expected both users in list, got %s", body)
	}
}

func TestAdminSuspendAndReactivate(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u2", Email: "b@example.org", Role: "researcher", Status: "active"})
	s := authTestServer(m)
	// Caller is a different admin.
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "admin1", Role: auth.RoleAdmin})

	resp, _ := s.PostAdminUserSuspend(ctx, PostAdminUserSuspendRequestObject{ID: "u2"})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUserSuspendResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on suspend, got %d", rec.Code)
	}
	if m.byID["u2"].Status != "suspended" {
		t.Fatalf("expected suspended, got %s", m.byID["u2"].Status)
	}

	resp2, _ := s.PostAdminUserReactivate(ctx, PostAdminUserReactivateRequestObject{ID: "u2"})
	rec = httptest.NewRecorder()
	_ = resp2.VisitPostAdminUserReactivateResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on reactivate, got %d", rec.Code)
	}
	if m.byID["u2"].Status != "active" {
		t.Fatalf("expected active, got %s", m.byID["u2"].Status)
	}
}

func TestAdminSuspendUnknownIs404(t *testing.T) {
	s := authTestServer(newMockAuth())
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "admin1", Role: auth.RoleAdmin})
	resp, _ := s.PostAdminUserSuspend(ctx, PostAdminUserSuspendRequestObject{ID: "ghost"})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUserSuspendResponse(rec)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown user, got %d", rec.Code)
	}
}

func TestAdminCannotSuspendSelf(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "admin1", Email: "admin@example.org", Role: "admin", Status: "active"})
	s := authTestServer(m)
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "admin1", Role: auth.RoleAdmin})

	resp, _ := s.PostAdminUserSuspend(ctx, PostAdminUserSuspendRequestObject{ID: "admin1"})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUserSuspendResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (cannot suspend self), got %d", rec.Code)
	}
	if m.byID["admin1"].Status != "active" {
		t.Fatal("self must not be suspended")
	}
}

func TestAdminResetPasswordIssuesLink(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u2", Email: "b@example.org", Role: "researcher", Status: "active"})
	s := authTestServer(m)

	resp, _ := s.PostAdminUserResetPassword(adminCtx(), PostAdminUserResetPasswordRequestObject{ID: "u2"})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUserResetPasswordResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/reset-password#token=") {
		t.Fatalf("expected a reset link, got %s", rec.Body.String())
	}
	// SEC-008 — the admin reset must also revoke the target's live sessions so
	// a compromised account is locked out immediately, not only on link use.
	if len(m.revokedAll) != 1 || m.revokedAll[0] != "u2" {
		t.Fatalf("expected admin reset to revoke u2's sessions, got %v", m.revokedAll)
	}

	// Unknown user → 404.
	resp2, _ := s.PostAdminUserResetPassword(adminCtx(), PostAdminUserResetPasswordRequestObject{ID: "ghost"})
	rec = httptest.NewRecorder()
	_ = resp2.VisitPostAdminUserResetPasswordResponse(rec)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown user, got %d", rec.Code)
	}
}

// failingMailer simulates a relay outage (Phase 153 graceful-failure path).
type failingMailer struct{}

func (failingMailer) SendInvite(context.Context, string, string) error {
	return errors.New("relay unreachable")
}
func (failingMailer) SendPasswordReset(context.Context, string, string) error {
	return errors.New("relay unreachable")
}

// TestAdminCreateUserDeliveredFlag covers the Phase 153 `delivered` signal:
// a real relay that accepts the message reports delivered=true; a relay outage
// reports delivered=false but STILL returns the one-time link (break-glass), so
// the invite is never silently dropped.
func TestAdminCreateUserDeliveredFlag(t *testing.T) {
	t.Run("real relay succeeds → delivered true", func(t *testing.T) {
		s := authTestServer(newMockAuth())
		s.emailEnabled = true // stubMailer returns nil

		resp, err := s.PostAdminUsers(adminCtx(), PostAdminUsersRequestObject{
			Body: &PostAdminUsersJSONRequestBody{Email: openapi_types.Email("ok@example.org"), Role: "researcher"},
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		r, ok := resp.(PostAdminUsers201JSONResponse)
		if !ok {
			t.Fatalf("expected 201 response, got %T", resp)
		}
		if r.Delivered == nil || !*r.Delivered {
			t.Errorf("Delivered = %v, want true", r.Delivered)
		}
	})

	t.Run("relay outage → delivered false, link still returned", func(t *testing.T) {
		s := authTestServer(newMockAuth())
		s.mailer = failingMailer{}
		s.emailEnabled = true

		resp, err := s.PostAdminUsers(adminCtx(), PostAdminUsersRequestObject{
			Body: &PostAdminUsersJSONRequestBody{Email: openapi_types.Email("down@example.org"), Role: "researcher"},
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		r, ok := resp.(PostAdminUsers201JSONResponse)
		if !ok {
			t.Fatalf("expected 201 response, got %T", resp)
		}
		if r.Delivered == nil || *r.Delivered {
			t.Errorf("Delivered = %v, want false on send failure", r.Delivered)
		}
		if !strings.Contains(r.Link, "/accept-invite#token=") {
			t.Errorf("break-glass link missing on send failure: %q", r.Link)
		}
	})

	t.Run("LogSender fallback → delivered false", func(t *testing.T) {
		s := authTestServer(newMockAuth()) // emailEnabled stays false
		resp, err := s.PostAdminUsers(adminCtx(), PostAdminUsersRequestObject{
			Body: &PostAdminUsersJSONRequestBody{Email: openapi_types.Email("log@example.org"), Role: "researcher"},
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		r := resp.(PostAdminUsers201JSONResponse)
		if r.Delivered == nil || *r.Delivered {
			t.Errorf("Delivered = %v, want false for LogSender fallback", r.Delivered)
		}
	})
}

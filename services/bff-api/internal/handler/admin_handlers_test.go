package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func TestAdminCreateUserIssuesInvite(t *testing.T) {
	m := newMockAuth()
	s := authTestServer(m)

	resp, err := s.PostAdminUsers(context.Background(), PostAdminUsersRequestObject{
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
	if !strings.Contains(rec.Body.String(), "/accept-invite?token=") {
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

	resp, _ := s.PostAdminUsers(context.Background(), PostAdminUsersRequestObject{
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
	resp, _ := s.PostAdminUsers(context.Background(), PostAdminUsersRequestObject{
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

	resp, _ := s.GetAdminUsers(context.Background(), GetAdminUsersRequestObject{})
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

	resp, _ := s.PostAdminUserSuspend(ctx, PostAdminUserSuspendRequestObject{Id: "u2"})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUserSuspendResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on suspend, got %d", rec.Code)
	}
	if m.byID["u2"].Status != "suspended" {
		t.Fatalf("expected suspended, got %s", m.byID["u2"].Status)
	}

	resp2, _ := s.PostAdminUserReactivate(ctx, PostAdminUserReactivateRequestObject{Id: "u2"})
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
	resp, _ := s.PostAdminUserSuspend(ctx, PostAdminUserSuspendRequestObject{Id: "ghost"})
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

	resp, _ := s.PostAdminUserSuspend(ctx, PostAdminUserSuspendRequestObject{Id: "admin1"})
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

	resp, _ := s.PostAdminUserResetPassword(context.Background(), PostAdminUserResetPasswordRequestObject{Id: "u2"})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAdminUserResetPasswordResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/reset-password?token=") {
		t.Fatalf("expected a reset link, got %s", rec.Body.String())
	}

	// Unknown user → 404.
	resp2, _ := s.PostAdminUserResetPassword(context.Background(), PostAdminUserResetPasswordRequestObject{Id: "ghost"})
	rec = httptest.NewRecorder()
	_ = resp2.VisitPostAdminUserResetPasswordResponse(rec)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown user, got %d", rec.Code)
	}
}

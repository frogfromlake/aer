package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func TestExportRequiresSession(t *testing.T) {
	s := authTestServer(newMockAuth())

	resp, _ := s.GetAuthMeExport(context.Background(), GetAuthMeExportRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitGetAuthMeExportResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}
}

func TestExportReturnsUserData(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "alice@example.org", Role: "researcher", Status: "active"})
	s := authTestServer(m)

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Email: "alice@example.org", Role: auth.RoleResearcher})
	resp, _ := s.GetAuthMeExport(ctx, GetAuthMeExportRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitGetAuthMeExportResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "alice@example.org") {
		t.Fatalf("expected email in export, got %s", rec.Body.String())
	}
}

func TestDeleteAccountClearsCookieAndRemovesUser(t *testing.T) {
	m := newMockAuth()
	m.addUser(&storage.AuthUser{ID: "u1", Email: "alice@example.org", Role: "researcher", Status: "active"})
	s := authTestServer(m)

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", SessionIDHash: "h"})
	resp, _ := s.DeleteAuthMe(ctx, DeleteAuthMeRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitDeleteAuthMeResponse(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	c := setCookie(rec)
	if c == nil || c.MaxAge >= 0 {
		t.Fatalf("expected a cleared cookie, got %+v", c)
	}
	if _, ok := m.byID["u1"]; ok {
		t.Fatal("expected the user to be deleted")
	}
}

func TestDeleteRequiresSession(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.DeleteAuthMe(context.Background(), DeleteAuthMeRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitDeleteAuthMeResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}
}

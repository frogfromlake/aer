package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// All WebAuthn endpoints require a real user session; a request with no
// identity in context must be rejected before any backend is touched (so these
// run without a WebAuthn backend wired).
func TestWebAuthnEndpointsRequireSession(t *testing.T) {
	s := &Server{authConfig: AuthConfig{CookieName: "aer_session"}}
	ctx := context.Background()

	t.Run("register begin", func(t *testing.T) {
		resp, _ := s.PostAuthWebauthnRegisterBegin(ctx, PostAuthWebauthnRegisterBeginRequestObject{})
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthWebauthnRegisterBeginResponse(rec)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
	t.Run("register finish", func(t *testing.T) {
		resp, _ := s.PostAuthWebauthnRegisterFinish(ctx, PostAuthWebauthnRegisterFinishRequestObject{})
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthWebauthnRegisterFinishResponse(rec)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
	t.Run("list credentials", func(t *testing.T) {
		resp, _ := s.GetAuthWebauthnCredentials(ctx, GetAuthWebauthnCredentialsRequestObject{})
		rec := httptest.NewRecorder()
		_ = resp.VisitGetAuthWebauthnCredentialsResponse(rec)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
	t.Run("delete credential", func(t *testing.T) {
		resp, _ := s.DeleteAuthWebauthnCredential(ctx, DeleteAuthWebauthnCredentialRequestObject{Id: "x"})
		rec := httptest.NewRecorder()
		_ = resp.VisitDeleteAuthWebauthnCredentialResponse(rec)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
	t.Run("assert begin", func(t *testing.T) {
		resp, _ := s.PostAuthWebauthnAssertBegin(ctx, PostAuthWebauthnAssertBeginRequestObject{})
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthWebauthnAssertBeginResponse(rec)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
	t.Run("assert finish", func(t *testing.T) {
		resp, _ := s.PostAuthWebauthnAssertFinish(ctx, PostAuthWebauthnAssertFinishRequestObject{})
		rec := httptest.NewRecorder()
		_ = resp.VisitPostAuthWebauthnAssertFinishResponse(rec)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

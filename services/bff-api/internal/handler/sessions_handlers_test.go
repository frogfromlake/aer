package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
)

func TestGetAuthSessionsListsWithCurrentFlag(t *testing.T) {
	// SEC-005 — the user sees their own active sessions, with the requesting
	// session marked `current`. The id hash is never exposed in the response.
	m := newMockAuth()
	m.sessions["hash-this"] = "u1"
	m.sessions["hash-other"] = "u1"
	m.sessions["hash-someone-else"] = "u2"
	s := authTestServer(m)

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{
		UserID: "u1", Role: auth.RoleResearcher, SessionIDHash: "hash-this",
	})
	resp, err := s.GetAuthSessions(ctx, GetAuthSessionsRequestObject{})
	if err != nil {
		t.Fatalf("GetAuthSessions: %v", err)
	}
	out, ok := resp.(GetAuthSessions200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if len(out.Sessions) != 2 {
		t.Fatalf("expected 2 sessions for u1, got %d", len(out.Sessions))
	}
	currentCount := 0
	for _, sess := range out.Sessions {
		if sess.Current {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly one session marked current, got %d", currentCount)
	}

	// The raw id hash must never appear in the serialized body.
	rec := httptest.NewRecorder()
	_ = out.VisitGetAuthSessionsResponse(rec)
	if body := rec.Body.String(); strings.Contains(body, "hash-this") || strings.Contains(body, "hash-other") {
		t.Fatalf("session id hash leaked into the response body: %s", body)
	}
}

func TestGetAuthSessionsRejectsMachineAndAnonymous(t *testing.T) {
	s := authTestServer(newMockAuth())

	// No identity → 401.
	resp, _ := s.GetAuthSessions(context.Background(), GetAuthSessionsRequestObject{})
	if _, ok := resp.(GetAuthSessions401JSONResponse); !ok {
		t.Fatalf("expected 401 without identity, got %T", resp)
	}

	// Machine (X-API-Key) identity has no user → 401.
	mctx := auth.WithIdentity(context.Background(), &auth.Identity{Machine: true})
	resp2, _ := s.GetAuthSessions(mctx, GetAuthSessionsRequestObject{})
	if _, ok := resp2.(GetAuthSessions401JSONResponse); !ok {
		t.Fatalf("expected 401 for a machine caller, got %T", resp2)
	}
}

func TestDeleteAuthSessionsRevokesAllAndClearsCookie(t *testing.T) {
	// SEC-005 — log out everywhere: revoke all of the user's sessions and clear
	// the current session cookie.
	m := newMockAuth()
	s := authTestServer(m)
	ctx := auth.WithIdentity(context.Background(), &auth.Identity{
		UserID: "u1", Role: auth.RoleResearcher, SessionIDHash: "hash-this",
	})

	resp, _ := s.DeleteAuthSessions(ctx, DeleteAuthSessionsRequestObject{})
	rec := httptest.NewRecorder()
	_ = resp.VisitDeleteAuthSessionsResponse(rec)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if len(m.revokedAll) != 1 || m.revokedAll[0] != "u1" {
		t.Fatalf("expected all u1 sessions revoked, got %v", m.revokedAll)
	}
	c := setCookie(rec)
	if c == nil || c.MaxAge != -1 || c.Value != "" {
		t.Fatalf("expected the session cookie to be cleared, got %+v", c)
	}
}

func TestDeleteAuthSessionsRejectsAnonymous(t *testing.T) {
	s := authTestServer(newMockAuth())
	resp, _ := s.DeleteAuthSessions(context.Background(), DeleteAuthSessionsRequestObject{})
	if _, ok := resp.(DeleteAuthSessions401JSONResponse); !ok {
		t.Fatalf("expected 401 without identity, got %T", resp)
	}
}

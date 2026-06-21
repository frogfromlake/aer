package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stubValidator is a SessionValidator that returns a fixed identity/error,
// recording the idHash it was asked to validate.
type stubValidator struct {
	id         *Identity
	err        error
	seenIDHash string
}

func (s *stubValidator) ValidateAndTouchSession(_ context.Context, idHash string, _ time.Duration) (*Identity, error) {
	s.seenIDHash = idHash
	return s.id, s.err
}

func okNext(marker *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if marker != nil {
			*marker = true
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestClientIPFromRequest(t *testing.T) {
	cases := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{"x-forwarded-for left-most wins", "203.0.113.7, 10.0.0.1", "10.0.0.2:5555", "203.0.113.7"},
		{"x-forwarded-for trims whitespace", "  198.51.100.9  ", "10.0.0.2:5555", "198.51.100.9"},
		{"remote addr host:port fallback", "", "192.0.2.4:443", "192.0.2.4"},
		{"remote addr without port returned verbatim", "", "192.0.2.55", "192.0.2.55"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := clientIPFromRequest(req); got != tc.want {
				t.Fatalf("clientIPFromRequest = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientIP_InjectsAndRoundTrips(t *testing.T) {
	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
	rec := httptest.NewRecorder()

	ClientIP(next).ServeHTTP(rec, req)

	if seen != "203.0.113.42" {
		t.Fatalf("ClientIPFromContext = %q, want %q", seen, "203.0.113.42")
	}
}

func TestClientIPFromContext_AbsentIsEmpty(t *testing.T) {
	if got := ClientIPFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty IP for bare context, got %q", got)
	}
}

func TestApiKeyFromRequest(t *testing.T) {
	cases := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{"x-api-key wins", "X-API-Key", "secret-key", "secret-key"},
		{"bearer authorization", "Authorization", "Bearer tok-123", "tok-123"},
		{"non-bearer authorization ignored", "Authorization", "Basic abc", ""},
		{"no credential", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			if got := apiKeyFromRequest(req); got != tc.want {
				t.Fatalf("apiKeyFromRequest = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApiKeyFromRequest_XAPIKeyPreferredOverBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "from-header")
	req.Header.Set("Authorization", "Bearer from-bearer")
	if got := apiKeyFromRequest(req); got != "from-header" {
		t.Fatalf("expected X-API-Key to win, got %q", got)
	}
}

func TestSessionOrAPIKey_ExemptPathBypassesAuth(t *testing.T) {
	v := &stubValidator{} // would return (nil,nil) → 401 if consulted
	cfg := MiddlewareConfig{
		APIKey:      "machine-key",
		CookieName:  "__Host-aer_session",
		IdleTTL:     time.Hour,
		ExemptPaths: []string{"/api/v1/auth/login", "/api/v1/healthz"},
	}
	var reached bool
	mw := SessionOrAPIKey(v, cfg)(okNext(&reached))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !reached {
		t.Fatalf("exempt path should bypass auth: code=%d reached=%v", rec.Code, reached)
	}
	if v.seenIDHash != "" {
		t.Fatal("exempt path must not consult the session validator")
	}
}

func TestSessionOrAPIKey_CraftedSuffixPathIsNotExempt(t *testing.T) {
	// SEC-013 — a path that merely *ends* in an exempt token must NOT bypass
	// the gate. `/api/v1/articles/healthz` is not the health probe; with no
	// credential it must 401, never reach the handler, and never be treated as
	// exempt (the old unanchored HasSuffix match let it through).
	v := &stubValidator{}
	cfg := MiddlewareConfig{
		APIKey:      "machine-key",
		CookieName:  "__Host-aer_session",
		IdleTTL:     time.Hour,
		ExemptPaths: []string{"/api/v1/healthz", "/api/v1/readyz"},
	}
	var reached bool
	mw := SessionOrAPIKey(v, cfg)(okNext(&reached))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/healthz", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("crafted suffix path must be gated (401), got %d", rec.Code)
	}
	if reached {
		t.Fatal("crafted suffix path must not reach the handler unauthenticated")
	}
	if v.seenIDHash != "" {
		t.Fatal("crafted suffix path must not be treated as exempt")
	}
}

func TestSessionOrAPIKey_ValidSessionCookie(t *testing.T) {
	want := &Identity{UserID: "u-1", Email: "a@b.c", Role: RoleResearcher}
	v := &stubValidator{id: want}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", IdleTTL: 30 * time.Minute}

	var gotID *Identity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := IdentityFromContext(r.Context())
		gotID = id
		w.WriteHeader(http.StatusOK)
	})
	mw := SessionOrAPIKey(v, cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-aer_session", Value: "raw-session-token"})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid session, got %d", rec.Code)
	}
	if gotID != want {
		t.Fatalf("expected injected identity %+v, got %+v", want, gotID)
	}
	if v.seenIDHash != HashOpaqueToken("raw-session-token") {
		t.Fatalf("middleware must hash the cookie value before lookup: got %q", v.seenIDHash)
	}
}

func TestSessionOrAPIKey_SessionValidationInfraErrorIs500(t *testing.T) {
	v := &stubValidator{err: errors.New("db down")}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "machine-key", IdleTTL: time.Hour}

	var reached bool
	mw := SessionOrAPIKey(v, cfg)(okNext(&reached))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-aer_session", Value: "raw"})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("infra error must surface as 500, got %d", rec.Code)
	}
	if reached {
		t.Fatal("handler must not run after an infra error")
	}
}

func TestSessionOrAPIKey_InvalidCookieFallsThroughToKey(t *testing.T) {
	// Cookie present but the session is missing/expired → (nil,nil); a valid
	// X-API-Key must still authenticate (machine fallback).
	v := &stubValidator{id: nil}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "machine-key", IdleTTL: time.Hour}

	var gotID *Identity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := IdentityFromContext(r.Context())
		gotID = id
		w.WriteHeader(http.StatusOK)
	})
	mw := SessionOrAPIKey(v, cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-aer_session", Value: "stale-token"})
	req.Header.Set("X-API-Key", "machine-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected key fallback to authenticate, got %d", rec.Code)
	}
	if gotID == nil || !gotID.Machine {
		t.Fatalf("expected a machine identity from key fallback, got %+v", gotID)
	}
}

func TestSessionOrAPIKey_ValidAPIKeyNoCookie(t *testing.T) {
	v := &stubValidator{} // never consulted
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "machine-key", IdleTTL: time.Hour}

	var gotID *Identity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := IdentityFromContext(r.Context())
		gotID = id
		w.WriteHeader(http.StatusOK)
	})
	mw := SessionOrAPIKey(v, cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "machine-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid API key, got %d", rec.Code)
	}
	if gotID == nil || !gotID.Machine {
		t.Fatalf("expected machine identity, got %+v", gotID)
	}
	if v.seenIDHash != "" {
		t.Fatal("no cookie present: validator must not be consulted")
	}
}

func TestSessionOrAPIKey_WrongAPIKeyIs401(t *testing.T) {
	v := &stubValidator{}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "right-key", IdleTTL: time.Hour}

	var reached bool
	mw := SessionOrAPIKey(v, cfg)(okNext(&reached))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong key, got %d", rec.Code)
	}
	if reached {
		t.Fatal("handler must not run on a wrong key")
	}
}

func TestSessionOrAPIKey_NoCredentialIs401(t *testing.T) {
	v := &stubValidator{}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "machine-key", IdleTTL: time.Hour}

	var reached bool
	mw := SessionOrAPIKey(v, cfg)(okNext(&reached))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no credential, got %d", rec.Code)
	}
	if reached {
		t.Fatal("handler must not run without a credential")
	}
}

func TestSessionOrAPIKey_EmptyCookieValueSkippedThenKeyTried(t *testing.T) {
	// An empty cookie value must not be hashed/looked up; with key auth
	// disabled (APIKey=="") the request is 401.
	v := &stubValidator{id: &Identity{UserID: "should-not-happen"}}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "", IdleTTL: time.Hour}

	mw := SessionOrAPIKey(v, cfg)(okNext(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-aer_session", Value: ""})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty cookie + no key must be 401, got %d", rec.Code)
	}
	if v.seenIDHash != "" {
		t.Fatal("empty cookie value must not be looked up")
	}
}

func TestSessionOrAPIKey_KeyDisabledWhenConfigEmpty(t *testing.T) {
	// APIKey == "" disables key auth even if the caller sends X-API-Key.
	v := &stubValidator{}
	cfg := MiddlewareConfig{CookieName: "__Host-aer_session", APIKey: "", IdleTTL: time.Hour}

	mw := SessionOrAPIKey(v, cfg)(okNext(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "anything")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("key auth disabled: expected 401, got %d", rec.Code)
	}
}

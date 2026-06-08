package auth

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type clientIPCtxKey struct{}

// ClientIP injects the client IP into the request context so strict handlers
// (which receive no *http.Request) can key the login throttle by IP. Traefik is
// the sole ingress and sets X-Forwarded-For; the left-most entry is the
// original client. Falls back to RemoteAddr.
func ClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPCtxKey{}, clientIPFromRequest(r))))
	})
}

// ClientIPFromContext returns the injected client IP, or "" if absent.
func ClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPCtxKey{}).(string)
	return ip
}

func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// SessionValidator validates a session by its hashed id and slides its idle
// expiry, returning the authenticated identity. storage.AuthStore implements
// it. A (nil, nil) return means the session is missing / expired / revoked or
// the user is not active — i.e. authentication failed without an error.
type SessionValidator interface {
	ValidateAndTouchSession(ctx context.Context, idHash string, idleTTL time.Duration) (*Identity, error)
}

// MiddlewareConfig configures SessionOrAPIKey.
type MiddlewareConfig struct {
	// APIKey is the machine credential (ADR-040: demoted from the browser
	// path). Empty disables key auth entirely.
	APIKey string
	// CookieName is the session cookie name (`__Host-aer_session`, or
	// `aer_session` when SecureCookies is off).
	CookieName string
	// IdleTTL is how far each authenticated request slides the idle window.
	IdleTTL time.Duration
	// ExemptSuffixes are path suffixes that bypass auth entirely (health
	// probes + the pre-auth endpoints: login, accept-invite, forgot/reset
	// password).
	ExemptSuffixes []string
}

// SessionOrAPIKey authenticates every request by EITHER a valid session cookie
// OR a valid X-API-Key, injecting the resulting Identity into the context. The
// browser path carries only the opaque cookie; the key is the machine path.
// This is the whole-app gate (ADR-040 / LICENSE §4c).
func SessionOrAPIKey(v SessionValidator, cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, sfx := range cfg.ExemptSuffixes {
				if strings.HasSuffix(r.URL.Path, sfx) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 1) Session cookie (the browser path).
			if c, err := r.Cookie(cfg.CookieName); err == nil && c.Value != "" {
				idHash := HashOpaqueToken(c.Value)
				id, vErr := v.ValidateAndTouchSession(r.Context(), idHash, cfg.IdleTTL)
				if vErr != nil {
					// Infra failure — do not silently log the user out; 500.
					slog.Error("session validation failed", "error", vErr)
					writeJSON(w, http.StatusInternalServerError, `{"error":"internal server error"}`)
					return
				}
				if id != nil {
					next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
					return
				}
				// Cookie present but invalid → fall through to key, then 401.
			}

			// 2) Machine X-API-Key (CI / internal callers).
			if cfg.APIKey != "" {
				if key := apiKeyFromRequest(r); key != "" &&
					subtle.ConstantTimeCompare([]byte(key), []byte(cfg.APIKey)) == 1 {
					next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), &Identity{Machine: true})))
					return
				}
			}

			writeJSON(w, http.StatusUnauthorized, `{"error":"unauthorized"}`)
		})
	}
}

// RequireAdminForSegment gates any request whose path contains `segment` (e.g.
// "/admin/") behind the admin role. It runs AFTER SessionOrAPIKey, so the
// identity is already in context. Machine (X-API-Key) callers are NOT admins —
// admin is a user concept; user management requires an admin session.
func RequireAdminForSegment(segment string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, segment) {
				id, ok := IdentityFromContext(r.Context())
				if !ok || id.Machine || id.Role != RoleAdmin {
					writeJSON(w, http.StatusForbidden, `{"code":"forbidden_role","message":"admin role required"}`)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// apiKeyFromRequest extracts the key from X-API-Key or `Authorization: Bearer`,
// mirroring pkg/middleware/apikey.go.
func apiKeyFromRequest(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

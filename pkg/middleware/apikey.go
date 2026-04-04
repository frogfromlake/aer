package middleware

import (
	"net/http"
	"strings"
)

// APIKeyAuth returns a middleware that requires a valid API key on all routes
// except /healthz and /readyz, which must remain unauthenticated for probes.
// The key is accepted via the X-API-Key header or Authorization: Bearer <key>.
func APIKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/healthz") || strings.HasSuffix(path, "/readyz") {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-API-Key")
			if token == "" {
				if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
					token = strings.TrimPrefix(bearer, "Bearer ")
				}
			}

			if token == "" || token != apiKey {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

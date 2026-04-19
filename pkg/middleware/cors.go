package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// NewCORSHandler returns a chi-compatible CORS middleware pre-configured for AĒR
// services. allowedOrigins comes from CORS_ALLOWED_ORIGINS; allowedMethods should
// list the HTTP verbs the service actually exposes (e.g. ["GET","OPTIONS"] for
// read-only APIs, add "POST" for write paths).
func NewCORSHandler(allowedOrigins, allowedMethods []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   allowedMethods,
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-API-Key"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}

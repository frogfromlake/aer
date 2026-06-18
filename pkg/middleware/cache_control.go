package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CacheControlForPaths returns a middleware that sets a Cache-Control
// response header for GET requests whose URL path begins with any of
// the given prefixes. Other requests pass through untouched.
//
// Phase 122j J3 — the BFF's `/content/*` responses are derived from
// versioned YAML files loaded at service startup and only change when
// an operator commits new catalog content + restarts the BFF. They are
// hit on every Cell mount in the dashboard, so a long browser cache
// significantly cuts request volume without sacrificing correctness:
// when content changes, the operator restarts the BFF; the version is
// also surfaced as `contentVersion` in the response body for clients
// that want stronger validation.
//
// The header is only attached on success-class responses (2xx); 4xx
// and 5xx pass through with no Cache-Control so clients re-try on
// recovery without stale negative cache.
func CacheControlForPaths(maxAge time.Duration, pathPrefixes ...string) func(http.Handler) http.Handler {
	directive := fmt.Sprintf("public, max-age=%d, must-revalidate", int(maxAge.Seconds()))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			matched := false
			for _, prefix := range pathPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					matched = true
					break
				}
			}
			if !matched {
				next.ServeHTTP(w, r)
				return
			}
			ww := &cacheControlResponseWriter{ResponseWriter: w, directive: directive}
			next.ServeHTTP(ww, r)
		})
	}
}

// cacheControlResponseWriter intercepts WriteHeader so Cache-Control is
// set ONLY on 2xx responses. Setting the header in the middleware
// before invoking the inner handler would taint 4xx/5xx responses as
// cacheable, which is wrong for refusals (e.g. a transient 502 should
// not be cached for a day).
type cacheControlResponseWriter struct {
	http.ResponseWriter
	directive   string
	wroteHeader bool
}

// WriteHeader attaches the Cache-Control directive only when the status is
// 2xx, then delegates to the wrapped writer. It runs once per response.
func (w *cacheControlResponseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		if status >= 200 && status < 300 {
			w.ResponseWriter.Header().Set("Cache-Control", w.directive)
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write implicitly emits a 200 (and thus the directive) on first use for
// handlers that write a body without calling WriteHeader explicitly.
func (w *cacheControlResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

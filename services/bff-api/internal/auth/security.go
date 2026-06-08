package auth

import "net/http"

// SecurityHeaders adds HSTS to every response (ADR-040). The BFF sits behind
// Traefik TLS termination, so the client connection is HTTPS and the header
// travels back to the browser. CSP Level 3 + Trusted Types are the dashboard's
// (nginx) responsibility — they govern the HTML document, not JSON responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 2 years, includeSubDomains. No `preload` until the operator opts in
		// to the HSTS preload list deliberately.
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// FetchMetadataCSRF rejects cross-site state-changing requests using the Fetch
// Metadata `Sec-Fetch-Site` header — listed by the OWASP CSRF Prevention Cheat
// Sheet (2025) as a complete, standalone defense. It is defense-in-depth on top
// of the SameSite=Strict session cookie (ADR-040).
//
// Browsers set Sec-Fetch-Site automatically; legitimate same-origin SPA calls
// carry `same-origin`. Non-browser clients (machine X-API-Key callers, curl,
// CI smoke tests) send NO Sec-Fetch-Site header and are unaffected. Only
// `cross-site` is blocked (per OWASP) — `same-origin`, `same-site`, `none` and
// absent all pass. Safe methods (GET/HEAD/OPTIONS, incl. CORS preflight) pass.
func FetchMetadataCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUnsafeMethod(r.Method) && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeJSON(w, http.StatusForbidden, `{"code":"cross_site_blocked","message":"cross-site request blocked"}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

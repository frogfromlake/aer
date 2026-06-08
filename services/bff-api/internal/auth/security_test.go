package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchMetadataCSRF(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := FetchMetadataCSRF(next)

	cases := []struct {
		name   string
		method string
		site   string // Sec-Fetch-Site; "" = header absent
		want   int
	}{
		{"cross-site POST blocked", http.MethodPost, "cross-site", http.StatusForbidden},
		{"same-origin POST allowed", http.MethodPost, "same-origin", http.StatusOK},
		{"same-site POST allowed", http.MethodPost, "same-site", http.StatusOK},
		{"none POST allowed", http.MethodPost, "none", http.StatusOK},
		{"machine POST (no header) allowed", http.MethodPost, "", http.StatusOK},
		{"cross-site GET allowed (safe method)", http.MethodGet, "cross-site", http.StatusOK},
		{"cross-site DELETE blocked", http.MethodDelete, "cross-site", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/v1/auth/login", nil)
			if tc.site != "" {
				req.Header.Set("Sec-Fetch-Site", tc.site)
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, rec.Code)
			}
		})
	}
}

func TestSecurityHeadersSetsHSTS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	rec := httptest.NewRecorder()
	SecurityHeaders(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil))
	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("expected HSTS header to be set")
	}
}

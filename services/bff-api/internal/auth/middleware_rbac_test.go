package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdminForPrefix(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireAdminForPrefix("/api/v1/admin/")(next)

	cases := []struct {
		name     string
		path     string
		identity *Identity
		want     int
	}{
		{"admin reaches admin path", "/api/v1/admin/users", &Identity{UserID: "a", Role: RoleAdmin}, http.StatusOK},
		{"researcher blocked", "/api/v1/admin/users", &Identity{UserID: "r", Role: RoleResearcher}, http.StatusForbidden},
		{"machine blocked", "/api/v1/admin/users", &Identity{Machine: true}, http.StatusForbidden},
		{"no identity blocked", "/api/v1/admin/users", nil, http.StatusForbidden},
		{"non-admin path passes for researcher", "/api/v1/metrics", &Identity{UserID: "r", Role: RoleResearcher}, http.StatusOK},
		{"non-admin path passes with no identity", "/api/v1/metrics", nil, http.StatusOK},
		// SEC-026 — a content path that merely embeds "admin" must NOT be forced
		// through the admin gate (the old substring match over-blocked it).
		{"content/admin not gated for researcher", "/api/v1/content/admin/x", &Identity{UserID: "r", Role: RoleResearcher}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.identity != nil {
				req = req.WithContext(WithIdentity(req.Context(), tc.identity))
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("path %s: expected %d, got %d", tc.path, tc.want, rec.Code)
			}
		})
	}
}

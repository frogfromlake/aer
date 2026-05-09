package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveSourceID_URLEncodesSpecialCharacters(t *testing.T) {
	tests := []struct {
		name         string
		feedName     string
		wantEncoded  string
		responseID   int
		responseName string
	}{
		{
			name:         "spaces and ampersand",
			feedName:     "Süddeutsche Zeitung & More",
			wantEncoded:  "S%C3%BCddeutsche+Zeitung+%26+More",
			responseID:   42,
			responseName: "Süddeutsche Zeitung & More",
		},
		{
			name:         "hash and question mark",
			feedName:     "Feed #1?v=2",
			wantEncoded:  "Feed+%231%3Fv%3D2",
			responseID:   7,
			responseName: "Feed #1?v=2",
		},
		{
			name:         "plain ASCII no encoding needed",
			feedName:     "tagesschau",
			wantEncoded:  "tagesschau",
			responseID:   1,
			responseName: "tagesschau",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(sourceResponse{
					ID:   tc.responseID,
					Name: tc.responseName,
				})
			}))
			defer srv.Close()

			id, err := resolveSourceID(context.Background(), srv.URL, tc.feedName, "test-key")
			if err != nil {
				t.Fatalf("resolveSourceID returned error: %v", err)
			}
			if id != tc.responseID {
				t.Errorf("got ID %d, want %d", id, tc.responseID)
			}
			if gotQuery != "name="+tc.wantEncoded {
				t.Errorf("got query %q, want %q", gotQuery, "name="+tc.wantEncoded)
			}
		})
	}
}

func TestResolveSourceID_SetsAPIKeyHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceResponse{ID: 1, Name: "test"})
	}))
	defer srv.Close()

	_, err := resolveSourceID(context.Background(), srv.URL, "test", "my-secret-key")
	if err != nil {
		t.Fatalf("resolveSourceID returned error: %v", err)
	}
	if gotKey != "my-secret-key" {
		t.Errorf("got X-API-Key %q, want %q", gotKey, "my-secret-key")
	}
}

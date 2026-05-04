// Phase 118a / ADR-024 — language-validator coverage for the BFF.
//
// Verifies that `?language=` rejects unknown codes with the structured
// invalid_language refusal payload, and accepts manifest-declared codes.

package handler

import (
	"context"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// stubManifest builds a minimal in-memory manifest for handler tests so the
// validator can fire without touching the on-disk YAML.
func stubManifest(codes ...string) *config.LanguageManifest {
	entries := make(map[string]config.LanguageManifestEntry, len(codes))
	for _, code := range codes {
		entries[code] = config.LanguageManifestEntry{IsoCode: code, DisplayName: code}
	}
	return &config.LanguageManifest{
		ManifestVersion: 1,
		Languages:       entries,
	}
}

func newServerWithManifest(store Store, m *config.LanguageManifest) *Server {
	return NewServerWithOptions(store, nil, nil, nil, nil, ServerOptions{
		LanguageManifest: m,
	})
}

func TestGetLanguages_RejectsUnknownLanguageWithRefusalPayload(t *testing.T) {
	s := newServerWithManifest(&mockStore{}, stubManifest("de", "fr"))

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	bogus := "xx"

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{
			StartDate: start,
			EndDate:   end,
			Language:  &bogus,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body, ok := resp.(GetLanguages400JSONResponse)
	if !ok {
		t.Fatalf("expected GetLanguages400JSONResponse, got %T", resp)
	}
	if body.Gate == nil || *body.Gate != string(InvalidLanguage) {
		t.Fatalf("expected gate=invalid_language, got %+v", body.Gate)
	}
	if body.Alternatives == nil {
		t.Fatalf("expected alternatives to be populated")
	}
	alts := *body.Alternatives
	// Sorted manifest keys.
	if len(alts) != 2 || alts[0] != "de" || alts[1] != "fr" {
		t.Fatalf("expected alternatives=[de fr], got %v", alts)
	}
	if body.WorkingPaperAnchor == nil || *body.WorkingPaperAnchor == "" {
		t.Fatalf("expected workingPaperAnchor to be populated")
	}
}

func TestGetLanguages_AcceptsKnownLanguage(t *testing.T) {
	store := &mockStore{
		languageDetections: []storage.LanguageDetectionRow{
			{DetectedLanguage: "de", Count: 1, AvgConfidence: 0.9, Sources: []string{"x"}},
		},
	}
	s := newServerWithManifest(store, stubManifest("de"))

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	known := "de"

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{
			StartDate: start,
			EndDate:   end,
			Language:  &known,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetLanguages200JSONResponse); !ok {
		t.Fatalf("expected GetLanguages200JSONResponse, got %T", resp)
	}
}

func TestGetLanguages_NilManifestPermitsLegacyCallers(t *testing.T) {
	// Pre-Phase-118a server constructors do not wire the manifest. Those
	// callers must keep working — the validator becomes a no-op rather than
	// firing a 500.
	store := &mockStore{
		languageDetections: []storage.LanguageDetectionRow{
			{DetectedLanguage: "de", Count: 1, AvgConfidence: 0.9, Sources: []string{"x"}},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	bogus := "xx"

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{
			StartDate: start,
			EndDate:   end,
			Language:  &bogus,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetLanguages200JSONResponse); !ok {
		t.Fatalf("expected GetLanguages200JSONResponse (manifest absent → no-op), got %T", resp)
	}
}

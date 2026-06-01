package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// --- GetEntities ---

// TestGetEntities_WindowOptional pins the unbounded-default contract: omitting
// BOTH dates returns the whole dataset (200); supplying one bound opens the
// other side (still 200). Inverted windows are rejected elsewhere.
func TestGetEntities_WindowOptional(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, nil))

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"no params → whole dataset", "", http.StatusOK},
		{"only startDate → open-ended", "?startDate=2025-01-01T00:00:00Z", http.StatusOK},
		{"only endDate → open-ended", "?endDate=2025-01-02T00:00:00Z", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/entities"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestGetEntities_Returns400WhenLimitTooLow(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	limit := 0

	resp, err := s.GetEntities(context.Background(), GetEntitiesRequestObject{
		Params: GetEntitiesParams{StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetEntities400JSONResponse); !ok {
		t.Fatalf("expected GetEntities400JSONResponse, got %T", resp)
	}
}

func TestGetEntities_Returns400WhenLimitTooHigh(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	limit := 5000

	resp, err := s.GetEntities(context.Background(), GetEntitiesRequestObject{
		Params: GetEntitiesParams{StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetEntities400JSONResponse); !ok {
		t.Fatalf("expected GetEntities400JSONResponse, got %T", resp)
	}
}

func TestGetEntities_ReturnsEntities(t *testing.T) {
	store := &mockStore{
		entities: []storage.EntityRow{
			{EntityText: "Bundesregierung", EntityLabel: "ORG", Count: 5, Sources: []string{"tagesschau"}},
			{EntityText: "Berlin", EntityLabel: "LOC", Count: 3, Sources: []string{"tagesschau", "bundesregierung"}},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	label := "ORG"

	resp, err := s.GetEntities(context.Background(), GetEntitiesRequestObject{
		Params: GetEntitiesParams{
			StartDate: &start,
			EndDate:   &end,
			Label:     &label,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetEntities200JSONResponse)
	if !ok {
		t.Fatalf("expected GetEntities200JSONResponse, got %T", resp)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].EntityText != "Bundesregierung" {
		t.Errorf("expected entityText Bundesregierung, got %q", got[0].EntityText)
	}
	if got[0].Count != 5 {
		t.Errorf("expected count 5, got %d", got[0].Count)
	}
	if store.capturedLabel == nil || *store.capturedLabel != "ORG" {
		t.Errorf("expected label filter ORG to be passed to store")
	}
	if store.capturedLimit != 100 {
		t.Errorf("expected default limit 100, got %d", store.capturedLimit)
	}
}

func TestGetEntities_Returns500OnStorageError(t *testing.T) {
	store := &mockStore{entitiesErr: errors.New("clickhouse timeout")}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetEntities(context.Background(), GetEntitiesRequestObject{
		Params: GetEntitiesParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	got, ok := resp.(GetEntities500JSONResponse)
	if !ok {
		t.Fatalf("expected GetEntities500JSONResponse, got %T", resp)
	}
	if got.Message != genericInternalError {
		t.Errorf("expected generic internal error message, got %q", got.Message)
	}
}

func TestGetEntities_RespectsCustomLimit(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	limit := 50

	_, err := s.GetEntities(context.Background(), GetEntitiesRequestObject{
		Params: GetEntitiesParams{StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.capturedLimit != 50 {
		t.Errorf("expected limit 50, got %d", store.capturedLimit)
	}
}

// --- GetLanguages ---

// TestGetLanguages_WindowOptional pins the unbounded-default contract: omitting
// BOTH dates returns the whole dataset (200); supplying one bound opens the
// other side (still 200). Inverted windows are rejected elsewhere.
func TestGetLanguages_WindowOptional(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, nil))

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"no params → whole dataset", "", http.StatusOK},
		{"only startDate → open-ended", "?startDate=2025-01-01T00:00:00Z", http.StatusOK},
		{"only endDate → open-ended", "?endDate=2025-01-02T00:00:00Z", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/languages"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestGetLanguages_Returns400WhenLimitTooLow(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	limit := 0

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetLanguages400JSONResponse); !ok {
		t.Fatalf("expected GetLanguages400JSONResponse, got %T", resp)
	}
}

func TestGetLanguages_Returns400WhenLimitTooHigh(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	limit := 5000

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetLanguages400JSONResponse); !ok {
		t.Fatalf("expected GetLanguages400JSONResponse, got %T", resp)
	}
}

func TestGetLanguages_ReturnsDetections(t *testing.T) {
	store := &mockStore{
		languageDetections: []storage.LanguageDetectionRow{
			{DetectedLanguage: "de", Count: 42, AvgConfidence: 0.9876, Sources: []string{"tagesschau"}},
			{DetectedLanguage: "en", Count: 5, AvgConfidence: 0.8512, Sources: []string{"tagesschau", "bundesregierung"}},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	lang := "de"

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{
			StartDate: &start,
			EndDate:   &end,
			Language:  &lang,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetLanguages200JSONResponse)
	if !ok {
		t.Fatalf("expected GetLanguages200JSONResponse, got %T", resp)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].DetectedLanguage != "de" {
		t.Errorf("expected detectedLanguage de, got %q", got[0].DetectedLanguage)
	}
	if got[0].Count != 42 {
		t.Errorf("expected count 42, got %d", got[0].Count)
	}
	if got[0].AvgConfidence != 0.9876 {
		t.Errorf("expected avgConfidence 0.9876, got %v", got[0].AvgConfidence)
	}
	if store.capturedLanguage == nil || *store.capturedLanguage != "de" {
		t.Errorf("expected language filter de to be passed to store")
	}
	if store.capturedLimit != 100 {
		t.Errorf("expected default limit 100, got %d", store.capturedLimit)
	}
}

func TestGetLanguages_Returns500OnStorageError(t *testing.T) {
	store := &mockStore{languageDetectionsErr: errors.New("clickhouse timeout")}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	got, ok := resp.(GetLanguages500JSONResponse)
	if !ok {
		t.Fatalf("expected GetLanguages500JSONResponse, got %T", resp)
	}
	if got.Message != genericInternalError {
		t.Errorf("expected generic internal error message, got %q", got.Message)
	}
}

func TestGetLanguages_RespectsCustomLimit(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	limit := 25

	_, err := s.GetLanguages(context.Background(), GetLanguagesRequestObject{
		Params: GetLanguagesParams{StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.capturedLimit != 25 {
		t.Errorf("expected limit 25, got %d", store.capturedLimit)
	}
}

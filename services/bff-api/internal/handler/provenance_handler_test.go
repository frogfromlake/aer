package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
)

// Phase 67 — GET /metrics/{metricName}/provenance handler coverage.
//
// Exercises the three result paths the endpoint can take:
//   - 200 with static fields from metric_provenance.yaml plus dynamic
//     validation_status/cultural_context_notes joined from ClickHouse
//   - 404 when the metric is not registered in the provenance config
//   - 500 when the downstream ClickHouse queries fail
//
// Also verifies the validation-status join: a metric without any entry
// in metric_validity surfaces as "unvalidated", a metric with a current
// entry surfaces as "validated".

func testProvenance() config.MetricProvenanceMap {
	return config.MetricProvenanceMap{
		"word_count": {
			TierClassification:   1,
			AlgorithmDescription: "Whitespace token count over cleaned text.",
			KnownLimitations:     []string{},
			ExtractorVersionHash: "v1",
		},
		"sentiment_score": {
			TierClassification:   1,
			AlgorithmDescription: "Lexicon-based polarity score using SentiWS v2.0.",
			KnownLimitations: []string{
				"Negation blindness — SentiWS does not propagate negation scope.",
				"Compound word failure — German noun compounds are not decomposed.",
			},
			ExtractorVersionHash: "sentiws-2.0",
		},
	}
}

func TestGetMetricProvenance_WordCountReturnsTier1AndEmptyLimitations(t *testing.T) {
	store := &mockStore{validationStatus: "unvalidated"}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "word_count",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.MetricName != "word_count" {
		t.Errorf("expected metricName=word_count, got %q", got.MetricName)
	}
	if got.TierClassification != N1 {
		t.Errorf("expected tier 1, got %v", got.TierClassification)
	}
	if got.AlgorithmDescription == "" {
		t.Error("expected non-empty algorithm description")
	}
	if len(got.KnownLimitations) != 0 {
		t.Errorf("expected empty known_limitations, got %v", got.KnownLimitations)
	}
	if got.ValidationStatus != MetricProvenanceValidationStatusUnvalidated {
		t.Errorf("expected validation_status=unvalidated, got %q", got.ValidationStatus)
	}
	if got.ExtractorVersionHash != "v1" {
		t.Errorf("expected extractor version v1, got %q", got.ExtractorVersionHash)
	}
}

func TestGetMetricProvenance_SentimentScoreSurfacesKnownLimitations(t *testing.T) {
	store := &mockStore{validationStatus: "unvalidated"}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "sentiment_score",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.TierClassification != N1 {
		t.Errorf("expected tier 1, got %v", got.TierClassification)
	}
	if len(got.KnownLimitations) != 2 {
		t.Fatalf("expected 2 known limitations, got %d", len(got.KnownLimitations))
	}
	joined := got.KnownLimitations[0] + "|" + got.KnownLimitations[1]
	if !contains(joined, "Negation blindness") {
		t.Errorf("expected Negation blindness in known_limitations, got %v", got.KnownLimitations)
	}
	if !contains(joined, "Compound word") {
		t.Errorf("expected Compound word in known_limitations, got %v", got.KnownLimitations)
	}
}

func TestGetMetricProvenance_UnknownMetricReturns404(t *testing.T) {
	s := NewServer(&mockStore{}, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "nonexistent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance404JSONResponse)
	if !ok {
		t.Fatalf("expected 404 response, got %T", resp)
	}
	if got.Message == "" {
		t.Error("expected non-empty 404 message")
	}
}

func TestGetMetricProvenance_ValidationStatusJoinMetricProvenanceValidationStatusValidated(t *testing.T) {
	store := &mockStore{validationStatus: "validated"}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "word_count",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.ValidationStatus != MetricProvenanceValidationStatusValidated {
		t.Errorf("expected validation_status=validated, got %q", got.ValidationStatus)
	}
}

func TestGetMetricProvenance_ValidationStatusJoinMetricProvenanceValidationStatusUnvalidated(t *testing.T) {
	store := &mockStore{validationStatus: "unvalidated"}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "word_count",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.ValidationStatus != MetricProvenanceValidationStatusUnvalidated {
		t.Errorf("expected validation_status=unvalidated, got %q", got.ValidationStatus)
	}
}

func TestGetMetricProvenance_CulturalContextNotesPopulated(t *testing.T) {
	store := &mockStore{
		validationStatus:     "unvalidated",
		culturalContextNotes: "Cross-cultural equivalence established at \"deviation\" level for etic construct \"evaluative_polarity\".",
	}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "sentiment_score",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.CulturalContextNotes == nil || *got.CulturalContextNotes == "" {
		t.Error("expected non-empty cultural_context_notes")
	}
}

func TestGetMetricProvenance_CulturalContextNotesOmittedWhenEmpty(t *testing.T) {
	store := &mockStore{validationStatus: "unvalidated", culturalContextNotes: ""}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "word_count",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.CulturalContextNotes != nil {
		t.Errorf("expected nil cultural_context_notes, got %v", *got.CulturalContextNotes)
	}
}

func TestGetMetricProvenance_Returns500WhenValidationStatusQueryFails(t *testing.T) {
	store := &mockStore{validationStatusErr: errors.New("clickhouse timeout")}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "word_count",
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance500JSONResponse)
	if !ok {
		t.Fatalf("expected 500 response, got %T", resp)
	}
	if got.Message != genericInternalError {
		t.Errorf("expected generic internal error message, got %q", got.Message)
	}
}

func TestGetMetricProvenance_Returns500WhenCulturalContextQueryFails(t *testing.T) {
	store := &mockStore{
		validationStatus:        "unvalidated",
		culturalContextNotesErr: errors.New("clickhouse timeout"),
	}
	s := NewServer(store, testProvenance(), nil, nil, nil)

	resp, err := s.GetMetricProvenance(context.Background(), GetMetricProvenanceRequestObject{
		MetricName: "word_count",
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	got, ok := resp.(GetMetricProvenance500JSONResponse)
	if !ok {
		t.Fatalf("expected 500 response, got %T", resp)
	}
	if got.Message != genericInternalError {
		t.Errorf("expected generic internal error message, got %q", got.Message)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

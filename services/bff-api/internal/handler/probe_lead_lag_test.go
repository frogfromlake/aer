package handler

import (
	"context"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// twoProbeRegistry is a DE + FR probe pair for the cross-probe lead-lag /
// equivalence tests (Phase 124).
func twoProbeRegistry() config.ProbeRegistry {
	return config.ProbeRegistry{
		"probe-0-de-institutional-web": config.ProbeEntry{
			ProbeID:  "probe-0-de-institutional-web",
			Language: "de",
			Sources:  []string{"tagesschau", "bundesregierung"},
		},
		"probe-1-fr-institutional-web": config.ProbeEntry{
			ProbeID:  "probe-1-fr-institutional-web",
			Language: "fr",
			Sources:  []string{"franceinfo", "elysee"},
		},
	}
}

func TestGetProbeLeadLag_RefusesWithoutGrant(t *testing.T) {
	store := &mockStore{checkNormalizationEquivForLanguagesValue: false}
	s := NewServer(store, nil, nil, nil, twoProbeRegistry())

	resp, err := s.GetProbeLeadLag(context.Background(), GetProbeLeadLagRequestObject{
		ProbeID: "probe-0-de-institutional-web",
		Params:  GetProbeLeadLagParams{ComparedTo: "probe-1-fr-institutional-web"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetProbeLeadLag400JSONResponse)
	if !ok {
		t.Fatalf("expected 400 refusal, got %T", resp)
	}
	if got.Gate == nil || *got.Gate != crossFrameGateID {
		t.Errorf("expected gate=%q, got %+v", crossFrameGateID, got.Gate)
	}
	if got.WorkingPaperAnchor == nil || *got.WorkingPaperAnchor != leadLagAnchor {
		t.Errorf("expected anchor=%q, got %+v", leadLagAnchor, got.WorkingPaperAnchor)
	}
}

func TestGetProbeLeadLag_ReturnsResultWhenGranted(t *testing.T) {
	peakLag := 1
	peakCorr := 0.91
	level := "temporal"
	validatedBy := "1"
	store := &mockStore{
		checkNormalizationEquivForLanguagesValue: true,
		leadLag: storage.LeadLagResult{
			MaxLagHours:       2,
			BucketCountAtZero: 120,
			Points: []storage.LeadLagPoint{
				{LagHours: -2}, {LagHours: -1}, {LagHours: 0}, {LagHours: 1}, {LagHours: 2},
			},
			PeakLagHours:    &peakLag,
			PeakCorrelation: &peakCorr,
		},
		equivalenceStatus: &storage.EquivalenceStatusRow{
			Level:       &level,
			Notes:       "temporal Level-1 grant per WP-004 App. B",
			ValidatedBy: &validatedBy,
		},
	}
	s := NewServer(store, nil, nil, nil, twoProbeRegistry())

	resp, err := s.GetProbeLeadLag(context.Background(), GetProbeLeadLagRequestObject{
		ProbeID: "probe-0-de-institutional-web",
		Params:  GetProbeLeadLagParams{ComparedTo: "probe-1-fr-institutional-web"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetProbeLeadLag200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if got.ReferenceProbe != "probe-0-de-institutional-web" || got.ComparedProbe != "probe-1-fr-institutional-web" {
		t.Errorf("unexpected probe ids: %s / %s", got.ReferenceProbe, got.ComparedProbe)
	}
	if got.Signal != leadLagSignal {
		t.Errorf("expected signal %q, got %q", leadLagSignal, got.Signal)
	}
	if got.Grant.Level != "temporal" || got.Grant.WorkingPaperAnchor != leadLagAnchor {
		t.Errorf("unexpected grant block: %+v", got.Grant)
	}
	if got.Grant.Notes == nil || got.Grant.ValidatedBy == nil {
		t.Errorf("expected grant notes + validatedBy populated, got %+v", got.Grant)
	}
	if len(got.Points) != 5 {
		t.Errorf("expected 5 lag points, got %d", len(got.Points))
	}
	if got.PeakLagHours == nil || *got.PeakLagHours != 1 {
		t.Errorf("expected peak lag 1, got %+v", got.PeakLagHours)
	}
}

func TestGetProbeLeadLag_404ForUnknownComparedTo(t *testing.T) {
	store := &mockStore{checkNormalizationEquivForLanguagesValue: true}
	s := NewServer(store, nil, nil, nil, twoProbeRegistry())

	resp, err := s.GetProbeLeadLag(context.Background(), GetProbeLeadLagRequestObject{
		ProbeID: "probe-0-de-institutional-web",
		Params:  GetProbeLeadLagParams{ComparedTo: "probe-9-xx-nonexistent"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetProbeLeadLag404JSONResponse); !ok {
		t.Fatalf("expected 404 for unknown comparedTo, got %T", resp)
	}
}

func TestGetProbeLeadLag_RejectsIdenticalProbes(t *testing.T) {
	store := &mockStore{checkNormalizationEquivForLanguagesValue: true}
	s := NewServer(store, nil, nil, nil, twoProbeRegistry())

	resp, err := s.GetProbeLeadLag(context.Background(), GetProbeLeadLagRequestObject{
		ProbeID: "probe-0-de-institutional-web",
		Params:  GetProbeLeadLagParams{ComparedTo: "probe-0-de-institutional-web"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetProbeLeadLag400JSONResponse); !ok {
		t.Fatalf("expected 400 for identical probes, got %T", resp)
	}
}

// Phase 124: comparedTo unions both probes' sources and echoes the pair.
func TestGetProbeEquivalence_ComparedToUnionsSources(t *testing.T) {
	store := &mockStore{
		probeEquivalenceRows: []storage.ProbeEquivalenceMetric{
			{MetricName: "publication_hour", Level1Available: true},
		},
	}
	s := NewServer(store, nil, nil, nil, twoProbeRegistry())

	comparedTo := "probe-1-fr-institutional-web"
	resp, err := s.GetProbeEquivalence(context.Background(), GetProbeEquivalenceRequestObject{
		ProbeID: "probe-0-de-institutional-web",
		Params:  GetProbeEquivalenceParams{ComparedTo: &comparedTo},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetProbeEquivalence200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if got.ComparedTo == nil || *got.ComparedTo != comparedTo {
		t.Errorf("expected comparedTo echoed, got %+v", got.ComparedTo)
	}
	if got.Sources == nil {
		t.Fatal("expected unioned sources")
	}
	want := map[string]bool{"tagesschau": false, "bundesregierung": false, "franceinfo": false, "elysee": false}
	for _, src := range *got.Sources {
		if _, ok := want[src]; ok {
			want[src] = true
		}
	}
	for src, seen := range want {
		if !seen {
			t.Errorf("expected unioned sources to include %q", src)
		}
	}
}

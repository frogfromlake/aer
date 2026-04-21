package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
)

func testProbeRegistry() config.ProbeRegistry {
	return config.ProbeRegistry{
		"probe-0-de-institutional-rss": config.ProbeEntry{
			ProbeID:  "probe-0-de-institutional-rss",
			Language: "de",
			Sources:  []string{"tagesschau", "bundesregierung"},
			EmissionPoints: []config.EmissionPoint{
				{Latitude: 53.5511, Longitude: 9.9937, Label: "Hamburg (Tagesschau / NDR)"},
				{Latitude: 52.5170, Longitude: 13.3888, Label: "Berlin (Bundesregierung / BPA)"},
			},
		},
	}
}

func TestGetProbes_ReturnsRegistryEntries(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry()))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var probes []Probe
	if err := json.Unmarshal(rec.Body.Bytes(), &probes); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(probes) != 1 {
		t.Fatalf("expected 1 probe, got %d", len(probes))
	}
	p := probes[0]
	if p.ProbeId != "probe-0-de-institutional-rss" {
		t.Errorf("probeId mismatch: %s", p.ProbeId)
	}
	if p.Language != "de" {
		t.Errorf("language mismatch: %s", p.Language)
	}
	if len(p.EmissionPoints) != 2 {
		t.Fatalf("expected 2 emission points, got %d", len(p.EmissionPoints))
	}
	if len(p.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(p.Sources))
	}
}

func TestGetProbes_EmptyRegistryReturnsEmptyArray(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, config.ProbeRegistry{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "[]" && body != "[]\n" {
		t.Errorf("expected empty array, got %q", body)
	}
}

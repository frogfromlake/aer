package storage

import (
	"testing"
	"time"
)

func TestAssembleCoverage_PopulationRateAndAbsence(t *testing.T) {
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	cells := []MetadataCoverageCell{
		// bundesregierung — author is structurally absent (60 articles, all null).
		{Source: "bundesregierung", Field: "author", Method: "null", Articles: 60, LastSeen: now},
		// bundesregierung — published_date is partly populated.
		{Source: "bundesregierung", Field: "published_date", Method: "html_meta", Articles: 49, LastSeen: now},
		{Source: "bundesregierung", Field: "published_date", Method: "heuristic_htmldate", Articles: 8, LastSeen: now},
		{Source: "bundesregierung", Field: "published_date", Method: "null", Articles: 63, LastSeen: now},
		// tagesschau — author always populated.
		{Source: "tagesschau", Field: "author", Method: "json_ld", Articles: 200, LastSeen: now},
		// short-sample author absence — under 50 threshold so NOT structural.
		{Source: "fictional", Field: "author", Method: "null", Articles: 30, LastSeen: now},
	}

	out := AssembleCoverage(cells)

	if len(out) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(out))
	}

	// Check sort order: bundesregierung, fictional, tagesschau.
	if out[0].Name != "bundesregierung" || out[1].Name != "fictional" || out[2].Name != "tagesschau" {
		t.Errorf("unexpected source order: %v", []string{out[0].Name, out[1].Name, out[2].Name})
	}

	// bundesregierung.author — structurally absent.
	bundes := out[0]
	authorIdx := -1
	for i, f := range bundes.Fields {
		if f.Field == "author" {
			authorIdx = i
		}
	}
	if authorIdx < 0 {
		t.Fatalf("bundesregierung.author missing from result")
	}
	bAuthor := bundes.Fields[authorIdx]
	if !bAuthor.StructurallyAbsent {
		t.Errorf("expected bundesregierung.author structurallyAbsent=true, got false")
	}
	if bAuthor.PopulationRate != 0 {
		t.Errorf("expected populationRate=0 for absent field, got %v", bAuthor.PopulationRate)
	}
	if bAuthor.TotalArticles != 60 {
		t.Errorf("expected totalArticles=60, got %d", bAuthor.TotalArticles)
	}

	// bundesregierung.published_date — partial population, NOT structurally absent.
	pubIdx := -1
	for i, f := range bundes.Fields {
		if f.Field == "published_date" {
			pubIdx = i
		}
	}
	if pubIdx < 0 {
		t.Fatalf("bundesregierung.published_date missing from result")
	}
	pub := bundes.Fields[pubIdx]
	if pub.StructurallyAbsent {
		t.Errorf("expected published_date structurallyAbsent=false, got true")
	}
	if pub.TotalArticles != 120 {
		t.Errorf("expected totalArticles=120, got %d", pub.TotalArticles)
	}
	wantRate := 57.0 / 120.0 // (49 + 8) / 120
	if pub.PopulationRate < wantRate-0.001 || pub.PopulationRate > wantRate+0.001 {
		t.Errorf("expected populationRate=%v, got %v", wantRate, pub.PopulationRate)
	}

	// fictional.author — under threshold, NOT structurally absent even at 0 % population.
	fic := out[1]
	if len(fic.Fields) != 1 {
		t.Fatalf("expected fictional to have 1 field, got %d", len(fic.Fields))
	}
	if fic.Fields[0].StructurallyAbsent {
		t.Errorf("expected fictional.author structurallyAbsent=false (under threshold), got true")
	}

	// tagesschau.author — fully populated.
	tag := out[2]
	if len(tag.Fields) != 1 {
		t.Fatalf("expected tagesschau to have 1 field, got %d", len(tag.Fields))
	}
	if tag.Fields[0].PopulationRate != 1.0 {
		t.Errorf("expected tagesschau.author populationRate=1.0, got %v", tag.Fields[0].PopulationRate)
	}
}

func TestAssembleCoverage_EmptyInput(t *testing.T) {
	if got := AssembleCoverage(nil); len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

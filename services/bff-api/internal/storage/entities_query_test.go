package storage

import (
	"testing"
	"time"
)

func TestGetEntities(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert test entities
	for _, e := range []struct {
		ts     time.Time
		source string
		text   string
		label  string
	}{
		{now.Add(-1 * time.Hour), "tagesschau", "Bundesregierung", "ORG"},
		{now.Add(-1 * time.Hour), "tagesschau", "Bundesregierung", "ORG"},
		{now.Add(-30 * time.Minute), "bundesregierung", "Bundesregierung", "ORG"},
		{now.Add(-1 * time.Hour), "tagesschau", "Berlin", "LOC"},
		{now.Add(-3 * time.Hour), "tagesschau", "OutOfRange", "PER"}, // outside range
	} {
		err := store.conn.Exec(ctx, "INSERT INTO aer_gold.entities (timestamp, source, article_id, entity_text, entity_label, start_char, end_char) VALUES (?, ?, ?, ?, ?, ?, ?)",
			e.ts, e.source, nil, e.text, e.label, 0, 0)
		if err != nil {
			t.Fatalf("failed to insert entity: %v", err)
		}
	}

	start := now.Add(-90 * time.Minute)
	end := now

	// TEST: GetEntities without filters
	results, err := store.GetEntities(ctx, start, end, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 distinct entities, got %d", len(results))
	}
	// Ordered by count DESC — Bundesregierung (3) then Berlin (1)
	if results[0].EntityText != "Bundesregierung" {
		t.Errorf("expected first entity Bundesregierung, got %q", results[0].EntityText)
	}
	if results[0].Count != 3 {
		t.Errorf("expected count 3, got %d", results[0].Count)
	}
	if len(results[0].Sources) != 2 {
		t.Errorf("expected 2 distinct sources for Bundesregierung, got %d", len(results[0].Sources))
	}

	// TEST: GetEntities filtered by label
	orgLabel := "ORG"
	results, err = store.GetEntities(ctx, start, end, nil, &orgLabel, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entity for label=ORG, got %d", len(results))
	}

	// TEST: GetEntities filtered by source
	results, err = store.GetEntities(ctx, start, end, []string{"tagesschau"}, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entities for source=tagesschau, got %d", len(results))
	}

	// TEST: GetEntities respects limit
	results, err = store.GetEntities(ctx, start, end, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entity with limit=1, got %d", len(results))
	}
}

func TestGetLanguageDetections(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert test language detections (rank 1 = top candidate)
	for _, d := range []struct {
		ts     time.Time
		source string
		lang   string
		conf   float64
		rank   uint8
	}{
		{now.Add(-1 * time.Hour), "tagesschau", "de", 0.9999, 1},
		{now.Add(-1 * time.Hour), "tagesschau", "en", 0.0001, 2},   // rank 2, should be excluded from aggregation
		{now.Add(-30 * time.Minute), "bundesregierung", "de", 0.985, 1},
		{now.Add(-30 * time.Minute), "tagesschau", "en", 0.92, 1},
		{now.Add(-3 * time.Hour), "tagesschau", "de", 0.99, 1}, // outside range
	} {
		err := store.conn.Exec(ctx, "INSERT INTO aer_gold.language_detections (timestamp, source, article_id, detected_language, confidence, rank) VALUES (?, ?, ?, ?, ?, ?)",
			d.ts, d.source, nil, d.lang, d.conf, d.rank)
		if err != nil {
			t.Fatalf("failed to insert language detection: %v", err)
		}
	}

	start := now.Add(-90 * time.Minute)
	end := now

	// TEST: GetLanguageDetections without filters (rank=1 only)
	results, err := store.GetLanguageDetections(ctx, start, end, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 distinct languages, got %d", len(results))
	}
	// Ordered by count DESC — de (2) then en (1)
	if results[0].DetectedLanguage != "de" {
		t.Errorf("expected first language de, got %q", results[0].DetectedLanguage)
	}
	if results[0].Count != 2 {
		t.Errorf("expected count 2, got %d", results[0].Count)
	}
	if len(results[0].Sources) != 2 {
		t.Errorf("expected 2 distinct sources for de, got %d", len(results[0].Sources))
	}

	// TEST: GetLanguageDetections filtered by language
	deLang := "de"
	results, err = store.GetLanguageDetections(ctx, start, end, nil, &deLang, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 language for language=de, got %d", len(results))
	}

	// TEST: GetLanguageDetections filtered by source
	results, err = store.GetLanguageDetections(ctx, start, end, []string{"tagesschau"}, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 languages for source=tagesschau, got %d", len(results))
	}

	// TEST: GetLanguageDetections respects limit
	results, err = store.GetLanguageDetections(ctx, start, end, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 language with limit=1, got %d", len(results))
	}
}

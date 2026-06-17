package storage

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// scope_args.go — positional placeholder + IN-clause builders.
// ---------------------------------------------------------------------------

func TestScopeArgs_PhAssignsSequentialPlaceholders(t *testing.T) {
	sa := newScopeArgs()
	if got := sa.ph("a"); got != "$1" {
		t.Errorf("first ph: want $1, got %q", got)
	}
	if got := sa.ph(42); got != "$2" {
		t.Errorf("second ph: want $2, got %q", got)
	}
	if got := sa.ph("c"); got != "$3" {
		t.Errorf("third ph: want $3, got %q", got)
	}
	want := []any{"a", 42, "c"}
	if len(sa.Args) != len(want) {
		t.Fatalf("Args length: want %d, got %d", len(want), len(sa.Args))
	}
	for i, w := range want {
		if sa.Args[i] != w {
			t.Errorf("Args[%d]: want %v, got %v", i, w, sa.Args[i])
		}
	}
}

func TestScopeArgs_SrcIn(t *testing.T) {
	t.Run("multiple sources produce joined placeholders in order", func(t *testing.T) {
		sa := newScopeArgs()
		if got := sa.srcIn([]string{"tagesschau", "elysee", "franceinfo"}); got != "$1, $2, $3" {
			t.Errorf("placeholder list: want %q, got %q", "$1, $2, $3", got)
		}
		if len(sa.Args) != 3 || sa.Args[0] != "tagesschau" || sa.Args[2] != "franceinfo" {
			t.Errorf("Args mismatch: %v", sa.Args)
		}
	})

	t.Run("empty set returns empty string and binds nothing", func(t *testing.T) {
		sa := newScopeArgs()
		if got := sa.srcIn(nil); got != "" {
			t.Errorf("empty srcIn: want %q, got %q", "", got)
		}
		if len(sa.Args) != 0 {
			t.Errorf("empty srcIn must bind no args, got %v", sa.Args)
		}
	})

	t.Run("placeholders continue after prior ph binds", func(t *testing.T) {
		sa := newScopeArgs()
		_ = sa.ph("field")
		if got := sa.srcIn([]string{"a", "b"}); got != "$2, $3" {
			t.Errorf("continuation: want %q, got %q", "$2, $3", got)
		}
	})
}

func TestScopeArgs_MetadataFilterClause(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	sources := []string{"tagesschau"}

	t.Run("nil filter yields empty clause and binds nothing", func(t *testing.T) {
		sa := newScopeArgs()
		if got := sa.metadataFilterClause(nil, start, end, sources); got != "" {
			t.Errorf("nil filter: want empty, got %q", got)
		}
		if len(sa.Args) != 0 {
			t.Errorf("nil filter must bind nothing, got %v", sa.Args)
		}
	})

	t.Run("empty field/value yields empty clause", func(t *testing.T) {
		sa := newScopeArgs()
		if got := sa.metadataFilterClause(&MetadataFilter{Field: "", Value: "x"}, start, end, sources); got != "" {
			t.Errorf("empty field: want empty, got %q", got)
		}
		if got := sa.metadataFilterClause(&MetadataFilter{Field: "section", Value: ""}, start, end, sources); got != "" {
			t.Errorf("empty value: want empty, got %q", got)
		}
	})

	t.Run("empty source set yields empty clause", func(t *testing.T) {
		sa := newScopeArgs()
		got := sa.metadataFilterClause(&MetadataFilter{Field: "section", Value: "Politik"}, start, end, nil)
		if got != "" {
			t.Errorf("empty sources: want empty, got %q", got)
		}
	})

	t.Run("active filter binds field, value, window, sources in order", func(t *testing.T) {
		sa := newScopeArgs()
		got := sa.metadataFilterClause(&MetadataFilter{Field: "section", Value: "Politik"}, start, end, sources)
		if got == "" {
			t.Fatal("active filter must produce a clause")
		}
		// field, value, start, end, one source = 5 bound args.
		if len(sa.Args) != 5 {
			t.Fatalf("active filter args: want 5, got %d (%v)", len(sa.Args), sa.Args)
		}
		if sa.Args[0] != "section" || sa.Args[1] != "Politik" || sa.Args[4] != "tagesschau" {
			t.Errorf("bound args mismatch: %v", sa.Args)
		}
		if !contains(got, "article_metadata") || !contains(got, "has(value, $2)") {
			t.Errorf("clause shape unexpected: %q", got)
		}
	})
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// silver_aggregation_query.go — kind validators + field-expr map.
// ---------------------------------------------------------------------------

func TestSilverFieldExpr(t *testing.T) {
	cases := []struct {
		kind     SilverAggregationKind
		wantExpr string
		wantOK   bool
	}{
		{SilverAggCleanedTextLength, "cleaned_text_length", true},
		{SilverAggWordCount, "word_count", true},
		{SilverAggRawEntityCount, "raw_entity_count", true},
		{SilverAggCleanedTextLengthByHour, "", false},
		{SilverAggWordCountBySource, "", false},
		{SilverAggCleanedTextLengthVsWords, "", false},
		{SilverAggregationKind("nonsense"), "", false},
	}
	for _, tc := range cases {
		expr, ok := silverFieldExpr(tc.kind)
		if expr != tc.wantExpr || ok != tc.wantOK {
			t.Errorf("silverFieldExpr(%q) = (%q, %v), want (%q, %v)", tc.kind, expr, ok, tc.wantExpr, tc.wantOK)
		}
	}
}

func TestSilverKindClassifiers(t *testing.T) {
	cases := []struct {
		kind                      SilverAggregationKind
		isDist, isHeatmap, isCorr bool
	}{
		{SilverAggCleanedTextLength, true, false, false},
		{SilverAggWordCount, true, false, false},
		{SilverAggRawEntityCount, true, false, false},
		{SilverAggCleanedTextLengthByHour, false, true, false},
		{SilverAggWordCountBySource, false, true, false},
		{SilverAggCleanedTextLengthVsWords, false, false, true},
		{SilverAggregationKind("unknown"), false, false, false},
	}
	for _, tc := range cases {
		if got := IsSilverDistributionKind(tc.kind); got != tc.isDist {
			t.Errorf("IsSilverDistributionKind(%q) = %v, want %v", tc.kind, got, tc.isDist)
		}
		if got := IsSilverHeatmapKind(tc.kind); got != tc.isHeatmap {
			t.Errorf("IsSilverHeatmapKind(%q) = %v, want %v", tc.kind, got, tc.isHeatmap)
		}
		if got := IsSilverCorrelationKind(tc.kind); got != tc.isCorr {
			t.Errorf("IsSilverCorrelationKind(%q) = %v, want %v", tc.kind, got, tc.isCorr)
		}
	}
}

// ---------------------------------------------------------------------------
// revisions_diff_query.go — DecodeDiffParagraphs + join helpers.
// ---------------------------------------------------------------------------

func TestDecodeDiffParagraphs(t *testing.T) {
	t.Run("valid mixed ops decode with optional fields", func(t *testing.T) {
		raw := []string{
			`{"op": "add", "after": "new line"}`,
			`{"op": "del", "before": "old line"}`,
			`{"op": "mod", "before": "a", "after": "b"}`,
		}
		ops, err := DecodeDiffParagraphs(raw)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(ops) != 3 {
			t.Fatalf("want 3 ops, got %d", len(ops))
		}
		if ops[0].Op != "add" || ops[0].After != "new line" || ops[0].Before != "" {
			t.Errorf("add op decoded wrong: %+v", ops[0])
		}
		if ops[1].Op != "del" || ops[1].Before != "old line" {
			t.Errorf("del op decoded wrong: %+v", ops[1])
		}
		if ops[2].Op != "mod" || ops[2].Before != "a" || ops[2].After != "b" {
			t.Errorf("mod op decoded wrong: %+v", ops[2])
		}
	})

	t.Run("identity sentinel decodes cleanly", func(t *testing.T) {
		ops, err := DecodeDiffParagraphs([]string{`{"op": "identical"}`})
		if err != nil {
			t.Fatalf("decode sentinel: %v", err)
		}
		if len(ops) != 1 || ops[0].Op != "identical" {
			t.Errorf("sentinel decode wrong: %+v", ops)
		}
	})

	t.Run("empty input yields empty slice no error", func(t *testing.T) {
		ops, err := DecodeDiffParagraphs(nil)
		if err != nil {
			t.Fatalf("decode empty: %v", err)
		}
		if len(ops) != 0 {
			t.Errorf("want empty, got %v", ops)
		}
	})

	t.Run("malformed entry returns error naming the index", func(t *testing.T) {
		_, err := DecodeDiffParagraphs([]string{`{"op": "add"}`, `{not json`})
		if err == nil {
			t.Fatal("expected error on malformed entry")
		}
		if !contains(err.Error(), "decode diff op 1") {
			t.Errorf("error should name index 1, got %q", err.Error())
		}
	})
}

func TestJoinHelpers(t *testing.T) {
	t.Run("joinAndClauses", func(t *testing.T) {
		cases := []struct {
			in   []string
			want string
		}{
			{nil, ""},
			{[]string{"a >= 1"}, "a >= 1"},
			{[]string{"a >= 1", "b = true", "c < 9"}, "a >= 1 AND b = true AND c < 9"},
		}
		for _, tc := range cases {
			if got := joinAndClauses(tc.in); got != tc.want {
				t.Errorf("joinAndClauses(%v) = %q, want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("joinPlaceholders", func(t *testing.T) {
		cases := []struct {
			in   []string
			want string
		}{
			{nil, ""},
			{[]string{"?"}, "?"},
			{[]string{"?", "?", "?"}, "?, ?, ?"},
		}
		for _, tc := range cases {
			if got := joinPlaceholders(tc.in); got != tc.want {
				t.Errorf("joinPlaceholders(%v) = %q, want %q", tc.in, got, tc.want)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// heatmap_query.go — dimensionExpr.
// ---------------------------------------------------------------------------

func TestDimensionExpr(t *testing.T) {
	cases := []struct {
		dim          HeatmapDimension
		wantExpr     string
		wantJoinKind string
		wantOK       bool
	}{
		{HeatmapDimDayOfWeek, "toString(toDayOfWeek(m.timestamp))", "", true},
		{HeatmapDimHour, "toString(toHour(m.timestamp))", "", true},
		{HeatmapDimSource, "m.source", "", true},
		{HeatmapDimEntityLabel, "e.entity_label", "entities", true},
		{HeatmapDimLanguage, "ld.detected_language", "languages", true},
		{HeatmapDimension("nonsense"), "", "", false},
	}
	for _, tc := range cases {
		expr, joinKind, ok := dimensionExpr(tc.dim, "m")
		if expr != tc.wantExpr || joinKind != tc.wantJoinKind || ok != tc.wantOK {
			t.Errorf("dimensionExpr(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.dim, expr, joinKind, ok, tc.wantExpr, tc.wantJoinKind, tc.wantOK)
		}
	}
}

// ---------------------------------------------------------------------------
// revisions_query.go — revisionBucketExpr.
// ---------------------------------------------------------------------------

func TestRevisionBucketExpr(t *testing.T) {
	end := time.Date(2026, 5, 31, 12, 30, 45, 0, time.UTC)

	cases := []struct {
		res     RevisionActivityResolution
		want    string
		wantErr bool
	}{
		{RevisionResolutionSnapshot, "toDateTime('2026-05-31 12:30:45')", false},
		{"", "toDateTime('2026-05-31 12:30:45')", false}, // empty defaults to snapshot
		{RevisionResolutionDaily, "toStartOfDay(snapshot_at)", false},
		{RevisionResolutionWeekly, "toStartOfWeek(snapshot_at, 1)", false},
		{RevisionResolutionMonthly, "toStartOfMonth(snapshot_at)", false},
		{RevisionActivityResolution("bogus"), "", true},
	}
	for _, tc := range cases {
		got, err := revisionBucketExpr(tc.res, time.Time{}, end)
		if tc.wantErr {
			if err == nil {
				t.Errorf("revisionBucketExpr(%q) expected error, got %q", tc.res, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("revisionBucketExpr(%q): unexpected error %v", tc.res, err)
		}
		if got != tc.want {
			t.Errorf("revisionBucketExpr(%q) = %q, want %q", tc.res, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// clickhouse.go — Conn / Ping over the shared store.
// ---------------------------------------------------------------------------

func TestClickHouseStorage_ConnAndPing(t *testing.T) {
	store, ctx := setupTestStore(t)

	if store.Conn() == nil {
		t.Fatal("Conn() returned nil for a live store")
	}
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping on a live store: %v", err)
	}
	// Ping must surface a cancelled context.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Ping(cancelled); err == nil {
		t.Error("Ping with a cancelled context should error")
	}
}

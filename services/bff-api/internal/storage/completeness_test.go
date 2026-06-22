package storage

import (
	"database/sql"
	"testing"
)

func chRow(channel string, declared int64, declaredValid, indet bool) DiscoveryCoverageRow {
	return DiscoveryCoverageRow{
		Channel:               channel,
		Declared:              sql.NullInt64{Int64: declared, Valid: declaredValid},
		DeclaredIndeterminate: indet,
	}
}

func TestDeriveCompleteness_AllTrustworthy(t *testing.T) {
	// sitemap 411 + rss 32 = 443 declared; 333 gold → 0.7517 completeness.
	got := DeriveCompleteness([]DiscoveryCoverageRow{
		chRow("sitemap", 411, true, false),
		chRow("rss", 32, true, false),
	}, 333)
	if got.Indeterminate {
		t.Fatal("should not be indeterminate")
	}
	if !got.DeclaredTotal.Valid || got.DeclaredTotal.Int64 != 443 {
		t.Errorf("declared total: want 443, got %v", got.DeclaredTotal)
	}
	if !got.Completeness.Valid || got.Completeness.Float64 < 0.751 || got.Completeness.Float64 > 0.752 {
		t.Errorf("completeness: want ~0.7517, got %v", got.Completeness)
	}
	if got.IndeterminateChannelCount != 0 {
		t.Errorf("indeterminate channel count: want 0, got %d", got.IndeterminateChannelCount)
	}
}

func TestDeriveCompleteness_IndeterminateChannelIsNamedRemainder(t *testing.T) {
	// archive 262 + rss 72 trustworthy; html_sitemap indeterminate (dateless)
	// contributes nothing to the denominator but is the named remainder. A
	// completeness figure is still reported against the measurable channels.
	got := DeriveCompleteness([]DiscoveryCoverageRow{
		chRow("archive_index", 262, true, false),
		chRow("rss", 72, true, false),
		chRow("html_sitemap", 44, true, true), // indeterminate
		chRow("sitemap", 0, false, false),     // not configured: declared NULL
	}, 277)
	if got.Indeterminate {
		t.Fatal("a trustworthy channel exists → must not be globally indeterminate")
	}
	if got.DeclaredTotal.Int64 != 334 { // 262 + 72; html_sitemap excluded
		t.Errorf("declared total: want 334 (indeterminate channel excluded), got %d", got.DeclaredTotal.Int64)
	}
	if got.IndeterminateChannelCount != 1 {
		t.Errorf("named remainder: want 1 indeterminate channel, got %d", got.IndeterminateChannelCount)
	}
}

func TestDeriveCompleteness_NoTrustworthyDenominatorIsIndeterminate(t *testing.T) {
	// Fully-undated sitemap (declared 0 + indeterminate) and no other dated
	// channel → no trustworthy denominator → refuse to form a ratio.
	got := DeriveCompleteness([]DiscoveryCoverageRow{
		chRow("sitemap", 0, true, true),
		chRow("html_sitemap", 12, true, true),
	}, 5)
	if !got.Indeterminate {
		t.Fatal("no trustworthy denominator → must be indeterminate")
	}
	if got.Completeness.Valid {
		t.Error("completeness must be null when indeterminate (never a fabricated ratio)")
	}
	if got.IndeterminateChannelCount != 2 {
		t.Errorf("indeterminate channel count: want 2, got %d", got.IndeterminateChannelCount)
	}
}

func TestDeriveCompleteness_ZeroGoldIsMeasuredNotIndeterminate(t *testing.T) {
	// A real denominator with zero Gold is a measured 0 %, NOT indeterminate.
	got := DeriveCompleteness([]DiscoveryCoverageRow{chRow("rss", 10, true, false)}, 0)
	if got.Indeterminate {
		t.Fatal("a real denominator with 0 gold is a measured 0 %, not indeterminate")
	}
	if !got.Completeness.Valid || got.Completeness.Float64 != 0 {
		t.Errorf("completeness: want 0, got %v", got.Completeness)
	}
}

func TestFillFunnelRates(t *testing.T) {
	f := &FunnelSummary{Submitted: 393, GoldRows: 333, Fetched: 439, ThinContentDropped: 44}
	FillFunnelRates(f)
	if !f.ExtractionSuccessRate.Valid || f.ExtractionSuccessRate.Float64 < 0.847 || f.ExtractionSuccessRate.Float64 > 0.848 {
		t.Errorf("extraction success rate: want ~0.847, got %v", f.ExtractionSuccessRate)
	}
	if !f.NonArticleRate.Valid || f.NonArticleRate.Float64 < 0.100 || f.NonArticleRate.Float64 > 0.101 {
		t.Errorf("non-article rate: want ~0.100, got %v", f.NonArticleRate)
	}
}

func TestFillFunnelRates_ZeroDenominatorsInvalid(t *testing.T) {
	f := &FunnelSummary{Submitted: 0, Fetched: 0}
	FillFunnelRates(f)
	if f.ExtractionSuccessRate.Valid || f.NonArticleRate.Valid {
		t.Error("rates must be invalid (null) when their denominators are zero")
	}
}

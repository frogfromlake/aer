package storage

import "database/sql"

// Phase 148d (WP-007) — the completeness + funnel derivations.
//
// These are the scientifically load-bearing computations, kept pure and
// DB-free so they are unit-tested directly. The store wires the measured
// inputs (per-channel declared counts from Postgres, the funnel row from
// Postgres, the Gold count from ClickHouse) and calls these.

// FunnelSummary is the per-source collection funnel for the last run
// (WP-007 §5) plus the reconciled Gold tail and the Layer-3 rates.
type FunnelSummary struct {
	Present            bool
	Discovered         int64
	URLFiltered        int64
	AlreadyCollected   int64
	Fetched            int64
	NotModified        int64
	ContentDropped     int64
	ThinContentDropped int64
	Submitted          int64
	Errored            int64
	GoldRows           int64
	// ExtractionSuccessRate = GoldRows / Submitted (Layer-3, WP-007 §4.3);
	// invalid when Submitted is 0.
	ExtractionSuccessRate sql.NullFloat64
	// NonArticleRate = ThinContentDropped / Fetched (Layer-3 over-collection,
	// WP-007 §4.3); invalid when Fetched is 0.
	NonArticleRate sql.NullFloat64
}

// CompletenessResult is the source-level completeness verdict (WP-007 §4.1).
type CompletenessResult struct {
	// DeclaredTotal is the sum of `declared` over the TRUSTWORTHY channels
	// (declared valid AND not indeterminate). Invalid when no channel
	// supplied a measurable denominator.
	DeclaredTotal sql.NullInt64
	// Completeness is GoldRows / DeclaredTotal — invalid when Indeterminate.
	Completeness sql.NullFloat64
	// Indeterminate is true when no trustworthy denominator exists at all,
	// so AĒR refuses to form a ratio (never 100 %, never a fabricated figure).
	Indeterminate bool
	// IndeterminateChannelCount is the "named remainder" (WP-007 §3): how
	// many channels had a lower-bound (indeterminate) denominator. When > 0,
	// the completeness figure is against the measurable channels only.
	IndeterminateChannelCount int
}

// DeriveCompleteness computes the source-level completeness verdict from the
// per-channel declared denominators and the reconciled Gold row count.
//
// The denominator is the sum of declared over channels that supplied a
// TRUSTWORTHY (dated, in-window, non-indeterminate) count. Indeterminate
// channels are NOT folded into the denominator (their declared is only a
// lower bound) — they are counted as the named remainder instead. A source
// with at least one trustworthy channel still reports a completeness figure
// "against its measurable channels" (WP-007 §3); only a source with NO
// trustworthy denominator is reported as fully indeterminate.
func DeriveCompleteness(channels []DiscoveryCoverageRow, goldRows int64) CompletenessResult {
	var declaredTotal int64
	var trustworthy bool
	var indeterminateCount int
	for _, c := range channels {
		if c.DeclaredIndeterminate {
			indeterminateCount++
			continue
		}
		if c.Declared.Valid {
			declaredTotal += c.Declared.Int64
			trustworthy = true
		}
	}

	res := CompletenessResult{IndeterminateChannelCount: indeterminateCount}
	if !trustworthy || declaredTotal <= 0 {
		res.Indeterminate = true
		return res
	}
	res.DeclaredTotal = sql.NullInt64{Int64: declaredTotal, Valid: true}
	res.Completeness = sql.NullFloat64{
		Float64: float64(goldRows) / float64(declaredTotal),
		Valid:   true,
	}
	return res
}

// FillFunnelRates computes the Layer-3 derived rates on a funnel summary
// in place (extraction-success + non-article), guarding the zero denominators.
func FillFunnelRates(f *FunnelSummary) {
	if f == nil {
		return
	}
	if f.Submitted > 0 {
		f.ExtractionSuccessRate = sql.NullFloat64{
			Float64: float64(f.GoldRows) / float64(f.Submitted),
			Valid:   true,
		}
	}
	if f.Fetched > 0 {
		f.NonArticleRate = sql.NullFloat64{
			Float64: float64(f.ThinContentDropped) / float64(f.Fetched),
			Valid:   true,
		}
	}
}

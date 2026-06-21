package storage

import (
	"context"
	"testing"
	"time"
)

// Resolution routing tests (pure + DB-backed). Extracted from
// metrics_query_test.go to keep that file focused on the core metric queries.

// seedMVBucket replays a fixed value set through avgState/countState into one
// MV-backing table row (mirrors the production trigger's INSERT-INTO-SELECT).
func seedMVBucket(t *testing.T, ctx context.Context, s *ClickHouseStorage, table string, bucket time.Time, source, metric string, values []float64) {
	t.Helper()
	sel := ""
	for i, v := range values {
		if i > 0 {
			sel += " UNION ALL "
		}
		sel += "SELECT " + ftoa(v) + " AS value"
	}
	q := "INSERT INTO " + table + ` SELECT ? AS bucket, '` + source + `' AS source, '` + metric +
		`' AS metric_name, avgState(value) AS value_avg_state, countState() AS sample_count_state FROM (` + sel + `)`
	if err := s.conn.Exec(ctx, q, bucket); err != nil {
		t.Fatalf("seed %s: %v", table, err)
	}
}

// ftoa renders a float64 as a SQL Float64 literal (always with a decimal point
// so ClickHouse infers Float64, not an integer type). Avoids a strconv import.
func ftoa(v float64) string {
	whole := int64(v)
	frac := int64((v - float64(whole)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return itoa(int(whole)) + "." + itoa(int(frac))
}

func TestGetMetrics_ResolutionBucketing(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour)

	points := []time.Time{
		now.Add(-3 * time.Hour), now.Add(-2*time.Hour - 30*time.Minute),
		now.Add(-1*time.Hour - 15*time.Minute), now.Add(-15 * time.Minute),
	}
	for i, p := range points {
		seedMetric(t, ctx, store, p, float64(10*(i+1)), "test", "word_count", "art-"+itoa(i))
	}
	// Mirror the MV triggers explicitly (test uses plain tables, see clickhouse_test.go).
	for table, bucketExpr := range map[string]string{
		"aer_gold.metrics_hourly":  "toStartOfHour(timestamp)",
		"aer_gold.metrics_daily":   "toStartOfDay(timestamp)",
		"aer_gold.metrics_monthly": "toStartOfMonth(timestamp)",
	} {
		ddl := "INSERT INTO " + table + " SELECT " + bucketExpr +
			" AS bucket, source, metric_name, avgState(value), countState() FROM aer_gold.metrics GROUP BY bucket, source, metric_name"
		if err := store.conn.Exec(ctx, ddl); err != nil {
			t.Fatalf("seed %s: %v", table, err)
		}
	}

	// Window must cover every resolution's bucket (monthly starts on the 1st).
	start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(time.Hour)

	fiveMin, err := store.GetMetrics(ctx, start, end, nil, nil, ResolutionFiveMinute)
	if err != nil || len(fiveMin.Rows) != 4 {
		t.Fatalf("5min expected 4 buckets, got %d (err %v)", len(fiveMin.Rows), err)
	}
	hourly, err := store.GetMetrics(ctx, start, end, nil, nil, ResolutionHourly)
	if err != nil {
		t.Fatalf("hourly: %v", err)
	}
	if len(hourly.Rows) >= len(fiveMin.Rows) || len(hourly.Rows) == 0 {
		t.Errorf("hourly should collapse 5-min buckets: hourly=%d fiveMin=%d", len(hourly.Rows), len(fiveMin.Rows))
	}
	if daily, err := store.GetMetrics(ctx, start, end, nil, nil, ResolutionDaily); err != nil || len(daily.Rows) > 2 {
		t.Errorf("daily expected ≤2 buckets, got %d (err %v)", len(daily.Rows), err)
	}
	monthly, err := store.GetMetrics(ctx, start, end, nil, nil, ResolutionMonthly)
	if err != nil || len(monthly.Rows) < 1 || len(monthly.Rows) > 2 {
		t.Errorf("monthly expected 1–2 buckets, got %d (err %v)", len(monthly.Rows), err)
	}
	for _, b := range monthly.Rows {
		if !b.TS.Equal(time.Date(b.TS.Year(), b.TS.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("monthly bucket not month-aligned: %v", b.TS)
		}
	}
}

// TestGetMetrics_ResolutionRouting verifies GetMetrics reads from the right MV
// per resolution and combines state columns via avgMerge/countMerge.
func TestGetMetrics_ResolutionRouting(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour)
	hourlyBucket := now.Truncate(time.Hour)
	dailyBucket := now.Truncate(24 * time.Hour)
	monthlyBucket := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	seedMVBucket(t, ctx, store, "aer_gold.metrics_hourly", hourlyBucket, "src_hourly", "metric_h", []float64{10, 20})
	seedMVBucket(t, ctx, store, "aer_gold.metrics_daily", dailyBucket, "src_daily", "metric_d", []float64{5, 7, 9})
	seedMVBucket(t, ctx, store, "aer_gold.metrics_monthly", monthlyBucket, "src_monthly", "metric_m", []float64{100, 200})

	cases := []struct {
		name           string
		resolution     Resolution
		bucket         time.Time
		source, metric string
		wantValue      float64
		wantCount      uint64
	}{
		{"hourly", ResolutionHourly, hourlyBucket, "src_hourly", "metric_h", 15.0, 2},
		{"daily", ResolutionDaily, dailyBucket, "src_daily", "metric_d", 7.0, 3},
		{"monthly", ResolutionMonthly, monthlyBucket, "src_monthly", "metric_m", 150.0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := tc.bucket.Add(-time.Hour)
			end := tc.bucket.Add(31 * 24 * time.Hour)
			metricName := tc.metric
			rows, err := store.GetMetrics(ctx, start, end, []string{tc.source}, &metricName, tc.resolution)
			if err != nil {
				t.Fatalf("GetMetrics(%s): %v", tc.name, err)
			}
			if len(rows.Rows) != 1 {
				t.Fatalf("expected 1 row, got %d: %+v", len(rows.Rows), rows.Rows)
			}
			got := rows.Rows[0]
			if got.Source != tc.source || got.MetricName != tc.metric {
				t.Errorf("projection mismatch: %+v", got)
			}
			if got.Value != tc.wantValue {
				t.Errorf("value: want %v, got %v", tc.wantValue, got.Value)
			}
			if got.Count != tc.wantCount {
				t.Errorf("count: want %d, got %d", tc.wantCount, got.Count)
			}
		})
	}
}

// TestGetMetrics_WeeklyRebucket verifies weekly reads metrics_daily and
// rebuckets via toStartOfWeek at query time.
func TestGetMetrics_WeeklyRebucket(t *testing.T) {
	store, ctx := setupTestStore(t)
	day1 := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) // Monday
	day2 := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC) // Tuesday
	for _, day := range []time.Time{day1, day2} {
		seedMVBucket(t, ctx, store, "aer_gold.metrics_daily", day, "src_w", "metric_w", []float64{4, 6})
	}

	metric := "metric_w"
	rows, err := store.GetMetrics(ctx, day1.Add(-24*time.Hour), day2.Add(48*time.Hour), []string{"src_w"}, &metric, ResolutionWeekly)
	if err != nil {
		t.Fatalf("GetMetrics(weekly): %v", err)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("expected 1 weekly bucket, got %d: %+v", len(rows.Rows), rows.Rows)
	}
	if rows.Rows[0].Value != 5.0 { // avg of {4,6,4,6}
		t.Errorf("weekly avg: want 5.0, got %v", rows.Rows[0].Value)
	}
	if rows.Rows[0].Count != 4 {
		t.Errorf("weekly count: want 4, got %d", rows.Rows[0].Count)
	}
}

// --- pure unit tests (no ClickHouse) ---------------------------------------

func TestResolutionRowLimitMultiplier(t *testing.T) {
	cases := []struct {
		res      Resolution
		expected int
	}{
		{ResolutionFiveMinute, 1}, {ResolutionHourly, 12}, {ResolutionDaily, 288},
		{ResolutionWeekly, 2016}, {ResolutionMonthly, 8640},
	}
	for _, tc := range cases {
		if got := tc.res.rowLimitMultiplier(); got != tc.expected {
			t.Errorf("multiplier for %v: want %d, got %d", tc.res, tc.expected, got)
		}
	}
}

func TestResolutionBucketExpr(t *testing.T) {
	cases := []struct {
		res      Resolution
		expected string
	}{
		{ResolutionFiveMinute, "toStartOfFiveMinute(timestamp)"}, {ResolutionHourly, "toStartOfHour(timestamp)"},
		{ResolutionDaily, "toStartOfDay(timestamp)"}, {ResolutionWeekly, "toStartOfWeek(timestamp)"},
		{ResolutionMonthly, "toStartOfMonth(timestamp)"},
	}
	for _, tc := range cases {
		if got := tc.res.bucketExpr("timestamp"); got != tc.expected {
			t.Errorf("bucketExpr for %v: want %q, got %q", tc.res, tc.expected, got)
		}
	}
}

// TestResolutionQueryShape pins the Phase 122c routing matrix: each resolution
// maps to the right physical table + aggregate-merge expressions.
func TestResolutionQueryShape(t *testing.T) {
	mvShape := func(table, bucket string) metricsQueryShape {
		return metricsQueryShape{
			Table: table, TimestampColumn: "bucket", BucketExpr: bucket,
			ValueExpr: "avgMerge(value_avg_state)", CountExpr: "countMerge(sample_count_state)",
		}
	}
	cases := []struct {
		name string
		res  Resolution
		want metricsQueryShape
	}{
		{"5min raw", ResolutionFiveMinute, metricsQueryShape{
			Table: "aer_gold.metrics", TimestampColumn: "timestamp",
			BucketExpr: "toStartOfFiveMinute(timestamp)", ValueExpr: "avg(value)", CountExpr: "count()"}},
		{"hourly MV", ResolutionHourly, mvShape("aer_gold.metrics_hourly", "bucket")},
		{"daily MV", ResolutionDaily, mvShape("aer_gold.metrics_daily", "bucket")},
		{"weekly rebucket", ResolutionWeekly, mvShape("aer_gold.metrics_daily", "toStartOfWeek(bucket)")},
		{"monthly MV", ResolutionMonthly, mvShape("aer_gold.metrics_monthly", "bucket")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.queryShape(); got != tc.want {
				t.Errorf("queryShape mismatch:\n  want: %+v\n  got:  %+v", tc.want, got)
			}
		})
	}
}

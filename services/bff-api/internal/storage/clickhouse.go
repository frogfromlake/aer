package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cenkalti/backoff/v5"
)

type ClickHouseStorage struct {
	conn clickhouse.Conn
}

func NewClickHouseStorage(ctx context.Context, addr, user, password, db string) (*ClickHouseStorage, error) {
	operation := func() (clickhouse.Conn, error) {
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{addr},
			Auth: clickhouse.Auth{
				Database: db,
				Username: user,
				Password: password,
			},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return nil, err
		}

		if err := conn.Ping(ctx); err != nil {
			return nil, err
		}
		return conn, nil
	}

	notify := func(err error, d time.Duration) {
		slog.Warn("ClickHouse not ready, retrying...", "error", err, "backoff", d)
	}

	conn, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(30*time.Second),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse after retries: %w", err)
	}

	return &ClickHouseStorage{conn: conn}, nil
}

// Ping checks if the ClickHouse connection is alive.
func (s *ClickHouseStorage) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

// GetMetrics retrieves aggregated time-series data from the gold layer.
// It downsamples the data to 5-minute intervals to prevent OOM errors on large time ranges.
// Optional source and metricName filters narrow results to specific dimensions.
func (s *ClickHouseStorage) GetMetrics(ctx context.Context, start, end time.Time, source, metricName *string) ([]struct {
	TS    time.Time
	Value float64
}, error) {
	var results []struct {
		TS    time.Time
		Value float64
	}

	// Use toStartOfFiveMinute and avg() to aggregate data on the DB level.
	// We also apply a hard limit to guarantee memory safety.
	// Build dynamic WHERE clause based on optional dimension filters.
	query := `
		SELECT
			toStartOfFiveMinute(timestamp) as TS,
			avg(value) as Value
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if metricName != nil {
		query += fmt.Sprintf(" AND metric_name = $%d", argIdx)
		args = append(args, *metricName)
	}

	query += `
		GROUP BY TS
		ORDER BY TS ASC
		LIMIT 10000
	`

	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query metrics from ClickHouse", "error", err)
		return nil, err
	}

	return results, nil
}

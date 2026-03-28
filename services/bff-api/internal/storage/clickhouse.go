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

// GetMetrics retrieves aggregated time-series data from the gold layer.
func (s *ClickHouseStorage) GetMetrics(ctx context.Context, start, end time.Time) ([]struct {
	TS    time.Time
	Value float64
}, error) {
	var results []struct {
		TS    time.Time
		Value float64
	}

	query := `SELECT timestamp as TS, value as Value FROM aer_gold.metrics 
              WHERE timestamp >= $1 AND timestamp <= $2 
              ORDER BY timestamp ASC`

	err := s.conn.Select(ctx, &results, query, start, end)
	return results, err
}

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickHouseStorage is a wrapper around the ClickHouse database connection.
type ClickHouseStorage struct {
	conn driver.Conn
}

// NewClickHouseStorage establishes a connection to ClickHouse.
func NewClickHouseStorage(addr, user, pass, db string) (*ClickHouseStorage, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: db,
			Username: user,
			Password: pass,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse connection failed: %w", err)
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
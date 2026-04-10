package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresDB struct {
	DB *sql.DB
}

func NewPostgresDB(ctx context.Context, connStr string) (*PostgresDB, error) {
	// 1. Define an operation returning the connection directly (thanks to v5 generics!)
	operation := func() (*sql.DB, error) {
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			return nil, err
		}

		// PingContext ensures the actual network connection is established
		if err = db.PingContext(ctx); err != nil {
			db.Close()
			return nil, err
		}
		return db, nil
	}

	notify := func(err error, d time.Duration) {
		slog.Warn("PostgreSQL not ready, retrying...", "error", err, "backoff", d)
	}

	// v5 uses functional options for MaxElapsedTime
	db, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(30*time.Second),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres after retries: %w", err)
	}

	return &PostgresDB{DB: db}, nil
}

// GetSourceByName returns the ID and name of a source matching the given name.
func (p *PostgresDB) GetSourceByName(ctx context.Context, name string) (int, string, error) {
	var id int
	var sourceName string
	query := `SELECT id, name FROM sources WHERE name = $1`

	err := p.DB.QueryRowContext(ctx, query, name).Scan(&id, &sourceName)
	if err != nil {
		return 0, "", fmt.Errorf("failed to find source by name: %w", err)
	}
	return id, sourceName, nil
}

// Ping verifies the PostgreSQL connection is alive.
func (p *PostgresDB) Ping(ctx context.Context) error {
	return p.DB.PingContext(ctx)
}

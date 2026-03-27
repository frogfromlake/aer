package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresDB is a wrapper around the SQL database connection.
type PostgresDB struct {
	DB *sql.DB
}

// NewPostgresDB establishes a connection to PostgreSQL and verifies it via Ping.
func NewPostgresDB(connStr string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &PostgresDB{DB: db}, nil
}
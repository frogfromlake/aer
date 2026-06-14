package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/frogfromlake/aer/pkg/testutils"
)

// Shared testcontainers for the whole storage package.
//
// Previously every test spun up its own ClickHouse/Postgres container via
// setupTestStore / setupAuthStore. With ~37 ClickHouse + ~22 Postgres callers
// that meant ~59 sequential container starts (~10–15 s each), which pushed the
// package past Go's default 10-minute test timeout (`panic: test timed out`).
//
// TestMain now starts ONE container of each kind for the whole package run.
// Per-test isolation is restored cheaply at the schema level instead of the
// container level:
//   - ClickHouse: setupTestStore drops + recreates aer_gold / aer_silver, so
//     every test starts from a pristine schema (queries hard-code the aer_gold
//     database name, so the database itself is reset rather than renamed).
//   - Postgres: setupAuthStore creates a fresh database per test on the shared
//     container (the Go code carries the dbname in the DSN, so a unique name
//     per test gives full isolation).
//
// Tests in this package do not call t.Parallel(), so they run sequentially and
// the shared schema reset is race-free.
var (
	sharedCHAddr string // host:port of the shared ClickHouse native endpoint
	sharedPGHost string // host of the shared Postgres container
	sharedPGPort string // mapped 5432 port of the shared Postgres container
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	chImage, err := testutils.GetImageFromCompose("clickhouse")
	if err != nil {
		log.Fatalf("get clickhouse image from compose: %v", err)
	}
	chContainer, err := tcclickhouse.Run(ctx, chImage,
		tcclickhouse.WithDatabase("aer_gold"),
		tcclickhouse.WithUsername("aer_admin"),
		tcclickhouse.WithPassword("aer_secret"),
	)
	if err != nil {
		log.Fatalf("start shared clickhouse container: %v", err)
	}
	chHost, err := chContainer.Host(ctx)
	if err != nil {
		log.Fatalf("clickhouse host: %v", err)
	}
	chPort, err := chContainer.MappedPort(ctx, "9000/tcp")
	if err != nil {
		log.Fatalf("clickhouse port: %v", err)
	}
	sharedCHAddr = chHost + ":" + chPort.Port()

	pgImage, err := testutils.GetImageFromCompose("postgres")
	if err != nil {
		log.Fatalf("get postgres image from compose: %v", err)
	}
	pgContainer, err := postgres.Run(ctx, pgImage,
		postgres.WithDatabase("aer_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432/tcp", "pgx/v5", func(host string, port nat.Port) string {
				return fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=aer_test sslmode=disable", host, port.Port())
			}).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start shared postgres container: %v", err)
	}
	pgHost, err := pgContainer.Host(ctx)
	if err != nil {
		log.Fatalf("postgres host: %v", err)
	}
	pgPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		log.Fatalf("postgres port: %v", err)
	}
	sharedPGHost = pgHost
	sharedPGPort = pgPort.Port()

	code := m.Run()

	// Terminate explicitly before os.Exit (which skips deferred cleanups).
	if err := chContainer.Terminate(ctx); err != nil {
		log.Printf("terminate shared clickhouse container: %v", err)
	}
	if err := pgContainer.Terminate(ctx); err != nil {
		log.Printf("terminate shared postgres container: %v", err)
	}

	os.Exit(code)
}

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/frogfromlake/aer/pkg/testutils"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

func TestClickHouseStorage(t *testing.T) {
	ctx := context.Background()

	chImage, err := testutils.GetImageFromCompose("clickhouse")
	if err != nil {
		t.Fatalf("failed to get clickhouse image from compose: %v", err)
	}

	// 1. Start ephemeral ClickHouse container
	chContainer, err := tcclickhouse.Run(ctx, chImage,
		tcclickhouse.WithDatabase("aer_gold"),
		tcclickhouse.WithUsername("aer_admin"),
		tcclickhouse.WithPassword("aer_secret"),
	)
	if err != nil {
		t.Fatalf("failed to start clickhouse container: %v", err)
	}
	defer func() {
		if err := chContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate clickhouse container: %v", err)
		}
	}()

	host, err := chContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	// clickhouse-go uses the native protocol on port 9000
	port, err := chContainer.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	addr := host + ":" + port.Port()

	// 2. Initialize our Adapter
	store, err := NewClickHouseStorage(ctx, addr, "aer_admin", "aer_secret", "aer_gold")
	if err != nil {
		t.Fatalf("failed to initialize clickhouse storage: %v", err)
	}

	// 3. Apply test schema (We use Memory engine for fast ephemeral testing)
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metrics (
			timestamp DateTime,
			value Float64
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// 4. Setup Dummy Data
	now := time.Now().UTC().Truncate(time.Second) // ClickHouse DateTime resolution is in seconds

	// Insert one point outside of our test range (2 hours ago)
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value) VALUES (?, ?)", now.Add(-2*time.Hour), 10.5)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Insert one point INSIDE our test range (1 hour ago)
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value) VALUES (?, ?)", now.Add(-1*time.Hour), 42.0)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// 5. TEST: GetMetrics (Time range filtering)
	// We query the last 90 minutes. It should only return the 42.0 value.
	start := now.Add(-90 * time.Minute)
	end := now

	results, err := store.GetMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected exactly 1 result inside time range, got %d", len(results))
	}

	if results[0].Value != 42.0 {
		t.Errorf("expected value 42.0, got %f", results[0].Value)
	}
}

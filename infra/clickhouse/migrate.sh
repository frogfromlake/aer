#!/bin/sh
# ClickHouse Migration Runner
# Executes versioned SQL migrations idempotently via a tracking table.

set -e

CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-clickhouse}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"
MIGRATIONS_DIR="/migrations"

ch_query() {
    clickhouse-client --host "$CLICKHOUSE_HOST" --port "$CLICKHOUSE_PORT" --multiquery --query "$1"
}

ch_file() {
    clickhouse-client --host "$CLICKHOUSE_HOST" --port "$CLICKHOUSE_PORT" --multiquery < "$1"
}

# Ensure tracking database and table exist
ch_query "CREATE DATABASE IF NOT EXISTS aer_gold;"
ch_query "
CREATE TABLE IF NOT EXISTS aer_gold.schema_migrations (
    version UInt32,
    applied_at DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY version;
"

# Iterate migration files in order
for migration_file in "$MIGRATIONS_DIR"/*.sql; do
    [ -f "$migration_file" ] || continue

    filename=$(basename "$migration_file")
    version=$(echo "$filename" | grep -oE '^[0-9]+' | sed 's/^0*//')

    if [ -z "$version" ]; then
        echo "WARN: Skipping $filename — could not extract version number"
        continue
    fi

    already_applied=$(ch_query "SELECT count() FROM aer_gold.schema_migrations WHERE version = $version;" | tr -d '[:space:]')

    if [ "$already_applied" -gt 0 ]; then
        echo "SKIP: Migration $filename (version $version already applied)"
        continue
    fi

    echo "APPLY: Migration $filename (version $version)..."
    ch_file "$migration_file"
    ch_query "INSERT INTO aer_gold.schema_migrations (version) VALUES ($version);"
    echo "  OK: Migration $version applied successfully"
done

echo "ClickHouse migrations complete."

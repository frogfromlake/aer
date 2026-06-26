# Backup, Restore & Rollback Runbook

> Covers SEC-031 (backup of the four irreplaceable stores), SEC-047 (ClickHouse
> AggregatingMergeTree partitions captured directly, never replayed), SEC-048
> (restore order-of-operations), SEC-035 (rollback = restore-from-backup), and
> SEC-050 (DSGVO residency + right-to-erasure interaction).
>
> **This runbook MUST be exercised end-to-end (a real backup + a test restore
> into a throwaway stack) before launch and re-validated whenever the backup
> format changes.** An untested restore discovered mid-incident is the failure
> this document exists to prevent.

## 1. What is backed up, and why these four

The documented "wipe-and-recrawl" recovery (ADR-022) covers **none** of these:

| Store | Captured by | Why it is irreplaceable |
|---|---|---|
| **Postgres `aer_metadata`** | `pg_dump -Fc` | Auth (users + argon2id hashes + consent timestamps), sessions, single-use tokens, saved analyses, **crawler_state** conditional-GET cursors, sources/documents metadata. Pure identity/consent state — unreconstructable by re-crawl. |
| **ClickHouse `aer_gold`** | native `BACKUP DATABASE` | The Gold metrics **and the AggregatingMergeTree resolution MVs** (`metrics_hourly/daily/monthly`). The monthly MV is the **indefinite** "climate record" and is unrebuildable once raw metrics age past their 365d TTL — so we snapshot the aggregate parts directly (SEC-047), never via replay. |
| **MinIO `bronze`** | `mc mirror` | Raw HTML verbatim — the **only** replay source for any future Silver/Gold extractor upgrade. Web pages mutate/404, so once gone it is gone forever. |
| **MinIO `silver`** | `mc mirror` | Refined text + metadata; re-derivable from Bronze only if Bronze survives. |

All four live on plain local named volumes on one box — one disk failure takes
them simultaneously. The backup ships them **off-box**, **encrypted**, **in-EU**.

## 2. Architecture (decisions D1–D3)

- **Transport + encryption + retention:** a single **restic** repository on an
  in-EU **Hetzner Storage Box** (SFTP). restic does client-side encryption
  (a leaked provider cannot read backups), dedup, and retention prune.
- **Retention:** `BACKUP_RETENTION_DAYS=35` (D2) — bounds DSGVO right-to-erasure
  persistence (see §6).
- **Key custody:** `RESTIC_PASSWORD` is escrowed **off-box** (operator's password
  manager). **Losing it = permanently unrecoverable backups, by design.**
- **Heartbeat:** `backup.sh` writes Prometheus textfile metrics
  (`aer_backup_last_success_timestamp_seconds`, `aer_backup_last_exit_code`) read
  by node-exporter → `BackupStale` / `BackupFailed` / `BackupNeverRan` alerts.

## 3. Host prerequisites (one-time, at deploy)

1. **Install restic** on the production host (`apt-get install restic`).
2. **SSH to the Storage Box:** generate/upload an SSH key for the Storage Box and
   add its host key to `~/.ssh/known_hosts` so the `sftp:` restic repo resolves
   non-interactively. Test: `ssh u123456@u123456.your-storagebox.de`.
3. **Create the heartbeat dir:** `sudo mkdir -p /var/lib/aer/textfile` (node-exporter
   reads it via `--collector.textfile.directory=/host/var/lib/aer/textfile`).
4. **Set the backup secrets** in `.env` (see `.env.example` → "OFF-BOX BACKUPS"):
   `RESTIC_REPOSITORY`, `RESTIC_PASSWORD`, `BACKUP_RETENTION_DAYS`.
5. For a prod stack, export `COMPOSE_FILE=compose.yaml:compose.prod.yaml` so the
   script targets the prod composition.

## 4. Taking a backup

```bash
make backup          # loads .env, runs scripts/operations/backup.sh
```

The first run runs `restic init` automatically. Subsequent runs are incremental
(dedup). The script writes its heartbeat metric on every run (success or fail).

**Scheduling:** in production the backup runs **daily at 04:00** via the host
systemd-timer `infra/systemd/aer-backup.timer` (SEC-041 — install steps in
[scheduled_work.md](scheduled_work.md#production-runtime-scheduling-host-systemd-timers-phase-149-sec-041)).
The `BackupStale`/`BackupFailed` alerts backstop a missed or failed run. You can
always take an out-of-band backup with `make backup`.

**Verify a backup exists off-box:**
```bash
set -a; . ./.env; set +a
restic snapshots --tag aer
```

## 5. Restore (SEC-048) — order matters

ClickHouse `ReplacingMergeTree` + insert-deduplication and the medallion
key-continuity make a **naïve restore corrupt**. Follow this order exactly.

> **Golden rule:** restore is **restore-from-snapshot**, NEVER a pipeline replay.
> Replaying repopulates raw metrics that re-TTL and silently loses the >365d
> aggregate record (SEC-047).

**0. Pause the pipeline** so no new writes race the restore:
```bash
docker compose stop web-crawler analysis-worker ingestion-api
```

**1. Roles first.** The `bff_readonly` / `bff_auth` grants live in
`infra/postgres/init-roles.sh`, NOT in the data dump. A restored DB needs them
re-applied before the BFF can connect. Bring up Postgres + let
`postgres-init-roles` run (or re-run it) against the restored DB.

**2. Pull the snapshot** into a staging dir:
```bash
set -a; . ./.env; set +a
restic restore latest --tag aer --target /tmp/aer-restore
```

**3. Postgres** — restore the logical dump (this is the seeded + live schema; do
**not** also re-run seed migrations, which would double-apply seeded sources /
the equivalence grant):
```bash
docker compose exec -T -e PGPASSWORD="$POSTGRES_PASSWORD" postgres \
  pg_restore -U "$POSTGRES_USER" --clean --if-exists -d "$POSTGRES_DB" \
  < /tmp/aer-restore/postgres/${POSTGRES_DB}.dump
```

**4. ClickHouse** — restore the native backup, then `OPTIMIZE FINAL` the restored
tables so ReplacingMergeTree collapses any duplicate parts **before** the
pipeline resumes (prevents permanent MV over-counts):
```bash
# copy the tarred backup into the clickhouse_backups volume, untar, then:
docker compose exec -T clickhouse clickhouse-client -u "$CLICKHOUSE_USER" \
  --password "$CLICKHOUSE_PASSWORD" \
  -q "RESTORE DATABASE ${CLICKHOUSE_DB} FROM Disk('backups', '<name>')"
docker compose exec -T clickhouse clickhouse-client -u "$CLICKHOUSE_USER" \
  --password "$CLICKHOUSE_PASSWORD" \
  -q "OPTIMIZE TABLE ${CLICKHOUSE_DB}.metrics FINAL"   # + entities, etc.
```

**5. MinIO** — mirror the buckets back from the staging dir (`--remove` keeps the
bucket an exact image of the snapshot; omit it to merge):
```bash
docker compose run --rm --no-deps -T -v /tmp/aer-restore/minio:/mirror \
  --entrypoint /bin/sh minio-init -c '
    mc alias set src "http://minio:9000" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
    mc mirror --overwrite /mirror/bronze src/bronze
    mc mirror --overwrite /mirror/silver src/silver'
```

**6. Resume** the pipeline:
```bash
docker compose start ingestion-api analysis-worker web-crawler
```

A scripted version of the above (for the pre-launch test into a throwaway stack)
is `scripts/operations/restore.sh`. Run the **test restore at least once before
launch** and after any backup-format change; record the measured RTO.

## 6. DSGVO residency + right-to-erasure (SEC-050)

- **Residency:** the restic repository MUST be in-EU under a GDPR processor
  (Hetzner Storage Box = Germany). Encryption (restic client-side) means a leaked
  backup is not a breach.
- **Erasure interaction:** the live app hard-deletes a user (`DeleteAuthMe` →
  cascade), but the email/hash/consent persists in backups until it **ages out**.
  The 35d retention window (`BACKUP_RETENTION_DAYS`) is the bound: an erased user
  is gone from all backups within 35 days. Do **not** raise retention without
  re-documenting this erasure tail.

## 7. Rollback (SEC-035)

Migrations are **forward-only**; image-tag downgrade against already-migrated
volumes is undefined. Therefore **rollback = restore from the pre-upgrade
backup**, not a tag downgrade:

1. **Before any schema-changing upgrade**, take a fresh backup (`make backup`)
   and confirm it (`restic snapshots`). This is a mandatory pre-upgrade gate.
2. If the upgrade goes bad, restore that snapshot via §5.

This is why the backup mechanism (SEC-031) is a prerequisite for safe upgrades.

#!/bin/sh
set -e

# Wait for MinIO to be ready
echo "Waiting for MinIO at ${MINIO_ENDPOINT}..."
# The 'mc alias set' command returns a non-zero exit code if the server is unreachable
until /usr/bin/mc alias set myminio "http://${MINIO_ENDPOINT}" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" > /dev/null 2>&1; do
  echo "MinIO not reachable - retrying..."
  sleep 1
done

# SEC-060 — run an idempotent mc command without swallowing real errors. The old
# blanket `|| true` hid auth failures, an unreachable broker, and malformed
# policies just as silently as the benign "already configured" case. This helper
# tolerates ONLY the idempotent re-run outcome and aborts (set -e) on anything
# else, so a genuinely broken event-notification or policy-attach fails the init
# container loudly instead of leaving the pipeline silently mis-provisioned.
mc_idempotent() {
  if out="$("$@" 2>&1)"; then
    return 0
  fi
  case "$out" in
    # "overlapping": `mc event add` is NOT idempotent — re-adding the SAME NATS
    # notification rule fails with "...overlapping prefixes... for the same event
    # types", which is precisely the already-configured case (seen on every
    # re-run of minio-init after the first successful provisioning). Tolerate it.
    *already*|*Already*|*exists*|*attached*|*in\ effect*|*overlapping*)
      echo "  (idempotent no-op: $out)"
      return 0 ;;
    *)
      echo "ERROR: mc command failed: $* :: $out" >&2
      return 1 ;;
  esac
}

# SEC-057 — read the lifecycle config back after import and assert the expected
# expiry is actually stored. `mc ilm import` can succeed-but-no-op or store a
# malformed rule; without this read-back the Bronze/Silver/Quarantine TTLs (the
# ILM that bounds disk growth — Data Lifecycle in CLAUDE.md) could be silently
# absent. Hard-fail on a present-but-wrong config; soft-warn only if the read-back
# tool itself yields nothing (so a future mc output-format change cannot wedge
# the boot on a false negative — the import's own exit still gates correctness).
assert_ilm_days() {
  bucket="$1"; want="$2"
  dump="$(/usr/bin/mc ilm rule export "myminio/${bucket}" 2>/dev/null || true)"
  if [ -z "$dump" ]; then
    echo "  ! WARN: could not read back ILM for '${bucket}' (mc output empty); relying on import exit status" >&2
    return 0
  fi
  # The minio/mc image is minimal — no grep/tr/awk. Match with the POSIX shell
  # `case` builtin only (same approach as mc_idempotent). The export is compact
  # JSON, e.g. {"Expiration":{"Days":90},...}; require a non-digit boundary
  # ("}" or ",") after the value so 90 cannot match 900.
  case "$dump" in
    *"\"Days\":${want}}"*|*"\"Days\":${want},"*|*"\"Days\": ${want}}"*|*"\"Days\": ${want},"*)
      echo "  ✔ ${bucket} ILM: ${want}-day expiry confirmed" ;;
    *)
      echo "ERROR: ILM applied but '${bucket}' is missing the expected ${want}-day expiry. Dump: ${dump}" >&2
      exit 1 ;;
  esac
}

# Create required buckets
echo "Creating buckets..."
/usr/bin/mc mb myminio/bronze --ignore-existing
/usr/bin/mc mb myminio/silver --ignore-existing
/usr/bin/mc mb myminio/bronze-quarantine --ignore-existing

# Configure Information Lifecycle Management (ILM)
echo "Applying Data Lifecycle Policies (ILM)..."
# Import JSON policy for the bronze bucket (90-day retention)
/usr/bin/mc ilm import myminio/bronze <<EOF
{
    "Rules": [
        {
            "ID": "ExpireOldBronzeData",
            "Status": "Enabled",
            "Expiration": { "Days": 90 }
        }
    ]
}
EOF

# Import JSON policy for the quarantine bucket (30-day retention)
/usr/bin/mc ilm import myminio/bronze-quarantine <<EOF
{
    "Rules": [
        {
            "ID": "ExpireOldQuarantineData",
            "Status": "Enabled",
            "Expiration": { "Days": 30 }
        }
    ]
}
EOF

# Import JSON policy for the silver bucket (365-day retention)
# The Gold layer (ClickHouse) retains all derived metrics independently, so Silver
# objects are safe to expire after one year. This conservative TTL was adopted in
# Phase 32 (R-3) before long-term growth data was available; revisit if measured
# Silver growth materially exceeds Bronze volume over a sustained period.
/usr/bin/mc ilm import myminio/silver <<EOF
{
    "Rules": [
        {
            "ID": "ExpireOldSilverData",
            "Status": "Enabled",
            "Expiration": { "Days": 365 }
        }
    ]
}
EOF

# SEC-057 — verify the three lifecycle policies actually landed (TTLs that bound
# disk growth). Aborts the init container if any expected expiry is missing.
echo "Verifying ILM policies were applied..."
assert_ilm_days bronze 90
assert_ilm_days bronze-quarantine 30
assert_ilm_days silver 365

# Enable Event Notifications
echo "Linking bucket events to NATS..."
# SEC-060 — was `|| true`, which hid a broken NATS event wiring (the pipeline's
# Bronze→worker trigger). mc_idempotent tolerates the already-configured re-run
# but fails the init on a real error.
mc_idempotent /usr/bin/mc event add myminio/bronze arn:minio:sqs::aer:nats --event put

# ----------------------------------------------------------------------------
# Service accounts (Phase 79)
# Replace MINIO_ROOT_USER usage in long-running services with least-privilege
# user accounts scoped to the buckets each service actually touches.
#   - aer_ingestion : write-only on bronze
#   - aer_worker    : read on bronze, read/write on silver and bronze-quarantine
# Root credentials remain for setup.sh + minio-init only.
# ----------------------------------------------------------------------------
echo "Provisioning service accounts and policies..."

: "${INGESTION_MINIO_ACCESS_KEY:?INGESTION_MINIO_ACCESS_KEY must be set}"
: "${INGESTION_MINIO_SECRET_KEY:?INGESTION_MINIO_SECRET_KEY must be set}"
: "${WORKER_MINIO_ACCESS_KEY:?WORKER_MINIO_ACCESS_KEY must be set}"
: "${WORKER_MINIO_SECRET_KEY:?WORKER_MINIO_SECRET_KEY must be set}"
: "${BFF_MINIO_ACCESS_KEY:?BFF_MINIO_ACCESS_KEY must be set}"
: "${BFF_MINIO_SECRET_KEY:?BFF_MINIO_SECRET_KEY must be set}"

cat > /tmp/aer_ingestion_policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetBucketLocation", "s3:ListBucket"],
      "Resource": ["arn:aws:s3:::bronze"]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:PutObject", "s3:GetObject"],
      "Resource": ["arn:aws:s3:::bronze/*"]
    }
  ]
}
EOF

cat > /tmp/aer_bff_policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetBucketLocation", "s3:ListBucket"],
      "Resource": ["arn:aws:s3:::silver", "arn:aws:s3:::bronze"]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::silver/*", "arn:aws:s3:::bronze/*"]
    }
  ]
}
EOF

cat > /tmp/aer_worker_policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetBucketLocation", "s3:ListBucket"],
      "Resource": [
        "arn:aws:s3:::bronze",
        "arn:aws:s3:::silver",
        "arn:aws:s3:::bronze-quarantine"
      ]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::bronze/*"]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:PutObject", "s3:GetObject"],
      "Resource": [
        "arn:aws:s3:::silver/*",
        "arn:aws:s3:::bronze-quarantine/*"
      ]
    }
  ]
}
EOF

# Idempotent user + policy provisioning. `mc admin user add` is a no-op if the
# user already exists with the same secret; `policy create` rewrites in-place.
/usr/bin/mc admin user add myminio "${INGESTION_MINIO_ACCESS_KEY}" "${INGESTION_MINIO_SECRET_KEY}"
/usr/bin/mc admin user add myminio "${WORKER_MINIO_ACCESS_KEY}" "${WORKER_MINIO_SECRET_KEY}"
/usr/bin/mc admin user add myminio "${BFF_MINIO_ACCESS_KEY}" "${BFF_MINIO_SECRET_KEY}"

/usr/bin/mc admin policy create myminio aer_ingestion_policy /tmp/aer_ingestion_policy.json || \
  /usr/bin/mc admin policy update myminio aer_ingestion_policy /tmp/aer_ingestion_policy.json
/usr/bin/mc admin policy create myminio aer_worker_policy /tmp/aer_worker_policy.json || \
  /usr/bin/mc admin policy update myminio aer_worker_policy /tmp/aer_worker_policy.json
/usr/bin/mc admin policy create myminio aer_bff_policy /tmp/aer_bff_policy.json || \
  /usr/bin/mc admin policy update myminio aer_bff_policy /tmp/aer_bff_policy.json

# SEC-060 — was `|| true`, which silently left a service account with NO policy
# (every authd request would then 403). mc_idempotent tolerates the
# already-attached re-run but fails the init on a real attach error.
mc_idempotent /usr/bin/mc admin policy attach myminio aer_ingestion_policy --user "${INGESTION_MINIO_ACCESS_KEY}"
mc_idempotent /usr/bin/mc admin policy attach myminio aer_worker_policy --user "${WORKER_MINIO_ACCESS_KEY}"
mc_idempotent /usr/bin/mc admin policy attach myminio aer_bff_policy --user "${BFF_MINIO_ACCESS_KEY}"

rm -f /tmp/aer_ingestion_policy.json /tmp/aer_worker_policy.json /tmp/aer_bff_policy.json

echo "AĒR Data Lake provisioned successfully."
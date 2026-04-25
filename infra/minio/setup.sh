#!/bin/sh
set -e

# Wait for MinIO to be ready
echo "Waiting for MinIO at ${MINIO_ENDPOINT}..."
# The 'mc alias set' command returns a non-zero exit code if the server is unreachable
until /usr/bin/mc alias set myminio "http://${MINIO_ENDPOINT}" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" > /dev/null 2>&1; do
  echo "MinIO not reachable - retrying..."
  sleep 1
done

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

# Enable Event Notifications
echo "Linking bucket events to NATS..."
# Force the event addition. We use '|| true' to ensure the script continues
# even if the event notification is already configured.
/usr/bin/mc event add myminio/bronze arn:minio:sqs::aer:nats --event put || true

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

/usr/bin/mc admin policy attach myminio aer_ingestion_policy --user "${INGESTION_MINIO_ACCESS_KEY}" || true
/usr/bin/mc admin policy attach myminio aer_worker_policy --user "${WORKER_MINIO_ACCESS_KEY}" || true
/usr/bin/mc admin policy attach myminio aer_bff_policy --user "${BFF_MINIO_ACCESS_KEY}" || true

rm -f /tmp/aer_ingestion_policy.json /tmp/aer_worker_policy.json /tmp/aer_bff_policy.json

echo "AĒR Data Lake provisioned successfully."
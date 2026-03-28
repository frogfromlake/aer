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

# Enable Event Notifications
echo "Linking bucket events to NATS..."
# Force the event addition. We use '|| true' to ensure the script continues 
# even if the event notification is already configured.
/usr/bin/mc event add myminio/bronze arn:minio:sqs::aer:nats --event put || true

echo "AĒR Data Lake provisioned successfully."
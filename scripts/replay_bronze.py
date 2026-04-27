"""One-shot Bronze replay: publishes a synthetic MinIO S3 notification to
aer.lake.bronze for every object in the bronze bucket so the analysis worker
re-processes historical data without a re-crawl.

Run inside the analysis-worker container:
  docker cp scripts/replay_bronze.py aer_analysis_worker:/tmp/replay_bronze.py
  docker compose exec analysis-worker python3 /tmp/replay_bronze.py

The worker's idempotency check (PostgreSQL documents table) will skip objects
that have already been processed; only unprocessed objects get new Gold rows.
"""
import asyncio
import json
import os

import nats as nats_client
from minio import Minio


BUCKET = "bronze"
SUBJECT = "aer.lake.bronze"


def build_minio_client() -> Minio:
    return Minio(
        endpoint=os.getenv("MINIO_ENDPOINT", "localhost:9000"),
        access_key=os.getenv("WORKER_MINIO_ACCESS_KEY", ""),
        secret_key=os.getenv("WORKER_MINIO_SECRET_KEY", ""),
        secure=os.getenv("MINIO_SECURE", "false").lower() == "true",
    )


def make_event(key: str, size: int, event_time_iso: str) -> bytes:
    payload = {
        "EventName": "s3:ObjectCreated:Put",
        "Key": f"{BUCKET}/{key}",
        "Records": [{
            "eventVersion": "2.0",
            "eventSource": "minio:s3",
            "awsRegion": "",
            "eventTime": event_time_iso,
            "eventName": "s3:ObjectCreated:Put",
            "s3": {
                "s3SchemaVersion": "1.0",
                "bucket": {
                    "name": BUCKET,
                    "arn": f"arn:aws:s3:::{BUCKET}",
                },
                "object": {
                    "key": key,
                    "size": size,
                    "contentType": "application/json",
                    "userMetadata": {},
                },
            },
        }],
    }
    return json.dumps(payload).encode()


async def main() -> None:
    minio = build_minio_client()
    nc = await nats_client.connect(os.getenv("NATS_URL", "nats://localhost:4222"))
    js = nc.jetstream()

    objects = list(minio.list_objects(BUCKET, recursive=True))
    total = len(objects)
    print(f"Found {total} objects in {BUCKET}/", flush=True)

    published = 0
    for obj in objects:
        if not obj.object_name.endswith(".json"):
            continue
        stat = minio.stat_object(BUCKET, obj.object_name)
        event_time = stat.last_modified.strftime("%Y-%m-%dT%H:%M:%S.000Z")
        msg = make_event(obj.object_name, obj.size or 0, event_time)
        await js.publish(SUBJECT, msg)
        published += 1
        if published % 20 == 0:
            print(f"  {published}/{total} published…", flush=True)

    await nc.drain()
    print(f"Done. Published {published} events.", flush=True)


if __name__ == "__main__":
    asyncio.run(main())

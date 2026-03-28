import asyncio
import os
import json
import io
from datetime import datetime
import structlog
from minio import Minio
import clickhouse_connect
from nats.aio.client import Client as NATS
from opentelemetry import trace, propagate
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

# --- 1. Observability Setup ---
resource = Resource(attributes={SERVICE_NAME: "aer-analysis-worker"})
provider = TracerProvider(resource=resource)
# Send traces to our Collector container
processor = BatchSpanProcessor(OTLPSpanExporter(endpoint="http://localhost:4318/v1/traces"))
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)
tracer = trace.get_tracer(__name__)

logger = structlog.get_logger()

async def main():
    nc = NATS()
    
    # Init MinIO Client
    minio_client = Minio(
        os.getenv("MINIO_ENDPOINT", "localhost:9000"),
        access_key=os.getenv("MINIO_ROOT_USER", "aer_admin"),
        secret_key=os.getenv("MINIO_ROOT_PASSWORD", "aer_password_123"),
        secure=False
    )
    
    # Init ClickHouse Client
    ch_client = clickhouse_connect.get_client(
        host='localhost', port=8123, 
        username=os.getenv("CLICKHOUSE_USER", "aer_admin"), 
        password=os.getenv("CLICKHOUSE_PASSWORD", "aer_password_123")
    )

    # Automatically provision the Gold database and table if they don't exist
    ch_client.command('CREATE DATABASE IF NOT EXISTS aer_gold')
    ch_client.command('CREATE TABLE IF NOT EXISTS aer_gold.metrics (timestamp DateTime, value Float64) ENGINE = MergeTree() ORDER BY timestamp')

    async def message_handler(msg):
        event_data = json.loads(msg.data.decode())
        obj_key = event_data['Records'][0]['s3']['object']['key']
        
        # --- 2. TRACE CONTINUATION ---
        # MinIO wraps our metadata with 'X-Amz-Meta-'. We normalize it so OTel can find 'traceparent'.
        raw_meta = event_data['Records'][0]['s3']['object'].get('userMetadata', {})
        normalized_meta = {k.lower().replace('x-amz-meta-', ''): v for k, v in raw_meta.items()}
        
        context = propagate.extract(normalized_meta)
        
        with tracer.start_as_current_span("Process-Harmonization-And-Analytics", context=context) as span:
            logger.info("Processing event", object=obj_key)

            # --- 3. BRONZE -> SILVER ---
            response = minio_client.get_object("bronze", obj_key)
            raw_content = json.loads(response.read().decode('utf-8'))
            
            # Harmonization: Lowercase the message
            raw_content['message'] = raw_content['message'].lower()
            raw_content['status'] = 'harmonized'
            
            silver_payload = json.dumps(raw_content).encode('utf-8')
            minio_client.put_object(
                "silver", obj_key, 
                io.BytesIO(silver_payload), len(silver_payload),
                content_type="application/json"
            )
            logger.info("Silver layer updated")

            # --- 4. SILVER -> GOLD ---
            metric_val = raw_content.get('metric_value', 0)
            ch_client.insert(
                'aer_gold.metrics', 
                [[datetime.now(), metric_val]], 
                column_names=['timestamp', 'value']
            )
            logger.info("Gold layer updated", metric=metric_val)
            
            # Attach business data to the trace!
            span.set_attribute("aer.metric_value", metric_val)

    # Connect and Subscribe
    await nc.connect("nats://localhost:4222")
    await nc.subscribe("aer.lake.bronze", cb=message_handler)
    
    logger.info("Analysis Worker is running and waiting for events...")
    while True:
        await asyncio.sleep(1)

if __name__ == '__main__':
    asyncio.run(main())
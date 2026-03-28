import asyncio
import json
import structlog
from nats.aio.client import Client as NATS

# OpenTelemetry imports
from opentelemetry import trace, propagate
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

# Internal application imports
from internal.storage import init_minio, init_clickhouse
from internal.processor import DataProcessor

# --- Observability Setup ---
resource = Resource(attributes={SERVICE_NAME: "aer-analysis-worker"})
provider = TracerProvider(resource=resource)
# Send traces to the OpenTelemetry Collector container
processor = BatchSpanProcessor(OTLPSpanExporter(endpoint="http://localhost:4318/v1/traces"))
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)
tracer = trace.get_tracer(__name__)

logger = structlog.get_logger()

async def main():
    # 1. Initialize Infrastructure Clients
    minio_client = init_minio()
    ch_client = init_clickhouse()
    
    # 2. Dependency Injection for Core Processor
    data_processor = DataProcessor(minio_client, ch_client)

    # 3. Message Broker Setup
    nc = NATS()

    async def message_handler(msg):
        event_data = json.loads(msg.data.decode())
        
        # MinIO structured event payload extraction
        obj_key = event_data['Records'][0]['s3']['object']['key']
        
        # --- TRACE CONTINUATION ---
        # Normalize MinIO metadata to extract the OTel 'traceparent'
        raw_meta = event_data['Records'][0]['s3']['object'].get('userMetadata', {})
        normalized_meta = {k.lower().replace('x-amz-meta-', ''): v for k, v in raw_meta.items()}
        context = propagate.extract(normalized_meta)
        
        # Start the span and pass it to the processor
        with tracer.start_as_current_span("Process-Harmonization-And-Analytics", context=context) as span:
            logger.info("Received event", object=obj_key)
            
            # Delegate all business logic to the processor
            data_processor.process_event(obj_key, span)

    # 4. Connect and Subscribe
    await nc.connect("nats://localhost:4222")
    await nc.subscribe("aer.lake.bronze", cb=message_handler)
    
    logger.info("Analysis Worker initialized and awaiting events...")
    
    # Keep the worker running
    while True:
        await asyncio.sleep(1)

if __name__ == '__main__':
    asyncio.run(main())
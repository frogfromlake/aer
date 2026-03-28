import asyncio
import json
import signal
import structlog
from nats.aio.client import Client as NATS
from tenacity import retry, wait_exponential, stop_after_delay
from dotenv import load_dotenv

# OpenTelemetry imports
from opentelemetry import trace, propagate
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

# Internal application imports
from internal.storage import init_minio, init_clickhouse
from internal.processor import DataProcessor

# Load environment variables from .env file
load_dotenv()

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
        obj_key = event_data['Records'][0]['s3']['object']['key']
        
        # --- TRACE CONTINUATION ---
        raw_meta = event_data['Records'][0]['s3']['object'].get('userMetadata', {})
        normalized_meta = {k.lower().replace('x-amz-meta-', ''): v for k, v in raw_meta.items()}
        context = propagate.extract(normalized_meta)
        
        with tracer.start_as_current_span("Process-Harmonization-And-Analytics", context=context) as span:
            logger.info("Received event", object=obj_key)
            data_processor.process_event(obj_key, span)

    # 4. Connect and Subscribe with Backoff
    @retry(
        wait=wait_exponential(multiplier=1, min=1, max=10),
        stop=stop_after_delay(30),
        before_sleep=lambda rs: logger.warning("NATS not ready, retrying...", attempt=rs.attempt_number)
    )
    async def connect_nats():
        await nc.connect("nats://localhost:4222")

    await connect_nats()
    await nc.subscribe("aer.lake.bronze", cb=message_handler)
    
    logger.info("Analysis Worker initialized and awaiting events...")
    
    # --- GRACEFUL SHUTDOWN LOGIC ---
    stop_event = asyncio.Event()
    
    def shutdown_signal(*args):
        logger.info("Shutdown signal received. Draining NATS and closing worker...")
        stop_event.set()

    # Register handlers for termination signals
    loop = asyncio.get_running_loop()
    loop.add_signal_handler(signal.SIGINT, shutdown_signal)
    loop.add_signal_handler(signal.SIGTERM, shutdown_signal)

    try:
        # Wait until the stop event is set
        await stop_event.wait()
    except asyncio.CancelledError:
        pass # Ignore cancellation when forced to quit
    finally:
        # Cleanup before exiting
        if nc.is_connected:
            await nc.drain()
            await nc.close()
        logger.info("Analysis Worker shut down cleanly.")

if __name__ == '__main__':
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        # Catch the final exit to avoid stack traces in the terminal
        pass
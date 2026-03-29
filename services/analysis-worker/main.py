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
from internal.storage import init_minio, init_clickhouse, init_postgres
from internal.processor import DataProcessor

load_dotenv()

# --- Observability Setup ---
resource = Resource(attributes={SERVICE_NAME: "aer-analysis-worker"})
provider = TracerProvider(resource=resource)
processor = BatchSpanProcessor(OTLPSpanExporter(endpoint="http://localhost:4318/v1/traces"))
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)
tracer = trace.get_tracer(__name__)

logger = structlog.get_logger()

# --- Concurrency Setup ---
WORKER_COUNT = 5  # Number of parallel tasks
task_queue = asyncio.Queue()

async def worker_task(worker_id: int, data_processor: DataProcessor):
    """Background worker that processes events asynchronously from the queue."""
    logger.info("Worker started", worker_id=worker_id)
    while True:
        msg = await task_queue.get()
        try:
            event_data = json.loads(msg.data.decode())
            obj_key = event_data['Records'][0]['s3']['object']['key']
            
            # --- EXTRACT DETERMINISTIC TIMESTAMP ---
            event_time_str = event_data['Records'][0]['eventTime'] # e.g. "2023-10-25T12:34:56.000Z"
            
            # --- TRACE CONTINUATION ---
            raw_meta = event_data['Records'][0]['s3']['object'].get('userMetadata', {})
            normalized_meta = {k.lower().replace('x-amz-meta-', ''): v for k, v in raw_meta.items()}
            context = propagate.extract(normalized_meta)
            
            with tracer.start_as_current_span("Process-Harmonization-And-Analytics", context=context) as span:
                logger.info("Processing event", worker_id=worker_id, object=obj_key)
                # Since MinIO/ClickHouse clients are synchronous, we offload them to a thread pool
                # to avoid blocking the asyncio event loop. Now passing event_time_str!
                await asyncio.to_thread(data_processor.process_event, obj_key, event_time_str, span)

            # IMPORTANT: Manual Ack only AFTER error-free processing (JetStream)
            await msg.ack()
        
        except Exception as e:
            logger.error("Error processing message. Event will be redelivered.", worker_id=worker_id, error=str(e))
            # NAK triggers a faster redelivery than a timeout
            await msg.nak()
        finally:
            task_queue.task_done()


async def main():
    minio_client = init_minio()
    ch_client = init_clickhouse()
    pg_pool = init_postgres()
    
    # Inject the pool
    data_processor = DataProcessor(minio_client, ch_client, pg_pool)
    nc = NATS()

    @retry(
        wait=wait_exponential(multiplier=1, min=1, max=10),
        stop=stop_after_delay(30),
        before_sleep=lambda rs: logger.warning("NATS not ready, retrying...", attempt=rs.attempt_number)
    )
    async def connect_nats():
        await nc.connect("nats://localhost:4222")

    await connect_nats()
    
    # 1. Enable JetStream
    js = nc.jetstream()

    # Stream provisioning: Ensure the stream exists (idempotent)
    try:
        await js.add_stream(name="AER_LAKE", subjects=["aer.lake.bronze"])
        logger.info("JetStream Stream 'AER_LAKE' ensured.")
    except Exception as e:
        logger.warning("Note on Stream creation", error=str(e))

    # 2. Start worker tasks
    workers = [
        asyncio.create_task(worker_task(i, data_processor)) 
        for i in range(WORKER_COUNT)
    ]

    # 3. Message Handler: Does not block, just pushes to the queue
    async def message_handler(msg):
        await task_queue.put(msg)

    # 4. Durable subscription to the MinIO stream
    await js.subscribe(
        "aer.lake.bronze", 
        durable="aer_analysis_worker",
        cb=message_handler,
        manual_ack=True
    )
    
    logger.info("Analysis Worker initialized (JetStream + Queue) and awaiting events...")
    
    # --- GRACEFUL SHUTDOWN LOGIC ---
    stop_event = asyncio.Event()
    
    def shutdown_signal(*args):
        logger.info("Shutdown signal received. Stopping worker...")
        stop_event.set()

    loop = asyncio.get_running_loop()
    loop.add_signal_handler(signal.SIGINT, shutdown_signal)
    loop.add_signal_handler(signal.SIGTERM, shutdown_signal)

    try:
        await stop_event.wait()
    except asyncio.CancelledError:
        pass
    finally:
        logger.info("Draining NATS...")
        if nc.is_connected:
            await nc.drain()
            
        logger.info("Cancelling background workers...")
        for w in workers:
            w.cancel()
            
        await nc.close()
        logger.info("Analysis Worker shut down cleanly.")

if __name__ == '__main__':
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
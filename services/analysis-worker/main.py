import asyncio
import json
import os
import signal
import structlog
from urllib.parse import unquote
from dataclasses import dataclass, field
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
from prometheus_client import start_http_server
from internal.storage import init_minio, init_clickhouse, init_postgres
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter
from internal.extractors import WordCountExtractor

load_dotenv()

logger = structlog.get_logger()


@dataclass
class WorkerConfig:
    """Configuration for the analysis worker, injectable for testing."""
    nats_url: str = field(default_factory=lambda: os.getenv("NATS_URL", "nats://localhost:4222"))
    otel_endpoint: str = field(default_factory=lambda: os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"))
    worker_count: int = field(default_factory=lambda: int(os.getenv("WORKER_COUNT", "5")))
    stream_name: str = "AER_LAKE"
    subject: str = "aer.lake.bronze"
    durable_name: str = "aer_analysis_worker"


def init_telemetry(otel_endpoint: str) -> trace.Tracer:
    """Initialize OpenTelemetry tracing. Call once from main(), not at module scope."""
    resource = Resource(attributes={SERVICE_NAME: "aer-analysis-worker"})
    provider = TracerProvider(resource=resource)
    processor = BatchSpanProcessor(OTLPSpanExporter(endpoint=f"{otel_endpoint}/v1/traces"))
    provider.add_span_processor(processor)
    trace.set_tracer_provider(provider)
    return trace.get_tracer(__name__)


async def worker_task(worker_id: int, data_processor: DataProcessor, task_queue: asyncio.Queue, tracer: trace.Tracer):
    """Background worker that processes events asynchronously from the queue."""
    logger.info("Worker started", worker_id=worker_id)
    while True:
        msg = await task_queue.get()

        # --- CLEAN SHUTDOWN SENTINEL CHECK ---
        if msg is None:
            logger.info("Worker received shutdown signal. Exiting cleanly...", worker_id=worker_id)
            task_queue.task_done()
            break  # Exit the loop safely, releasing all resources

        try:
            event_data = json.loads(msg.data.decode())
            obj_key = unquote(event_data['Records'][0]['s3']['object']['key'])

            # --- EXTRACT DETERMINISTIC TIMESTAMP ---
            event_time_str = event_data['Records'][0]['eventTime'] # e.g. "2023-10-25T12:34:56.000Z"

            # --- TRACE CONTINUATION ---
            raw_meta = event_data['Records'][0]['s3']['object'].get('userMetadata', {})
            normalized_meta = {k.lower().replace('x-amz-meta-', ''): v for k, v in raw_meta.items()}
            context = propagate.extract(normalized_meta)

            with tracer.start_as_current_span("Process-Harmonization-And-Analytics", context=context) as span:
                logger.info("Processing event", worker_id=worker_id, object=obj_key)
                # Since MinIO/ClickHouse clients are synchronous, we offload them to a thread pool
                # to avoid blocking the asyncio event loop!
                await asyncio.to_thread(data_processor.process_event, obj_key, event_time_str, span)

            # IMPORTANT: Manual Ack only AFTER error-free processing (JetStream)
            await msg.ack()

        except Exception as e:
            logger.error("Error processing message. Event will be redelivered.", worker_id=worker_id, error=str(e))
            # NAK triggers a faster redelivery than a timeout
            await msg.nak()
        finally:
            task_queue.task_done()


async def main(config: WorkerConfig | None = None):
    if config is None:
        config = WorkerConfig()

    tracer = init_telemetry(config.otel_endpoint)

    metrics_port = int(os.getenv("METRICS_PORT", "8001"))
    start_http_server(metrics_port)
    logger.info("Prometheus metrics server started", port=metrics_port)

    minio_client = init_minio()
    ch_client = init_clickhouse()
    pg_pool = init_postgres()

    adapter_registry = AdapterRegistry({"legacy": LegacyAdapter(), "rss": RssAdapter()})
    extractors = [WordCountExtractor()]
    data_processor = DataProcessor(minio_client, ch_client, pg_pool, adapter_registry, extractors)
    nc = NATS()
    task_queue = asyncio.Queue()

    @retry(
        wait=wait_exponential(multiplier=1, min=1, max=10),
        stop=stop_after_delay(30),
        before_sleep=lambda rs: logger.warning("NATS not ready, retrying...", attempt=rs.attempt_number)
    )
    async def connect_nats():
        await nc.connect(config.nats_url)

    await connect_nats()

    # 1. Enable JetStream
    js = nc.jetstream()

    # Stream provisioning: Ensure the stream exists (idempotent)
    try:
        await js.add_stream(name=config.stream_name, subjects=[config.subject])
        logger.info("JetStream Stream ensured.", stream=config.stream_name)
    except Exception as e:
        logger.warning("Note on Stream creation", error=str(e))

    # 2. Start worker tasks
    workers = [
        asyncio.create_task(worker_task(i, data_processor, task_queue, tracer))
        for i in range(config.worker_count)
    ]

    # 3. Message Handler: Does not block, just pushes to the queue
    async def message_handler(msg):
        await task_queue.put(msg)

    # 4. Durable subscription to the MinIO stream
    await js.subscribe(
        config.subject,
        durable=config.durable_name,
        cb=message_handler,
        manual_ack=True
    )

    logger.info("Analysis Worker initialized (JetStream + Queue) and awaiting events...")

    # --- GRACEFUL SHUTDOWN LOGIC ---
    stop_event = asyncio.Event()

    def shutdown_signal(*args):
        logger.info("Shutdown signal received. Initiating graceful shutdown...")
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

        logger.info("Sending shutdown sentinels to background workers...")
        # Push exactly one Sentinel (None) per worker to the queue
        for _ in range(config.worker_count):
            await task_queue.put(None)

        logger.info("Waiting for workers to complete current tasks...")
        # Gracefully wait for all workers to finish instead of cancelling them
        await asyncio.gather(*workers)

        await nc.close()
        logger.info("Analysis Worker shut down cleanly.")

if __name__ == '__main__':
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass

import asyncio
import json
import os
import signal
import structlog
from urllib.parse import unquote
from dataclasses import dataclass, field
from nats.aio.client import Client as NATS
from nats.js import api as js_api
from tenacity import retry, wait_exponential, stop_after_delay
from dotenv import load_dotenv

# OpenTelemetry imports
from opentelemetry import trace, propagate
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.sampling import ParentBased, TraceIdRatioBased
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
# Internal application imports
from prometheus_client import start_http_server
from internal.storage import init_minio, init_clickhouse, init_postgres, PG_POOL_HEADROOM
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter
from internal.extractors import (
    WordCountExtractor,
    TemporalDistributionExtractor,
    LanguageDetectionExtractor,
    SentimentExtractor,
    NamedEntityExtractor,
    EntityCoOccurrenceExtractor,
)
from internal.corpus import CorpusConfig, corpus_extraction_loop

load_dotenv()

logger = structlog.get_logger()

REQUIRED_ENV_VARS = [
    "POSTGRES_PASSWORD",
    "WORKER_MINIO_ACCESS_KEY",
    "WORKER_MINIO_SECRET_KEY",
    "CLICKHOUSE_PASSWORD",
]


def validate_required_env(env_vars: list[str] | None = None) -> None:
    """Validate that required credentials are set and non-empty.

    Mirrors the Go services' boot-time validation pattern (config.Load()
    returns fmt.Errorf). Raises SystemExit so the container restarts
    instead of silently running with missing credentials.
    """
    if env_vars is None:
        env_vars = REQUIRED_ENV_VARS
    missing = [v for v in env_vars if not os.getenv(v, "").strip()]
    if missing:
        raise SystemExit(
            f"Fatal: required environment variables are empty or unset: {', '.join(missing)}"
        )


DEFAULT_EXTRACTOR_CLASSES = [
    WordCountExtractor,
    TemporalDistributionExtractor,
    LanguageDetectionExtractor,
    SentimentExtractor,
    NamedEntityExtractor,
]


def init_extractors(extractor_classes):
    """
    Instantiate extractors one-by-one, skipping any that raise during init.

    Hard-Rule graceful-degradation gate: a single misconfigured extractor
    (missing model, missing lexicon, unexpected environment) must never take
    down the worker. Failed extractors are logged and omitted from the pipeline.
    """
    extractors = []
    for cls in extractor_classes:
        try:
            extractors.append(cls())
        except Exception as e:
            logger.warning(
                "Extractor init failed — skipping",
                extractor=getattr(cls, "__name__", repr(cls)),
                error=str(e),
                error_type=type(e).__name__,
            )
    return extractors


@dataclass
class WorkerConfig:
    """Configuration for the analysis worker, injectable for testing."""
    nats_url: str = field(default_factory=lambda: os.getenv("NATS_URL", "nats://localhost:4222"))
    otel_endpoint: str = field(default_factory=lambda: os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"))
    otel_sample_rate: float = field(default_factory=lambda: float(os.getenv("OTEL_TRACE_SAMPLE_RATE", "1.0")))
    worker_count: int = field(default_factory=lambda: int(os.getenv("WORKER_COUNT", "5")))
    stream_name: str = "AER_LAKE"
    subject: str = "aer.lake.bronze"
    durable_name: str = "aer_analysis_worker"
    # Phase 83: JetStream consumer safety parameters. See `main.py` subscribe
    # call and ROADMAP §Phase 83 for rationale.
    max_deliver: int = 5
    ack_wait_seconds: float = 60.0


def init_telemetry(otel_endpoint: str, sample_rate: float = 1.0) -> trace.Tracer:
    """Initialize OpenTelemetry tracing. Call once from main(), not at module scope.

    `sample_rate` is wrapped in ParentBased so child spans inherit the
    parent's sampling decision, matching the Go services (see
    `pkg/telemetry/otel.go`).
    """
    resource = Resource(attributes={SERVICE_NAME: "aer-analysis-worker"})
    sampler = ParentBased(root=TraceIdRatioBased(sample_rate))
    provider = TracerProvider(resource=resource, sampler=sampler)
    processor = BatchSpanProcessor(OTLPSpanExporter(endpoint=f"{otel_endpoint}/v1/traces"))
    provider.add_span_processor(processor)
    trace.set_tracer_provider(provider)
    return trace.get_tracer(__name__)


def _num_delivered(msg) -> int:
    """Return the JetStream delivery count or 0 if unavailable (e.g. non-JS msg)."""
    try:
        return int(msg.metadata.num_delivered)
    except Exception:
        return 0


async def worker_task(
    worker_id: int,
    data_processor: DataProcessor,
    task_queue: asyncio.Queue,
    tracer: trace.Tracer,
    max_deliver: int,
):
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
            await _handle_message(worker_id, msg, data_processor, tracer, max_deliver)
        finally:
            task_queue.task_done()


async def _handle_message(
    worker_id: int,
    msg,
    data_processor: DataProcessor,
    tracer: trace.Tracer,
    max_deliver: int,
):
    """Process a single JetStream message with poison-pill containment.

    Phase 83: a deterministically-failing message (adapter bug, malformed
    envelope) must not recycle through the consumer forever. On the final
    allowed delivery attempt, the message is routed to `bronze-quarantine`
    via `DataProcessor.quarantine_poison_message` and ack'd — breaking the
    NAK→redeliver→NAK loop that would otherwise spin until `ack_wait`
    starves the whole pipeline.
    """
    try:
        event_data = json.loads(msg.data.decode())
        record = event_data["Records"][0]
        # MinIO URL-encodes the key in notifications.
        obj_key = unquote(record["s3"]["object"]["key"])
        event_time_str = record["eventTime"]

        raw_meta = record["s3"]["object"].get("userMetadata", {})
        normalized_meta = {k.lower().replace("x-amz-meta-", ""): v for k, v in raw_meta.items()}
        context = propagate.extract(normalized_meta)

        with tracer.start_as_current_span("Process-Harmonization-And-Analytics", context=context) as span:
            logger.info("Processing event", worker_id=worker_id, object=obj_key)
            # Sync MinIO/ClickHouse clients — offload to a thread pool to keep
            # the event loop responsive.
            await asyncio.to_thread(data_processor.process_event, obj_key, event_time_str, span)

        await msg.ack()

    except Exception as e:
        num_delivered = _num_delivered(msg)
        error_type = type(e).__name__
        if num_delivered >= max_deliver:
            logger.error(
                "Poison pill — max_deliver exhausted. Routing to quarantine.",
                worker_id=worker_id,
                num_delivered=num_delivered,
                max_deliver=max_deliver,
                error=str(e),
                error_type=error_type,
            )
            try:
                await asyncio.to_thread(
                    data_processor.quarantine_poison_message,
                    msg.data,
                    error_type,
                    str(e),
                )
                await msg.ack()
                return
            except Exception as quarantine_err:
                logger.error(
                    "Poison quarantine write failed; letting NATS drop via max_deliver.",
                    worker_id=worker_id,
                    error=str(quarantine_err),
                )
                await msg.nak()
                return

        logger.error(
            "Error processing message. Event will be redelivered.",
            worker_id=worker_id,
            num_delivered=num_delivered,
            error=str(e),
        )
        await msg.nak()


async def main(config: WorkerConfig | None = None):
    if config is None:
        config = WorkerConfig()

    validate_required_env()

    tracer = init_telemetry(config.otel_endpoint, config.otel_sample_rate)

    metrics_port = int(os.getenv("METRICS_PORT", "8001"))
    start_http_server(metrics_port)
    logger.info("Prometheus metrics server started", port=metrics_port)

    minio_client = init_minio()
    ch_client = init_clickhouse(pool_size=config.worker_count)
    # Phase 85: size the PG pool symmetrically with the CH pool so the
    # worker does not starve on PG connections when WORKER_COUNT is raised.
    pg_pool = init_postgres(maxconn=config.worker_count + PG_POOL_HEADROOM)

    adapter_registry = AdapterRegistry({"legacy": LegacyAdapter(), "rss": RssAdapter(pg_pool=pg_pool)})
    extractors = init_extractors(DEFAULT_EXTRACTOR_CLASSES)
    data_processor = DataProcessor(minio_client, ch_client, pg_pool, adapter_registry, extractors)
    nc = NATS()
    # Phase 83: bounded queue enforces backpressure. `put` blocks when
    # workers fall behind, which propagates back to JetStream via the
    # `max_ack_pending` cap set on the consumer below.
    queue_max_size = config.worker_count * 4
    task_queue: asyncio.Queue = asyncio.Queue(maxsize=queue_max_size)

    @retry(
        wait=wait_exponential(multiplier=1, min=1, max=10),
        stop=stop_after_delay(30),
        before_sleep=lambda rs: logger.warning("NATS not ready, retrying...", attempt=rs.attempt_number)
    )
    async def connect_nats():
        await nc.connect(config.nats_url)

    await connect_nats()

    # 1. Enable JetStream. Stream provisioning is IaC — handled by the
    #    `nats-init` container (see infra/nats/streams/AER_LAKE.json) and
    #    gated via `depends_on: nats-init: service_completed_successfully`.
    js = nc.jetstream()

    # 2. Start worker tasks
    workers = [
        asyncio.create_task(
            worker_task(i, data_processor, task_queue, tracer, config.max_deliver)
        )
        for i in range(config.worker_count)
    ]

    # 3. Message Handler: Does not block, just pushes to the queue.
    # `put` blocks when the queue is full, which pushes backpressure all the
    # way to the NATS consumer via `max_ack_pending`.
    async def message_handler(msg):
        await task_queue.put(msg)

    # 4. Durable subscription to the Bronze MinIO stream.
    #
    # Phase 83: consumer safety parameters. `max_ack_pending` matches the
    # bounded queue size so JetStream never delivers more than the worker
    # pool can hold in flight. `ack_wait` gives each document a generous
    # processing window before NATS retries. `max_deliver` caps retry
    # storms; the poison-pill handler in `_handle_message` catches the
    # final attempt and routes it to `bronze-quarantine`.
    consumer_config = js_api.ConsumerConfig(
        max_ack_pending=queue_max_size,
        ack_wait=config.ack_wait_seconds,
        max_deliver=config.max_deliver,
    )
    await js.subscribe(
        config.subject,
        durable=config.durable_name,
        cb=message_handler,
        manual_ack=True,
        config=consumer_config,
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

    # Phase 102: corpus-extraction loop (entity co-occurrence). Runs in the
    # same process as the per-document workers; idempotent via
    # ReplacingMergeTree(ingestion_version).
    corpus_config = CorpusConfig()
    corpus_task = asyncio.create_task(
        corpus_extraction_loop(
            ch_client,
            pg_pool,
            EntityCoOccurrenceExtractor(),
            corpus_config,
            stop_event,
        )
    )

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

        logger.info("Waiting for corpus-extraction loop to drain...")
        await corpus_task

        await nc.close()

        logger.info("Closing database connection pools...")
        try:
            ch_client.close_all()
        except Exception as e:
            logger.warning("Error closing ClickHouse pool", error=str(e))
        try:
            pg_pool.closeall()
        except Exception as e:
            logger.warning("Error closing PostgreSQL pool", error=str(e))

        logger.info("Analysis Worker shut down cleanly.")

if __name__ == '__main__':
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass

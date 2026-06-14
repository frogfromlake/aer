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
from internal.logging import configure_logging
from internal.metrics import nats_consumer_pending, nats_consumer_ack_pending
from internal.storage import init_minio, init_clickhouse, init_postgres, PG_POOL_HEADROOM
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter, WebAdapter
from internal.wayback import WaybackCDXCache, WaybackCDXClient
from internal.extractors import (
    WordCountExtractor,
    TemporalDistributionExtractor,
    LanguageDetectionExtractor,
    SentimentExtractor,
    MultilingualBertSentimentExtractor,
    GermanNewsBertSentimentExtractor,
    NamedEntityExtractor,
    EntityCoOccurrenceExtractor,
    MetricBaselineExtractor,
    TopicModelingExtractor,
    WikidataAliasIndex,
)
from internal.corpus import (
    BaselineConfig,
    CorpusConfig,
    TopicConfig,
    corpus_extraction_loop,
)
from internal.corpus_baseline_topic import (
    baseline_extraction_loop,
    topic_extraction_loop,
)
from internal.corpus_revision_diff import revision_diff_extraction_loop
from internal.corpus_revision_io import RevisionDiffConfig
from internal.reattempt import ReAttemptConfig, enrichment_reattempt_loop
from internal.wayback_reattempt import WaybackReAttemptTask
from internal.models.probe_scope import ProbeLanguageScope
from internal.wayback import WaybackSnapshotFetcher

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
    # Phase 119 / ADR-023: Tier-2 default + Tier-2.5 German-news refinement
    # register alongside Tier-1 SentiWS. Both produce no metric row when
    # transformers/torch are absent at runtime (graceful degradation), so
    # ordering them after the Tier-1 extractor keeps SentiWS's coverage
    # whole even if the BERT path fails to initialise.
    MultilingualBertSentimentExtractor,
    GermanNewsBertSentimentExtractor,
    NamedEntityExtractor,
]

# Phase 123 hardening — the background extraction loops (corpus co-occurrence,
# topic modelling, metric baselines, revision diffs, and the ADR-036 enrichment
# re-attempt loop) each hold a ClickHouse + Postgres connection for the duration
# of their sweep. They MUST NOT draw from the document-worker connection budget:
# when the pool was sized to worker_count only, a large backlog let the sweeps
# starve the doc-workers of connections and wedge the per-document hot path. The
# pools are therefore sized for workers + these loops + headroom, so the
# real-time path always has its own connections. Keep in sync with the
# create_task(...loop...) calls in main() — currently 5 background loops.
BACKGROUND_LOOP_COUNT = 5


def _init_wayback_clients(pg_pool) -> tuple[WaybackCDXClient | None, WaybackCDXClient | None]:
    """Construct the (inline, sweep) Wayback CDX clients, or (None, None) when
    disabled (ADR-036).

    Two clients, one shared Postgres cache:
      * **inline** — used by the WebAdapter on the per-article ingest path.
        Fail-fast: short timeout + a SINGLE retry (absorbs a transient blip so
        the breaker does not over-trip on jitter, but never adds a long retry
        chain to queue-drain). The circuit breaker bounds the blast radius.
      * **sweep** — used by the ADR-036 re-attempt loop. Patient: longer
        timeout + more retries, because background latency is irrelevant and we
        want each retry-tick to succeed if IA is merely slow.

    `WAYBACK_CDX_ENABLED=false` returns (None, None) — the WebAdapter then
    leaves `wayback_lookup_status=""`. Any construction error degrades to
    (None, None); we never abort worker boot for a CDX-side config issue.
    """
    if os.getenv("WAYBACK_CDX_ENABLED", "false").strip().lower() not in {"1", "true", "yes", "on"}:
        logger.info("Wayback CDX integration disabled (WAYBACK_CDX_ENABLED is off).")
        return None, None
    try:
        base_url = os.getenv("WAYBACK_CDX_BASE_URL", "https://web.archive.org/cdx/search/cdx").strip()
        rate = float(os.getenv("WAYBACK_CDX_RATE_LIMIT_PER_SECOND", "5.0"))
        user_agent = os.getenv(
            "WEB_CRAWLER_USER_AGENT",
            "AerWebCrawler/0.1 (+https://aer.example/about)",
        )
        cache = WaybackCDXCache(pg_pool) if pg_pool is not None else None
        cb_threshold = int(os.getenv("WAYBACK_CDX_CIRCUIT_FAILURE_THRESHOLD", "5"))
        cb_reset = float(os.getenv("WAYBACK_CDX_CIRCUIT_RESET_SECONDS", "60"))

        # Inline (ingest hot path): fail-fast — 5s + 1 retry.
        inline_timeout = float(os.getenv("WAYBACK_CDX_TIMEOUT_SECONDS", "5.0"))
        inline_retries = int(os.getenv("WAYBACK_CDX_INLINE_MAX_RETRIES", "1"))
        # Sweep (background re-attempt): patient — 30s + 3 retries.
        sweep_timeout = float(os.getenv("WAYBACK_CDX_REATTEMPT_TIMEOUT_SECONDS", "30.0"))
        sweep_retries = int(os.getenv("WAYBACK_CDX_REATTEMPT_MAX_RETRIES", "3"))

        inline = WaybackCDXClient(
            enabled=True,
            base_url=base_url,
            timeout_seconds=inline_timeout,
            rate_limit_per_second=rate,
            user_agent=user_agent,
            cache=cache,
            max_retries=inline_retries,
            circuit_failure_threshold=cb_threshold,
            circuit_reset_seconds=cb_reset,
        )
        sweep = WaybackCDXClient(
            enabled=True,
            base_url=base_url,
            timeout_seconds=sweep_timeout,
            rate_limit_per_second=rate,
            user_agent=user_agent,
            cache=cache,
            max_retries=sweep_retries,
            circuit_failure_threshold=cb_threshold,
            circuit_reset_seconds=cb_reset,
        )
        logger.info(
            "Wayback CDX integration enabled",
            base_url=base_url,
            inline_timeout_seconds=inline_timeout,
            inline_max_retries=inline_retries,
            sweep_timeout_seconds=sweep_timeout,
            sweep_max_retries=sweep_retries,
            cache_enabled=cache is not None,
        )
        return inline, sweep
    except Exception as exc:
        logger.warning(
            "Wayback CDX client init failed; continuing without silent-edit observability.",
            error=str(exc),
        )
        return None, None


def _init_snapshot_fetcher() -> WaybackSnapshotFetcher | None:
    """Construct the Phase 122d.1 snapshot fetcher, or None when disabled.

    Independent enable-flag from the CDX integration (Phase 122d.0).
    An operator can run with CDX enabled (revision *counts* observable)
    but snapshot diffs disabled (revision *substance* not yet wanted —
    e.g. when IA full-HTML bandwidth is constrained).
    """
    if os.getenv("REVISION_DIFF_EXTRACTION_ENABLED", "false").strip().lower() not in {
        "1", "true", "yes", "on",
    }:
        logger.info("Wayback snapshot fetcher disabled (REVISION_DIFF_EXTRACTION_ENABLED is off).")
        return None
    try:
        timeout = float(os.getenv("WAYBACK_SNAPSHOT_TIMEOUT_SECONDS", "15.0"))
        rate = float(os.getenv("WAYBACK_SNAPSHOT_RATE_LIMIT_PER_SECOND", "2.0"))
        user_agent = os.getenv(
            "WEB_CRAWLER_USER_AGENT",
            "AerWebCrawler/0.1 (+https://aer.example/about)",
        )
        fetcher = WaybackSnapshotFetcher(
            enabled=True,
            timeout_seconds=timeout,
            rate_limit_per_second=rate,
            user_agent=user_agent,
        )
        logger.info(
            "Wayback snapshot fetcher enabled",
            timeout_seconds=timeout,
            rate_limit_per_second=rate,
        )
        return fetcher
    except Exception as exc:
        logger.warning(
            "Wayback snapshot fetcher init failed; continuing without diff loop.",
            error=str(exc),
        )
        return None


def _load_wikidata_index() -> WikidataAliasIndex | None:
    """Load the Wikidata alias index for Phase 118 entity linking.

    Returns None if the path is unset or the index cannot be opened — the
    worker continues without entity linking in that case (graceful
    degradation: aer_gold.entities still receives raw spans, only the
    aer_gold.entity_links sidecar is empty). A configured-but-mismatched
    hash is fatal — this is the silent-drift guard.
    """
    path = os.getenv("WIKIDATA_INDEX_PATH", "").strip()
    if not path:
        logger.info(
            "Wikidata alias index disabled (WIKIDATA_INDEX_PATH unset). "
            "NER will run without entity linking."
        )
        return None
    expected = os.getenv("WIKIDATA_INDEX_SHA256", "").strip() or None
    try:
        return WikidataAliasIndex(path, expected_sha256=expected)
    except FileNotFoundError as e:
        logger.warning(
            "Wikidata alias index file missing; NER will run without linking",
            path=path,
            error=str(e),
        )
        return None
    # Hash mismatch (RuntimeError) is intentionally not caught — it is the
    # fail-fast drift guard required by the spec.


def init_extractors(extractor_classes, alias_index: WikidataAliasIndex | None = None):
    """
    Instantiate extractors one-by-one, skipping any that raise during init.

    Hard-Rule graceful-degradation gate: a single misconfigured extractor
    (missing model, missing lexicon, unexpected environment) must never take
    down the worker. Failed extractors are logged and omitted from the pipeline.

    The optional `alias_index` is forwarded to NamedEntityExtractor when
    present (Phase 118). Other extractors take no constructor arguments.
    """
    extractors = []
    for cls in extractor_classes:
        try:
            if cls is NamedEntityExtractor:
                extractors.append(cls(alias_index=alias_index))
            else:
                extractors.append(cls())
        except Exception as e:
            logger.warning(
                "Extractor init failed — skipping",
                extractor=getattr(cls, "__name__", repr(cls)),
                error=str(e),
                error_type=type(e).__name__,
            )
    return extractors


def _build_revision_delta_tools(extractors, alias_index):
    """Assemble the Phase-122d.3 discourse-shift re-extraction toolset.

    Reuses the already-loaded multilingual-sentiment + NER extractor
    instances (no second model load) and loads the E5 embedder ONCE. The
    E5 load is gated on the revision-diff loop being enabled
    (``REVISION_DIFF_EXTRACTION_ENABLED``) so a worker that does not run
    the sweep does not pay the ~2 GB E5 resident-memory cost. Returns
    ``None`` when the loop is disabled; callers then pass ``None`` into
    the loop (delta path off, diffs unaffected).
    """
    from internal.extractors.revision_deltas import E5PairEmbedder, RevisionDeltaTools

    if os.getenv("REVISION_DIFF_EXTRACTION_ENABLED", "false").lower() != "true":
        logger.info("revision_diff.delta_tools.disabled")
        return None

    sentiment = next(
        (e for e in extractors if isinstance(e, MultilingualBertSentimentExtractor)),
        None,
    )
    ner = next(
        (e for e in extractors if isinstance(e, NamedEntityExtractor)),
        None,
    )
    try:
        embedder = E5PairEmbedder()
    except Exception as exc:
        logger.warning(
            "revision_diff.e5_embedder.unavailable",
            error=str(exc),
            error_type=type(exc).__name__,
        )
        embedder = None

    logger.info(
        "revision_diff.delta_backbones",
        sentiment_model=getattr(sentiment, "_model_name", None),
        sentiment_revision=getattr(sentiment, "_model_revision", None),
        e5_model=getattr(embedder, "model", None),
        e5_revision=getattr(embedder, "revision", None),
        ner_models=sorted((getattr(ner, "_language_to_model", {}) or {}).values()),
    )
    return RevisionDeltaTools(sentiment=sentiment, ner=ner, embedder=embedder)


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
    #
    # Phase 123 hardening — both env-wired (were hardcoded, a scalability
    # ceiling). `ack_wait` raised 60→300s: one document runs the full heavy-NLP
    # stack (2× BERT sentiment + 2× spaCy NER + per-doc topic + entity-linking)
    # and under a large backlog can exceed 60s. A too-tight ack_wait redelivers
    # a still-processing document, compounding contention into a redelivery
    # death-spiral that wedges the pipeline. 300s gives real head-room while
    # still bounding poison-pill recovery.
    max_deliver: int = field(default_factory=lambda: int(os.getenv("NATS_MAX_DELIVER", "5")))
    ack_wait_seconds: float = field(
        default_factory=lambda: float(os.getenv("NATS_ACK_WAIT_SECONDS", "300"))
    )


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


async def consumer_lag_loop(subscription, stop_event: asyncio.Event, interval_seconds: float = 10.0):
    """Poll JetStream consumer info and publish the lag gauges (Phase 154).

    Fail-silent by construction: a transient NATS error must never crash the
    worker, so a failed poll logs at debug and retries on the next tick. The
    gauges are the operator dashboard's primary "is the pipeline keeping up?"
    signal alongside dlq_size.
    """
    while not stop_event.is_set():
        try:
            info = await subscription.consumer_info()
            nats_consumer_pending.set(info.num_pending or 0)
            nats_consumer_ack_pending.set(info.num_ack_pending or 0)
        except Exception as e:  # noqa: BLE001 — observability poll is best-effort
            logger.debug("Consumer-lag poll failed", error=str(e))
        try:
            await asyncio.wait_for(stop_event.wait(), timeout=interval_seconds)
        except asyncio.TimeoutError:
            continue


async def main(config: WorkerConfig | None = None):
    if config is None:
        config = WorkerConfig()

    # Phase 154 — structured logging with trace-id correlation. Configure
    # before the first log line so every record (incl. boot logs) carries the
    # active trace-id and uses the env-appropriate renderer.
    configure_logging()

    validate_required_env()

    tracer = init_telemetry(config.otel_endpoint, config.otel_sample_rate)

    metrics_port = int(os.getenv("METRICS_PORT", "8001"))
    start_http_server(metrics_port)
    logger.info("Prometheus metrics server started", port=metrics_port)

    minio_client = init_minio()
    # Phase 123: pools serve the document workers AND the background sweep loops
    # (BACKGROUND_LOOP_COUNT) so the batch sweeps can never starve the real-time
    # document path of connections. Headroom covers transient overlap.
    # (Phase 85: CH and PG sized symmetrically so PG never starves first.)
    pool_budget = config.worker_count + BACKGROUND_LOOP_COUNT + PG_POOL_HEADROOM
    ch_client = init_clickhouse(pool_size=pool_budget)
    pg_pool = init_postgres(maxconn=pool_budget)

    # Phase 122d.0 — Silent-Edit Observability (ADR-032). The Wayback
    # CDX client is fail-silent by construction; the WebAdapter calls
    # it as the last step of harmonisation, and `None` disables the
    # lookup entirely without changing the rest of the pipeline.
    # ADR-036: two clients — `wayback_inline` (fail-fast, used inline by the
    # WebAdapter) and `wayback_sweep` (patient, used by the re-attempt loop).
    wayback_inline, wayback_sweep = _init_wayback_clients(pg_pool)

    adapter_registry = AdapterRegistry(
        {
            "legacy": LegacyAdapter(),
            "rss": RssAdapter(pg_pool=pg_pool),
            "web": WebAdapter(pg_pool=pg_pool, wayback_client=wayback_inline),
        }
    )
    alias_index = _load_wikidata_index()
    extractors = init_extractors(DEFAULT_EXTRACTOR_CLASSES, alias_index=alias_index)
    # Phase 122d.3 — Silent-Edit Discourse Shift. Reuse the ALREADY-LOADED
    # sentiment + NER extractor instances (no second model load) for the
    # revision-diff re-extraction; the E5 embedder is the only new load and
    # is gated on the revision-diff loop being enabled. The boot log
    # re-records the active backbone revisions that produce the deltas
    # ("re-record the active backbone" — provenance lives in the pinned
    # manifest + this log, no new provenance table).
    revision_delta_tools = _build_revision_delta_tools(extractors, alias_index)
    # Phase 122e A17: per-source language scope. Documents whose detected
    # language falls outside the source's allow-list quarantine before the
    # Silver write. See `configs/probe_language_scope.yaml`.
    language_scope = ProbeLanguageScope.load()
    data_processor = DataProcessor(
        minio_client, ch_client, pg_pool, adapter_registry, extractors,
        language_scope=language_scope,
    )
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
    subscription = await js.subscribe(
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

    # Phase 115: periodic baseline-maintenance loop. Promotes the
    # standalone scripts/operations/compute_baselines.py into a NATS-cron-style
    # automated extractor; manual script retained for ad-hoc operations.
    baseline_config = BaselineConfig()
    baseline_task = asyncio.create_task(
        baseline_extraction_loop(
            ch_client,
            MetricBaselineExtractor(),
            baseline_config,
            stop_event,
        )
    )

    # Phase 120: BERTopic topic-modeling sweep. Opt-in (default disabled)
    # until the E5-large-bearing image is deployed; once enabled, runs on
    # a weekly cadence over a 30-day rolling window per WP-004 §3.4.
    topic_config = TopicConfig()
    topic_task = asyncio.create_task(
        topic_extraction_loop(
            ch_client,
            minio_client,
            TopicModelingExtractor(),
            topic_config,
            stop_event,
        )
    )

    # Phase 122d.1 / ADR-032 amendment — Silent-Edit Diff Substance.
    # Polls aer_gold.article_revisions for undiffed consecutive CDX
    # snapshot pairs, fetches archived HTML, computes paragraph-level
    # diffs + headline-change detection, re-writes the row with the
    # diff columns filled in. Opt-in (default disabled) — flip
    # REVISION_DIFF_EXTRACTION_ENABLED=true in .env to exercise.
    snapshot_fetcher = _init_snapshot_fetcher()
    revision_diff_config = RevisionDiffConfig()
    revision_diff_task = asyncio.create_task(
        revision_diff_extraction_loop(
            ch_client,
            snapshot_fetcher,
            minio_client,
            os.getenv("WORKER_SILVER_BUCKET", "silver"),
            revision_diff_config,
            stop_event,
            revision_delta_tools,
        )
    )

    # ADR-036: general enrichment re-attempt loop — runs at boot + every
    # interval, re-attempting per-article enrichments whose ingest-time lookup
    # was incomplete (the "no silent permanent gaps" guardrail). Wayback is the
    # first registered task; future external/degradable enrichments register
    # here. Gated on the Wayback client existing (the only task today).
    reattempt_tasks = (
        [WaybackReAttemptTask(ch_client, wayback_sweep)] if wayback_sweep is not None else []
    )
    reattempt_task = asyncio.create_task(
        enrichment_reattempt_loop(reattempt_tasks, stop_event, ReAttemptConfig())
    )

    # Phase 154 — periodic JetStream consumer-lag gauge poller.
    consumer_lag_task = asyncio.create_task(
        consumer_lag_loop(subscription, stop_event)
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

        logger.info("Waiting for baseline-extraction loop to drain...")
        await baseline_task

        logger.info("Waiting for topic-extraction loop to drain...")
        await topic_task

        logger.info("Waiting for revision-diff loop to drain...")
        await revision_diff_task

        logger.info("Waiting for enrichment re-attempt loop to drain...")
        await reattempt_task

        logger.info("Waiting for consumer-lag poller to drain...")
        await consumer_lag_task

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

import asyncio
import os
import signal
import structlog
from dotenv import load_dotenv
from nats.aio.client import Client as NATS

# Initialize structured logger
logger = structlog.get_logger()

async def main():
    # Load environment variables (fallback to local .env in project root)
    load_dotenv(dotenv_path="../../.env")
    
    nats_url = os.getenv("NATS_URL", "nats://localhost:4222")
    subject = "aer.lake.bronze"

    nc = NATS()

    async def disconnected_cb():
        logger.warning("Disconnected from NATS!")

    async def reconnected_cb():
        logger.info("Reconnected to NATS!")

    # Connect to NATS
    try:
        await nc.connect(
            nats_url,
            disconnected_cb=disconnected_cb,
            reconnected_cb=reconnected_cb
        )
        logger.info("Connected to NATS Broker", url=nats_url)
    except Exception as e:
        logger.error("Failed to connect to NATS", error=str(e))
        return

    # Message Handler (The Foundation)
    async def message_handler(msg):
        # In Phase 4, we only log the event to verify the pipeline.
        # In later phases, this will trigger the Bronze -> Silver harmonization.
        logger.info(
            "Event received from Data Lake",
            subject=msg.subject,
            payload_size_bytes=len(msg.data)
        )
        # Uncomment below to inspect the actual MinIO JSON payload during testing
        # logger.debug("Event Payload", data=msg.data.decode())

    # Subscribe to the MinIO bronze bucket events
    try:
        sub = await nc.subscribe(subject, cb=message_handler)
        logger.info("Subscribed to Event Subject", subject=subject)
    except Exception as e:
        logger.error("Failed to subscribe", error=str(e))
        return

    # Graceful shutdown handler
    loop = asyncio.get_running_loop()
    stop = loop.create_future()
    loop.add_signal_handler(signal.SIGINT, stop.set_result, None)
    loop.add_signal_handler(signal.SIGTERM, stop.set_result, None)

    logger.info("Analysis Worker is running and waiting for events...")
    await stop

    # Cleanup
    logger.info("Shutting down worker...")
    await sub.unsubscribe()
    await nc.close()

if __name__ == '__main__':
    asyncio.run(main())
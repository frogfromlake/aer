import structlog
from internal.adapters.base import SourceAdapter

logger = structlog.get_logger()


class AdapterRegistry:
    """
    Registry mapping source_type strings to SourceAdapter instances.

    Assembled in main.py via dependency injection — not hardcoded in the processor.
    """

    def __init__(self, adapters: dict[str, SourceAdapter]):
        self._adapters = adapters

    def get(self, source_type: str) -> SourceAdapter | None:
        """Look up an adapter by source_type. Returns None if not found."""
        return self._adapters.get(source_type)

    def supported_types(self) -> list[str]:
        """Return all registered source_type keys."""
        return list(self._adapters.keys())

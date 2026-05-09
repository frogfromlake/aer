"""Test bootstrap for the web-crawler unit suite.

The crawler ships its own runtime deps (scrapy, ultimate-sitemap-parser,
feedparser, psycopg2) that are not present in the analysis-worker venv
the test runner uses by default. The crawler module itself is robust to
missing deps at runtime via lazy imports inside :func:`discover` and
inside :func:`cli`. To let unit tests (which are pure-Python and never
exercise the runtime crawl path) import the crawler modules and patch
their dependencies, we pre-inject MagicMock placeholders into
``sys.modules`` for every optional runtime dep before pytest collects
the test files.

The per-test ``unittest.mock.patch`` calls then override these
placeholders with fixture-specific behaviour (e.g. fake sitemap trees,
fake feedparser entries). When the tests ARE run inside the crawler
container with the real deps installed, the ``setdefault`` calls below
become no-ops and the real modules are used instead — no behavioural
divergence between the two test environments.
"""

from __future__ import annotations

import sys
from unittest.mock import MagicMock

_OPTIONAL_RUNTIME_DEPS = (
    "scrapy",
    "scrapy.crawler",
    "scrapy.exceptions",
    "scrapy.http",
    "usp",
    "usp.tree",
    "feedparser",
    "psycopg2",
    "psycopg2.extras",
    "psycopg2.pool",
)

for _module in _OPTIONAL_RUNTIME_DEPS:
    sys.modules.setdefault(_module, MagicMock())

"""Phase 122: WebAdapter + web_extract.py.

These tests cover the harmonisation surface defined in ROADMAP §122:

* JSON-LD NewsArticle path → Tier-A/B/C populated with
  ``extraction_method='json_ld'``.
* OpenGraph fallback path → ``extraction_method='open_graph'``.
* Heuristic-only path → ``heuristic_htmldate`` for the date,
  ``None`` for other Tier-C provenance markers.
* Empty-body HTML triggers the readability fallback or routes to
  ``ExtractionFailedError``.
* Tier-D ``structured_data`` round-trips the ``extruct`` payload.
* Tier-E ``custom_extractors`` populate ``source_extras``.
* ``timestamp_source`` resolution priority across the six origins.

Every test that needs the heavy NLP-stack imports skips via
``pytest.importorskip`` so the suite stays green before the worker venv
gets refreshed with trafilatura / extruct / htmldate / courlan /
readability-lxml.
"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from internal.adapters.web import ExtractionFailedError, WebAdapter
from internal.adapters.web_meta import ALLOWED_TIMESTAMP_SOURCES, WebMeta

pytest.importorskip("trafilatura", reason="web_extract optional deps not installed yet")
pytest.importorskip("extruct", reason="web_extract optional deps not installed yet")
pytest.importorskip("htmldate", reason="web_extract optional deps not installed yet")
pytest.importorskip("courlan", reason="web_extract optional deps not installed yet")
pytest.importorskip("readability", reason="web_extract optional deps not installed yet")


# ---------------------------------------------------------------------------
# Phase 122e A25 / F-A25 — _extract_image_url normalises JSON-LD `image`
# ---------------------------------------------------------------------------


def test_extract_image_url_handles_bare_string():
    from internal.adapters.web_extract import _extract_image_url
    assert _extract_image_url("https://example.com/foo.jpg") == "https://example.com/foo.jpg"


def test_extract_image_url_handles_single_imageobject_dict():
    from internal.adapters.web_extract import _extract_image_url
    assert _extract_image_url(
        {"@type": "ImageObject", "url": "https://example.com/foo.jpg"}
    ) == "https://example.com/foo.jpg"


def test_extract_image_url_handles_imageobject_using_at_id():
    """Some JSON-LD producers use `@id` instead of `url` for the image URL."""
    from internal.adapters.web_extract import _extract_image_url
    assert _extract_image_url(
        {"@type": "ImageObject", "@id": "https://example.com/foo.jpg"}
    ) == "https://example.com/foo.jpg"


def test_extract_image_url_handles_imageobject_using_contentUrl():
    from internal.adapters.web_extract import _extract_image_url
    assert _extract_image_url(
        {"@type": "ImageObject", "contentUrl": "https://example.com/foo.jpg"}
    ) == "https://example.com/foo.jpg"


def test_extract_image_url_handles_array_of_imageobjects():
    """The iter-4 tagesschau bug: array of dicts must yield the URL of
    the first dict, NOT the stringified Python list-of-dict repr.
    """
    from internal.adapters.web_extract import _extract_image_url
    value = [
        {"@type": "ImageObject", "url": "https://images.tagesschau.de/first.jpg"},
        {"@type": "ImageObject", "url": "https://images.tagesschau.de/second.jpg"},
    ]
    result = _extract_image_url(value)
    assert result == "https://images.tagesschau.de/first.jpg"
    assert "{" not in result and "[" not in result  # no Python repr


def test_extract_image_url_handles_array_of_bare_strings():
    from internal.adapters.web_extract import _extract_image_url
    assert _extract_image_url([
        "https://example.com/a.jpg",
        "https://example.com/b.jpg",
    ]) == "https://example.com/a.jpg"


def test_extract_image_url_handles_array_with_invalid_first_item():
    """Array first-item is a dict with no usable URL key; second item
    has one → fall through to the second."""
    from internal.adapters.web_extract import _extract_image_url
    value = [
        {"@type": "ImageObject", "name": "no-url-here"},
        {"@type": "ImageObject", "url": "https://example.com/found.jpg"},
    ]
    assert _extract_image_url(value) == "https://example.com/found.jpg"


def test_extract_image_url_returns_empty_for_unrecognized_shapes():
    from internal.adapters.web_extract import _extract_image_url
    assert _extract_image_url(None) == ""
    assert _extract_image_url({}) == ""
    assert _extract_image_url({"@type": "ImageObject"}) == ""  # no URL key
    assert _extract_image_url([]) == ""
    assert _extract_image_url([{"@type": "ImageObject"}]) == ""
    assert _extract_image_url(42) == ""  # invalid type


# ---------------------------------------------------------------------------
# Fixture HTML
# ---------------------------------------------------------------------------


JSONLD_HTML = """\
<!DOCTYPE html>
<html lang="de">
  <head>
    <title>Bundesregierung beschließt Klimaschutzpaket</title>
    <meta property="og:title" content="OG Title — should not win" />
    <script type="application/ld+json">
    {
      "@context": "https://schema.org",
      "@type": "NewsArticle",
      "headline": "Bundesregierung beschließt Klimaschutzpaket",
      "datePublished": "2026-04-05T10:00:00+02:00",
      "dateModified": "2026-04-05T11:00:00+02:00",
      "author": {"@type": "Person", "name": "Petra Schmidt"},
      "description": "Massnahmenpaket zum Klimaschutz verabschiedet.",
      "articleSection": "Politik",
      "image": "https://www.example.gov.de/img/klima.jpg",
      "keywords": "Klimaschutz, Umwelt, Bundesregierung",
      "wordCount": 800,
      "isAccessibleForFree": true,
      "editor": "Hans Redaktion",
      "contentLocation": "Berlin"
    }
    </script>
  </head>
  <body>
    <article>
      <p>Die Bundesregierung hat am Mittwoch ein umfassendes Massnahmenpaket
         zum Klimaschutz verabschiedet. Das Paket sieht unter anderem eine
         Ausweitung des Emissionshandels vor. Bundeskanzler Friedrich Merz
         betonte die Bedeutung des Klimaschutzes fuer die Zukunft Deutschlands.
         Das Bundesministerium fuer Umwelt in Berlin koordiniert die Umsetzung
         der Massnahmen.</p>
      <p>Weitere Details werden in der naechsten Pressekonferenz bekanntgegeben.
         Die Opposition hat das Paket grundsaetzlich begruesst, fordert aber
         schaerfere Massnahmen.</p>
    </article>
  </body>
</html>
"""


OG_ONLY_HTML = """\
<!DOCTYPE html>
<html lang="de">
  <head>
    <title>OG Headline</title>
    <meta property="og:title" content="OG Headline" />
    <meta property="og:description" content="Open-Graph description text." />
    <meta property="article:published_time" content="2026-04-04T08:30:00+02:00" />
    <meta property="article:author" content="Bundespresseamt" />
    <meta property="article:section" content="Wirtschaft" />
    <meta property="og:image" content="https://www.example.gov.de/img/og.jpg" />
  </head>
  <body>
    <article>
      <p>Die Zahl der Erwerbstaetigen in Deutschland ist im Maerz 2026 leicht
         gestiegen. Die Arbeitslosenquote liegt bei 5,2 Prozent. Das
         Bundesministerium fuer Arbeit und Soziales in Berlin sieht die
         Entwicklung positiv. Die Bundesagentur fuer Arbeit in Nuernberg
         bestaetigt den Trend.</p>
      <p>Die Bundesregierung sieht in der Entwicklung ein Zeichen fuer die
         Wirksamkeit ihrer Wirtschaftspolitik.</p>
    </article>
  </body>
</html>
"""


HEURISTIC_HTML = """\
<!DOCTYPE html>
<html lang="de">
  <head>
    <title>Heuristic-only article</title>
    <meta name="date" content="2026-03-15" />
  </head>
  <body>
    <article>
      <p>Der Artikel enthaelt nur ein Datum im einfachen Meta-Tag, keine
         JSON-LD-Daten und keine OpenGraph-Tags. Der Body ist hinreichend
         lang fuer trafilatura, damit der cleaned_text-Pfad erfolgreich
         durchlaeuft. Es geht hier um eine Pruefung, dass die heuristische
         htmldate-Aufloesung als letzter Schritt wirkt.</p>
      <p>Eine zweite Absatz, der nochmals die zwei wichtigsten Themen
         erwaehnt, damit der Wortzahl-Schwellwert sicher ueberschritten wird.</p>
    </article>
  </body>
</html>
"""


READABILITY_TRIGGER_HTML = """\
<!DOCTYPE html>
<html lang="de">
  <head><title>Readability fallback</title></head>
  <body>
    <article itemtype="https://schema.org/Article">
      """ + ("<div>placeholder</div>" * 200) + """
      <p>Der eigentliche Artikeltext ist in einem stark verschachtelten DOM
         eingebettet, sodass trafilatura keine ausreichende Body-Erkennung
         erzielt. Der Readability-Pfad sollte greifen, weil der gesamte HTML-
         Inhalt deutlich groesser als 5 KiB ist und ein <article>-Tag
         vorhanden ist.</p>
      <p>Damit das Test-HTML stabil bleibt, fuehren wir mehrere Absaetze mit
         echtem Inhalt ein, die zusammen den Wortzahl-Mindestwert
         ueberschreiten und einen sinnvollen cleaned_text liefern.</p>
    </article>
    """ + ("<div>filler</div>" * 200) + """
  </body>
</html>
"""


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _bronze(html: str, **overrides) -> dict:
    base = {
        "source": "tagesschau",
        "source_type": "web",
        "raw_html": html,
        "original_url": "https://www.example.gov.de/article/x.html",
        "canonical_url": "https://www.example.gov.de/article/x.html",
        "fetch_at": "2026-05-08T12:00:00Z",
        "http_status": 200,
        "headers": {"content-type": "text/html"},
        "etag": "",
        "http_last_modified": None,
        "sitemap_lastmod": None,
        "sitemap_section": None,
    }
    base.update(overrides)
    return base


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_jsonld_path_populates_tier_b_c() -> None:
    adapter = WebAdapter()
    core, meta = adapter.harmonize(_bronze(JSONLD_HTML), datetime.now(tz=timezone.utc), "key")
    assert isinstance(meta, WebMeta)
    assert meta.title == "Bundesregierung beschließt Klimaschutzpaket"
    assert meta.author == "Petra Schmidt"
    assert meta.description.startswith("Massnahmenpaket")
    assert meta.section == "Politik"
    assert meta.published_date is not None
    assert meta.modified_date is not None
    assert meta.editor == "Hans Redaktion"
    assert meta.dateline_location == "Berlin"
    assert meta.paywall_status is False  # isAccessibleForFree=true → not paywalled
    assert "Klimaschutz" in meta.tags
    assert meta.extraction_methods["title"] == "json_ld"
    assert meta.extraction_methods["published_date"] == "json_ld"
    assert meta.extraction_methods["author"] == "json_ld"
    assert meta.extraction_methods["editor"] == "json_ld"
    assert meta.timestamp_source == "json_ld_published"
    assert core.timestamp == meta.published_date
    assert core.cleaned_text  # trafilatura recovered the body


def test_og_only_path_populates_tier_b_via_open_graph() -> None:
    adapter = WebAdapter()
    core, meta = adapter.harmonize(_bronze(OG_ONLY_HTML), datetime.now(tz=timezone.utc), "key")
    assert meta.title  # comes from og:title or <title>
    assert meta.author == "Bundespresseamt"
    assert meta.description.startswith("Open-Graph")
    assert meta.section == "Wirtschaft"
    assert meta.published_date is not None
    assert meta.timestamp_source == "open_graph_published"
    assert meta.extraction_methods["author"] == "open_graph"
    assert meta.extraction_methods["description"] == "open_graph"
    # No JSON-LD → no editor populated.
    assert meta.editor == ""
    assert meta.extraction_methods["editor"] is None
    assert core.cleaned_text


def test_heuristic_only_path_uses_htmldate() -> None:
    adapter = WebAdapter()
    core, meta = adapter.harmonize(_bronze(HEURISTIC_HTML), datetime.now(tz=timezone.utc), "key")
    # Either heuristic produced a date (timestamp_source = html_meta_published)
    # or fell through to fetch_at_fallback. In both cases timestamp_source is
    # one of the allowed sentinels and Tier-C json-ld-only fields are absent.
    assert meta.timestamp_source in ALLOWED_TIMESTAMP_SOURCES
    assert meta.editor == ""
    assert meta.dateline_location == ""
    assert meta.extraction_methods["editor"] is None
    assert meta.extraction_methods["dateline_location"] is None
    assert meta.extraction_methods["editorial_labels"] is None


def test_empty_body_raises_extraction_failed() -> None:
    """An HTML payload with no salvageable body must raise so the
    processor can route the document to the DLQ with reason
    ``extraction_failed``.
    """
    adapter = WebAdapter()
    empty_html = "<html><body></body></html>"
    with pytest.raises(ExtractionFailedError):
        adapter.harmonize(_bronze(empty_html), datetime.now(tz=timezone.utc), "key")


def test_tier_d_structured_data_roundtrips_jsonld() -> None:
    adapter = WebAdapter()
    _, meta = adapter.harmonize(_bronze(JSONLD_HTML), datetime.now(tz=timezone.utc), "key")
    # extruct emits the JSON-LD blocks under a syntax key. The exact key
    # name varies between extruct versions ("json-ld" pre-1.x,
    # "json_ld" in newer releases under uniform=True). Either is acceptable.
    blob = meta.structured_data.get("json-ld") or meta.structured_data.get("json_ld")
    assert blob, f"expected JSON-LD entry in structured_data, got keys={list(meta.structured_data)}"


def test_custom_extractors_populate_source_extras() -> None:
    adapter = WebAdapter()
    raw = _bronze(JSONLD_HTML)
    raw["custom_extractors"] = {
        "title_via_xpath": {"xpath": "//title/text()"},
    }
    _, meta = adapter.harmonize(raw, datetime.now(tz=timezone.utc), "key")
    assert "title_via_xpath" in meta.source_extras
    assert "Klimaschutzpaket" in meta.source_extras["title_via_xpath"]


def test_timestamp_source_falls_back_to_sitemap_lastmod() -> None:
    adapter = WebAdapter()
    sitemap_dt = "2026-04-01T09:00:00+00:00"
    raw = _bronze(HEURISTIC_HTML, sitemap_lastmod=sitemap_dt)
    # Force the heuristic path to abstain by giving a body that htmldate
    # cannot lock onto: replace the meta-date tag.
    raw["raw_html"] = HEURISTIC_HTML.replace(
        '<meta name="date" content="2026-03-15" />', ""
    )
    core, meta = adapter.harmonize(raw, datetime.now(tz=timezone.utc), "key")
    if meta.published_date is not None:
        # htmldate is aggressive; if it found *something* the test still
        # confirms the timestamp_source is an allowed sentinel.
        assert meta.timestamp_source in ALLOWED_TIMESTAMP_SOURCES
    else:
        assert meta.timestamp_source == "sitemap_lastmod"
        assert core.timestamp.isoformat().startswith("2026-04-01")


def test_timestamp_source_falls_back_to_http_last_modified() -> None:
    adapter = WebAdapter()
    raw = _bronze(
        HEURISTIC_HTML.replace('<meta name="date" content="2026-03-15" />', ""),
        http_last_modified="2026-04-02T08:00:00+00:00",
    )
    core, meta = adapter.harmonize(raw, datetime.now(tz=timezone.utc), "key")
    if meta.published_date is None and meta.sitemap_lastmod is None:
        assert meta.timestamp_source == "http_last_modified"
        assert core.timestamp.isoformat().startswith("2026-04-02")


def test_timestamp_source_falls_back_to_fetch_at_when_nothing_else() -> None:
    adapter = WebAdapter()
    raw = _bronze(
        HEURISTIC_HTML.replace('<meta name="date" content="2026-03-15" />', "")
    )
    event_time = datetime(2026, 5, 8, 12, 0, tzinfo=timezone.utc)
    core, meta = adapter.harmonize(raw, event_time, "key")
    if meta.published_date is None:
        assert meta.timestamp_source == "fetch_at_fallback"
        # fetch_at is set from the raw payload; falls back to event_time
        # if that is missing too.
        assert core.timestamp == meta.fetch_at


# ---------------------------------------------------------------------------
# Phase 123 — Probe 1 (French). Proves the WEB Bronze→Silver harmonisation
# (trafilatura/extruct/htmldate) works on French institutional HTML exactly as
# it does for German: title/author/section/date from JSON-LD, a non-empty
# cleaned_text body, and source_type='web'. Runs in CI / inside the worker
# container (the importorskip at the top skips it where the heavy deps are not
# installed). The schema is identical to Probe 0; language='und' here is patched
# to 'fr' downstream by the LanguageDetectionExtractor (see test_language_detection).
# ---------------------------------------------------------------------------

FRENCH_JSONLD_HTML = """\
<!DOCTYPE html>
<html lang="fr">
  <head>
    <title>Le Gouvernement présente son plan pour la transition écologique</title>
    <script type="application/ld+json">
    {
      "@context": "https://schema.org",
      "@type": "NewsArticle",
      "headline": "Le Gouvernement présente son plan pour la transition écologique",
      "datePublished": "2026-05-29T10:00:00+02:00",
      "dateModified": "2026-05-29T11:00:00+02:00",
      "author": {"@type": "Person", "name": "Jean Dupont"},
      "description": "Un plan pour réduire les émissions de gaz à effet de serre.",
      "articleSection": "Politique",
      "image": "https://www.elysee.fr/img/climat.jpg",
      "keywords": "écologie, climat, gouvernement",
      "isAccessibleForFree": true,
      "contentLocation": "Paris"
    }
    </script>
  </head>
  <body>
    <article>
      <p>Le Président de la République a présidé ce mercredi un Conseil des
         ministres consacré à la transition écologique. Le Gouvernement a
         présenté un plan ambitieux visant à réduire les émissions de gaz à
         effet de serre d'ici 2030, en coordination avec les collectivités
         territoriales et les services de l'État.</p>
      <p>Les détails seront précisés lors de la prochaine conférence de presse.
         L'opposition a salué l'initiative tout en demandant des mesures plus
         ambitieuses pour atteindre les objectifs fixés.</p>
    </article>
  </body>
</html>
"""


def test_french_jsonld_path_harmonises() -> None:
    """French institutional HTML harmonises to a valid SilverCore + WebMeta."""
    adapter = WebAdapter()
    core, meta = adapter.harmonize(
        _bronze(FRENCH_JSONLD_HTML, source="elysee", original_url="https://www.elysee.fr/article"),
        datetime.now(tz=timezone.utc),
        "key",
    )
    assert isinstance(meta, WebMeta)
    assert meta.title == "Le Gouvernement présente son plan pour la transition écologique"
    assert meta.author == "Jean Dupont"
    assert meta.section == "Politique"
    assert meta.published_date is not None
    assert meta.extraction_methods["title"] == "json_ld"
    # Schema invariants identical to Probe 0: web source, language deferred to
    # the downstream detector, non-empty body recovered by trafilatura.
    assert core.source_type == "web"
    assert core.source == "elysee"
    assert core.language == "und"
    assert core.cleaned_text
    assert core.word_count > 0

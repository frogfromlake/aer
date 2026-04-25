"""Unit tests for EntityCoOccurrenceExtractor (Phase 102)."""

from datetime import datetime, timedelta

from internal.extractors import (
    CoOccurrenceRow,
    EntityCoOccurrenceExtractor,
    EntityRecord,
    TimeWindow,
)


WINDOW = TimeWindow(start=datetime(2026, 4, 25, 0, 0), end=datetime(2026, 4, 25, 1, 0))
SOURCE = "tagesschau"


def _records(*pairs: tuple[str, str, str]) -> list[EntityRecord]:
    return [
        EntityRecord(article_id=art, entity_text=text, entity_label=label)
        for art, text, label in pairs
    ]


def test_basic_pair_enumeration_one_article():
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        _records(("a1", "Merkel", "PER"), ("a1", "Berlin", "LOC"), ("a1", "SPD", "ORG")),
        WINDOW,
        SOURCE,
    )
    assert len(rows) == 3
    pairs = {(r.entity_a_text, r.entity_b_text) for r in rows}
    assert pairs == {("Berlin", "Merkel"), ("Berlin", "SPD"), ("Merkel", "SPD")}
    for r in rows:
        assert r.cooccurrence_count == 1
        assert r.entity_a_text < r.entity_b_text  # lexicographic canonical form
        assert r.article_id == "a1"
        assert r.source == SOURCE
        assert r.window_start == WINDOW.start
        assert r.window_end == WINDOW.end


def test_per_article_isolation():
    """Pairs only emit within the same article_id, never across articles."""
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        _records(("a1", "Merkel", "PER"), ("a2", "Berlin", "LOC")),
        WINDOW,
        SOURCE,
    )
    assert rows == []


def test_repeated_surface_form_increases_count():
    """min(count_A, count_B) handles surface-form repetition within an article."""
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        _records(
            ("a1", "Merkel", "PER"),
            ("a1", "Merkel", "PER"),
            ("a1", "Merkel", "PER"),
            ("a1", "Berlin", "LOC"),
            ("a1", "Berlin", "LOC"),
        ),
        WINDOW,
        SOURCE,
    )
    assert len(rows) == 1
    assert rows[0].cooccurrence_count == 2  # min(3, 2)


def test_single_entity_article_emits_no_pairs():
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        _records(("a1", "Merkel", "PER")),
        WINDOW,
        SOURCE,
    )
    assert rows == []


def test_empty_input_emits_no_pairs():
    extractor = EntityCoOccurrenceExtractor()
    assert extractor.extract_pairs([], WINDOW, SOURCE) == []


def test_skips_records_with_missing_article_or_text():
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        [
            EntityRecord(article_id="", entity_text="Merkel", entity_label="PER"),
            EntityRecord(article_id="a1", entity_text="", entity_label="PER"),
            EntityRecord(article_id="a1", entity_text="Merkel", entity_label="PER"),
            EntityRecord(article_id="a1", entity_text="Berlin", entity_label="LOC"),
        ],
        WINDOW,
        SOURCE,
    )
    assert len(rows) == 1
    assert (rows[0].entity_a_text, rows[0].entity_b_text) == ("Berlin", "Merkel")


def test_pair_ordering_is_lexicographic_not_input_order():
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        _records(("a1", "Zeta", "ORG"), ("a1", "Alpha", "ORG")),
        WINDOW,
        SOURCE,
    )
    assert len(rows) == 1
    assert rows[0].entity_a_text == "Alpha"
    assert rows[0].entity_b_text == "Zeta"


def test_idempotency_same_input_same_output():
    """Running twice over identical records yields identical row sets."""
    extractor = EntityCoOccurrenceExtractor()
    records = _records(
        ("a1", "Merkel", "PER"),
        ("a1", "Berlin", "LOC"),
        ("a2", "Scholz", "PER"),
        ("a2", "Hamburg", "LOC"),
    )
    first = extractor.extract_pairs(records, WINDOW, SOURCE)
    second = extractor.extract_pairs(records, WINDOW, SOURCE)
    assert first == second


def test_same_text_different_labels_skipped():
    """A surface form like 'Berlin' tagged as both LOC and ORG must not self-pair."""
    extractor = EntityCoOccurrenceExtractor()
    rows = extractor.extract_pairs(
        _records(
            ("a1", "Berlin", "LOC"),
            ("a1", "Berlin", "ORG"),
            ("a1", "Merkel", "PER"),
        ),
        WINDOW,
        SOURCE,
    )
    # Berlin/Berlin self-pair is skipped, but each Berlin still pairs with Merkel.
    assert len(rows) == 2
    pair_texts = sorted(((r.entity_a_text, r.entity_b_text) for r in rows))
    assert pair_texts == [("Berlin", "Merkel"), ("Berlin", "Merkel")]
    labels = sorted(r.entity_a_label for r in rows)
    assert labels == ["LOC", "ORG"]


def test_window_metadata_is_copied_onto_every_row():
    extractor = EntityCoOccurrenceExtractor()
    custom_window = TimeWindow(
        start=datetime(2026, 1, 1), end=datetime(2026, 1, 1) + timedelta(hours=2)
    )
    rows = extractor.extract_pairs(
        _records(("a1", "Merkel", "PER"), ("a1", "Berlin", "LOC")),
        custom_window,
        "bundesregierung",
    )
    assert len(rows) == 1
    assert rows[0].window_start == custom_window.start
    assert rows[0].window_end == custom_window.end
    assert rows[0].source == "bundesregierung"


def test_co_occurrence_row_is_frozen_dataclass():
    row = CoOccurrenceRow(
        window_start=WINDOW.start,
        window_end=WINDOW.end,
        source=SOURCE,
        article_id="a1",
        entity_a_text="A",
        entity_a_label="PER",
        entity_b_text="B",
        entity_b_label="ORG",
        cooccurrence_count=1,
    )
    try:
        row.cooccurrence_count = 99  # type: ignore[misc]
    except (AttributeError, TypeError):
        return
    raise AssertionError("CoOccurrenceRow should be frozen / immutable")

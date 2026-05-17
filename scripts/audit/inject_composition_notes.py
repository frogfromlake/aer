#!/usr/bin/env python3
"""Phase 122j Slice J1 — inject pillar-aware composition paragraphs into
every dual-register YAML entry under
`services/bff-api/configs/content/{en,de}/{view_modes,metrics}/` that
does not already mention composition.

Strategy:
- Treat each YAML as text. Locate the trailing edge of the
  `registers.methodological.long: >` folded-scalar block (the last
  non-blank line that ends with ".") and insert a blank line + a new
  paragraph (6-space indented to match the existing block) before the
  `contentVersion:` line.
- Mirror the same insertion into `registers.semantic.long` so the
  user-visible semantic register also surfaces the composition reading.
- Idempotent: if the file already contains the keyword "Composition —"
  in either long block, skip.

Pillar mapping (view_modes only):
- time_series_*, distribution_*       -> Aleph
- topic_distribution_*, topic_evolution_* -> Episteme
- cooccurrence_network_*              -> Rhizome

Metric entries get a metric-specific aggregation note (mean for
sentiment, sum for counts, mode-distribution for categorical).
"""
from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Literal

ROOT = Path("services/bff-api/configs/content")

Locale = Literal["en", "de"]
Pillar = Literal["aleph", "episteme", "rhizome"]


def classify_view_mode(entity_id: str) -> Pillar:
    if entity_id.startswith("cooccurrence_network_"):
        return "rhizome"
    if entity_id.startswith(("topic_distribution_", "topic_evolution_")):
        return "episteme"
    if entity_id.startswith(("time_series_", "distribution_")):
        return "aleph"
    raise ValueError(f"unclassified view_mode: {entity_id}")


# --- Composition note templates (folded-scalar continuation paragraphs).
# Each template is a single paragraph; leading 6-space indent applied at
# write time. Markdown not allowed (YAML scalar, rendered as plain text).

VIEW_MODE_SEMANTIC_EN = {
    "aleph": (
        "Composition — when the Panel composes more than one source under "
        "composition='merged', this view renders ONE chart over the unioned "
        "corpus (BFF query ?sourceIds=a,b,...); under composition='split' "
        "the same source-set fans out into one chart per source, "
        "side-by-side under split-direction='horizontal' or stacked under "
        "'vertical'. The merged reading is appropriate when the analytical "
        "question is about the union (e.g. 'what does the institutional "
        "press collectively show?'); the split reading is appropriate when "
        "source-level distinction is the question. Cross-language merged "
        "scope is permitted in Aleph and surfaces a soft methodology "
        "banner (WP-004 §3.4) rather than a refusal."
    ),
    "episteme": (
        "Composition — composition='merged' fits the topic model on the "
        "joint corpus of all selected sources; the topics that emerge are "
        "a property of the union, not an aggregation of per-source topics. "
        "The dashboard renders a Joint-Corpus methodology banner over the "
        "view (WP-005 §6.2) so the reader does not silently slide between "
        "'what these sources collectively talk about' and 'what each "
        "source talks about'. composition='split' fits one model per "
        "source and renders the resulting topic sets side-by-side, which "
        "is the appropriate reading when source-level framing is the "
        "question. Cross-language merged scope is refused at the BFF "
        "(422 cross_language_merge_unsupported) because the embedding "
        "geometry is language-conditioned."
    ),
    "rhizome": (
        "Composition — composition='merged' computes one entity "
        "co-occurrence graph over the joint corpus; nodes inherit "
        "salience from the union, edges weight by co-mention frequency "
        "across all selected sources. composition='split' renders one "
        "graph per source. Cross-language merged scope is refused at the "
        "BFF (422 cross_language_merge_unsupported) because the upstream "
        "NER models (spaCy/HanLP) are language-conditioned and edge "
        "weights would be incommensurable across language partitions; "
        "split-composition with one language per Cell is the supported "
        "way to inspect cross-language entity networks."
    ),
}

VIEW_MODE_METHODOLOGICAL_EN = {
    "aleph": (
        "Composition (Phase 122i / ADR-034) — for composition='merged' "
        "the handler executes a single BFF call with the unioned "
        "sourceIds and the Cell renders one chart; for 'split' the Cell "
        "fans out one query per source and renders one chart per source. "
        "Split-direction is a presentation toggle (horizontal raster vs "
        "vertical stack) and does not affect the query shape. Cross-"
        "language merged scope is NOT refused in Aleph — the "
        "MethodologyBanner primitive renders a soft cross-frame-"
        "comparability note (WP-004 §3.4) when "
        "composition='merged' AND sources.length > 1, because Aleph "
        "metrics (lexicon-based polarity, structural counts) are "
        "language-agnostic enough that aggregation is meaningful with "
        "caveats. The asymmetry against Episteme/Rhizome (which DO "
        "refuse) is intentional and documented in WP-005 §6.2 §8b."
    ),
    "episteme": (
        "Composition (Phase 122i / ADR-034) — merged composition fits "
        "BERTopic (intfloat/multilingual-e5-large → UMAP → HDBSCAN, "
        "min_cluster_size=10, n_neighbors=15) on the union of the "
        "selected sources within the resolved scope. The model sees the "
        "union as a single corpus; per-source structure is not preserved "
        "in the topic labels (recoverable from per-document assignments). "
        "Split composition fits one model per source; topic IDs are not "
        "comparable across Cells. Cross-language merged scope is refused "
        "at the BFF (HTTP 422 cross_language_merge_unsupported, "
        "anchor=ADR-034#cross-language) because UMAP+HDBSCAN clustering "
        "is unstable across language partitions even with a multilingual "
        "embedding model. Small-corpus warning triggers below 500 "
        "documents per language partition (engineering threshold, "
        "src/lib/config/topic-thresholds.ts). See WP-005 §6.2."
    ),
    "rhizome": (
        "Composition (Phase 122i / ADR-034) — merged composition runs "
        "one entity co-occurrence aggregation (POST "
        "/entities/cooccurrence/query) over the unioned scope (with the "
        "100-source / 25-entity cap per ADR-034). Edge weights derive "
        "from per-language NER (spaCy de_core_news_lg, en_core_web_lg; "
        "HanLP for ZH per ADR-024) and ARE comparable within a language "
        "partition. Cross-language merged scope is refused at the BFF "
        "(HTTP 422 cross_language_merge_unsupported, "
        "anchor=ADR-034#cross-language) because NER tag sets and entity "
        "frequencies are not commensurable across language pipelines. "
        "Split composition renders one graph per source — appropriate "
        "for cross-source structural comparison within a single language."
    ),
}

VIEW_MODE_SEMANTIC_DE = {
    "aleph": (
        "Komposition — komponiert das Panel mehr als eine Quelle unter "
        "composition='merged', rendert diese Ansicht EINE Grafik über "
        "den vereinigten Korpus (BFF-Abfrage ?sourceIds=a,b,...); unter "
        "composition='split' fächert dieselbe Quellenmenge in eine "
        "Grafik pro Quelle auf, nebeneinander bei "
        "split-direction='horizontal' oder gestapelt bei 'vertical'. "
        "Die merged-Lesart ist angemessen, wenn die analytische Frage "
        "die Vereinigung betrifft (z. B. 'was zeigt die institutionelle "
        "Presse insgesamt?'); die split-Lesart ist angemessen, wenn "
        "Quellen-spezifische Unterscheidung die Frage ist. Mehrsprachige "
        "merged-Bereiche sind in Aleph erlaubt und blenden einen weichen "
        "Methodikhinweis ein (WP-004 §3.4) statt einer Verweigerung."
    ),
    "episteme": (
        "Komposition — composition='merged' passt das Themenmodell auf "
        "den gemeinsamen Korpus aller ausgewählten Quellen an; die "
        "entstehenden Themen sind eine Eigenschaft der Vereinigung, "
        "keine Aggregation von Pro-Quelle-Themen. Das Dashboard rendert "
        "über der Ansicht einen Joint-Corpus-Methodikhinweis (WP-005 "
        "§6.2), damit der Leser nicht still zwischen 'worüber diese "
        "Quellen gemeinsam sprechen' und 'worüber jede Quelle spricht' "
        "wechselt. composition='split' passt ein Modell pro Quelle an "
        "und rendert die jeweiligen Themenmengen nebeneinander — "
        "angemessen, wenn Quellen-spezifische Rahmung die Frage ist. "
        "Mehrsprachige merged-Bereiche werden in der BFF verweigert "
        "(422 cross_language_merge_unsupported), da die Einbettungs-"
        "geometrie sprachgebunden ist."
    ),
    "rhizome": (
        "Komposition — composition='merged' berechnet einen Entitäts-"
        "Ko-Vorkommen-Graphen über den gemeinsamen Korpus; Knoten "
        "erben Salienz aus der Vereinigung, Kantengewichte ergeben sich "
        "aus der gemeinsamen Erwähnungshäufigkeit über alle gewählten "
        "Quellen. composition='split' rendert einen Graphen pro Quelle. "
        "Mehrsprachige merged-Bereiche werden in der BFF verweigert "
        "(422 cross_language_merge_unsupported), weil die zugrunde-"
        "liegenden NER-Modelle (spaCy/HanLP) sprachgebunden sind und "
        "Kantengewichte über Sprach-Partitionen nicht vergleichbar wären; "
        "Split-Komposition mit einer Sprache pro Zelle ist der "
        "unterstützte Weg, mehrsprachige Entitätsnetzwerke zu inspizieren."
    ),
}

VIEW_MODE_METHODOLOGICAL_DE = {
    "aleph": (
        "Komposition (Phase 122i / ADR-034) — für composition='merged' "
        "führt der Handler einen einzelnen BFF-Aufruf mit den "
        "vereinigten sourceIds aus, und die Zelle rendert eine Grafik; "
        "für 'split' führt die Zelle eine Abfrage pro Quelle aus und "
        "rendert eine Grafik pro Quelle. Split-Direction ist ein reiner "
        "Layout-Schalter (horizontale Reihe vs. vertikaler Stapel) und "
        "beeinflusst die Abfrageform nicht. Mehrsprachige merged-"
        "Bereiche werden in Aleph NICHT verweigert — das "
        "MethodologyBanner-Primitive rendert einen weichen Cross-Frame-"
        "Vergleichbarkeit-Hinweis (WP-004 §3.4), sobald "
        "composition='merged' AND sources.length > 1, weil Aleph-"
        "Metriken (lexikon-basierte Polarität, strukturelle Zählungen) "
        "sprachunabhängig genug sind, dass eine Aggregation mit "
        "Vorbehalten sinnvoll bleibt. Die Asymmetrie gegenüber "
        "Episteme/Rhizome (die verweigern) ist beabsichtigt und in "
        "WP-005 §6.2 §8b dokumentiert."
    ),
    "episteme": (
        "Komposition (Phase 122i / ADR-034) — merged-Komposition passt "
        "BERTopic (intfloat/multilingual-e5-large → UMAP → HDBSCAN, "
        "min_cluster_size=10, n_neighbors=15) auf die Vereinigung der "
        "ausgewählten Quellen im aufgelösten Geltungsbereich an. Das "
        "Modell sieht die Vereinigung als einen einzigen Korpus; pro-"
        "Quelle-Struktur ist in den Themen-Labels nicht erhalten "
        "(wiederherstellbar aus den Pro-Dokument-Zuordnungen). "
        "Split-Komposition passt ein Modell pro Quelle an; Themen-IDs "
        "sind über Zellen hinweg nicht vergleichbar. Mehrsprachige "
        "merged-Bereiche werden in der BFF verweigert (HTTP 422 "
        "cross_language_merge_unsupported, "
        "anchor=ADR-034#cross-language), da UMAP+HDBSCAN-Clustering "
        "über Sprach-Partitionen auch mit mehrsprachigem Embedding-"
        "Modell instabil ist. Small-Corpus-Warnung greift unter 500 "
        "Dokumenten pro Sprachpartition (Engineering-Schwelle, "
        "src/lib/config/topic-thresholds.ts). Siehe WP-005 §6.2."
    ),
    "rhizome": (
        "Komposition (Phase 122i / ADR-034) — merged-Komposition führt "
        "eine Entitäts-Ko-Vorkommen-Aggregation (POST "
        "/entities/cooccurrence/query) über den vereinigten "
        "Geltungsbereich aus (mit den 100-Quellen-/25-Entitäten-Caps "
        "gemäß ADR-034). Kantengewichte leiten sich aus sprach-"
        "spezifischem NER ab (spaCy de_core_news_lg, en_core_web_lg; "
        "HanLP für ZH gemäß ADR-024) und SIND innerhalb einer Sprach-"
        "Partition vergleichbar. Mehrsprachige merged-Bereiche werden "
        "in der BFF verweigert (HTTP 422 cross_language_merge_unsupported, "
        "anchor=ADR-034#cross-language), weil NER-Tag-Mengen und "
        "Entitäts-Frequenzen über Sprach-Pipelines hinweg nicht "
        "kommensurabel sind. Split-Komposition rendert einen Graphen "
        "pro Quelle — geeignet für strukturellen Quell-Vergleich "
        "innerhalb einer Sprache."
    ),
}

# Metric-level composition templates. A metric entry is pillar-agnostic
# but its aggregation behaviour under sourceIds=a,b,c is metric-specific.
METRIC_AGG_EN: dict[str, str] = {
    "sentiment_score_sentiws":          "mean polarity (range −1..+1)",
    "sentiment_score_bert_de_news":     "mean polarity (range −1..+1)",
    "sentiment_score_bert_multilingual":"mean polarity (range −1..+1)",
    "entity_count":                     "mean per-article entity count",
    "word_count":                       "mean per-article word count",
    "language_confidence":              "mean detection confidence (0..1)",
    "publication_hour":                 "mode-aligned hour-of-day distribution",
    "publication_weekday":              "mode-aligned weekday distribution",
}
METRIC_AGG_DE: dict[str, str] = {
    "sentiment_score_sentiws":          "mittlere Polarität (Bereich −1..+1)",
    "sentiment_score_bert_de_news":     "mittlere Polarität (Bereich −1..+1)",
    "sentiment_score_bert_multilingual":"mittlere Polarität (Bereich −1..+1)",
    "entity_count":                     "mittlere Entitätszahl pro Artikel",
    "word_count":                       "mittlere Wortzahl pro Artikel",
    "language_confidence":              "mittlere Erkennungskonfidenz (0..1)",
    "publication_hour":                 "modus-ausgerichtete Stundenverteilung",
    "publication_weekday":              "modus-ausgerichtete Wochentagsverteilung",
}


def metric_semantic_para(entity_id: str, locale: Locale) -> str:
    agg = (METRIC_AGG_EN if locale == "en" else METRIC_AGG_DE).get(entity_id, "aggregation per metric definition")
    if locale == "en":
        return (
            "Composition — when a Workbench Panel pools more than one "
            f"source under composition='merged', this metric aggregates as "
            f"{agg} over the unioned corpus. Sentiment-class metrics inherit "
            "the merged-mean's sensitivity to source-mix imbalance: a "
            "single high-volume source can dominate the aggregate. Use the "
            "split composition (one Cell per source) when source-level "
            "differences are the question. The merged reading is "
            "permitted across languages with a soft methodology banner "
            "(Aleph) or refused (Episteme, Rhizome) — see the per-view-"
            "mode entry for the exact behaviour."
        )
    return (
        "Komposition — wenn ein Werkbank-Panel mehr als eine Quelle "
        f"unter composition='merged' bündelt, aggregiert diese Metrik als "
        f"{agg} über den vereinigten Korpus. Sentiment-Metriken erben die "
        "Empfindlichkeit des Merged-Mittelwerts gegenüber Ungleich-"
        "verteilung im Quell-Mix: eine einzelne volumenstarke Quelle kann "
        "den Aggregatwert dominieren. Split-Komposition (eine Zelle pro "
        "Quelle) ist angemessen, wenn Quellen-Unterschiede die Frage "
        "sind. Die merged-Lesart ist über Sprachen mit weichem Methodik-"
        "hinweis erlaubt (Aleph) oder verweigert (Episteme, Rhizome) — "
        "siehe den jeweiligen View-Mode-Eintrag für das genaue Verhalten."
    )


def metric_methodological_para(entity_id: str, locale: Locale) -> str:
    agg = (METRIC_AGG_EN if locale == "en" else METRIC_AGG_DE).get(entity_id, "aggregation per metric definition")
    if locale == "en":
        return (
            "Composition (Phase 122i / ADR-034) — multi-source merged "
            f"queries aggregate as {agg} over the unioned article set; the "
            "BFF accepts ?sourceIds=a,b,... and unions server-side. "
            "Equivalence gates (WP-004 §3) still apply when the merge "
            "spans heterogeneous source classes — the gate evaluates on "
            "the resolved scope before aggregation. Split composition is "
            "the per-source fanout; the metric definition is unchanged "
            "per Cell. Cross-language merged scope is governed at the "
            "view-mode handler: Aleph view-modes for this metric render "
            "the soft MethodologyBanner; Episteme/Rhizome view-modes "
            "(when applicable) refuse with HTTP 422 "
            "cross_language_merge_unsupported (ADR-034 §3)."
        )
    return (
        "Komposition (Phase 122i / ADR-034) — mehrquellige merged-"
        f"Abfragen aggregieren als {agg} über die vereinigte Artikelmenge; "
        "die BFF akzeptiert ?sourceIds=a,b,... und vereinigt serverseitig. "
        "Äquivalenz-Gates (WP-004 §3) gelten weiterhin, wenn die Merge "
        "heterogene Quellklassen umfasst — das Gate bewertet auf dem "
        "aufgelösten Geltungsbereich vor der Aggregation. Split-"
        "Komposition ist die Pro-Quelle-Auffächerung; die Metrik-"
        "Definition bleibt pro Zelle unverändert. Mehrsprachige merged-"
        "Bereiche werden auf der View-Mode-Handler-Ebene gesteuert: "
        "Aleph-View-Modes für diese Metrik rendern den weichen "
        "MethodologyBanner; Episteme-/Rhizome-View-Modes (wo "
        "anwendbar) verweigern mit HTTP 422 "
        "cross_language_merge_unsupported (ADR-034 §3)."
    )


def already_has_composition(text: str) -> bool:
    return ("Composition —" in text) or ("Komposition —" in text)


def inject_paragraph(yaml_text: str, semantic_para: str, methodological_para: str) -> str:
    """Insert a new paragraph (folded-scalar continuation, 6-space indent)
    at the END of `registers.semantic.long` and `registers.methodological.long`.
    Both `long: >` blocks are followed (eventually) by the next sibling field
    at 2-space indent (e.g. `methodological:`) or by the next top-level key
    (e.g. `contentVersion:`)."""

    def append_to_block(text: str, register_name: str, paragraph: str) -> str:
        # Pattern: find `  <register_name>:\n` (a 2-space-indented key),
        # then the next top-level/2-space key after it. Append paragraph
        # to the LAST `long: >` block within this register section.
        anchor = f"  {register_name}:\n"
        idx = text.find(anchor)
        if idx == -1:
            return text
        # Find end of this register's section: the next line that starts
        # with non-space, OR with a 2-space-indented key, after idx.
        end = idx + len(anchor)
        lines = text[end:].splitlines(keepends=True)
        consumed = 0
        for i, line in enumerate(lines):
            stripped = line.rstrip("\n")
            if not stripped:
                consumed += len(line)
                continue
            # 2-space-indented key (e.g. "  methodological:") or top-level
            if re.match(r"^[A-Za-z]", line) or re.match(r"^  [A-Za-z]", line):
                # That's the start of the next sibling — section ends here.
                break
            consumed += len(line)
        section_end_abs = end + consumed
        # Now insert paragraph before section_end_abs. Insert a blank
        # line (folded-scalar paragraph break) + the 6-space-indented
        # paragraph + a final newline.
        # Strip trailing blank lines from existing section so we land
        # cleanly right after the last content line.
        before = text[:section_end_abs].rstrip(" \n")
        after = text[section_end_abs:]
        # Wrap paragraph at ~78 chars with 6-space indent (folded scalar
        # respects line breaks inside a paragraph as soft spaces).
        wrapped = wrap_paragraph(paragraph, indent="      ", width=78)
        injection = "\n\n" + wrapped + "\n"
        return before + injection + ("\n" if after and not after.startswith("\n") else "") + after

    out = append_to_block(yaml_text, "semantic", semantic_para)
    out = append_to_block(out, "methodological", methodological_para)
    return out


def wrap_paragraph(text: str, *, indent: str, width: int) -> str:
    """Greedy word-wrap; keeps single paragraph (folded scalar will
    collapse the soft line breaks into spaces on read)."""
    words = text.split()
    lines: list[str] = []
    cur = indent
    for w in words:
        if len(cur) + 1 + len(w) > width and cur != indent:
            lines.append(cur.rstrip())
            cur = indent + w
        else:
            cur = (cur + " " + w) if cur != indent else cur + w
    if cur.strip():
        lines.append(cur.rstrip())
    return "\n".join(lines)


def process_view_mode(path: Path, locale: Locale) -> bool:
    text = path.read_text(encoding="utf-8")
    if already_has_composition(text):
        return False
    entity_id = path.stem
    try:
        pillar = classify_view_mode(entity_id)
    except ValueError as e:
        print(f"SKIP {path}: {e}", file=sys.stderr)
        return False
    if locale == "en":
        sem = VIEW_MODE_SEMANTIC_EN[pillar]
        method = VIEW_MODE_METHODOLOGICAL_EN[pillar]
    else:
        sem = VIEW_MODE_SEMANTIC_DE[pillar]
        method = VIEW_MODE_METHODOLOGICAL_DE[pillar]
    new_text = inject_paragraph(text, sem, method)
    if new_text != text:
        path.write_text(new_text, encoding="utf-8")
        return True
    return False


def process_metric(path: Path, locale: Locale) -> bool:
    text = path.read_text(encoding="utf-8")
    if already_has_composition(text):
        return False
    entity_id = path.stem
    sem = metric_semantic_para(entity_id, locale)
    method = metric_methodological_para(entity_id, locale)
    new_text = inject_paragraph(text, sem, method)
    if new_text != text:
        path.write_text(new_text, encoding="utf-8")
        return True
    return False


def main() -> int:
    if not ROOT.exists():
        print(f"ERR: {ROOT} not found", file=sys.stderr)
        return 2
    touched = 0
    for locale_dir in sorted(ROOT.iterdir()):
        if not locale_dir.is_dir() or locale_dir.name not in ("en", "de"):
            continue
        locale: Locale = locale_dir.name  # type: ignore[assignment]
        for f in sorted((locale_dir / "view_modes").glob("*.yaml")):
            if process_view_mode(f, locale):
                touched += 1
                print(f"updated {f}")
        for f in sorted((locale_dir / "metrics").glob("*.yaml")):
            if process_metric(f, locale):
                touched += 1
                print(f"updated {f}")
    print(f"--- touched {touched} files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

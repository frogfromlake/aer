"""Wikidata alias-index lookup for Phase 118 entity linking.

The index is a read-only SQLite database produced by
`scripts/build_wikidata_index.py` and shipped to the worker via the
`wikidata-index-init` compose service. At runtime, `WikidataAliasIndex`
opens it `mode=ro` and exposes a single `lookup(alias, language)` entry
point used by `NamedEntityExtractor`.

Disambiguation: when multiple QIDs share an alias for the same language,
the candidate with the highest sitelink count wins. This is the
sitelink-tiebreaker described in WP-002 §4.2 and ROADMAP Phase 118.

Confidence scoring (Tier-1.5 heuristic; not validated):
    1.00 — `exact_match`  : surface form matched rdfs:label
    0.85 — `alias_lookup` : surface form matched skos:altLabel
    0.70 — `accent_fold`  : match required accent-folding
Below 0.7 → no row is written; the canonical record of the span remains
`aer_gold.entities`.
"""

from __future__ import annotations

import hashlib
import sqlite3
import string
import unicodedata
from dataclasses import dataclass
from pathlib import Path
from typing import Optional

import structlog

logger = structlog.get_logger()


_ACCENT_FOLD_LANGUAGES = {"fr"}
# Per-method confidence weights — pinned here so the heuristic is a single
# source of truth across the extractor and tests.
CONFIDENCE_EXACT = 1.0
CONFIDENCE_ALIAS = 0.85
CONFIDENCE_ACCENT_FOLD = 0.7
CONFIDENCE_THRESHOLD = 0.7  # Below this, no row is written.


@dataclass(frozen=True, slots=True)
class LinkCandidate:
    wikidata_qid: str
    confidence: float
    method: str  # exact_match | alias_lookup | accent_fold


def _strip_punctuation(text: str) -> str:
    return text.translate(str.maketrans("", "", string.punctuation))


def _accent_fold(text: str) -> str:
    return "".join(
        c for c in unicodedata.normalize("NFKD", text) if not unicodedata.combining(c)
    )


def _normalise(text: str) -> str:
    """Mirror the build-time normalisation: lowercase + strip + drop punctuation."""
    return _strip_punctuation(text.strip().lower())


class WikidataAliasIndex:
    """Read-only handle to the Wikidata alias SQLite index.

    The handle is thread-safe in the sense that each `lookup` opens its
    own cursor on a single shared connection (SQLite's GIL-protected
    serialised mode). This matches the analysis-worker's per-document
    asyncio.to_thread offload pattern — at most `WORKER_COUNT` concurrent
    lookups run against the same connection.
    """

    def __init__(self, db_path: str | Path, expected_sha256: Optional[str] = None) -> None:
        path = Path(db_path)
        if not path.exists():
            raise FileNotFoundError(
                f"Wikidata alias index not found at {path}. "
                "Verify the wikidata-index-init service ran successfully."
            )
        if expected_sha256:
            actual = hashlib.sha256(path.read_bytes()).hexdigest()
            if actual != expected_sha256:
                raise RuntimeError(
                    "Wikidata alias index hash mismatch — "
                    f"expected={expected_sha256} actual={actual}. "
                    "The index on the volume does not match the worker's "
                    "expected build; refusing to start to prevent silent "
                    "index drift."
                )
            logger.info(
                "Wikidata alias index hash verified",
                path=str(path),
                sha256=actual,
            )
        else:
            logger.warning(
                "Wikidata alias index loaded without hash verification",
                path=str(path),
            )

        # `mode=ro` forbids writes; `immutable=1` further tells SQLite the
        # file will not change beneath us, enabling read-only optimisations.
        uri = f"file:{path}?mode=ro&immutable=1"
        self._conn = sqlite3.connect(
            uri,
            uri=True,
            check_same_thread=False,
        )
        self._path = path

    def lookup(self, surface: str, language: str) -> Optional[LinkCandidate]:
        """Resolve a surface form to a single best Wikidata candidate.

        Returns None when no candidate clears `CONFIDENCE_THRESHOLD`. The
        sitelink tiebreaker is applied across both `label` and `altLabel`
        sources — the highest-sitelink QID wins regardless of which side
        of the alias-source ladder it came from. On equal sitelinks
        `label` wins over `altLabel` (lexicographic DESC on `alias_source`
        because `'l'abel > 'a'ltLabel`); on equal sitelinks AND equal
        source, the lexicographically-earliest QID wins for determinism.

        Phase 118b post-mortem fix (2026-05-04). The original tiered
        lookup matched `label` first regardless of sitelink count, then
        fell back to `altLabel`. That misranked the German news-domain
        case `"Bundestag"` — Q547751 (Federal Convention 1815-1848,
        13 sitelinks, primary `label`) preempted Q154797 (modern
        German Bundestag, 90 sitelinks, `altLabel`). Conflating the
        two sources at the SQL layer and ranking purely by sitelinks
        moves the news-domain bias toward the modern entity without
        a Tier-2 transformer linker — see WP-002 §4.2 footnote¹ for
        the open evaluation work that distinguishes Tier-1.5 from
        validated entity linking. Confidence weights remain unchanged
        (1.00 / 0.85) so the methodology contract is preserved.
        """
        if not surface or not language:
            return None

        normalised = _normalise(surface)
        if not normalised:
            return None

        # Combined label + altLabel match. `alias_source DESC` makes
        # 'label' > 'altLabel' on lexicographic sort, which is the
        # correct tiebreaker on equal sitelinks.
        row = self._conn.execute(
            "SELECT wikidata_qid, sitelink_count, alias_source FROM aliases "
            "WHERE alias = ? AND language = ? "
            "AND alias_source IN ('label', 'altLabel') "
            "ORDER BY sitelink_count DESC, alias_source DESC, wikidata_qid ASC "
            "LIMIT 1",
            (normalised, language),
        ).fetchone()
        if row is not None:
            qid, _sitelinks, alias_source = row
            if alias_source == "label":
                return LinkCandidate(
                    wikidata_qid=qid,
                    confidence=CONFIDENCE_EXACT,
                    method="exact_match",
                )
            return LinkCandidate(
                wikidata_qid=qid,
                confidence=CONFIDENCE_ALIAS,
                method="alias_lookup",
            )

        # Tier 3 (language-gated): accent-fold both sides and try again.
        # Triggered when the surface form has diacritics not present in the
        # stored aliases, and equally when the aliases have diacritics not
        # present in the surface form. The build-time alias table is not
        # pre-folded — folding happens client-side over the per-language
        # alias subset.
        if language in _ACCENT_FOLD_LANGUAGES:
            folded = _accent_fold(normalised)
            rows = self._conn.execute(
                "SELECT wikidata_qid, sitelink_count, alias "
                "FROM aliases WHERE language = ?",
                (language,),
            ).fetchall()
            best: tuple[str, int] | None = None
            for qid, sitelinks, alias in rows:
                if _accent_fold(alias) == folded and alias != normalised:
                    if best is None or sitelinks > best[1]:
                        best = (qid, sitelinks)
            if best is not None:
                return LinkCandidate(
                    wikidata_qid=best[0],
                    confidence=CONFIDENCE_ACCENT_FOLD,
                    method="accent_fold",
                )
        return None

    def close(self) -> None:
        try:
            self._conn.close()
        except Exception:
            pass

    def __repr__(self) -> str:  # pragma: no cover
        return f"WikidataAliasIndex(path={self._path})"

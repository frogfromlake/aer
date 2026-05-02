# Adding a Source Type

This guide answers: *"I want AĒR to ingest from a platform that isn't RSS — a forum, a social network, an API, an archive — what does that involve?"*

The mechanism is the **Source Adapter** pattern (ADR-015). It is the architectural contract that allows AĒR to harmonize structurally different data sources into the universal Silver layer without leaking platform specifics into the analysis pipeline.

---

## When you actually need a new source type

Before adding a source type, confirm it is genuinely necessary. The existing `rss` source type handles surprisingly much:

- Atom feeds (gofeed parses both transparently)
- Pseudo-RSS feeds with non-standard extensions (the `RssMeta` envelope is extensible)
- Press-release feeds, government bulletins, broadcaster news feeds — all RSS

You need a new source type when **the wire format is structurally different from RSS** (no item-list shape, no per-document URL, requires authentication, or carries platform-specific metadata that does not fit `RssMeta`). Examples:

- A forum archive with thread / reply structure (parent-child relationships, not a flat item list).
- A social media platform with engagement signals (likes, shares, replies).
- A REST or GraphQL API requiring authentication and pagination.
- A research archive with structured metadata (authors, abstracts, citations).

If the new source produces flat item-lists with title/description/url/date, **use the existing RSS adapter** — there is no benefit to creating a new source type for the same shape.

---

## What gets touched

| Component | Change | Why | Effort |
| :--- | :--- | :--- | :--- |
| `crawlers/<source-type>-crawler/` | New standalone Go binary | Per ADR-015, each source type has its own crawler with its own `go.mod`. The Phase 40 RSS crawler is the reference. | 4–8 h |
| `internal/adapters/<source_type>_adapter.py` | New adapter implementing the `SourceAdapter` protocol | Maps platform-specific Bronze data to `SilverCore` + optional `SilverMeta` subclass | 2–4 h |
| `internal/adapters/registry.py` | Registration entry | Adapter lookup by `source_type` string | 5 min |
| `internal/models/silver.py` (optional) | New `SilverMeta` subclass | Only if platform metadata genuinely doesn't fit `RssMeta` | 1–2 h |
| `internal/extractors/` | Possibly new extractors | Only if the new source type enables metrics that don't apply to RSS (e.g. engagement-based metrics for social media) | varies |
| Tests | Adapter unit + integration | Pytest fixtures + Testcontainers MinIO | 2–4 h |
| Documentation | ADR or §13 entry, source-type description in CLAUDE.md | If the new source type carries methodological implications | 1–2 h |

**Realistic minimum: 1 day. Realistic average: 2–3 days.**

---

## The SourceAdapter protocol

From ADR-015, every adapter implements the same Python protocol:

```python
class SourceAdapter(Protocol):
    """Maps source-specific Bronze data to the universal Silver contract."""
    
    source_type: str  # The string key registered in AdapterRegistry
    
    def harmonize(
        self,
        raw_bronze: dict,
        document_id: str,
        timestamp: datetime,
    ) -> tuple[SilverCore, SilverMeta | None]:
        """
        Transform a raw Bronze JSON object into the universal SilverCore
        record plus an optional SilverMeta envelope carrying source-type-
        specific metadata.
        
        SilverCore is the stable analytical contract — never mutate its shape.
        SilverMeta is explicitly unstable per ADR-015 and may evolve per
        adapter without a formal ADR.
        """
        ...
```

The adapter's job is *exactly* harmonization — extracting the universal fields (cleaned text, timestamp, source identifier, language hint, word count) from whatever shape the platform delivers, plus optionally packing platform-specific metadata into a `SilverMeta` subclass.

The adapter's job is *not* analysis. No NLP, no metric computation, no entity extraction. Those happen downstream in extractors, which read `SilverCore` (and optionally inspect `SilverMeta`) and produce Gold-layer rows.

---

## When to create a new SilverMeta subclass

`SilverMeta` is the source-type-specific metadata envelope. The base case is no subclass needed — many source types carry metadata that fits `RssMeta` (URL, feed name, raw description, language hint).

Create a subclass when the new source type carries **structurally different** metadata that downstream extractors will need access to:

- **Forum**: thread ID, parent post ID, depth in thread, reply count → `ForumMeta(SilverMeta)`.
- **Social media**: platform user ID (anonymized per WP-006 §7), engagement counts, hashtags, mention list → `SocialMeta(SilverMeta)`.
- **Research archive**: authors, DOI, abstract, citation count → `ArchiveMeta(SilverMeta)`.

Subclass methodology:

1. Define the Pydantic model in `services/analysis-worker/internal/models/silver_meta_<type>.py`.
2. Subclass `SilverMeta` and add the new fields. Stable fields are typed; experimental fields go in a `dict[str, Any]` extras field.
3. Document in CLAUDE.md "SilverMeta variants" table.
4. ADR-015 explicitly notes that `SilverMeta` evolves without a formal ADR — but the addition of a new subclass *is* worth a brief Arc42 §5.1 note.

---

## Privacy and ethical implications by source type

This is where source-type addition becomes more than engineering. Arc42 §13.10 and WP-006 §5.2 + §7 classify privacy risk by source type:

| Source type | Privacy risk | Anonymization required |
| :--- | :--- | :--- |
| Institutional RSS / API | Low | Identifier stripping only |
| Public news / media RSS | Low | Standard. Public-figure entities not anonymized. |
| Public social media | High | Full anonymization: identifier removal, temporal truncation, k-anonymity (k≥10), entity suppression for private persons, stylometric risk assessment |
| Forum / community archives | Medium–High | Hash pseudonymization, temporal truncation, k-anonymity |

Adding a source type in the **High** or **Medium–High** privacy band is **not solo-developer work**:

- Requires a real WP-006 §5.2 ethical review (you cannot credibly self-review).
- Requires implementing the anonymization pipeline (k-anonymity gate at the L5 Evidence layer is already there from Phase 101, but the *upstream* anonymization in the adapter is source-type-specific).
- Requires a Probe Dossier `observer_effect.md` that genuinely engages with the privacy concerns.

The `silver_eligible=false` default on the source classification is the technical guardrail. A high-privacy-risk source ships ineligible for Silver-layer access until the ethical review concludes. Do not flip this flag unilaterally.

---

## Worked example: Forum adapter (sketch)

This is *not* a complete implementation — it is a sketch showing the structural shape of a non-RSS adapter.

### `crawlers/forum-crawler/` (Go)

A new standalone Go binary, structurally analogous to the RSS crawler, but:

- Polls the forum's API or scrapes forum pages (depending on accessibility, ToS, robots.txt).
- Resolves thread structure (parent-child relationships).
- Submits documents to `POST /api/v1/ingest` with `source_type: "forum"` and full thread metadata in the `data` payload.

### `internal/adapters/forum_adapter.py`

```python
class ForumAdapter:
    source_type = "forum"
    
    def harmonize(self, raw_bronze, document_id, timestamp):
        core = SilverCore(
            document_id=document_id,
            source=raw_bronze["forum_name"],
            source_type="forum",
            raw_text=raw_bronze["post_body"],
            cleaned_text=clean_html(raw_bronze["post_body"]),
            timestamp=timestamp,
            word_count=count_words(raw_bronze["post_body"]),
        )
        
        meta = ForumMeta(
            thread_id=raw_bronze["thread_id"],
            parent_post_id=raw_bronze.get("parent_post_id"),
            depth_in_thread=raw_bronze["depth"],
            reply_count=raw_bronze["reply_count"],
            # Note: NO username, NO user_id — these are stripped
            # in the adapter per WP-006 §7 anonymization
        )
        
        return core, meta
```

### `internal/models/silver_meta_forum.py`

```python
class ForumMeta(SilverMeta):
    thread_id: str
    parent_post_id: str | None
    depth_in_thread: int
    reply_count: int
```

### `adapters/registry.py`

```python
ADAPTERS = {
    "rss": RSSAdapter(),
    "forum": ForumAdapter(),
}
```

### What downstream automatically gets

- Language detection runs on `cleaned_text` (Phase 116) — no adapter awareness needed.
- NER runs (Phase 116 + 42) — language-routed.
- Sentiment runs — language-routed.
- Topic modeling runs (Phase 120) — multilingual.
- Entity linking runs (Phase 118) — language-routed.

The downstream pipeline is **source-type-agnostic** by design. Extractors that want to use `SilverMeta` data (e.g. a future "thread depth distribution" extractor for forums) inspect the meta envelope and produce Gold rows accordingly. Extractors that don't care simply ignore meta.

---

## What you do *not* do

- **Do not put platform logic in extractors.** Extractors are source-type-agnostic. If you find yourself writing `if core.source_type == "forum"` in an extractor, the logic belongs in the adapter or in a SilverMeta-typed extractor that only fires on forum data.
- **Do not extend `SilverCore`** for source-type-specific fields. `SilverCore` is the stable analytical contract; new fields there are an ADR-worthy change. Source-specific fields go in `SilverMeta` subclasses.
- **Do not skip the ethical review for high-risk source types.** The `silver_eligible=false` default exists to prevent this.
- **Do not implement anonymization downstream of the adapter.** Anonymization is the adapter's responsibility — by the time data reaches Silver, it must already be cleaned.

---

## Cross-references

- [ADR-015: Source Adapter Pattern](../arc42/09_architecture_decisions.md) — the canonical architectural decision.
- [Arc42 §13.10: Privacy Risk Classification by Probe Type](../arc42/13_scientific_foundations.md) — the privacy band table.
- [WP-006 §5.2 + §7](../methodology/en/WP-006-en-observer_effect_reflexivity_and_the_ethics_of_discourse_measurement.md) — the ethical review framework for high-risk source types.
- [Adding a Probe](add-a-probe.md) — adding a probe with a new source type combines both procedures.
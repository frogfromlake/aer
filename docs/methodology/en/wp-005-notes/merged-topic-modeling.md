# WP-005 §6.2 — Merged-Corpus Topic Modeling (Working Note)

**Status:** Working note, supplementing WP-005 (*Temporal Granularity of Discourse Shifts*).
**Anchored by:** ADR-034 (Phase 122i — Multi-Panel Workbench), §3 Episteme constraints.
**Cited from:** the dashboard's Joint-Corpus methodology banner that appears on Episteme cells
whenever a single Cell aggregates BERTopic over two or more sources.

---

## 1 — Why this note exists

The Multi-Panel Workbench (ADR-034) lets the user *merge* an arbitrary set of sources into a
single Cell. For most analytical views — distributions, time series, co-occurrence — this is
simply set union over the Gold layer and produces the same metric every other view does, just
on a wider scope.

BERTopic is different. It is not a metric over articles; it is a **model fit on a corpus**. The
corpus the user composes is the corpus the model sees, and the topics that emerge are an
artifact of *that corpus*, not an aggregation of topics that exist independently per source.
This note clarifies the methodological consequences and the boundary the dashboard surfaces.

## 2 — What "joint corpus" means

When the dashboard runs BERTopic on N sources merged into one Cell:

1. The **language-specific embedding model** (currently `intfloat/multilingual-e5-large`,
   constrained to one language per request — see §4 below) computes vectors for all documents
   in the union.
2. UMAP reduces the embedding cloud to its dense regions; HDBSCAN clusters those regions into
   topics. The clustering sees the union as a single corpus — no a-priori per-source structure
   is preserved.
3. Each topic is **what these N sources collectively talk about**, weighted by document
   contribution.

This is a valid and useful analytical question. *"What is the shared semantic space across the
German institutional press?"* is a coherent research framing. But it is **not** equivalent to
*"What does each source talk about?"* — and the dashboard's Joint-Corpus banner exists so the
reader does not silently slide between these readings.

## 3 — What gets aggregated away

A source-specific framing — say, one outlet that consistently frames climate policy through
party politics and another that frames it through climate science — produces vectors that are
adjacent in the embedding space (same topical domain). The joint-corpus clustering tends to
collapse them into one "climate" topic whose document set spans both sources but whose label is
dominated by whichever vocabulary is more frequent in the union. The distinguishing register is
present in the data and recoverable from the per-document assignments — but it is *not* visible
in the topic labels the dashboard renders. A reader who does not pay attention to per-source
distribution within a topic can easily mistake a merged topic for "what source A talks about",
which it specifically is not.

This is the limitation the Joint-Corpus banner makes explicit.

## 4 — Cross-language is structurally different

BERTopic's embedding model is language-conditioned. Embeddings from a German article and an
English article are not in a comparable space, even with a multilingual base model: the
similarity geometry differs between language partitions, and UMAP/HDBSCAN tuned for one
language produces unstable clusters when the other appears at scale.

ADR-034 §3 therefore **refuses** cross-language merges at the BFF (HTTP 422
`cross_language_merge_unsupported`) rather than rendering a broken topic set. This is not a
limitation of the Workbench; it is a property of the methodology. The dashboard surfaces it as
a refusal, not a warning. The user remedy is:

- Narrow the scope to a single language, or
- Use **split** composition — each Cell renders one language at a time. The Cells appear
  side-by-side; the user reads them as parallel sub-corpora, not as a merged stream.

## 5 — When is merged topic modeling appropriate?

A merged Cell is appropriate when the analytical question is *about the union*:

- *"What does the German institutional press talk about this quarter?"* — yes, merged.
- *"What is the shared topical space of sources X, Y, Z?"* — yes, merged.
- *"What lexicon do these N sources share?"* — yes, merged.

A merged Cell is **not** appropriate when the question is about source-level distinction:

- *"Which source frames climate as a party-political issue?"* — no, use split.
- *"How does source A's coverage of immigration differ from source B's?"* — no, use split.
- *"Does source A talk about a topic that source B does not?"* — no, use split (or a per-source
  panel + visual comparison).

## 6 — Reading a merged Cell

When the banner appears, the appropriate interpretive moves are:

1. Read the topic label as a property of the union, not a property of any single source.
2. Cross-check via the per-source distribution (e.g. a split-composition panel on the same
   scope) when source-specific framing is the question.
3. Treat "uncategorised" assignments as the cluster boundary, not as missing data — they are
   documents whose embedding did not converge on a dense region under the merge.

## 7 — Stability thresholds

BERTopic's `min_cluster_size=10` plus UMAP's `n_neighbors=15` rarely converges on a coherent
topic set below approximately 500 documents per language partition. Below this threshold,
clusters either collapse into outliers or fragment into noise. The dashboard surfaces a
*small-corpus warning* on Cells whose article count falls below this boundary (see
`services/dashboard/src/lib/config/topic-thresholds.ts`).

The 500-document threshold is engineering guidance from empirical observation on the Probe 0
corpus across several windows; it is not a hard statistical floor. Users should treat it as
"interpret topics cautiously" rather than "this output is wrong".

## 8 — Provenance

- **Model:** `intfloat/multilingual-e5-large` (embedding), UMAP (`n_components=5`,
  `n_neighbors=15`, seeded), HDBSCAN (`min_cluster_size=10`, deterministic per ADR-024).
- **Partition:** strictly per-language (one language per request; cross-language refused).
- **Tier:** 2 (Phase 120) — reproducible with pinned model, not bit-for-bit deterministic across
  platforms.

## 8b — Aleph soft note, not refusal (Phase 122i revision)

Phase 122i revision (Q4 / C6) adds a related-but-distinct concern: Aleph cells (sentiment,
distribution, word-count) **do not** refuse cross-language scopes — they are language-agnostic
enough that a merged-multi-source query technically returns a number. They DO render a soft
methodology banner over the chart, citing WP-004 §3.4 (cross-frame comparability), so the
reader is reminded that the aggregate may obscure source-specific framings. The mechanism is
the same `MethodologyBanner` primitive that surfaces this note; the trigger is `composition =
'merged' AND sources.length > 1`. Episteme + Rhizome retain their HARD refusal for merged
cross-language scopes because the underlying models (BERTopic, language-conditioned embeddings,
entity-cooccurrence networks) are language-specific in a way Aleph metrics are not.

In short:

* **Aleph merged-multi-source** → soft banner, query proceeds.
* **Episteme merged cross-language** → BFF 422, RefusalSurface, query blocked.
* **Rhizome merged cross-language CoOccurrence (POST)** → BFF 422, RefusalSurface, query blocked.
* **Aleph + Episteme + Rhizome split-composition** → no banner needed (each Cell stays
  per-source and the joint-corpus question does not arise).

The soft-vs-hard split is intentional: methodological transparency is universal, but only
language-sensitive models warrant blocking the query outright. A future Phase 122j catalogue
audit may extend the soft Aleph banner with metric-specific copy (the BFF
`/content/metric/{name}` endpoint already serves dual-register methodology text — Phase 122j
wires it into the banner).

---

## 9 — Open work

- **Per-source topic models with cross-source alignment.** A future iteration may add a "named
  alignment" mode where each source produces its own topic set and the dashboard renders the
  alignment between them. This is a different research artefact from the joint-corpus mode and
  warrants its own working note.
- **Temporal stability.** Phase 121 ships `topic_evolution` (the stream graph) which is
  similarly sensitive to corpus size — the joint-corpus interpretation applies on every
  temporal bucket the cell renders.

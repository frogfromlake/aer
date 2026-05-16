// Phase 122i / ADR-034 — BERTopic methodology thresholds.
//
// These constants drive the Methodology Notes rendered on Episteme
// Topic cells when the active scope crosses a methodological boundary.
// Both notes are informational — they never refuse a render; they
// alert the user that the result must be interpreted with care.
//
// **TOPIC_MIN_DOCS** — minimum article count below which BERTopic
// produces unstable topic sets. Empirical guidance (WP-005 §6.2):
// HDBSCAN's `min_cluster_size=10` plus UMAP's `n_neighbors=15` rarely
// converges on a coherent topic set below ~500 documents per language
// partition. Below this threshold, individual sources or short
// windows often yield "all outlier" results that mislead the
// reader. The dashboard surfaces a banner that cites the working
// paper rather than hiding the model output.
//
// **JOINT_CORPUS_MIN_SOURCES** — minimum number of sources that
// counts as a "joint corpus" merge. With 2+ sources, the topic
// vocabulary reflects what the sources have in common; source-
// specific framings can be aggregated away. The dashboard surfaces
// the joint-corpus banner so the reader does NOT mistake a merged
// topic for "what source A talks about". One source is just one
// source — no banner needed.

export const TOPIC_MIN_DOCS = 500;

export const JOINT_CORPUS_MIN_SOURCES = 2;

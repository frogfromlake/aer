// Paper catalog — metadata index for WP-001 through WP-006.
//
// The raw markdown content lives in /content/papers/wp-NNN.md (served as
// static assets by the SvelteKit static adapter). The load function in the
// WP route fetches the file at navigation time and parses it with the
// md.ts renderer. Only metadata that is needed for the index/landing page
// lives here — no content is duplicated.

export interface PaperMeta {
  id: string; // 'wp-001'
  shortTitle: string; // short label for nav
  status: string; // 'Draft v2 — open for interdisciplinary review'
  date: string; // ISO date of latest version
  abstract: string; // 1–2 sentence summary for the landing index
  depends: string[]; // IDs of upstream papers, e.g. ['wp-001']
  downstream: string[]; // IDs of downstream papers
  // Section numbers that have inline interactive cells in the dashboard.
  // Used by the WP page to render InlineChart after the named section.
  interactiveCells: Array<{ afterSection: string; cellId: string }>;
}

export const PAPERS: PaperMeta[] = [
  {
    id: 'wp-001',
    shortTitle: 'Probe Catalog',
    status: 'Draft v2 — open for interdisciplinary review',
    date: '2026-04-07',
    abstract:
      'Establishes the functional taxonomy for probe selection using discursive function rather than institutional form, introducing the Etic/Emic Dual Tagging System as the architectural mechanism for cross-cultural comparability without epistemological colonialism.',
    depends: [],
    downstream: ['wp-002', 'wp-003', 'wp-004', 'wp-005', 'wp-006'],
    interactiveCells: []
  },
  {
    id: 'wp-002',
    shortTitle: 'Metric Validity',
    status: 'Draft — open for interdisciplinary review',
    date: '2026-04-07',
    abstract:
      'Maps the gap between computational text metrics and sociological interpretation, proposes a validation framework for sentiment, NER, and language detection, and defines the Tier 1–3 metric classification architecture.',
    depends: ['wp-001'],
    downstream: [],
    interactiveCells: [{ afterSection: '3', cellId: 'sentiment-window-demo' }]
  },
  {
    id: 'wp-003',
    shortTitle: 'Platform Bias',
    status: 'Draft — open for interdisciplinary review',
    date: '2026-04-07',
    abstract:
      'Analyzes how platform infrastructure, algorithmic amplification, and non-human actors systematically bias discourse data, and addresses the Digital Divide as a structural limit on what AĒR can observe.',
    depends: ['wp-001'],
    downstream: [],
    interactiveCells: []
  },
  {
    id: 'wp-004',
    shortTitle: 'Cross-Cultural Comparability',
    status: 'Draft — open for interdisciplinary review',
    date: '2026-04-07',
    abstract:
      'Examines the conditions under which discourse metrics can be meaningfully placed alongside each other across languages and cultural contexts, introducing the Comparison Level framework and the Equivalence Registry architecture.',
    depends: ['wp-001'],
    downstream: [],
    interactiveCells: []
  },
  {
    id: 'wp-005',
    shortTitle: 'Temporal Granularity',
    status: 'Draft — open for interdisciplinary review',
    date: '2026-04-07',
    abstract:
      'Defines the temporal signatures of discourse phenomena (breaking events, norm shifts, structural patterns), establishes the three Pillar temporal modes, and specifies the resolution parameters for the AĒR time-series architecture.',
    depends: ['wp-001'],
    downstream: [],
    interactiveCells: []
  },
  {
    id: 'wp-006',
    shortTitle: 'Observer Effect',
    status: 'Draft — open for interdisciplinary review',
    date: '2026-04-07',
    abstract:
      'Addresses the observer effect in social measurement, proposes a reflexive architecture for AĒR as "the instrument that knows it is an instrument," and specifies the data-protection-by-design commitments including the k-anonymity gate at L5.',
    depends: ['wp-001'],
    downstream: [],
    interactiveCells: []
  }
];

const BY_ID = new Map(PAPERS.map((p) => [p.id, p]));

export function getPaperMeta(id: string): PaperMeta | null {
  return BY_ID.get(id.toLowerCase()) ?? null;
}

export function getAllPapers(): PaperMeta[] {
  return PAPERS;
}

/** URL of the raw markdown content file for a given paper ID. */
export function paperContentUrl(id: string): string {
  return `/content/papers/${id.toLowerCase()}.md`;
}

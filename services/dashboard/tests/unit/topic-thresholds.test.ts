import { describe, expect, it } from 'vitest';

import { JOINT_CORPUS_MIN_SOURCES, TOPIC_MIN_DOCS } from '../../src/lib/config/topic-thresholds';

// Phase 122i / ADR-034 / 142 — the BERTopic methodology-note thresholds drive
// the Episteme topic-cell banners (WP-005 §6.2). Pin the empirical constants so
// a change to either is a deliberate, reviewed edit (it shifts when the banner
// shows, which is methodologically load-bearing).

describe('topic thresholds', () => {
  it('TOPIC_MIN_DOCS is the WP-005 §6.2 stability floor of 500 documents', () => {
    expect(TOPIC_MIN_DOCS).toBe(500);
  });

  it('JOINT_CORPUS_MIN_SOURCES treats 2+ sources as a joint-corpus merge', () => {
    expect(JOINT_CORPUS_MIN_SOURCES).toBe(2);
  });
});

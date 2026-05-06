import { describe, expect, it } from 'vitest';

import {
  hasExternalLinks,
  wikidataHref,
  wikipediaHref
} from '../../src/lib/components/viewmodes/cooccurrence-network-internals';

// Phase 118 / 121b — pin the Wikidata + Wikipedia URL shape so a future
// refactor of the cooccurrence cell cannot silently drop or rewrite the
// external-link affordance.

describe('wikidataHref', () => {
  it('builds the canonical Wikidata URL for a QID', () => {
    expect(wikidataHref('Q42')).toBe('https://www.wikidata.org/wiki/Q42');
  });

  it('URL-encodes a non-ASCII QID input (defensive — QIDs are ASCII in practice)', () => {
    expect(wikidataHref('Q42 ')).toBe('https://www.wikidata.org/wiki/Q42%20');
  });
});

describe('wikipediaHref', () => {
  it('routes through the Wikidata Special:GoToLinkedPage redirector with the default `en` wiki', () => {
    expect(wikipediaHref('Q42')).toBe(
      'https://www.wikidata.org/wiki/Special:GoToLinkedPage/enwiki/Q42'
    );
  });

  it('honours an explicit locale override', () => {
    expect(wikipediaHref('Q42', 'de')).toBe(
      'https://www.wikidata.org/wiki/Special:GoToLinkedPage/dewiki/Q42'
    );
  });
});

describe('hasExternalLinks', () => {
  it('is true for a node carrying a non-empty wikidataQid', () => {
    expect(hasExternalLinks({ wikidataQid: 'Q42' })).toBe(true);
  });

  it('is false when wikidataQid is null (linker found no match)', () => {
    expect(hasExternalLinks({ wikidataQid: null })).toBe(false);
  });

  it('is false when wikidataQid is undefined', () => {
    expect(hasExternalLinks({})).toBe(false);
  });

  it('is false for an empty-string wikidataQid', () => {
    expect(hasExternalLinks({ wikidataQid: '' })).toBe(false);
  });
});

import { describe, expect, it } from 'vitest';
import { renderPaper, renderInline, crossRefHref } from '../../src/lib/reflection/md';
import { getAllPapers, getPaperMeta, paperContentUrl } from '../../src/lib/reflection/papers';
import {
  OPEN_QUESTIONS,
  questionsByWp,
  getOpenQuestion
} from '../../src/lib/reflection/open-questions';

// ---------------------------------------------------------------------------
// renderInline — cross-reference resolution
// ---------------------------------------------------------------------------

describe('renderInline — WP cross-references', () => {
  it('resolves bracket cross-refs [WP-001 §3]', () => {
    const out = renderInline('[WP-001 §3]');
    expect(out).toContain('href="/reflection/wp/wp-001?section=3"');
    expect(out).toContain('class="cross-ref"');
    expect(out).toContain('WP-001 §3');
  });

  it('resolves bracket cross-refs with multi-part sections [WP-004 §3.2]', () => {
    const out = renderInline('[WP-004 §3.2]');
    expect(out).toContain('href="/reflection/wp/wp-004?section=3.2"');
  });

  it('resolves bare prose cross-refs WP-002 §7', () => {
    const out = renderInline('see WP-002 §7 for details');
    expect(out).toContain('href="/reflection/wp/wp-002?section=7"');
    expect(out).toContain('class="cross-ref"');
  });

  it('resolves bare WP refs without section [WP-003]', () => {
    const out = renderInline('[WP-003]');
    expect(out).toContain('href="/reflection/wp/wp-003"');
    expect(out).not.toContain('section=');
  });

  it('renders bold **text**', () => {
    expect(renderInline('**bold**')).toBe('<strong>bold</strong>');
  });

  it('renders italic *text*', () => {
    expect(renderInline('*italic*')).toBe('<em>italic</em>');
  });

  it('renders inline code `code`', () => {
    expect(renderInline('`sentiment_score`')).toBe('<code>sentiment_score</code>');
  });

  it('renders standard links', () => {
    const out = renderInline('[link text](/some/path)');
    expect(out).toBe('<a href="/some/path">link text</a>');
  });

  it('adds rel noopener for external links', () => {
    const out = renderInline('[ext](https://example.com)');
    expect(out).toContain('rel="noopener noreferrer"');
    expect(out).toContain('target="_blank"');
  });
});

// ---------------------------------------------------------------------------
// crossRefHref — utility function
// ---------------------------------------------------------------------------

describe('crossRefHref', () => {
  it('returns null for null/undefined/empty', () => {
    expect(crossRefHref(null)).toBeNull();
    expect(crossRefHref(undefined)).toBeNull();
    expect(crossRefHref('')).toBeNull();
  });

  it('resolves WP anchor with section', () => {
    expect(crossRefHref('WP-001 §3')).toBe('/reflection/wp/wp-001?section=3');
  });

  it('resolves WP anchor without section', () => {
    expect(crossRefHref('WP-006')).toBe('/reflection/wp/wp-006');
  });

  it('pads WP numbers to 3 digits', () => {
    expect(crossRefHref('WP-1')).toBe('/reflection/wp/wp-001');
  });

  it('returns null for non-WP strings', () => {
    expect(crossRefHref('some random text')).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// renderPaper — section parsing
// ---------------------------------------------------------------------------

describe('renderPaper', () => {
  const MINIMAL_WP = `---
wp: WP-TST
version: test
---
# Test Paper Title

## 1. Introduction

This is the introduction paragraph.

## 2. Methodology

Second section content.

## Appendix A: Supplementary Data

Appendix content here.
`;

  it('extracts title from H1', () => {
    const parsed = renderPaper(MINIMAL_WP);
    expect(parsed.title).toBe('Test Paper Title');
  });

  it('parses frontmatter', () => {
    const parsed = renderPaper(MINIMAL_WP);
    expect(parsed.meta['wp']).toBe('WP-TST');
    expect(parsed.meta['version']).toBe('test');
  });

  it('splits sections on ## headings', () => {
    const parsed = renderPaper(MINIMAL_WP);
    const mainSections = parsed.sections.filter((s) => !s.isAppendix);
    expect(mainSections.length).toBeGreaterThanOrEqual(2);
  });

  it('assigns correct section numbers', () => {
    const parsed = renderPaper(MINIMAL_WP);
    const s1 = parsed.sections.find((s) => s.number === '1');
    const s2 = parsed.sections.find((s) => s.number === '2');
    expect(s1).toBeDefined();
    expect(s1?.title).toBe('Introduction');
    expect(s2).toBeDefined();
    expect(s2?.title).toBe('Methodology');
  });

  it('assigns correct section IDs', () => {
    const parsed = renderPaper(MINIMAL_WP);
    const s1 = parsed.sections.find((s) => s.number === '1');
    expect(s1?.id).toBe('section-1');
  });

  it('marks appendix sections', () => {
    const parsed = renderPaper(MINIMAL_WP);
    const appendix = parsed.sections.find((s) => s.isAppendix);
    expect(appendix).toBeDefined();
    expect(appendix?.number).toBe('A');
    expect(appendix?.id).toBe('appendix-a');
    expect(appendix?.title).toBe('Supplementary Data');
  });

  it('renders section HTML content', () => {
    const parsed = renderPaper(MINIMAL_WP);
    const s1 = parsed.sections.find((s) => s.number === '1');
    expect(s1?.html).toContain('introduction paragraph');
  });

  it('renders WP without frontmatter', () => {
    const raw = '# Title\n\n## 1. Section\n\nContent here.\n';
    const parsed = renderPaper(raw);
    expect(parsed.title).toBe('Title');
    expect(parsed.sections.length).toBeGreaterThan(0);
  });

  it('handles fenced code blocks without corrupting them', () => {
    const raw = '# T\n\n## 1. Code\n\n```python\nx = 1\n```\n';
    const parsed = renderPaper(raw);
    const s = parsed.sections.find((s) => s.number === '1');
    expect(s?.html).toContain('<pre><code');
    expect(s?.html).toContain('x = 1');
  });

  it('renders table markup', () => {
    const raw = '# T\n\n## 1. Table\n\n| A | B |\n|---|---|\n| 1 | 2 |\n';
    const parsed = renderPaper(raw);
    const s = parsed.sections.find((s) => s.number === '1');
    expect(s?.html).toContain('<table>');
    expect(s?.html).toContain('<th>');
  });

  it('renders unordered lists', () => {
    const raw = '# T\n\n## 1. List\n\n- alpha\n- beta\n- gamma\n';
    const parsed = renderPaper(raw);
    const s = parsed.sections.find((s) => s.number === '1');
    expect(s?.html).toContain('<ul>');
    expect(s?.html).toContain('<li>alpha</li>');
  });

  it('renders blockquotes', () => {
    const raw = '# T\n\n## 1. Quote\n\n> This is a quote.\n';
    const parsed = renderPaper(raw);
    const s = parsed.sections.find((s) => s.number === '1');
    expect(s?.html).toContain('<blockquote>');
  });

  it('resolves cross-refs inside section prose', () => {
    const raw = '# T\n\n## 1. Refs\n\nSee [WP-002 §3] for more.\n';
    const parsed = renderPaper(raw);
    const s = parsed.sections.find((s) => s.number === '1');
    expect(s?.html).toContain('href="/reflection/wp/wp-002?section=3"');
  });
});

// ---------------------------------------------------------------------------
// papers.ts — catalog functions
// ---------------------------------------------------------------------------

describe('papers catalog', () => {
  it('getAllPapers returns exactly 6 papers', () => {
    expect(getAllPapers()).toHaveLength(6);
  });

  it('getPaperMeta returns correct metadata for wp-001', () => {
    const meta = getPaperMeta('wp-001');
    expect(meta).not.toBeNull();
    expect(meta?.id).toBe('wp-001');
    expect(meta?.shortTitle).toBe('Probe Catalog');
    expect(meta?.depends).toEqual([]);
    expect(meta?.downstream).toContain('wp-002');
  });

  it('getPaperMeta is case-insensitive', () => {
    expect(getPaperMeta('WP-002')).not.toBeNull();
    expect(getPaperMeta('WP-002')?.id).toBe('wp-002');
  });

  it('getPaperMeta returns null for unknown id', () => {
    expect(getPaperMeta('wp-999')).toBeNull();
  });

  it('wp-002 has sentiment-window-demo interactive cell after section 3', () => {
    const meta = getPaperMeta('wp-002');
    expect(meta?.interactiveCells).toContainEqual({
      afterSection: '3',
      cellId: 'sentiment-window-demo'
    });
  });

  it('paperContentUrl returns correct path', () => {
    expect(paperContentUrl('wp-003')).toBe('/content/papers/wp-003.md');
  });

  it('paperContentUrl lowercases the id', () => {
    expect(paperContentUrl('WP-004')).toBe('/content/papers/wp-004.md');
  });

  it('all papers have required fields', () => {
    for (const p of getAllPapers()) {
      expect(p.id).toMatch(/^wp-\d{3}$/);
      expect(p.shortTitle.length).toBeGreaterThan(0);
      expect(p.status.length).toBeGreaterThan(0);
      expect(p.abstract.length).toBeGreaterThan(0);
      expect(Array.isArray(p.depends)).toBe(true);
      expect(Array.isArray(p.downstream)).toBe(true);
      expect(Array.isArray(p.interactiveCells)).toBe(true);
    }
  });
});

// ---------------------------------------------------------------------------
// open-questions.ts — catalog functions
// ---------------------------------------------------------------------------

describe('open-questions catalog', () => {
  it('has exactly 50 questions', () => {
    expect(OPEN_QUESTIONS).toHaveLength(50);
  });

  it('all question IDs are unique', () => {
    const ids = OPEN_QUESTIONS.map((q) => q.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('all questions have required fields', () => {
    for (const q of OPEN_QUESTIONS) {
      expect(q.id).toMatch(/^wp-\d{3}-q\d+$/);
      expect(q.sourceWp).toMatch(/^wp-\d{3}$/);
      expect(q.sourceSection).toBeTruthy();
      expect(q.disciplinaryScope.length).toBeGreaterThan(0);
      expect(q.shortLabel.length).toBeGreaterThan(0);
      expect(q.question.length).toBeGreaterThan(0);
    }
  });

  it('questions span all 6 WPs', () => {
    const wps = new Set(OPEN_QUESTIONS.map((q) => q.sourceWp));
    for (const id of ['wp-001', 'wp-002', 'wp-003', 'wp-004', 'wp-005', 'wp-006']) {
      expect(wps.has(id)).toBe(true);
    }
  });

  it('questionsByWp groups correctly', () => {
    const grouped = questionsByWp();
    expect(grouped.size).toBe(6);
    for (const [wpId, questions] of grouped) {
      expect(questions.every((q) => q.sourceWp === wpId)).toBe(true);
      expect(questions.length).toBeGreaterThan(0);
    }
  });

  it('getOpenQuestion returns correct entry', () => {
    const q = getOpenQuestion('wp-001-q1');
    expect(q).not.toBeNull();
    expect(q?.sourceWp).toBe('wp-001');
    expect(q?.id).toBe('wp-001-q1');
  });

  it('getOpenQuestion returns null for unknown id', () => {
    expect(getOpenQuestion('wp-099-q1')).toBeNull();
  });

  it('WP-001 questions are in section 8', () => {
    const q1 = OPEN_QUESTIONS.filter((q) => q.sourceWp === 'wp-001');
    expect(q1.every((q) => q.sourceSection === '8')).toBe(true);
  });

  it('WP-002 through WP-006 questions are in section 7 or 8', () => {
    const rest = OPEN_QUESTIONS.filter((q) => q.sourceWp !== 'wp-001');
    expect(rest.every((q) => q.sourceSection === '7' || q.sourceSection === '8')).toBe(true);
  });
});

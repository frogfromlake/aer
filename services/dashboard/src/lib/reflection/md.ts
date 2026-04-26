// Minimal markdown renderer for AĒR Working Papers.
//
// Handles the exact subset of GFM used in the six WP files:
//   block: h2-h4, paragraphs, ul, ol, blockquote, fenced-code, table, hr
//   inline: bold, italic, inline-code, links, WP cross-refs
//
// Cross-reference resolution: bare `WP-NNN §N[.M]` patterns in prose
// become <a> links to /reflection/wp/wp-nnn?section=N (Phase 109 spec).
// Bracket-style [WP-NNN §N] is also resolved.

export interface PaperSection {
  number: string; // '1', '2.1', 'A', 'B', ''
  title: string;
  id: string; // CSS-safe anchor, e.g. 'section-1'
  html: string; // rendered body content
  isAppendix: boolean;
}

export interface ParsedPaper {
  title: string;
  meta: Record<string, string>; // frontmatter key-value
  sections: PaperSection[];
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

export function renderPaper(raw: string): ParsedPaper {
  const lines = raw.split('\n');
  let i = 0;

  // Strip YAML frontmatter (optional leading ---)
  let meta: Record<string, string> = {};
  if (lines[i] === '---') {
    i++;
    const fmLines: string[] = [];
    while (i < lines.length && lines[i] !== '---') {
      fmLines.push(lines[i] ?? '');
      i++;
    }
    if (lines[i] === '---') i++;
    meta = parseFrontmatter(fmLines);
  }

  // Skip blank lines after frontmatter
  while (i < lines.length && (lines[i] ?? '').trim() === '') i++;

  // Extract H1 title
  let title = '';
  if ((lines[i] ?? '').startsWith('# ')) {
    title = renderInline((lines[i] ?? '').slice(2).trim());
    i++;
  }

  // Collect remaining lines and split by ## headings
  const bodyLines = lines.slice(i);
  const sections = parseSections(bodyLines);

  return { title, meta, sections };
}

// ---------------------------------------------------------------------------
// Section splitting
// ---------------------------------------------------------------------------

function parseSections(lines: string[]): PaperSection[] {
  const groups: Array<{ heading: string; lines: string[] }> = [];
  let currentHeading = '';
  let currentLines: string[] = [];

  for (const line of lines) {
    if (line.startsWith('## ')) {
      groups.push({ heading: currentHeading, lines: currentLines });
      currentHeading = line.slice(3).trim();
      currentLines = [];
    } else {
      currentLines.push(line);
    }
  }
  groups.push({ heading: currentHeading, lines: currentLines });

  const sections: PaperSection[] = [];
  for (const g of groups) {
    if (!g.heading && g.lines.every((l) => l.trim() === '')) continue;
    const { number, title, id, isAppendix } = parseSectionHeading(g.heading);
    sections.push({
      number,
      title,
      id,
      html: renderBlock(g.lines),
      isAppendix
    });
  }
  return sections;
}

function parseSectionHeading(heading: string): {
  number: string;
  title: string;
  id: string;
  isAppendix: boolean;
} {
  if (!heading) return { number: '', title: '', id: 'intro', isAppendix: false };

  // "1. Objective" or "10. References"
  const numMatch = heading.match(/^(\d+(?:\.\d+)*)\.\s+(.+)$/);
  if (numMatch) {
    const number = numMatch[1] ?? '';
    const title = numMatch[2] ?? '';
    return { number, title, id: `section-${number.replace(/\./g, '-')}`, isAppendix: false };
  }

  // "Appendix A: Something" or "Appendix A"
  const appMatch = heading.match(/^Appendix\s+([A-Z])(?::\s+(.+))?$/);
  if (appMatch) {
    const letter = appMatch[1] ?? 'A';
    const title = appMatch[2] ?? '';
    return { number: letter, title, id: `appendix-${letter.toLowerCase()}`, isAppendix: true };
  }

  // Fallback
  const id = heading
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
  return { number: '', title: heading, id: id || 'section', isAppendix: false };
}

// ---------------------------------------------------------------------------
// Block renderer (state machine)
// ---------------------------------------------------------------------------

function renderBlock(lines: string[]): string {
  const out: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i] ?? '';

    // Blank line
    if (line.trim() === '') {
      i++;
      continue;
    }

    // H3
    if (line.startsWith('### ')) {
      const text = line.slice(4).trim();
      const slug = slugify(text);
      out.push(`<h3 id="${slug}">${renderInline(text)}</h3>`);
      i++;
      continue;
    }

    // H4
    if (line.startsWith('#### ')) {
      const text = line.slice(5).trim();
      out.push(`<h4>${renderInline(text)}</h4>`);
      i++;
      continue;
    }

    // Fenced code block
    if (line.startsWith('```')) {
      const lang = line.slice(3).trim();
      i++;
      const codeLines: string[] = [];
      while (i < lines.length && !(lines[i] ?? '').startsWith('```')) {
        codeLines.push(lines[i] ?? '');
        i++;
      }
      i++; // consume closing ```
      const code = escHtml(codeLines.join('\n'));
      out.push(`<pre><code class="language-${lang || 'text'}">${code}</code></pre>`);
      continue;
    }

    // Blockquote — collect contiguous > lines
    if (line.startsWith('> ') || line === '>') {
      const bqLines: string[] = [];
      while (i < lines.length && ((lines[i] ?? '').startsWith('> ') || lines[i] === '>')) {
        bqLines.push((lines[i] ?? '').replace(/^> ?/, ''));
        i++;
      }
      out.push(`<blockquote>${renderBlock(bqLines)}</blockquote>`);
      continue;
    }

    // Horizontal rule
    if (/^[-*_]{3,}$/.test(line.trim())) {
      out.push('<hr>');
      i++;
      continue;
    }

    // Table — collect contiguous | lines
    if (line.startsWith('|')) {
      const tableLines: string[] = [];
      while (i < lines.length && (lines[i] ?? '').startsWith('|')) {
        tableLines.push(lines[i] ?? '');
        i++;
      }
      out.push(renderTable(tableLines));
      continue;
    }

    // Unordered list
    if (/^[-*]\s/.test(line)) {
      const items = collectList(lines, i, /^[-*]\s/);
      i += items.length;
      const lis = items.map((l) => `<li>${renderInline(l.replace(/^[-*]\s/, ''))}</li>`).join('');
      out.push(`<ul>${lis}</ul>`);
      continue;
    }

    // Ordered list
    if (/^\d+\.\s/.test(line)) {
      const items = collectList(lines, i, /^\d+\.\s/);
      i += items.length;
      const lis = items.map((l) => `<li>${renderInline(l.replace(/^\d+\.\s/, ''))}</li>`).join('');
      out.push(`<ol>${lis}</ol>`);
      continue;
    }

    // Paragraph — collect until blank line or block element
    const paraLines: string[] = [];
    while (
      i < lines.length &&
      (lines[i] ?? '').trim() !== '' &&
      !(lines[i] ?? '').startsWith('#') &&
      !(lines[i] ?? '').startsWith('```') &&
      !(lines[i] ?? '').startsWith('|') &&
      !(lines[i] ?? '').startsWith('>') &&
      !/^[-*]\s/.test(lines[i] ?? '') &&
      !/^\d+\.\s/.test(lines[i] ?? '') &&
      !/^[-*_]{3,}$/.test((lines[i] ?? '').trim())
    ) {
      paraLines.push(lines[i] ?? '');
      i++;
    }
    if (paraLines.length > 0) {
      out.push(`<p>${renderInline(paraLines.join(' '))}</p>`);
    }
  }

  return out.join('\n');
}

// ---------------------------------------------------------------------------
// Table renderer
// ---------------------------------------------------------------------------

function renderTable(lines: string[]): string {
  if (lines.length < 2) return '';

  const parseRow = (line: string): string[] =>
    line
      .replace(/^\||\|$/g, '')
      .split('|')
      .map((c) => c.trim());

  const headerCells = parseRow(lines[0] ?? '');
  // lines[1] is the separator row — skip it
  const bodyRows = lines.slice(2);

  const ths = headerCells.map((c) => `<th>${renderInline(c)}</th>`).join('');
  const trs = bodyRows
    .map((row) => {
      const cells = parseRow(row);
      const tds = cells.map((c) => `<td>${renderInline(c)}</td>`).join('');
      return `<tr>${tds}</tr>`;
    })
    .join('');

  return `<table><thead><tr>${ths}</tr></thead><tbody>${trs}</tbody></table>`;
}

// ---------------------------------------------------------------------------
// Inline renderer
// ---------------------------------------------------------------------------

export function renderInline(text: string): string {
  return (
    text
      // Bold
      .replace(/\*\*\*([^*]+)\*\*\*/g, '<strong><em>$1</em></strong>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\*([^*\n]+)\*/g, '<em>$1</em>')
      // Inline code
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      // Bracket cross-refs [WP-NNN §N] or [WP-NNN]
      .replace(/\[WP-(\d+)\s*§([^\]]+)\]/gi, (_m, num, sec) => {
        const id = `wp-${String(num).padStart(3, '0')}`;
        const section = sec.trim();
        return `<a href="/reflection/wp/${id}?section=${encodeURIComponent(section)}" class="cross-ref">WP-${String(num).padStart(3, '0')} §${section}</a>`;
      })
      .replace(/\[WP-(\d+)\]/gi, (_m, num) => {
        const id = `wp-${String(num).padStart(3, '0')}`;
        return `<a href="/reflection/wp/${id}" class="cross-ref">WP-${String(num).padStart(3, '0')}</a>`;
      })
      // Standard links [text](url)
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, linkText, url) => {
        const external = !url.startsWith('/') && !url.startsWith('#');
        const rel = external ? ' rel="noopener noreferrer" target="_blank"' : '';
        return `<a href="${escAttr(url)}"${rel}>${linkText}</a>`;
      })
      // Inline WP cross-refs in prose: WP-NNN §N or (WP-NNN §N.M)
      .replace(/\bWP-(\d{3})\s*§\s*(\d[\d.]*\w*)/g, (_m, num, sec) => {
        const id = `wp-${num}`;
        return `<a href="/reflection/wp/${id}?section=${encodeURIComponent(sec)}" class="cross-ref">WP-${num} §${sec}</a>`;
      })
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function parseFrontmatter(lines: string[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of lines) {
    const m = line.match(/^(\w[\w-]*):\s*(.*)$/);
    if (m) result[m[1] ?? ''] = (m[2] ?? '').trim();
  }
  return result;
}

function collectList(lines: string[], start: number, pattern: RegExp): string[] {
  const items: string[] = [];
  let i = start;
  while (i < lines.length && pattern.test(lines[i] ?? '')) {
    items.push(lines[i] ?? '');
    i++;
  }
  return items;
}

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escAttr(s: string): string {
  return s.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');
}

// ---------------------------------------------------------------------------
// Cross-reference utilities (used by other modules)
// ---------------------------------------------------------------------------

/** Parse "WP-NNN §N" or "WP-NNN" into a /reflection/wp/ href.  Returns null on no match. */
export function crossRefHref(anchor: string | null | undefined): string | null {
  if (!anchor) return null;
  const m = anchor.match(/^WP-(\d+)\s*(?:§\s*(.+))?$/i);
  if (!m) return null;
  const id = `wp-${String(m[1]).padStart(3, '0')}`;
  const section = (m[2] ?? '').trim();
  return section
    ? `/reflection/wp/${id}?section=${encodeURIComponent(section)}`
    : `/reflection/wp/${id}`;
}

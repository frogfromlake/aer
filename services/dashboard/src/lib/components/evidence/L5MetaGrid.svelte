<script lang="ts">
  // Article metadata header for the L5 Evidence Reader (Phase 141 extraction
  // from L5EvidenceReader.svelte). Pure presentation of the Silver detail row.
  import type { ArticleDetailDto } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';
  import { formatTs } from './l5-evidence-internals';

  let { article }: { article: ArticleDetailDto } = $props();
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- article.url is an external link opened in a new tab -->
<dl class="meta-grid">
  <div class="meta-item">
    <dt>{m.evidence_meta_source()}</dt>
    <dd><code>{article.source}</code></dd>
  </div>
  <div class="meta-item">
    <dt>{m.evidence_meta_published()}</dt>
    <dd><time datetime={article.timestamp}>{formatTs(article.timestamp)}</time></dd>
  </div>
  {#if article.language}
    <div class="meta-item">
      <dt>{m.evidence_meta_language()}</dt>
      <dd><code>{article.language}</code></dd>
    </div>
  {/if}
  <div class="meta-item">
    <dt>{m.evidence_meta_words()}</dt>
    <dd>{article.wordCount.toLocaleString()}</dd>
  </div>
  {#if article.url}
    <div class="meta-item">
      <dt>{m.evidence_meta_url()}</dt>
      <dd>
        <a href={article.url} target="_blank" rel="noopener noreferrer" class="source-link">
          {article.url}
        </a>
      </dd>
    </div>
  {/if}
  <div class="meta-item">
    <dt>{m.evidence_meta_schema()}</dt>
    <dd><code>{article.schemaVersion}</code></dd>
  </div>
</dl>

<style>
  .meta-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(10rem, 1fr));
    gap: var(--space-3);
    margin: 0 0 var(--space-4) 0;
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
  }

  .meta-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .meta-item dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .meta-item dd {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    margin: 0;
  }

  .meta-item code {
    font-family: var(--font-mono);
  }

  .source-link {
    color: var(--color-accent-muted);
    text-decoration: none;
    overflow-wrap: break-word;
    word-break: break-all;
    font-size: var(--font-size-xs);
  }

  .source-link:hover {
    color: var(--color-accent);
    text-decoration: underline;
  }
</style>

<script>
  import { renderMarkdown } from './markdown.js';
  export let text = '';
  $: html = renderMarkdown(text);
</script>

<div class="markdown-content">{@html html}</div>

<style>
  .markdown-content :global(:first-child) { margin-top: 0; }
  .markdown-content :global(:last-child) { margin-bottom: 0; }
  .markdown-content :global(p), .markdown-content :global(li), .markdown-content :global(blockquote) { color: var(--ink-muted); font-size: 13px; line-height: 1.65; overflow-wrap: anywhere; }
  .markdown-content :global(h1), .markdown-content :global(h2), .markdown-content :global(h3) { margin: 12px 0 6px; color: var(--ink-primary); line-height: 1.35; }
  .markdown-content :global(ul), .markdown-content :global(ol) { padding-left: 22px; }
  .markdown-content :global(code) { padding: 1px 4px; border-radius: 3px; background: var(--surface-tint); color: var(--ink-primary); font-family: var(--font-mono); font-size: .92em; }
  .markdown-content :global(pre) { overflow: auto; padding: 12px; border-radius: var(--radius-sm); background: var(--surface-soft); }
  .markdown-content :global(pre code) { padding: 0; background: transparent; }
  .markdown-content :global(a) { color: var(--accent-deep); text-underline-offset: 3px; }
</style>

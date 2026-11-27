// shortcut: Supports single-level lists, three heading levels and safe HTTP(S) links; extend when profile cards need broader Markdown.
function escapeHtml(value) {
  return value.replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
}

function inline(value) {
  let text = escapeHtml(value);
  text = text.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (_, label, url) => `<a href="${url}" rel="nofollow noopener noreferrer">${label}</a>`);
  return text
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*])\*([^*]+)\*/g, '$1<em>$2</em>');
}

export function renderMarkdown(source = '') {
  const lines = String(source).replace(/\r\n?/g, '\n').split('\n');
  const html = [];
  let list = '';
  let code = false;
  let codeLines = [];
  const endList = () => { if (list) { html.push(`</${list}>`); list = ''; } };

  for (const line of lines) {
    if (/^```/.test(line)) {
      endList();
      if (code) { html.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`); codeLines = []; }
      code = !code;
    } else if (code) codeLines.push(line);
    else if (!line.trim()) endList();
    else if (/^\s*[-*+]\s+/.test(line) || /^\s*\d+\.\s+/.test(line)) {
      const ordered = /^\s*\d+\.\s+/.test(line);
      const nextList = ordered ? 'ol' : 'ul';
      if (list !== nextList) { endList(); list = nextList; html.push(`<${list}>`); }
      html.push(`<li>${inline(line.replace(/^\s*(?:[-*+]\s+|\d+\.\s+)/, ''))}</li>`);
    } else {
      endList();
      const heading = line.match(/^(#{1,3})\s+(.+)$/);
      if (heading) html.push(`<h${heading[1].length}>${inline(heading[2])}</h${heading[1].length}>`);
      else if (/^>\s?/.test(line)) html.push(`<blockquote>${inline(line.replace(/^>\s?/, ''))}</blockquote>`);
      else html.push(`<p>${inline(line)}</p>`);
    }
  }
  endList();
  if (code) html.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`);
  return html.join('');
}

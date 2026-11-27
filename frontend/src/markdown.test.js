import assert from 'node:assert/strict';
import { renderMarkdown } from './markdown.js';

assert.equal(renderMarkdown('**Bold** and `code`'), '<p><strong>Bold</strong> and <code>code</code></p>');
assert.equal(renderMarkdown('# Title\n\n- one\n- two'), '<h1>Title</h1><ul><li>one</li><li>two</li></ul>');
assert.match(renderMarkdown('<img src=x onerror=alert(1)>'), /&lt;img src=x onerror=alert\(1\)&gt;/);
assert.doesNotMatch(renderMarkdown('[click](javascript:alert(1))'), /href=/);

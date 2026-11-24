import { test } from 'node:test';
import assert from 'node:assert/strict';
import { get } from 'svelte/store';

const storage = new Map([['language', 'ja']]);
globalThis.localStorage = { getItem: key => storage.get(key), setItem: (key, value) => storage.set(key, value) };
globalThis.document = { documentElement: {} };
const { locale, t } = await import('./i18n.js');

test('language selection persists, updates document and translates parameters', () => {
  assert.equal(get(locale), 'ja');
  assert.equal(document.documentElement.lang, 'ja');
  assert.equal(get(t)('登录'), 'ログイン');
  assert.equal(get(t)('Invalid credentials.'), '認証情報が正しくありません。');
  locale.set('en');
  assert.equal(storage.get('language'), 'en');
  assert.equal(document.documentElement.lang, 'en');
  assert.equal(get(t)('请求失败（{status}）', { status: 503 }), 'Request failed (503)');
  locale.set('zh-TW');
  assert.equal(get(t)('登录'), '登入');
  locale.set('zh-CN');
  assert.equal(get(t)('登录'), '登录');
  assert.equal(get(t)('Invalid credentials.'), '凭据无效。');
});

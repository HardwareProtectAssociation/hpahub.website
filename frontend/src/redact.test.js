import { test } from 'node:test';
import assert from 'node:assert/strict';
import { isCipherText, redactCipher, shortenCipher } from './redact.js';

test('redactCipher is deterministic and content-dependent', () => {
  assert.equal(redactCipher('陈默'), redactCipher('陈默'));
  assert.notEqual(redactCipher('陈默'), redactCipher('苏禾'));
  assert.match(redactCipher('陈默'), /^0x[0-9a-f]+$/);
});

test('redactCipher does not expose the original text', () => {
  const cipher = redactCipher('secret-note-123');
  assert.equal(cipher.includes('secret'), false);
  assert.equal(cipher.includes('note'), false);
});

test('redactCipher handles empty and long values', () => {
  assert.equal(redactCipher(''), '0x00');
  const long = redactCipher('很长的一段隐藏简介'.repeat(20));
  assert.ok(long.endsWith('…'));
  assert.equal(long.replace('0x', '').replace('…', '').length, 48);
});

test('server ciphertext is detected and shortened for display', () => {
  assert.equal(isCipherText('0xdeadBEEF'), true);
  assert.equal(isCipherText('普通简介'), false);
  assert.equal(isCipherText(''), false);
  const long = `0x${'ab'.repeat(80)}`;
  assert.equal(shortenCipher(long).endsWith('…'), true);
  assert.equal(shortenCipher('0xdeadbeef'), '0xdeadbeef');
});

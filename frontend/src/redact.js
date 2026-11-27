const KEY = [0x5f, 0x31, 0xc7, 0x2a, 0x8e, 0x63, 0x1d, 0xb4];
const MAX_BYTES = 24;
const CIPHER_PATTERN = /^0x[0-9a-f]+$/i;

// 仅用于隐藏资料的展示：把内容变成一段随内容变化的十六进制密文，
// 不是安全加密，公众页面也不会收到真实内容（由后端先做脱敏）。
export function redactCipher(value = '') {
  const bytes = new TextEncoder().encode(String(value ?? ''));
  if (!bytes.length) return '0x00';
  const output = [];
  const limit = Math.min(bytes.length, MAX_BYTES);
  let state = 0x9e3779b9;
  for (let index = 0; index < limit; index += 1) {
    state = (Math.imul(state ^ bytes[index], 0x85ebca6b) + 0x7f4a7c15) >>> 0;
    const byte = (bytes[index] ^ KEY[index % KEY.length] ^ (state >>> 24)) & 0xff;
    output.push(byte.toString(16).padStart(2, '0'));
  }
  return `0x${output.join('')}${bytes.length > MAX_BYTES ? '…' : ''}`;
}

// 后端已经加密过的字段（0x 开头的十六进制密文）直接展示，不再二次加密。
export function isCipherText(value) {
  return CIPHER_PATTERN.test(String(value ?? ''));
}

export function shortenCipher(value, limit = 66) {
  const raw = String(value ?? '');
  return raw.length > limit ? `${raw.slice(0, limit)}…` : raw;
}

const KEY = [0x5f, 0x31, 0xc7, 0x2a, 0x8e, 0x63, 0x1d, 0xb4];
const MAX_BYTES = 24;

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

import { derived, writable } from 'svelte/store';
import app from './i18n-app.js';
import gpu from './i18n-gpu.js';
import warehouse from './i18n-warehouse.js';
import publicPages from './i18n-public.js';

export const languages = { 'zh-CN': '简体中文', 'zh-TW': '繁體中文', ja: '日本語', en: 'English' };
const translations = { ...app, ...gpu, ...warehouse, ...publicPages };
const simplifiedErrors = {
  'Invalid credentials.': '凭据无效。',
  'Authentication failed.': '认证失败。',
  'Authentication required.': '请先登录。',
  'Insufficient permissions.': '权限不足。',
  'Invalid registration or invitation.': '注册资料或邀请码无效。',
  'Too many requests. Try again later.': '请求过于频繁，请稍后再试。',
};
const supported = Object.keys(languages);
let stored;
try { stored = localStorage.getItem('language'); } catch { /* Storage may be unavailable. */ }
const browserLanguage = typeof navigator === 'undefined' ? '' : navigator.language;
const initial = supported.includes(stored) ? stored : /^zh-(TW|HK|MO)/i.test(browserLanguage) ? 'zh-TW' : /^zh/i.test(browserLanguage) ? 'zh-CN' : /^ja/i.test(browserLanguage) ? 'ja' : /^en/i.test(browserLanguage) ? 'en' : 'zh-CN';
export const locale = writable(initial);
locale.subscribe(value => {
  if (typeof document !== 'undefined') document.documentElement.lang = value;
  try { localStorage.setItem('language', value); } catch { /* Storage may be unavailable. */ }
});
export const t = derived(locale, value => (key, params = {}) => {
  const index = supported.indexOf(value) - 1;
  const text = index < 0 ? simplifiedErrors[key] || key : translations[key]?.[index] || key;
  return text.replace(/\{(\w+)\}/g, (match, name) => Object.hasOwn(params, name) ? String(params[name]) : match);
});

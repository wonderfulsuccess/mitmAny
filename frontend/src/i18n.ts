import messages from './locales/messages.json' with { type: 'json' };
import nativeMessages from '../../internal/i18n/messages.json' with { type: 'json' };

export type Locale = 'zh-CN' | 'en';
export const LANGUAGE_KEY = 'mitmany.language.v1';
export function chooseLocale(saved?: string | null): Locale {
  return saved === 'zh-CN' ? 'zh-CN' : 'en';
}
function initialLocale(): Locale {
  if (typeof window === 'undefined') return 'en';
  try { return chooseLocale(localStorage.getItem(LANGUAGE_KEY)); }
  catch { return chooseLocale(); }
}
let locale = initialLocale();
const listeners = new Set<() => void>();
export const getLocale = () => locale;
export const subscribeLocale = (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener); }; };
export function setLocale(value: Locale) {
  if (value !== 'zh-CN' && value !== 'en') throw new Error('Unsupported language');
  locale = value;
  if (typeof document !== 'undefined') document.documentElement.lang = value;
  try { if (typeof localStorage !== 'undefined') localStorage.setItem(LANGUAGE_KEY, value); } catch { /* Still switch for this session. */ }
  for (const listener of listeners) listener();
}
if (typeof document !== 'undefined') document.documentElement.lang = locale;
const catalog: Record<string, {zh: string; en: string}> = messages;
const normalize = (s: string) => s.trim().replace(/\s*\n\s*/g, ' ');
export function tr(key: string, values: Record<string, string | number> = {}): string {
  const entry = catalog[key] ?? catalog[normalize(key)];
  const text = entry ? entry[locale === 'en' ? 'en' : 'zh'] : key;
  // Substitute once, so network-derived values are never interpreted as templates.
  return text.replace(/\{(p\d+)\}/g, (match, name: string) => Object.prototype.hasOwnProperty.call(values,name) ? String(values[name]) : match);
}
const escapeRegex = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
const diagnostics = Object.entries(nativeMessages).map(([zh,en]) => ({zh,en}));
/** Translate only application-owned diagnostic messages, never bodies or headers. */
export function diagnostic(message: string, depth = 0): string {
  if (depth > 8) return message;
  for (const entry of Object.values(catalog)) {
    if (message === entry.zh || message === entry.en) return entry[locale === 'en' ? 'en' : 'zh'];
  }
  for (const entry of diagnostics) {
    const target = entry[locale === 'en' ? 'en' : 'zh'];
    for (const source of [entry.zh,entry.en]) {
      if (source === message) return target;
      // Prefix messages concatenate an underlying diagnostic.
      if (source.endsWith(': ') && message.startsWith(source)) return target + diagnostic(message.slice(source.length),depth+1);
      if (!/%[sdvw]/.test(source)) continue;
      const pattern = '^' + source.split(/%[sdvw]/).map(escapeRegex).join('([\\s\\S]*?)') + '$';
      const match = new RegExp(pattern).exec(message);
      if (match) { let i=0; return target.replace(/%[sdvw]/g,()=>diagnostic(match[++i],depth+1)); }
    }
  }
  if (message.includes('\n')) return message.split('\n').map(line=>diagnostic(line,depth+1)).join('\n');
  return message;
}

import test from 'node:test';
import assert from 'node:assert/strict';
import { chooseLocale, getLocale, setLocale, tr, diagnostic, LANGUAGE_KEY } from '../src/i18n.ts';
import messages from '../src/locales/messages.json' with { type: 'json' };

test('English is the default and Chinese requires an explicit saved preference', () => {
  for (const value of [undefined, null, '', 'en', 'fr', 'zh']) assert.equal(chooseLocale(value), 'en');
  assert.equal(chooseLocale('zh-CN'), 'zh-CN');
});
test('language switching persists and localizes UI and native diagnostics', () => {
  const values = new Map();
  globalThis.localStorage = {setItem: (k,v) => values.set(k,v)};
  try {
    setLocale('zh-CN');
    assert.equal(getLocale(), 'zh-CN');
    assert.equal(values.get(LANGUAGE_KEY), 'zh-CN');
    assert.equal(tr('语言'), '语言');
    setLocale('en');
    assert.equal(tr('语言'), 'Language');
    assert.equal(diagnostic('代理端口 9080 不可用: address already in use'), 'Proxy port 9080 is unavailable: address already in use');
    assert.equal(diagnostic('用户自定义内容'), '用户自定义内容');
  } finally { delete globalThis.localStorage; setLocale('en'); }
});
test('translations preserve placeholders and substitute external values only once', () => {
  for (const [key, entry] of Object.entries(messages)) {
    assert.ok(entry.en && entry.zh, key);
    assert.deepEqual([...entry.en.matchAll(/\{p\d+\}/g)].map(x=>x[0]).sort(), [...entry.zh.matchAll(/\{p\d+\}/g)].map(x=>x[0]).sort(), key);
  }
  assert.equal(tr('{p0}侧边栏 · {p1} B', {p0:'原文 {p1}', p1:42}), '原文 {p1} sidebar · 42 B');
});
